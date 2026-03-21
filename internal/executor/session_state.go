package executor

// SessionState 会话状态枚举
// 第一版状态集固定为最小闭环，不做 12~13 个细分状态
type SessionState int

const (
	// StateWaitInitialPrompt 等待初始提示符
	// 连接建立后的初始状态，等待设备返回第一个提示符
	StateWaitInitialPrompt SessionState = iota

	// StateWarmup 预热状态
	// 检测到首个提示符后，发送空行预热终端
	StateWarmup

	// StateReady 就绪状态
	// 预热完成，可以发送下一条命令
	StateReady

	// StateSendCommand 发送命令状态
	// 正在发送命令（瞬时状态）
	StateSendCommand

	// StateCollecting 收集输出状态
	// 命令已发送，正在收集输出
	StateCollecting

	// StateHandlingPager 处理分页状态
	// 检测到分页符，准备发送空格继续
	StateHandlingPager

	// StateWaitingFinalPrompt 等待最终提示符状态
	// 检测到结束提示符候选，需要二次确认
	StateWaitingFinalPrompt

	// StateCompleted 完成状态
	// 所有命令执行完成
	StateCompleted

	// StateFailed 失败状态
	// 执行失败（超时/异常/错误）
	StateFailed

	// StateHandlingError 错误处理状态
	// 检测到错误规则命中，等待外部决策（中止/继续）
	StateHandlingError
)

// String 返回状态的字符串表示
func (s SessionState) String() string {
	switch s {
	case StateWaitInitialPrompt:
		return "WaitInitialPrompt"
	case StateWarmup:
		return "Warmup"
	case StateReady:
		return "Ready"
	case StateSendCommand:
		return "SendCommand"
	case StateCollecting:
		return "Collecting"
	case StateHandlingPager:
		return "HandlingPager"
	case StateWaitingFinalPrompt:
		return "WaitingFinalPrompt"
	case StateCompleted:
		return "Completed"
	case StateFailed:
		return "Failed"
	case StateHandlingError:
		return "HandlingError"
	default:
		return "Unknown"
	}
}

// IsTerminal 判断状态是否为终态
func (s SessionState) IsTerminal() bool {
	return s == StateCompleted || s == StateFailed
}
