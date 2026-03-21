# 阶段五：Discovery 与 Parser 规范化输出链路最终设计

## 1. 目标

让 discovery 和 parser 全部建立在 normalized output 上，而不是建立在带控制字符污染的原始命令输出上。

本阶段只解决三件事：

1. discovery 保存 normalized output
2. parser 只读取 normalized output
3. 原始审计输出与解析输入彻底分离

---

## 2. 已确认的问题

当前代码中已确认的问题：

1. `runner.go` 直接把 `ExecuteCommandSync()` 返回值写到 `file_path`。
2. `parser/service.go` 直接读取 `RawCommandOutput.FilePath`。
3. `RawCommandOutput` 只有一个 `FilePath` 字段，无法区分：
   - 审计原始输出
   - 解析规范化输出

---

## 3. 设计原则

1. 建设期直接切换，不保留复杂运行时兼容层。
2. 为降低改动面：
   - `file_path` 直接定义为 **normalized output 路径**
   - `raw_file_path` 新增为审计原始输出路径
3. parser 继续读取 `file_path`，这样改动最小。
4. `raw_file_path` 只用于审计和排障，不进入 parser。
5. 不额外引入独立的 `CommandOutput` 结果模型，统一使用阶段四定义的唯一 `CommandResult`。

---

## 4. 数据模型调整

`internal/models/discovery.go` 中的 `RawCommandOutput` 改为：

```go
type RawCommandOutput struct {
    ID           uint      `json:"id" gorm:"primaryKey;autoIncrement"`
    TaskID       string    `json:"taskId" gorm:"index;not null"`
    DeviceIP     string    `json:"deviceIp" gorm:"index;not null"`
    CommandKey   string    `json:"commandKey" gorm:"index;not null"`
    Command      string    `json:"command"`

    FilePath     string    `json:"filePath"`     // normalized output 路径
    RawFilePath  string    `json:"rawFilePath"`  // 原始审计输出路径

    Status       string    `json:"status"`
    ErrorMessage string    `json:"errorMessage"`

    ParseStatus  string    `json:"parseStatus"`
    ParseError   string    `json:"parseError"`

    RawSize         int64  `json:"rawSize"`
    NormalizedSize  int64  `json:"normalizedSize"`
    LineCount       int    `json:"lineCount"`
    PagerCount      int    `json:"pagerCount"`
    EchoConsumed    bool   `json:"echoConsumed"`
    PromptMatched   string `json:"promptMatched"`

    CreatedAt    time.Time `json:"createdAt"`
    UpdatedAt    time.Time `json:"updatedAt"`
}
```

辅助方法：

```go
func (r *RawCommandOutput) GetParseFilePath() string {
    return r.FilePath
}

func (r *RawCommandOutput) GetAuditFilePath() string {
    return r.RawFilePath
}
```

---

## 5. discovery 保存流程

`runner.go` 中不再直接保存“原始返回字符串”到 `file_path`。

新的保存流程：

1. `ExecuteCommandSync()` 返回 `CommandResult` 或至少返回 `NormalizedText`
2. discovery 保存：
   - `file_path` -> normalized output
   - `raw_file_path` -> 原始审计输出
3. 数据库更新 `raw_size` / `normalized_size` / `line_count`

建议落地路径：

```text
topology/raw/<task>/<device>/<command>.txt           -> raw_file_path
topology/normalized/<task>/<device>/<command>.txt    -> file_path
```

---

## 6. parser 读取流程

`parser/service.go` 不再假设 `FilePath` 是原始输出。  
它只需继续读取 `output.FilePath`，但现在该字段语义已经变成：

- normalized output

因此 parser 代码改动最小：

```go
parseFilePath := output.GetParseFilePath()
rawText, err := os.ReadFile(parseFilePath)
rows, err := s.parser.Parse(output.CommandKey, string(rawText))
```

parser 不再承担：

1. ANSI 清理
2. 分页修复
3. 提示符修复

---

## 7. runner 的接口调整

当前 `saveRawOutput()` 需要改为更准确的语义，例如：

```go
func (r *Runner) saveCommandOutput(
    taskID string,
    deviceIP string,
    commandKey string,
    command string,
    result *executor.CommandResult,
    status string,
    errMsg string,
)
```

保存时：

1. `result.RawText` 写入 `raw_file_path`
2. `result.NormalizedText` 写入 `file_path`

不再继续使用“原始输出字符串就是解析输入”的旧模式。

---

## 8. 路径管理

路径管理继续复用 `config.PathManager`，只补充 normalized 路径获取方法。

建议新增：

```go
func (pm *PathManager) GetDiscoveryNormalizedFilePath(taskID, deviceIP, commandKey string) string
func (pm *PathManager) GetDiscoveryRawAuditFilePath(taskID, deviceIP, commandKey string) string
```

说明：

1. 原先的 `GetDiscoveryRawFilePath()` 可重命名或保留为 audit 路径
2. 命名以实现一致性为准，但必须让 `file_path` 与 `raw_file_path` 的语义明确

---

## 9. 数据迁移

本项目是建设期，不做复杂运行时兼容。  
只保留一次性迁移脚本。

推荐迁移策略：

1. 新增字段 `raw_file_path`
2. 保留 `file_path`
3. 将旧数据中的 `file_path` 临时复制到 `raw_file_path`
4. 新执行链路开始后，`file_path` 写 normalized output，`raw_file_path` 写原始审计输出

示例：

```sql
ALTER TABLE raw_command_outputs ADD COLUMN raw_file_path VARCHAR(512);

UPDATE raw_command_outputs
SET raw_file_path = file_path
WHERE file_path IS NOT NULL AND raw_file_path IS NULL;
```

说明：

1. 不新增 `normalized_file_path`
2. 不保留多层回退链
3. 后续 parser 一律读 `file_path`

---

## 10. 实施步骤

1. 修改 `RawCommandOutput` 模型
2. 增加 `raw_file_path` 数据库迁移
3. 修改 `runner.go` 的保存逻辑
4. 让 `ExecuteCommandSync()` 提供 normalized output
5. 修改 parser 读取逻辑，统一走 `GetParseFilePath()`
6. 删除文档和实现里 `NormalizedFilePath` / `CommandOutput` 的并行设计

---

## 11. 测试要求

本阶段必须具备：

1. discovery 保存 normalized output 的测试
2. parser 读取 `file_path` 的测试
3. `raw_file_path` 审计输出存在性的测试
4. migration 测试

最低验证项：

1. parser 读取的输入不含 ANSI 序列
2. parser 读取的输入不含分页覆盖残留
3. 原始审计输出仍可回溯

---

## 12. 验收标准

满足以下条件即视为阶段完成：

1. discovery 保存 normalized output 到 `file_path`
2. 原始审计输出保存到 `raw_file_path`
3. parser 只读取 `file_path`
4. parser 不再承担终端修复职责
5. 不保留复杂运行时兼容层
