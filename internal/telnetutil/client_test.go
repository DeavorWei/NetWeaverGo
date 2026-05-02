package telnetutil

import (
	"bytes"
	"net"
	"testing"
	"time"
)

// mockConn 实现 net.Conn 接口，用于测试。
type mockConn struct {
	writeBuf bytes.Buffer
}

func (m *mockConn) Read(b []byte) (n int, err error)         { return 0, nil }
func (m *mockConn) Write(b []byte) (n int, err error)         { return m.writeBuf.Write(b) }
func (m *mockConn) Close() error                              { return nil }
func (m *mockConn) LocalAddr() net.Addr                       { return &net.TCPAddr{} }
func (m *mockConn) RemoteAddr() net.Addr                      { return &net.TCPAddr{} }
func (m *mockConn) SetDeadline(t time.Time) error             { return nil }
func (m *mockConn) SetReadDeadline(t time.Time) error         { return nil }
func (m *mockConn) SetWriteDeadline(t time.Time) error        { return nil }

// ============================================================================
// OptionHandler 测试
// ============================================================================

func TestNewOptionHandler_DefaultValues(t *testing.T) {
	h := NewOptionHandler("", 0, 0)

	if h.terminalType != DefaultTerminalType {
		t.Errorf("期望终端类型 %q, 实际 %q", DefaultTerminalType, h.terminalType)
	}
	if h.termWidth != DefaultTermWidth {
		t.Errorf("期望宽度 %d, 实际 %d", DefaultTermWidth, h.termWidth)
	}
	if h.termHeight != DefaultTermHeight {
		t.Errorf("期望高度 %d, 实际 %d", DefaultTermHeight, h.termHeight)
	}
}

func TestNewOptionHandler_CustomValues(t *testing.T) {
	h := NewOptionHandler("xterm", 120, 40)

	if h.terminalType != "xterm" {
		t.Errorf("期望终端类型 %q, 实际 %q", "xterm", h.terminalType)
	}
	if h.termWidth != 120 {
		t.Errorf("期望宽度 %d, 实际 %d", 120, h.termWidth)
	}
	if h.termHeight != 40 {
		t.Errorf("期望高度 %d, 实际 %d", 40, h.termHeight)
	}
}

func TestHandleCommand_DO_SuppressGoAhead(t *testing.T) {
	h := NewOptionHandler("", 0, 0)

	responses := h.HandleCommand(DO, OPT_SUPPRESS_GO_AHEAD)
	if len(responses) != 1 {
		t.Fatalf("期望 1 个响应, 实际 %d", len(responses))
	}
	if responses[0].Command != WILL {
		t.Errorf("期望 WILL, 实际 %d", responses[0].Command)
	}
	if responses[0].Option != OPT_SUPPRESS_GO_AHEAD {
		t.Errorf("期望选项 %d, 实际 %d", OPT_SUPPRESS_GO_AHEAD, responses[0].Option)
	}
	if !h.IsLocalOptionEnabled(OPT_SUPPRESS_GO_AHEAD) {
		t.Error("期望 SUPPRESS_GO_AHEAD 本地选项已启用")
	}
}

func TestHandleCommand_DO_TerminalType(t *testing.T) {
	h := NewOptionHandler("", 0, 0)

	responses := h.HandleCommand(DO, OPT_TERMINAL_TYPE)
	if len(responses) != 1 {
		t.Fatalf("期望 1 个响应, 实际 %d", len(responses))
	}
	if responses[0].Command != WILL {
		t.Errorf("期望 WILL, 实际 %d", responses[0].Command)
	}
	if !h.IsLocalOptionEnabled(OPT_TERMINAL_TYPE) {
		t.Error("期望 TERMINAL_TYPE 本地选项已启用")
	}
}

func TestHandleCommand_DO_Unsupported(t *testing.T) {
	h := NewOptionHandler("", 0, 0)

	// 使用一个不支持的选项
	responses := h.HandleCommand(DO, 99)
	if len(responses) != 1 {
		t.Fatalf("期望 1 个响应, 实际 %d", len(responses))
	}
	if responses[0].Command != WONT {
		t.Errorf("期望 WONT, 实际 %d", responses[0].Command)
	}
}

func TestHandleCommand_WILL_Echo(t *testing.T) {
	h := NewOptionHandler("", 0, 0)

	responses := h.HandleCommand(WILL, OPT_ECHO)
	if len(responses) != 1 {
		t.Fatalf("期望 1 个响应, 实际 %d", len(responses))
	}
	if responses[0].Command != DO {
		t.Errorf("期望 DO, 实际 %d", responses[0].Command)
	}
	if !h.IsRemoteOptionEnabled(OPT_ECHO) {
		t.Error("期望 ECHO 远程选项已启用")
	}
}

func TestHandleCommand_WILL_SuppressGoAhead(t *testing.T) {
	h := NewOptionHandler("", 0, 0)

	responses := h.HandleCommand(WILL, OPT_SUPPRESS_GO_AHEAD)
	if len(responses) != 1 {
		t.Fatalf("期望 1 个响应, 实际 %d", len(responses))
	}
	if responses[0].Command != DO {
		t.Errorf("期望 DO, 实际 %d", responses[0].Command)
	}
	if !h.IsRemoteOptionEnabled(OPT_SUPPRESS_GO_AHEAD) {
		t.Error("期望 SUPPRESS_GO_AHEAD 远程选项已启用")
	}
}

func TestHandleCommand_WILL_NAWS(t *testing.T) {
	h := NewOptionHandler("", 0, 0)

	responses := h.HandleCommand(WILL, OPT_NAWS)
	if len(responses) != 1 {
		t.Fatalf("期望 1 个响应, 实际 %d", len(responses))
	}
	if responses[0].Command != DO {
		t.Errorf("期望 DO, 实际 %d", responses[0].Command)
	}
	if !h.IsRemoteOptionEnabled(OPT_NAWS) {
		t.Error("期望 NAWS 远程选项已启用")
	}
}

func TestHandleCommand_WILL_Unsupported(t *testing.T) {
	h := NewOptionHandler("", 0, 0)

	responses := h.HandleCommand(WILL, 99)
	if len(responses) != 1 {
		t.Fatalf("期望 1 个响应, 实际 %d", len(responses))
	}
	if responses[0].Command != DONT {
		t.Errorf("期望 DONT, 实际 %d", responses[0].Command)
	}
}

func TestHandleCommand_DONT(t *testing.T) {
	h := NewOptionHandler("", 0, 0)

	// 先启用选项
	h.HandleCommand(DO, OPT_SUPPRESS_GO_AHEAD)
	if !h.IsLocalOptionEnabled(OPT_SUPPRESS_GO_AHEAD) {
		t.Fatal("期望选项已启用")
	}

	// 然后禁用
	responses := h.HandleCommand(DONT, OPT_SUPPRESS_GO_AHEAD)
	if len(responses) != 1 {
		t.Fatalf("期望 1 个响应, 实际 %d", len(responses))
	}
	if responses[0].Command != WONT {
		t.Errorf("期望 WONT, 实际 %d", responses[0].Command)
	}
	if h.IsLocalOptionEnabled(OPT_SUPPRESS_GO_AHEAD) {
		t.Error("期望 SUPPRESS_GO_AHEAD 本地选项已禁用")
	}
}

func TestHandleCommand_WONT(t *testing.T) {
	h := NewOptionHandler("", 0, 0)

	// 先启用远程选项
	h.HandleCommand(WILL, OPT_ECHO)
	if !h.IsRemoteOptionEnabled(OPT_ECHO) {
		t.Fatal("期望远程选项已启用")
	}

	// 然后远程拒绝
	responses := h.HandleCommand(WONT, OPT_ECHO)
	if len(responses) != 1 {
		t.Fatalf("期望 1 个响应, 实际 %d", len(responses))
	}
	if responses[0].Command != DONT {
		t.Errorf("期望 DONT, 实际 %d", responses[0].Command)
	}
	if h.IsRemoteOptionEnabled(OPT_ECHO) {
		t.Error("期望 ECHO 远程选项已禁用")
	}
}

// ============================================================================
// 子协商测试
// ============================================================================

func TestBuildSubNegotiation_TerminalType(t *testing.T) {
	h := NewOptionHandler("VT100", 0, 0)

	data := h.BuildSubNegotiation(OPT_TERMINAL_TYPE)
	expected := []byte{IAC, SB, OPT_TERMINAL_TYPE, TTYPE_IS, 'V', 'T', '1', '0', '0', IAC, SE}
	if !bytes.Equal(data, expected) {
		t.Errorf("终端类型子协商数据不匹配\n期望: %v\n实际: %v", expected, data)
	}
}

func TestBuildSubNegotiation_NAWS(t *testing.T) {
	h := NewOptionHandler("", 80, 24)

	data := h.BuildSubNegotiation(OPT_NAWS)
	expected := []byte{IAC, SB, OPT_NAWS, 0, 80, 0, 24, IAC, SE}
	if !bytes.Equal(data, expected) {
		t.Errorf("窗口大小子协商数据不匹配\n期望: %v\n实际: %v", expected, data)
	}
}

func TestBuildSubNegotiation_NAWS_LargeValues(t *testing.T) {
	h := NewOptionHandler("", 300, 200)

	data := h.BuildSubNegotiation(OPT_NAWS)
	// 300 = 0x012C, 200 = 0x00C8
	expected := []byte{IAC, SB, OPT_NAWS, 1, 44, 0, 200, IAC, SE}
	if !bytes.Equal(data, expected) {
		t.Errorf("窗口大小子协商数据不匹配\n期望: %v\n实际: %v", expected, data)
	}
}

func TestBuildSubNegotiation_Unsupported(t *testing.T) {
	h := NewOptionHandler("", 0, 0)

	data := h.BuildSubNegotiation(99)
	if data != nil {
		t.Errorf("不支持的选项应返回 nil, 实际: %v", data)
	}
}

// ============================================================================
// 初始协商测试
// ============================================================================

func TestBuildInitialNegotiation(t *testing.T) {
	h := NewOptionHandler("", 0, 0)

	data := h.BuildInitialNegotiation()

	// 验证包含 WILL SUPPRESS_GO_AHEAD
	expectedPrefix := []byte{IAC, WILL, OPT_SUPPRESS_GO_AHEAD}
	if !bytes.HasPrefix(data, expectedPrefix) {
		t.Errorf("初始协商应以 WILL SUPPRESS_GO_AHEAD 开头")
	}

	// 验证长度：4 个协商命令 * 3 字节 = 12
	if len(data) != 12 {
		t.Errorf("期望初始协商数据长度 12, 实际 %d", len(data))
	}
}

// ============================================================================
// filterIAC 测试
// ============================================================================

func TestFilterIAC_PlainText(t *testing.T) {
	c := &Client{
		conn:       &mockConn{},
		optHandler: NewOptionHandler("", 0, 0),
	}
	input := []byte("Hello World")
	result := c.filterIAC(input)
	if !bytes.Equal(result, input) {
		t.Errorf("纯文本不应被过滤: 期望 %q, 实际 %q", input, result)
	}
}

func TestFilterIAC_WithIAC(t *testing.T) {
	mock := &mockConn{}
	c := &Client{
		optHandler: NewOptionHandler("", 0, 0),
		conn:       mock,
	}

	// 构造包含 IAC WILL ECHO 的数据
	input := []byte{'H', 'e', IAC, WILL, OPT_ECHO, 'l', 'o'}
	result := c.filterIAC(input)
	expected := []byte{'H', 'e', 'l', 'o'}
	if !bytes.Equal(result, expected) {
		t.Errorf("IAC 命令应被过滤: 期望 %q, 实际 %q", expected, result)
	}
}

func TestFilterIAC_EscapedIAC(t *testing.T) {
	c := &Client{
		optHandler: NewOptionHandler("", 0, 0),
		conn:       &mockConn{},
	}

	// IAC IAC 应该被解码为单个 0xFF
	input := []byte{'A', IAC, IAC, 'B'}
	result := c.filterIAC(input)
	expected := []byte{'A', IAC, 'B'}
	if !bytes.Equal(result, expected) {
		t.Errorf("转义 IAC 应被解码: 期望 %q, 实际 %q", expected, result)
	}
}

func TestFilterIAC_GA(t *testing.T) {
	c := &Client{
		optHandler: NewOptionHandler("", 0, 0),
		conn:       &mockConn{},
	}

	input := []byte{'A', IAC, GA, 'B'}
	result := c.filterIAC(input)
	expected := []byte{'A', 'B'}
	if !bytes.Equal(result, expected) {
		t.Errorf("GA 应被过滤: 期望 %q, 实际 %q", expected, result)
	}
}

// ============================================================================
// commandName 测试
// ============================================================================

func TestCommandName(t *testing.T) {
	tests := []struct {
		cmd      byte
		expected string
	}{
		{WILL, "WILL"},
		{WONT, "WONT"},
		{DO, "DO"},
		{DONT, "DONT"},
		{SB, "SB"},
		{SE, "SE"},
		{IAC, "IAC"},
		{GA, "GA"},
		{NOP, "NOP"},
		{200, "UNKNOWN(200)"},
	}

	for _, tt := range tests {
		result := commandName(tt.cmd)
		if result != tt.expected {
			t.Errorf("commandName(%d): 期望 %q, 实际 %q", tt.cmd, tt.expected, result)
		}
	}
}

// ============================================================================
// Client 状态测试
// ============================================================================

func TestClient_RemoteAddr(t *testing.T) {
	c := &Client{ip: "192.168.1.1", port: 23}
	expected := "192.168.1.1:23"
	if c.RemoteAddr() != expected {
		t.Errorf("期望 %q, 实际 %q", expected, c.RemoteAddr())
	}
}

func TestClient_IsClosed(t *testing.T) {
	c := &Client{}
	if c.IsClosed() {
		t.Error("新创建的客户端不应为已关闭状态")
	}
	c.closed.Store(true)
	if !c.IsClosed() {
		t.Error("标记为关闭后应返回 true")
	}
}
