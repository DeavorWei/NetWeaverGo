# 阶段四：输出模型与日志职责最终设计

## 1. 目标

统一命令输出模型，明确日志职责边界，让日志、discovery、parser 都建立在规范化输出上。

本阶段只解决三件事：

1. 只有一套权威命令结果结构
2. `raw.log` / `detail.log` / `summary.log` 职责清晰
3. `detail_logger` 从“终端修复器”变为“展示写入器”

---

## 2. 已确认的问题

当前代码中已确认的问题：

1. `detail_logger` 仍承担 ANSI 删除、分页删除、退格删除等错误职责。
2. 项目里没有统一的命令结果结构，执行结果散落在多处。
3. 日志职责虽有雏形，但数据来源仍不正确。

---

## 3. 设计原则

1. 只保留一套结果模型，不同时维护 `CommandResult` / `CommandOutput` / `V2` 多套结构。
2. `detail.log` 直接切换为 normalized output 展示。
3. `raw.log` 继续保留原始字节流。
4. `summary.log` 职责不变。
5. `normalized.log` 不是所有链路强制输出，仅在 discovery / parser 落盘时使用。
6. 当前项目是建设期，不做长期 V1/V2 双轨并存。

---

## 4. 唯一结果模型

建议在 `internal/executor/` 内定义唯一结构：

```go
package executor

import "time"

type CommandResult struct {
    DeviceIP        string
    Index           int
    Command         string
    RawText         string
    RawSize         int64
    NormalizedText  string
    NormalizedSize  int64
    PromptMatched   bool
    PaginationCount int
    EchoConsumed    bool
    StartedAt       time.Time
    CompletedAt     time.Time
    ErrorMessage    string
}

func (r *CommandResult) DurationMs() int64
func (r *CommandResult) Success() bool
```

说明：

1. 该结构是执行、日志、discovery 的统一输出源。
2. 第一版不保存 `RawChunks` 列表，避免过大内存占用。
3. 需要原始逐块审计时，直接依赖 `raw.log`。

---

## 5. 日志职责

### 5.1 raw.log

职责：

1. 保存原始 SSH 字节流
2. 保留操作标记，如发命令、发分页空格
3. 用于审计与深度排障

不得承担：

1. 解析输入
2. 文本修复

### 5.2 detail.log

职责：

1. 展示 normalized output
2. 写入用户可读的命令边界
3. 保持脱敏

不得承担：

1. ANSI 删除
2. 分页删除
3. 退格删除
4. 终端语义修复

### 5.3 summary.log

职责：

1. 记录执行摘要
2. 记录错误、跳过、终态

---

## 6. detail logger 的最终职责

`detail_logger.go` 在本阶段完成后应只做：

1. 写命令头
2. 写 normalized lines / text
3. 换行标准化
4. 脱敏

建议接口：

```go
package report

type DetailLogger struct {
    // ...
}

func (l *DetailLogger) WriteCommand(cmd string) error
func (l *DetailLogger) WriteNormalizedText(text string) error
func (l *DetailLogger) FlushPending() error
func (l *DetailLogger) Close() error
```

删除或废弃：

1. `cleanDetailChunk()` 中的 ANSI / pagination / backspace 清理职责

---

## 7. ExecutionLogStore 的调整

`ExecutionLogStore` 保留，但只调整数据来源，不重建一套 `V2` 日志体系。

最终会话结构：

```go
package report

type DeviceLogSession struct {
    IP      string
    Summary *SummaryLogger
    Detail  *DetailLogger
    Raw     *RawLogger
}
```

说明：

1. 不新建 `DeviceLogSessionV2`
2. 不新建 `ExecutionLogStoreV2`
3. 直接在现有结构上完成切换

---

## 8. 与执行器的衔接

执行器产出 `CommandResult` 后：

1. `RawText` 进入 raw 审计链路
2. `NormalizedText` 进入 detail 展示链路
3. `CommandResult` 进入 discovery / parser 落盘链路

流程：

```text
SSH bytes
  -> terminal.Replayer
  -> executor.CommandResult
  -> raw.log / detail.log / discovery file_path
```

---

## 9. 实施步骤

1. 在 `executor` 中定义唯一 `CommandResult`
2. 删除文档和实现中的重复结果模型设计
3. 改造 `detail_logger.go`，只保留展示职责
4. 改造 `executor`，让日志写入基于 `CommandResult`
5. 保持 `ExecutionLogStore` 现有框架不变，只切换输入来源

---

## 10. 测试要求

本阶段必须有：

1. `CommandResult` 单元测试
2. `detail.log` 写入 normalized text 的测试
3. `raw.log` 与 `detail.log` 分责测试
4. 日志脱敏测试

最低验证项：

1. `detail.log` 中不存在 ANSI 控制序列修复逻辑
2. `raw.log` 保持原始字节痕迹
3. `detail.log` 只反映 normalized output

---

## 11. 验收标准

满足以下条件即视为阶段完成：

1. 项目内只保留一套权威命令结果结构
2. `detail_logger` 不再承担终端语义修复
3. `detail.log` 展示 normalized output
4. `raw.log` 继续保留审计能力
5. 日志系统不引入长期 V1/V2 双轨
