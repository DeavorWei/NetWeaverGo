# SFTP连接失败问题分析报告

## 问题概述

**错误信息**: `建立SFTP会话失败: ssh: rejected: administratively prohibited ()`

**发生位置**: `internal/taskexec/backup_executor.go` 第201行

**设备类型**: 华为网络设备 (192.168.58.201)

---

## 根本原因分析

### 1. 华为设备的SFTP限制

华为网络设备(交换机/路由器)有一个**关键限制**:
> 在一个已经建立了交互式会话(shell/pty)的SSH连接上，**禁止**再打开SFTP子系统通道

这是华为设备的安全策略，在SecureFX日志中也有体现：
```
X Unable to open channel for sftp subsystem.  Server error details:    Opening the channel was administratively prohibited.
```

### 2. 代码中的问题

#### 问题代码位置
`internal/taskexec/backup_executor.go` 第158-206行：

```go
// [step-0] 获取配置文件路径
exec := executor.NewDeviceExecutor(
    device.IP,
    device.Port,
    device.Username,
    device.Password,
    opts,
)
defer exec.Close()

if err := exec.Connect(ctx.Context(), unit.Timeout); err != nil {
    // ... 连接失败处理
}

// 执行命令获取配置路径
cmdOutput, err := exec.ExecuteCommandSync(ctx.Context(), startupCommand, unit.Timeout)
// ...

// [step-1] SFTP下载配置文件 - 问题在这里！
sftpClient, err := sftp.NewClient(exec.Client.Client)  // 第201行
```

#### 问题描述

| 步骤 | 操作 | SSH连接状态 |
|------|------|-------------|
| 1 | `exec.Connect()` | 建立SSH连接 + 请求PTY + 启动Shell |
| 2 | `exec.ExecuteCommandSync()` | 在已建立的会话上执行命令 |
| 3 | `sftp.NewClient(exec.Client.Client)` | **尝试在同一个连接上打开SFTP子系统 - 失败!** |

**核心问题**: 
- `executor.NewDeviceExecutor` 创建的连接是**交互式会话**（包含PTY和Shell）
- 在已经建立shell session的SSH连接上，华为设备**拒绝**再打开sftp子系统

### 3. SecureFX的成功案例分析

SecureFX日志显示成功连接的关键点：

```
i RECV : AUTH_SUCCESS                    // 认证成功
i RECV : Server Sftp Version: 3          // 直接打开SFTP子系统
i SEND : fs-multiple-roots-supported request[On]
...
< 文件列表成功显示 >
```

**SecureFX的成功模式**:
1. 建立SSH连接
2. 认证成功
3. **直接打开SFTP子系统**（没有先打开shell）
4. SFTP操作成功

这与当前代码的执行流程完全相反。

---

## 现有解决方案参考

项目中的 `internal/sftputil/client.go` 已经正确实现了这个问题的解决方案：

```go
// NewSFTPClient 在一个全新的 SSH 连接上初始化 SFTP 连接。
// 注意：许多网络设备 (华为/华三) 拒绝在
// 已经有活跃的 "shell" 或 "pty" 通道的 SSH 连接上打开 "sftp" 子系统通道。
// 因此，我们必须专门为 SFTP 创建一个全新的 SSH 连接。
func NewSFTPClient(ctx context.Context, cfg sshutil.Config) (*SFTPClient, error) {
    // 1. 创建一个专用的原始 SSH 连接，不请求 PTY/Shell
    sshClient, err := sshutil.NewRawSSHClient(ctx, cfg)
    if err != nil {
        return nil, fmt.Errorf("SFTP专属SSH建连失败: %w", err)
    }

    // 2. 在这个干净的连接上初始化 SFTP 子系统
    client, err := sftp.NewClient(sshClient.Client)
    // ...
}
```

关键点：
- 使用 `sshutil.NewRawSSHClient()` - 创建**纯净**的SSH连接，**不**请求PTY/Shell
- 在这个干净的连接上再打开SFTP子系统

---

## 修复方案

### 方案1: 使用现有sftputil.NewSFTPClient（推荐）

修改 `backup_executor.go` 第201行，使用专门的SFTP客户端创建函数：

```go
// 修改前:
sftpClient, err := sftp.NewClient(exec.Client.Client)

// 修改后:
import "github.com/NetWeaverGo/core/internal/sftputil"

// ...

sshConfig := sshutil.Config{
    IP:       device.IP,
    Port:     device.Port,
    Username: device.Username,
    Password: device.Password,
    Timeout:  unit.Timeout,
}
sftpClient, err := sftputil.NewSFTPClient(ctx.Context(), sshConfig)
if err != nil {
    errMsg := fmt.Sprintf("建立SFTP会话失败: %v", err)
    failUnitExecution(handler, ctx, unit.ID, errMsg, "SFTP失败", nil)
    return fmt.Errorf("建立SFTP会话失败: %w", err)
}
defer sftpClient.Close()

// 然后使用 sftpClient.DownloadFile() 下载文件
```

**优点**:
- 复用已验证的解决方案
- 代码改动小
- 遵循项目已有架构

### 方案2: 直接在backup_executor中使用NewRawSSHClient

```go
// 创建专用的SFTP连接（不使用已建立shell的exec.Client）
sshConfig := sshutil.Config{
    IP:       device.IP,
    Port:     device.Port,
    Username: device.Username,
    Password: device.Password,
    Timeout:  unit.Timeout,
}
rawClient, err := sshutil.NewRawSSHClient(ctx.Context(), sshConfig)
if err != nil {
    return fmt.Errorf("建立SFTP专用SSH连接失败: %w", err)
}
defer rawClient.Close()

sftpClient, err := sftp.NewClient(rawClient.Client)
if err != nil {
    return fmt.Errorf("建立SFTP会话失败: %w", err)
}
defer sftpClient.Close()
```

**优点**:
- 更灵活的控制
- 不依赖sftputil包装层

---

## 总结

| 项目 | 内容 |
|------|------|
| **问题根因** | 在已建立shell session的SSH连接上尝试打开SFTP子系统，被华为设备安全策略拒绝 |
| **问题代码** | `backup_executor.go:201` - `sftp.NewClient(exec.Client.Client)` |
| **解决方案** | 为SFTP操作创建独立的、不带shell的纯净SSH连接 |
| **参考实现** | `internal/sftputil/client.go` - `NewSFTPClient()` |

---

## 附录：相关代码文件

1. **问题代码**: `internal/taskexec/backup_executor.go` (第158-206行)
2. **正确实现参考**: `internal/sftputil/client.go` (第22-49行)
3. **底层支持**: `internal/sshutil/client.go` - `NewRawSSHClient()` (第852-904行)
4. **设备执行器**: `internal/executor/executor.go` - `DeviceExecutor` 结构体

---

*报告生成时间: 2026-04-23*
*分析工具: NetWeaverGo 架构分析*
