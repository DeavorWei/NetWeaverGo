package terminal

// Replayer 终端重放器，消费原始文本块，产出规范化事件
// 第一版只维护当前活动行和已提交逻辑行，不做完整二维屏幕仿真
type Replayer struct {
	// ansi ANSI 解析器
	ansi *ANSIParser
	// lineBuf 当前行缓冲区
	lineBuf *LineBuffer
	// committed 已提交的逻辑行
	committed []string
	// width 终端宽度（可选，用于调试）
	width int
}

// NewReplayer 创建新的重放器
func NewReplayer(width int) *Replayer {
	return &Replayer{
		ansi:      NewANSIParser(),
		lineBuf:   NewLineBuffer(),
		committed: make([]string, 0),
		width:     width,
	}
}

// Process 处理输入数据，返回产生的事件
func (r *Replayer) Process(data string) []LineEvent {
	events := make([]LineEvent, 0)

	// 解析为 token 流（保持原始顺序）
	tokens := r.ansi.Parse(data)

	// 按 token 顺序处理
	for _, token := range tokens {
		if token.IsText {
			// 处理文本 token
			for _, ch := range token.Text {
				events = r.processRune(ch, events)
			}
		} else {
			// 处理命令 token
			events = r.processCommand(token.Cmd, events)
		}
	}

	return events
}

// processRune 处理单个 rune 字符
func (r *Replayer) processRune(ch rune, events []LineEvent) []LineEvent {
	switch ch {
	case '\r':
		// 回车：光标移到行首，不提交行
		r.lineBuf.CarriageReturn()

	case '\n':
		// 换行：提交当前行
		line := r.lineBuf.String()
		if line != "" {
			r.committed = append(r.committed, line)
			events = append(events, LineEvent{
				Type: EventLineCommitted,
				Line: line,
			})
		}
		r.lineBuf.Reset()

	case '\b':
		// 退格：光标左移一位
		r.lineBuf.Backspace()

	case '\t':
		// Tab：按 8 列展开为空格（第一版简化实现）
		pos := r.lineBuf.Cursor()
		spaces := 8 - (pos % 8)
		for i := 0; i < spaces; i++ {
			r.lineBuf.Put(' ')
		}

	default:
		// 普通字符：写入当前行
		r.lineBuf.Put(ch)
		events = append(events, LineEvent{
			Type: EventActiveLineUpdated,
			Line: r.lineBuf.String(),
		})
	}

	return events
}

// processCommand 处理 ANSI 命令
func (r *Replayer) processCommand(cmd ANSICommand, events []LineEvent) []LineEvent {
	switch cmd.Type {
	case CmdCursorLeft:
		// ESC[nD 光标左移 n 列
		n := getParam(cmd.Params, 0, 1)
		r.lineBuf.MoveLeft(n)

	case CmdCursorRight:
		// ESC[nC 光标右移 n 列
		n := getParam(cmd.Params, 0, 1)
		r.lineBuf.MoveRight(n)

	case CmdEraseInLine:
		// ESC[nK 擦除行
		// n=0 或省略: 擦除到行尾
		// n=1: 擦除到行首
		// n=2: 擦除整行
		n := getParam(cmd.Params, 0, 0)
		switch n {
		case 0:
			r.lineBuf.EraseToEnd()
		case 1:
			// 擦除到行首（第一版简化：擦除整行）
			r.lineBuf.EraseAll()
		case 2:
			r.lineBuf.EraseAll()
		}

	case CmdSGR:
		// ESC[m 样式设置，忽略（不影响文本内容）

	case CmdUnknown, CmdCursorUp, CmdCursorDown, CmdCursorHome, CmdEraseScreen:
		// 未支持的序列，生成控制序列事件用于调试
		events = append(events, LineEvent{
			Type: EventControlSequence,
			Raw:  cmd.Raw,
		})
	}

	return events
}

// ActiveLine 返回当前活动行（未提交的行）
func (r *Replayer) ActiveLine() string {
	return r.lineBuf.String()
}

// Lines 返回所有已提交的逻辑行
func (r *Replayer) Lines() []string {
	result := make([]string, len(r.committed))
	copy(result, r.committed)
	return result
}

// Reset 重置重放器状态
func (r *Replayer) Reset() {
	r.ansi.Reset()
	r.lineBuf.Reset()
	r.committed = r.committed[:0]
}

// UnknownCount 返回未支持的 ANSI 序列计数
func (r *Replayer) UnknownCount() int {
	return r.ansi.UnknownCount()
}
