package terminal

// CommandType ANSI 命令类型
type CommandType int

const (
	CmdNone CommandType = iota
	// CmdCursorLeft 光标左移 ESC[nD
	CmdCursorLeft
	// CmdCursorRight 光标右移 ESC[nC
	CmdCursorRight
	// CmdCursorUp 光标上移 ESC[nA
	CmdCursorUp
	// CmdCursorDown 光标下移 ESC[nB
	CmdCursorDown
	// CmdCursorHome 光标定位 ESC[H 或 ESC[f
	CmdCursorHome
	// CmdEraseInLine 擦除行 ESC[nK (n=0:到行尾, n=1:到行首, n=2:整行)
	CmdEraseInLine
	// CmdEraseScreen 擦除屏幕 ESC[J
	CmdEraseScreen
	// CmdSGR 样式设置 ESC[m
	CmdSGR
	// CmdUnknown 未支持的序列
	CmdUnknown
)

// Token 表示解析后的 token，可以是文本或命令
type Token struct {
	// IsText 是否为文本 token
	IsText bool
	// Text 文本内容（仅当 IsText 为 true 时有效）
	Text []rune
	// Cmd 命令（仅当 IsText 为 false 时有效）
	Cmd ANSICommand
}

// ANSICommand 解析后的 ANSI 命令
type ANSICommand struct {
	// Type 命令类型
	Type CommandType
	// Params 参数列表
	Params []int
	// Raw 原始序列文本
	Raw string
}

// parseState 解析器状态
type parseState int

const (
	stateGround parseState = iota
	stateEscape
	stateCSI
)

// ANSIParser ANSI 控制序列解析器
type ANSIParser struct {
	// state 当前解析状态
	state parseState
	// buffer 累积的原始字节
	buffer []byte
	// params 解析中的参数
	params []int
	// currentParam 当前参数值
	currentParam int
	// unknownCount 未支持的序列计数
	unknownCount int
}

// NewANSIParser 创建新的 ANSI 解析器
func NewANSIParser() *ANSIParser {
	return &ANSIParser{
		state:        stateGround,
		buffer:       make([]byte, 0, 64),
		params:       make([]int, 0, 4),
		currentParam: 0,
	}
}

// Parse 解析输入数据，返回 token 流（保持原始顺序）
func (p *ANSIParser) Parse(data string) []Token {
	tokens := make([]Token, 0)
	currentText := make([]rune, 0, len(data))

	flushText := func() {
		if len(currentText) > 0 {
			tokens = append(tokens, Token{
				IsText: true,
				Text:   currentText,
			})
			currentText = make([]rune, 0)
		}
	}

	for _, r := range data {
		switch p.state {
		case stateGround:
			if r == 0x1B { // ESC
				flushText()
				p.state = stateEscape
				p.buffer = append(p.buffer, byte(r))
			} else {
				currentText = append(currentText, r)
			}

		case stateEscape:
			p.buffer = append(p.buffer, byte(r))
			if r == '[' {
				p.state = stateCSI
				p.params = p.params[:0]
				p.currentParam = -1 // -1 表示尚未开始解析参数
			} else {
				// 非 CSI 序列，作为未知序列处理
				cmd := ANSICommand{
					Type: CmdUnknown,
					Raw:  string(p.buffer),
				}
				tokens = append(tokens, Token{
					IsText: false,
					Cmd:    cmd,
				})
				p.unknownCount++
				p.resetState()
			}

		case stateCSI:
			p.buffer = append(p.buffer, byte(r))
			if r >= '0' && r <= '9' {
				if p.currentParam < 0 {
					p.currentParam = 0
				}
				p.currentParam = p.currentParam*10 + int(r-'0')
			} else if r == ';' {
				p.params = append(p.params, p.currentParam)
				p.currentParam = 0
			} else if r >= 0x40 && r <= 0x7E {
				// 最终字节，完成命令解析
				if p.currentParam >= 0 {
					p.params = append(p.params, p.currentParam)
				}
				cmd := p.parseCommand(byte(r))
				tokens = append(tokens, Token{
					IsText: false,
					Cmd:    cmd,
				})
				p.resetState()
			}
		}
	}

	// 刷新剩余文本
	flushText()

	return tokens
}

// parseCommand 根据最终字节解析命令类型
func (p *ANSIParser) parseCommand(final byte) ANSICommand {
	raw := string(p.buffer)
	cmd := ANSICommand{
		Raw: raw,
	}

	// 复制参数
	params := make([]int, len(p.params))
	copy(params, p.params)
	cmd.Params = params

	switch final {
	case 'A':
		cmd.Type = CmdCursorUp
		p.unknownCount++
	case 'B':
		cmd.Type = CmdCursorDown
		p.unknownCount++
	case 'C':
		cmd.Type = CmdCursorRight
	case 'D':
		cmd.Type = CmdCursorLeft
	case 'H', 'f':
		cmd.Type = CmdCursorHome
		p.unknownCount++
	case 'J':
		cmd.Type = CmdEraseScreen
		p.unknownCount++
	case 'K':
		cmd.Type = CmdEraseInLine
	case 'm':
		cmd.Type = CmdSGR
	default:
		cmd.Type = CmdUnknown
		p.unknownCount++
	}

	return cmd
}

// resetState 重置解析状态
func (p *ANSIParser) resetState() {
	p.state = stateGround
	p.buffer = p.buffer[:0]
	p.currentParam = -1
}

// UnknownCount 返回未支持的序列计数
func (p *ANSIParser) UnknownCount() int {
	return p.unknownCount
}

// Reset 重置解析器状态
func (p *ANSIParser) Reset() {
	p.state = stateGround
	p.buffer = p.buffer[:0]
	p.params = p.params[:0]
	p.currentParam = 0
	p.unknownCount = 0
}

// getParam 安全获取参数值，如果不存在则返回默认值
func getParam(params []int, index int, defaultValue int) int {
	if index < len(params) && params[index] > 0 {
		return params[index]
	}
	return defaultValue
}
