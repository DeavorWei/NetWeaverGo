package executor

import (
	"strings"
	"time"
)

// MaxRawBufferSize 单条命令原始回显内存缓冲区上限 (8MB)，超限截断并打标保全系统内存
const MaxRawBufferSize = 8 * 1024 * 1024

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

	// Truncated 是否因回显超过内存上限而截断
	Truncated bool

	// MaxBufferSize 单命令内存缓冲区上限（字节，默认 8MB）
	MaxBufferSize int

	// NormalizedLines 由 terminal.Replayer 产出的规范化逻辑行
	NormalizedLines []string
	normalizedBytes int

	// ConfirmHandled 标记当前命令是否已处理过交互确认提示，防止重复回复
	ConfirmHandled bool

	// Cached 标记当前命令结果是否命中缓存
	Cached bool

	// EchoConsumed 是否已消费 echo 行
	// 若首个逻辑行等于发送的命令文本，则标记消费并在后续有效行中剥离
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
		MaxBufferSize:   MaxRawBufferSize,
	}
}

// SetMaxBufferSize 设置单命令最大内存缓冲区（字节）
func (c *CommandContext) SetMaxBufferSize(size int) {
	if size > 0 {
		c.MaxBufferSize = size
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

// AppendRawData 追加原始数据（单命令超出 MaxBufferSize 驻留内存时打标截断）
func (c *CommandContext) AppendRawData(data []byte) {
	if c.Truncated {
		return
	}
	limit := c.MaxBufferSize
	if limit <= 0 {
		limit = MaxRawBufferSize
	}
	if len(c.RawBuffer)+len(data) > limit {
		remaining := limit - len(c.RawBuffer)
		if remaining > 0 {
			c.RawBuffer = append(c.RawBuffer, data[:remaining]...)
		}
		c.Truncated = true
		return
	}
	c.RawBuffer = append(c.RawBuffer, data...)
}

// AddNormalizedLine 添加规范化行（若首行与命令匹配则触发 EchoConsumed，超出 MaxBufferSize 则打标截断）
func (c *CommandContext) AddNormalizedLine(line string) {
	if !c.EchoConsumed && len(c.NormalizedLines) == 0 && c.Command != "" {
		cleanLine := strings.TrimSpace(line)
		cleanCmd := strings.TrimSpace(c.Command)
		if cleanLine == cleanCmd {
			c.EchoConsumed = true
		}
	}
	limit := c.MaxBufferSize
	if limit <= 0 {
		limit = MaxRawBufferSize
	}
	if c.Truncated || c.normalizedBytes+len(line) > limit {
		c.Truncated = true
		return
	}
	c.normalizedBytes += len(line) + 1
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

// Duration 返回命令执行时长
func (c *CommandContext) Duration() time.Duration {
	if c.CompletedAt.IsZero() {
		return time.Since(c.StartedAt)
	}
	return c.CompletedAt.Sub(c.StartedAt)
}

// EffectiveLines 返回去除首行 Echo 后的有效逻辑行
func (c *CommandContext) EffectiveLines() []string {
	if c.EchoConsumed && len(c.NormalizedLines) > 0 {
		cleanFirst := strings.TrimSpace(c.NormalizedLines[0])
		cleanCmd := strings.TrimSpace(c.Command)
		if cleanFirst == cleanCmd {
			return c.NormalizedLines[1:]
		}
	}
	return c.NormalizedLines
}

// NormalizedText 返回规范化文本（若 EchoConsumed 生效则自动剥离首行命令行）
func (c *CommandContext) NormalizedText() string {
	lines := c.EffectiveLines()
	result := ""
	for i, line := range lines {
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

	// Truncated 是否因回显超过 8MB 而在内存中截断
	Truncated bool

	// Cached 是否命中文档/命令缓存
	Cached bool

	// NormalizedText 规范化输出文本（由 terminal.Replayer 产出，已去除首行 Echo）
	NormalizedText string

	// NormalizedSize 规范化输出大小（字节）
	NormalizedSize int64

	// NormalizedLines 规范化输出行（已去除首行 Echo）
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
	effectiveLines := c.EffectiveLines()
	normalizedText := c.NormalizedText()
	rawText := string(c.RawBuffer)

	return &CommandResult{
		Index:           c.Index,
		Command:         c.Command,
		RawText:         rawText,
		RawSize:         int64(len(c.RawBuffer)),
		Truncated:       c.Truncated,
		Cached:          c.Cached,
		NormalizedText:  normalizedText,
		NormalizedSize:  int64(len(normalizedText)),
		NormalizedLines: effectiveLines,
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
