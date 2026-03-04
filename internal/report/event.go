package report

type EventType string

const (
	EventDeviceStart   EventType = "start"   // 设备开始连接
	EventDeviceCmd     EventType = "cmd"     // 开始执行某条命令
	EventDeviceSuccess EventType = "success" // 单台设备全部指令执行成功
	EventDeviceError   EventType = "error"   // 单台设备发生报错
	EventDeviceSkip    EventType = "skip"    // 单台设备执行错误策略跳过
	EventDeviceAbort   EventType = "abort"   // 单台设备执行错误策略中断
)

type ExecutorEvent struct {
	IP       string    // 目标设备 IP
	Type     EventType // 事件类型
	Message  string    // 附加文本（如：下发的指令，或者发生的错误原因）
	CmdIndex int       // 当前命令处于组中的索引 (1-based)
	TotalCmd int       // 组内总命令数量
}
