package executor

import (
	"time"
)

// CommandContext 每条命令执行过程的独立上下文
type CommandContext struct {
	// Index 命令在队列中的索引
	Index int

	// Command 实际发送的命令（不含内联注释）
	Command string

	// RawCommand 原始命令（可能包含内联注释如超时设置）
	RawCommand string

	// StartedAt 命令开始时间
	StartedAt time.Time

	// CompletedAt 命令完成时间
	CompletedAt time.Time

	// RawBuffer 当前命令范围内的原始数据
	RawBuffer []byte

	// NormalizedLines 由 terminal.Replayer 产出的规范化逻辑行
	NormalizedLines []string

	// EchoConsumed 是否已消费 echo 行
	// 第一版采用保守策略：如果首个逻辑行明显等于命令文本，则消费
	EchoConsumed bool

	// PaginationCount 分页次数
	PaginationCount int

	// PromptMatched 是否匹配到提示符
	PromptMatched bool

	// ErrorMessage 错误信息
	ErrorMessage string

	// ResultRecorded 标记结果是否已经写入 SessionContext.Results，避免重复追加
	ResultRecorded bool

	// CustomTimeout 自定义超时时间（从内联注释解析）
	CustomTimeout time.Duration
}

// NewCommandContext 创建新的命令上下文
func NewCommandContext(index int, rawCommand string) *CommandContext {
	return &CommandContext{
		Index:           index,
		RawCommand:      rawCommand,
		StartedAt:       time.Now(),
		RawBuffer:       make([]byte, 0, 4096),
		NormalizedLines: make([]string, 0),
	}
}

// SetCommand 设置实际发送的命令
func (c *CommandContext) SetCommand(cmd string) {
	c.Command = cmd
}

// SetCustomTimeout 设置自定义超时
func (c *CommandContext) SetCustomTimeout(timeout time.Duration) {
	c.CustomTimeout = timeout
}

// AppendRawData 追加原始数据
func (c *CommandContext) AppendRawData(data []byte) {
	c.RawBuffer = append(c.RawBuffer, data...)
}

// AddNormalizedLine 添加规范化行
func (c *CommandContext) AddNormalizedLine(line string) {
	c.NormalizedLines = append(c.NormalizedLines, line)
}

// MarkCompleted 标记命令完成
func (c *CommandContext) MarkCompleted() {
	c.CompletedAt = time.Now()
	c.PromptMatched = true
}

// MarkFailed 标记命令失败
func (c *CommandContext) MarkFailed(errMsg string) {
	c.CompletedAt = time.Now()
	c.ErrorMessage = errMsg
}

// MarkPromptMatched 标记已经收到命令结束提示符。
// 对已失败命令，仅补充提示符收尾信息，不覆盖失败状态。
func (c *CommandContext) MarkPromptMatched() {
	c.PromptMatched = true
	if c.CompletedAt.IsZero() {
		c.CompletedAt = time.Now()
	}
}

// IncrementPagination 增加分页计数
func (c *CommandContext) IncrementPagination() {
	c.PaginationCount++
}

// ConsumeEcho 消费 echo 行
func (c *CommandContext) ConsumeEcho() {
	c.EchoConsumed = true
}

// Duration 返回命令执行时长
func (c *CommandContext) Duration() time.Duration {
	if c.CompletedAt.IsZero() {
		return time.Since(c.StartedAt)
	}
	return c.CompletedAt.Sub(c.StartedAt)
}

// NormalizedText 返回规范化文本（所有行合并）
func (c *CommandContext) NormalizedText() string {
	result := ""
	for i, line := range c.NormalizedLines {
		if i > 0 {
			result += "\n"
		}
		result += line
	}
	return result
}

// HasError 判断是否有错误
func (c *CommandContext) HasError() bool {
	return c.ErrorMessage != ""
}

// IsCompleted 判断命令是否完成
func (c *CommandContext) IsCompleted() bool {
	return !c.CompletedAt.IsZero()
}

// CommandResult 命令执行结果 - 统一的输出模型
// 该结构是执行、日志、discovery 的统一输出源
type CommandResult struct {
	// DeviceIP 设备 IP
	DeviceIP string

	// Index 命令索引
	Index int

	// CommandKey 命令标识 (如: version, lldp_neighbor)
	CommandKey string

	// Command 执行的命令
	Command string

	// RawText 原始输出文本
	RawText string

	// RawSize 原始输出大小（字节）
	RawSize int64

	// NormalizedText 规范化输出文本（由 terminal.Replayer 产出）
	NormalizedText string

	// NormalizedSize 规范化输出大小（字节）
	NormalizedSize int64

	// NormalizedLines 规范化输出行
	NormalizedLines []string

	// PromptMatched 是否匹配到提示符
	PromptMatched bool

	// PaginationCount 分页次数
	PaginationCount int

	// EchoConsumed 是否已消费 echo 行
	EchoConsumed bool

	// StartedAt 命令开始时间
	StartedAt time.Time

	// CompletedAt 命令完成时间
	CompletedAt time.Time

	// Duration 执行时长
	Duration time.Duration

	// Success 是否成功
	Success bool

	// ErrorMessage 错误信息
	ErrorMessage string
}

// DurationMs 返回执行时长（毫秒）
func (r *CommandResult) DurationMs() int64 {
	return r.Duration.Milliseconds()
}

// HasOutput 判断是否有输出
func (r *CommandResult) HasOutput() bool {
	return r.NormalizedText != "" || r.RawText != ""
}

// LineCount 返回规范化行数
func (r *CommandResult) LineCount() int {
	return len(r.NormalizedLines)
}

// ToResult 将 CommandContext 转换为 CommandResult
func (c *CommandContext) ToResult() *CommandResult {
	normalizedText := c.NormalizedText()
	rawText := string(c.RawBuffer)

	return &CommandResult{
		Index:           c.Index,
		Command:         c.Command,
		RawText:         rawText,
		RawSize:         int64(len(c.RawBuffer)),
		NormalizedText:  normalizedText,
		NormalizedSize:  int64(len(normalizedText)),
		NormalizedLines: c.NormalizedLines,
		PromptMatched:   c.PromptMatched,
		PaginationCount: c.PaginationCount,
		EchoConsumed:    c.EchoConsumed,
		StartedAt:       c.StartedAt,
		CompletedAt:     c.CompletedAt,
		Duration:        c.Duration(),
		Success:         !c.HasError(),
		ErrorMessage:    c.ErrorMessage,
	}
}

// ToResultWithIP 将 CommandContext 转换为 CommandResult（带设备 IP）
func (c *CommandContext) ToResultWithIP(deviceIP string) *CommandResult {
	result := c.ToResult()
	result.DeviceIP = deviceIP
	return result
}
