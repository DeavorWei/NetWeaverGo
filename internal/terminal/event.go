package terminal

// EventType 事件类型枚举
type EventType int

const (
	// EventLineCommitted 逻辑行已提交（遇到换行符）
	EventLineCommitted EventType = iota
	// EventActiveLineUpdated 活动行已更新
	EventActiveLineUpdated
	// EventControlSequence 控制序列事件（用于调试）
	EventControlSequence
)

// LineEvent 行事件
type LineEvent struct {
	// Type 事件类型
	Type EventType
	// Line 规范化后的行内容
	Line string
	// Raw 原始控制序列（仅 EventControlSequence 使用）
	Raw string
}
