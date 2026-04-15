# ICMP 缓冲区大小修复计划

## 问题描述

批量 Ping 功能在发送大数据包（如 300 字节）时失败，返回 `IP_GENERAL_FAILURE (11050)` 错误，而小数据包（32 字节）可以成功。

## 问题分析

### 日志对比

| 参数 | dataSize=32 | dataSize=300 |
|------|-------------|--------------|
| 计算大小 | 68 字节 | 336 字节 |
| 实际使用 | 256 字节 (最小值) | 336 字节 |
| 结果 | ✅ 成功 | ❌ IP_GENERAL_FAILURE |

### 根本原因

Windows `IcmpSendEcho` API 的缓冲区需求比理论计算值更大：

1. **理论计算**：`sizeof(ICMP_ECHO_REPLY) + dataSize + 8`
2. **实际需求**：需要额外的缓冲空间用于：
   - IP 头部信息（可能被包含在响应中）
   - Windows 内部处理开销
   - ICMP 错误消息（可能比预期更大）

### 为什么 dataSize=32 成功

因为触发了最小缓冲区限制（256 字节），实际分配的空间比计算值（68 字节）大得多，有 188 字节的额外空间。

### 为什么 dataSize=300 失败

计算值 336 字节超过了最小值 256，直接使用 336 字节，没有额外缓冲空间。

## 修复方案

### 方案：增加额外缓冲区空间 + 内存对齐

```go
// Calculate buffer size with extra padding
// Windows IcmpSendEcho requires more buffer space than the theoretical calculation
// Reference: https://docs.microsoft.com/en-us/windows/win32/api/icmpapi/nf-icmpapi-icmpsendecho
const (
    minBufferSize = 256  // Minimum recommended buffer size
    extraPadding  = 128  // Extra padding for IP headers and internal processing
    alignment     = 8    // 8-byte alignment for Windows API compatibility
)

// Calculate base size: reply structure + data + ICMP header overhead
baseSize := uint32(unsafe.Sizeof(ICMP_ECHO_REPLY{})) + uint32(len(sendData)) + 8

// Add extra padding for Windows internal processing
calculatedSize := baseSize + extraPadding

// Align to 8-byte boundary
alignedSize := (calculatedSize + alignment - 1) &^ (alignment - 1)

// Ensure minimum buffer size
replySize := alignedSize
if replySize < minBufferSize {
    replySize = minBufferSize
}
```

### 修改位置

文件：`internal/icmp/icmp_windows.go`
函数：`IcmpSendEcho`

### 修改内容

1. 添加常量定义：
   - `extraPadding = 128`：额外缓冲区空间
   - `alignment = 8`：8 字节对齐

2. 修改缓冲区计算逻辑：
   - 基础大小 = 结构体 + 数据 + ICMP 头部
   - 计算大小 = 基础大小 + 额外填充
   - 对齐大小 = 向上对齐到 8 字节边界
   - 最终大小 = max(对齐大小, 最小缓冲区)

## 验证步骤

1. 编译项目
2. 测试 dataSize=32（应该继续成功）
3. 测试 dataSize=300（应该现在成功）
4. 测试 dataSize=1000（验证更大包）
5. 测试 dataSize=65500（验证最大包）

## 技术背景

### Windows IcmpSendEcho 缓冲区要求

根据 Microsoft 文档：

> The size of the reply buffer, in bytes. This buffer should be large enough to hold at least one ICMP_ECHO_REPLY structure and the number of bytes of data specified in the RequestSize parameter on return. The buffer should also be large enough to hold at least 8 more bytes of data (the size of an ICMP error message).

实际测试表明，这个"至少 8 字节"的说法过于保守，实际需要更多的额外空间。

### 内存对齐

Windows API 通常要求缓冲区按 8 字节或 16 字节对齐，以确保最佳性能和兼容性。

## 风险评估

- **低风险**：增加缓冲区大小不会影响功能正确性
- **性能影响**：轻微增加内存使用，可忽略不计
- **兼容性**：完全向后兼容
