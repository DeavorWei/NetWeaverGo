package terminal

// MaxLineBufferSize 行缓冲区最大容量（64KB）
// 防止极端情况下的内存问题
const MaxLineBufferSize = 65536

// LineBuffer 行缓冲区，支持光标移动和覆盖写
// 第一版核心数据结构，维护单行文本内容和光标位置
type LineBuffer struct {
	// cells rune 数组存储行内容
	cells []rune
	// cursor 光标位置（rune 索引）
	cursor int
}

// NewLineBuffer 创建新的行缓冲区
func NewLineBuffer() *LineBuffer {
	return &LineBuffer{
		cells:  make([]rune, 0, 256),
		cursor: 0,
	}
}

// Put 在当前光标位置写入 rune，光标右移
// 如果光标不在行尾，则覆盖现有字符
// 如果达到容量上限，则丢弃最旧的字符
func (l *LineBuffer) Put(r rune) {
	// 容量上限保护
	if len(l.cells) >= MaxLineBufferSize {
		// 达到上限时，丢弃最旧的字符
		l.cells = l.cells[1:]
		if l.cursor > 0 {
			l.cursor--
		}
	}

	if l.cursor < len(l.cells) {
		// 覆盖现有字符
		l.cells[l.cursor] = r
	} else {
		// 追加到行尾
		l.cells = append(l.cells, r)
	}
	l.cursor++
}

// MoveLeft 光标左移 n 位，不超过行首
func (l *LineBuffer) MoveLeft(n int) {
	l.cursor -= n
	if l.cursor < 0 {
		l.cursor = 0
	}
}

// MoveRight 光标右移 n 位，不超过行尾
func (l *LineBuffer) MoveRight(n int) {
	l.cursor += n
	if l.cursor > len(l.cells) {
		l.cursor = len(l.cells)
	}
}

// CarriageReturn 回车：光标移到行首
// 注意：CR 只移动光标，不清空内容
// 后续写入会覆盖现有内容，但不会清除尾部残留
func (l *LineBuffer) CarriageReturn() {
	l.cursor = 0
}

// Backspace 退格：光标左移一位
func (l *LineBuffer) Backspace() {
	if l.cursor > 0 {
		l.cursor--
	}
}

// EraseToEnd 擦除从光标到行尾的内容
func (l *LineBuffer) EraseToEnd() {
	if l.cursor < len(l.cells) {
		l.cells = l.cells[:l.cursor]
	}
}

// EraseAll 擦除整行
func (l *LineBuffer) EraseAll() {
	l.cells = l.cells[:0]
	l.cursor = 0
}

// String 返回当前行的字符串表示
func (l *LineBuffer) String() string {
	return string(l.cells)
}

// Reset 重置行缓冲区
func (l *LineBuffer) Reset() {
	l.cells = l.cells[:0]
	l.cursor = 0
}

// Len 返回当前行长度
func (l *LineBuffer) Len() int {
	return len(l.cells)
}

// Cursor 返回当前光标位置
func (l *LineBuffer) Cursor() int {
	return l.cursor
}
