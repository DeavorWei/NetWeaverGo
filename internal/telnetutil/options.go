// Package telnetutil 实现 Telnet 协议客户端。
// 处理 Telnet 协议协商（IAC 命令）、认证和数据传输。
package telnetutil

// ============================================================================
// Telnet 协议常量 (RFC 854)
// ============================================================================

const (
	// IAC Interpret As Command - 转义字符，表示后续字节是命令
	IAC byte = 255

	// DONT 请求对方禁用某个选项
	DONT byte = 254

	// DO 请求对方启用某个选项
	DO byte = 253

	// WONT 拒绝对方的启用请求，或声明自己不支持某个选项
	WONT byte = 252

	// WILL 声明自己支持/启用某个选项，或同意对方的启用请求
	WILL byte = 251

	// SB Sub-negotiation Begin - 子协商开始
	SB byte = 250

	// GA Go Ahead - 请求对方继续发送
	GA byte = 249

	// EL Erase Line - 擦除当前行
	EL byte = 248

	// EC Erase Character - 擦除前一个字符
	EC byte = 247

	// AYT Are You There - 心跳检测
	AYT byte = 246

	// AO Abort Output - 中止输出
	AO byte = 245

	// IP Interrupt Process - 中断进程
	IP byte = 244

	// BREAK 中断信号
	BREAK byte = 243

	// SE Sub-negotiation End - 子协商结束
	SE byte = 240

	// NOP No Operation - 空操作
	NOP byte = 241
)

// ============================================================================
// Telnet 选项常量 (RFC 1091, RFC 1073, RFC 1079)
// ============================================================================

const (
	// OPT_ECHO 回显选项 (RFC 857)
	OPT_ECHO byte = 1

	// OPT_SUPPRESS_GO_AHEAD 抑制 Go Ahead 选项 (RFC 858)
	OPT_SUPPRESS_GO_AHEAD byte = 3

	// OPT_TERMINAL_TYPE 终端类型选项 (RFC 1091)
	OPT_TERMINAL_TYPE byte = 24

	// OPT_NAWS 窗口大小协商选项 (RFC 1073)
	OPT_NAWS byte = 31

	// OPT_TERMINAL_SPEED 终端速率选项 (RFC 1079)
	OPT_TERMINAL_SPEED byte = 32

	// OPT_LINEMODE 行模式选项 (RFC 1116)
	OPT_LINEMODE byte = 34
)

// ============================================================================
// 子协商命令
// ============================================================================

const (
	// TTYPE_IS 终端类型子协商：客户端声明自己的终端类型
	TTYPE_IS byte = 0

	// TTYPE_SEND 终端类型子协商：服务器请求客户端发送终端类型
	TTYPE_SEND byte = 1
)

// ============================================================================
// 默认配置
// ============================================================================

const (
	// DefaultTerminalType 默认终端类型
	DefaultTerminalType = "VT100"

	// DefaultTermWidth 默认终端宽度
	DefaultTermWidth = 256

	// DefaultTermHeight 默认终端高度
	DefaultTermHeight = 200
)

// ============================================================================
// 协议选项协商处理
// ============================================================================

// NegotiationResponse 表示协商响应。
type NegotiationResponse struct {
	Command byte // WILL/WONT/DO/DONT
	Option  byte // 选项字节
}

// OptionHandler 处理 Telnet 协议选项协商。
// 维护本地和远程的选项状态。
type OptionHandler struct {
	// localOptions 本地已启用的选项集合
	localOptions map[byte]bool
	// remoteOptions 远程已启用的选项集合
	remoteOptions map[byte]bool
	// terminalType 终端类型字符串
	terminalType string
	// termWidth 终端宽度
	termWidth int
	// termHeight 终端高度
	termHeight int
}

// NewOptionHandler 创建选项协商处理器。
func NewOptionHandler(termType string, width, height int) *OptionHandler {
	if termType == "" {
		termType = DefaultTerminalType
	}
	if width <= 0 {
		width = DefaultTermWidth
	}
	if height <= 0 {
		height = DefaultTermHeight
	}
	return &OptionHandler{
		localOptions:  make(map[byte]bool),
		remoteOptions: make(map[byte]bool),
		terminalType:  termType,
		termWidth:     width,
		termHeight:    height,
	}
}

// HandleCommand 处理收到的 IAC 命令并返回需要发送的响应。
// cmd 是命令类型（DO/DONT/WILL/WONT），opt 是选项字节。
func (h *OptionHandler) HandleCommand(cmd, opt byte) []NegotiationResponse {
	var responses []NegotiationResponse

	switch cmd {
	case DO:
		// 服务器请求我们启用某个选项
		switch opt {
		case OPT_SUPPRESS_GO_AHEAD:
			// 我们愿意抑制 GA
			h.localOptions[opt] = true
			responses = append(responses, NegotiationResponse{Command: WILL, Option: opt})
		case OPT_TERMINAL_TYPE:
			// 我们支持终端类型
			h.localOptions[opt] = true
			responses = append(responses, NegotiationResponse{Command: WILL, Option: opt})
		case OPT_ECHO:
			// 我们愿意回显
			h.localOptions[opt] = true
			responses = append(responses, NegotiationResponse{Command: WILL, Option: opt})
		default:
			// 拒绝不支持的选项
			responses = append(responses, NegotiationResponse{Command: WONT, Option: opt})
		}

	case DONT:
		// 服务器要求我们禁用某个选项
		if h.localOptions[opt] {
			h.localOptions[opt] = false
			responses = append(responses, NegotiationResponse{Command: WONT, Option: opt})
		}

	case WILL:
		// 服务器声明它支持某个选项
		switch opt {
		case OPT_ECHO:
			// 服务器愿意回显
			h.remoteOptions[opt] = true
			responses = append(responses, NegotiationResponse{Command: DO, Option: opt})
		case OPT_SUPPRESS_GO_AHEAD:
			// 服务器愿意抑制 GA
			h.remoteOptions[opt] = true
			responses = append(responses, NegotiationResponse{Command: DO, Option: opt})
		case OPT_NAWS:
			// 服务器支持窗口大小协商
			h.remoteOptions[opt] = true
			responses = append(responses, NegotiationResponse{Command: DO, Option: opt})
		default:
			// 拒绝不支持的选项
			responses = append(responses, NegotiationResponse{Command: DONT, Option: opt})
		}

	case WONT:
		// 服务器声明它不支持某个选项
		if h.remoteOptions[opt] {
			h.remoteOptions[opt] = false
			responses = append(responses, NegotiationResponse{Command: DONT, Option: opt})
		}
	}

	return responses
}

// BuildSubNegotiation 构建子协商响应数据。
// 当服务器发送 IAC SB TERMINAL-TYPE SEND IAC SE 时，客户端需要回复终端类型。
func (h *OptionHandler) BuildSubNegotiation(opt byte) []byte {
	switch opt {
	case OPT_TERMINAL_TYPE:
		// IAC SB TERMINAL-TYPE IS <type> IAC SE
		data := make([]byte, 0, 4+len(h.terminalType)+2)
		data = append(data, IAC, SB, OPT_TERMINAL_TYPE, TTYPE_IS)
		data = append(data, []byte(h.terminalType)...)
		data = append(data, IAC, SE)
		return data

	case OPT_NAWS:
		// IAC SB NAWS <width-high> <width-low> <height-high> <height-low> IAC SE
		data := make([]byte, 0, 9)
		data = append(data, IAC, SB, OPT_NAWS)
		data = append(data, byte(h.termWidth>>8), byte(h.termWidth&0xFF))
		data = append(data, byte(h.termHeight>>8), byte(h.termHeight&0xFF))
		data = append(data, IAC, SE)
		return data
	}

	return nil
}

// BuildInitialNegotiation 构建初始协商消息。
// 在连接建立后发送，声明客户端支持的选项。
func (h *OptionHandler) BuildInitialNegotiation() []byte {
	var data []byte

	// 声明我们愿意抑制 GA
	data = append(data, IAC, WILL, OPT_SUPPRESS_GO_AHEAD)

	// 声明我们支持终端类型
	data = append(data, IAC, WILL, OPT_TERMINAL_TYPE)

	// 请求服务器回显
	data = append(data, IAC, DO, OPT_ECHO)

	// 请求服务器抑制 GA
	data = append(data, IAC, DO, OPT_SUPPRESS_GO_AHEAD)

	return data
}

// IsLocalOptionEnabled 检查本地选项是否已启用。
func (h *OptionHandler) IsLocalOptionEnabled(opt byte) bool {
	return h.localOptions[opt]
}

// IsRemoteOptionEnabled 检查远程选项是否已启用。
func (h *OptionHandler) IsRemoteOptionEnabled(opt byte) bool {
	return h.remoteOptions[opt]
}
