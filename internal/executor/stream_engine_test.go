package executor

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/NetWeaverGo/core/internal/config"
)

// scriptReader 按顺序返回预设的字符串块，模拟设备输出流。
type scriptReader struct {
	chunks []string
	index  int
}

func (r *scriptReader) Read(p []byte) (int, error) {
	if r.index >= len(r.chunks) {
		return 0, io.EOF
	}
	chunk := r.chunks[r.index]
	r.index++
	copy(p, []byte(chunk))
	return len(chunk), nil
}

// writeBuffer 收集所有写入的数据，用于验证发送的命令。
type writeBuffer struct {
	strings.Builder
}

func (w *writeBuffer) Close() error { return nil }

// mockDeviceConnection 实现 connutil.DeviceConnection 接口，
// 用于测试 StreamEngine 的流处理逻辑。
type mockDeviceConnection struct {
	reader io.Reader
	writer io.Writer
}

func (m *mockDeviceConnection) Read(p []byte) (int, error)  { return m.reader.Read(p) }
func (m *mockDeviceConnection) Write(p []byte) (int, error) { return m.writer.Write(p) }
func (m *mockDeviceConnection) Close() error                { return nil }

func (m *mockDeviceConnection) SendCommand(cmd string) (string, error) {
	_, err := m.writer.Write([]byte(cmd + "\n"))
	return "", err
}

func (m *mockDeviceConnection) SendRawBytes(data []byte) error {
	_, err := m.writer.Write(data)
	return err
}

func (m *mockDeviceConnection) SetReadDeadline(deadline time.Time) error { return nil }
func (m *mockDeviceConnection) CancelRead()                              {}
func (m *mockDeviceConnection) IsClosed() bool                           { return false }
func (m *mockDeviceConnection) RemoteAddr() string                       { return "192.168.58.200:22" }

func TestStreamEngineRunPlaybook_UnifiedPathSendsWarmupAndCommand(t *testing.T) {
	reader := &scriptReader{
		chunks: []string{
			"Info: login ok\r\n<S1>",
			"\r\n<S1>",
			"disp int b\r\nline-1\r\n<S1>",
		},
	}
	writer := &writeBuffer{}

	conn := &mockDeviceConnection{
		reader: reader,
		writer: writer,
	}

	engine := NewStreamEngine(nil, conn, []string{"disp int b"}, 80)

	results, err := engine.RunPlaybook(context.Background(), 2*time.Second)
	if err != nil {
		t.Fatalf("统一执行路径不应失败: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("应返回 1 条命令结果，实际 %d", len(results))
	}

	got := writer.String()
	if !strings.Contains(got, "\n") {
		t.Fatalf("应至少发送过预热空行，实际输出为 %q", got)
	}
	if !strings.Contains(got, "disp int b\n") {
		t.Fatalf("应在初始化完成后立即发送首条命令，实际输出为 %q", got)
	}

	if engine.adapter.NewState() != NewStateCompleted {
		t.Fatalf("命令完成后状态应为 Completed，实际是 %s", engine.adapter.NewState())
	}

	if results[0].Command != "disp int b" {
		t.Fatalf("命令结果异常: %+v", results[0])
	}
}

func TestStreamEngineRunPlaybook_ContinueOnCmdErrorWaitsForPrompt(t *testing.T) {
	reader := &scriptReader{
		chunks: []string{
			"<SW>",
			"\r\n<SW>",
			"display arp all\r\n^\r\nError: Too many parameters found at '^' position.\r\n<SW>",
			"display device\r\nLSW's Device status:\r\nSlot  Card   Type\r\n1     -      LSW\r\n<SW>",
		},
	}
	writer := &writeBuffer{}

	conn := &mockDeviceConnection{
		reader: reader,
		writer: writer,
	}

	engine := NewStreamEngine(nil, conn, []string{"display arp all", "display device"}, 80)
	engine.adapter.SetContinueOnCmdError(true)

	results, err := engine.RunPlaybook(context.Background(), 2*time.Second)
	if err != nil {
		t.Fatalf("ContinueOnCmdError 场景不应失败: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("应返回 2 条命令结果，实际 %d", len(results))
	}
	if results[0].Success {
		t.Fatalf("第一条命令应失败: %+v", results[0])
	}
	if !strings.Contains(results[0].NormalizedText, "Error: Too many parameters") {
		t.Fatalf("第一条命令应保留设备错误输出，实际 %q", results[0].NormalizedText)
	}
	if !results[1].Success {
		t.Fatalf("第二条命令应成功: %+v", results[1])
	}
	if !strings.Contains(results[1].NormalizedText, "LSW's Device status:") {
		t.Fatalf("第二条命令应拿到 display device 回显，实际 %q", results[1].NormalizedText)
	}

	got := writer.String()
	if !strings.Contains(got, "display arp all\n") || !strings.Contains(got, "display device\n") {
		t.Fatalf("应按顺序发送两条命令，实际发送: %q", got)
	}
}

func TestStreamEngineRunPlaybook_EmitsCommandCompletionBeforeNextDispatch(t *testing.T) {
	reader := &scriptReader{
		chunks: []string{
			"<SW>",
			"\r\n<SW>",
			"display version\r\nVersion: VRP\r\n<SW>",
			"display interface brief\r\nGE0/0/1 up up\r\n<SW>",
		},
	}
	writer := &writeBuffer{}

	conn := &mockDeviceConnection{
		reader: reader,
		writer: writer,
	}

	engine := NewStreamEngine(nil, conn, []string{"display version", "display interface brief"}, 80)

	var events []ExecutionEvent
	engine.SetExecutionEventCallback(func(event ExecutionEvent) {
		events = append(events, event)
	})

	if _, err := engine.RunPlaybook(context.Background(), 2*time.Second); err != nil {
		t.Fatalf("统一执行路径不应失败: %v", err)
	}

	if len(events) != 4 {
		t.Fatalf("应产生 4 条命令记录，实际 %d: %+v", len(events), events)
	}

	expectedKinds := []ExecutionRecordKind{
		RecordCommandDispatched,
		RecordCommandCompleted,
		RecordCommandDispatched,
		RecordCommandCompleted,
	}
	expectedIndexes := []int{0, 0, 1, 1}

	for i, event := range events {
		if event.Kind != expectedKinds[i] {
			t.Fatalf("第 %d 条记录 kind 错误: 预期 %s，实际 %s", i, expectedKinds[i], event.Kind)
		}
		if event.Index != expectedIndexes[i] {
			t.Fatalf("第 %d 条记录 index 错误: 预期 %d，实际 %d", i, expectedIndexes[i], event.Index)
		}
		if event.SessionSeq != uint64(i+1) {
			t.Fatalf("第 %d 条记录 session_seq 错误: 预期 %d，实际 %d", i, i+1, event.SessionSeq)
		}
	}
}

func TestStreamEngine_RiskCommandMode_WarnDefault(t *testing.T) {
	// 设置全局模式为 warn（默认灰度放行）
	st := *config.GetGlobalSettings()
	oldMode := st.RiskCommandMode
	st.RiskCommandMode = "warn"
	config.SetGlobalSettings(st)
	defer func() {
		st.RiskCommandMode = oldMode
		config.SetGlobalSettings(st)
	}()

	reader := &scriptReader{
		chunks: []string{
			"login ok\r\n<S1>",
			"\r\n<S1>",
			"format flash:\r\nformating...\r\n<S1>",
		},
	}
	writer := &writeBuffer{}
	conn := &mockDeviceConnection{reader: reader, writer: writer}

	engine := NewStreamEngine(nil, conn, []string{"format flash:"}, 80)
	results, err := engine.RunPlaybook(context.Background(), 2*time.Second)
	if err != nil {
		t.Fatalf("warn 灰度模式下高危命令应被放行执行，实际报错: %v", err)
	}
	if len(results) != 1 || results[0].Command != "format flash:" {
		t.Fatalf("未正确产生执行结果: %+v", results)
	}
	if !strings.Contains(writer.String(), "format flash:\n") {
		t.Errorf("设备连接应当收到下发的命令，实际写入: %q", writer.String())
	}
}

func TestStreamEngine_RiskCommandMode_EnforceBlock_ContinueOnError(t *testing.T) {
	// 设置严格拦截模式
	st := *config.GetGlobalSettings()
	oldMode := st.RiskCommandMode
	st.RiskCommandMode = "enforce"
	config.SetGlobalSettings(st)
	defer func() {
		st.RiskCommandMode = oldMode
		config.SetGlobalSettings(st)
	}()

	reader := &scriptReader{
		chunks: []string{
			"login ok\r\n<S1>",
			"\r\n<S1>",
			"display version\r\nVersion 1.0\r\n<S1>",
		},
	}
	writer := &writeBuffer{}
	conn := &mockDeviceConnection{reader: reader, writer: writer}

	// 命令队列：第一条高危阻断，第二条正常查询
	engine := NewStreamEngine(nil, conn, []string{"format flash:", "display version"}, 80)
	// 启用单命令错误继续推进
	engine.adapter.SetContinueOnCmdError(true)

	results, err := engine.RunPlaybook(context.Background(), 2*time.Second)
	if err != nil {
		t.Fatalf("ContinueOnCmdError 开启时阻断不应中止整机 Run，但返回错误: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("应当产生 2 条命令的结果（包含失败的第一条），实际产生 %d", len(results))
	}

	// 验证第一条阻断降级为单命令失败
	if results[0].Success {
		t.Errorf("命中 block 的第一条命令应该标记失败")
	}
	if !strings.Contains(results[0].ErrorMessage, "风险命令阻断") {
		t.Errorf("第一条命令未记录阻断错误信息: %q", results[0].ErrorMessage)
	}

	// 验证第二条命令成功执行
	if !results[1].Success || results[1].Command != "display version" {
		t.Errorf("第二条命令应当正常完成: %+v", results[1])
	}

	// 验证高危命令没有被物理下发到设备
	if strings.Contains(writer.String(), "format flash:") {
		t.Errorf("高危阻断命令绝不应物理下发到设备连接，实际写入: %q", writer.String())
	}
}

func TestStreamEngine_CommandCache_ReadHit(t *testing.T) {
	st := *config.GetGlobalSettings()
	oldCache := st.CommandCacheEnabled
	st.CommandCacheEnabled = true
	config.SetGlobalSettings(st)
	defer func() {
		st.CommandCacheEnabled = oldCache
		config.SetGlobalSettings(st)
	}()

	reader := &scriptReader{
		chunks: []string{
			"login ok\r\n<S1>",
			"\r\n<S1>",
			"display version\r\nVersion 5.20\r\n<S1>",
		},
	}
	writer := &writeBuffer{}
	conn := &mockDeviceConnection{reader: reader, writer: writer}

	// 模拟带执行器与缓存的 DeviceExecutor
	executor := &DeviceExecutor{
		IP:           "192.168.1.1",
		commandCache: DefaultCommandCache(),
	}

	// 先在缓存中预填一条命令回显
	executor.commandCache.Put("display version", &CommandResult{
		Command:         "display version",
		RawText:         "display version\r\nVersion 5.20 (Pre-cached)\r\n<S1>",
		NormalizedText:  "Version 5.20 (Pre-cached)",
		NormalizedLines: []string{"Version 5.20 (Pre-cached)"},
		Success:         true,
	})

	engine := NewStreamEngine(executor, conn, []string{"display version"}, 80)
	results, err := engine.RunPlaybook(context.Background(), 2*time.Second)
	if err != nil {
		t.Fatalf("缓存命中执行失败: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("应返回 1 条结果，实际 %d", len(results))
	}
	if !results[0].Cached {
		t.Errorf("命中文档/命令缓存后，结果对象中 Cached 标志必须为 true")
	}
	if !strings.Contains(results[0].NormalizedText, "Pre-cached") {
		t.Errorf("未正确复用缓存内容: %q", results[0].NormalizedText)
	}

	// 验证命令未向设备物理下发（writeBuffer 中不含 display version\n）
	if strings.Contains(writer.String(), "display version\n") {
		t.Errorf("命中缓存后不应向设备网络连接发送命令，实际发送了: %q", writer.String())
	}
}

func TestStreamEngine_CommandKeyTimeoutMatching(t *testing.T) {
	reader := &scriptReader{
		chunks: []string{
			"login ok\r\n<S1>",
			"\r\n<S1>",
			"disp ver\r\nVersion 1.0\r\n<S1>",
		},
	}
	writer := &writeBuffer{}
	conn := &mockDeviceConnection{reader: reader, writer: writer}

	profile := &config.DeviceProfile{
		Vendor: "huawei",
		Commands: []config.CommandSpec{
			{
				Command:    "display version",
				CommandKey: "version",
				TimeoutSec: 75,
			},
		},
	}

	executor := &DeviceExecutor{
		IP:            "192.168.1.1",
		deviceProfile: profile,
	}

	engine := NewStreamEngine(executor, conn, []string{"disp ver"}, 80)
	// 设置命令对应的 key 为 "version"
	engine.adapter.SetCommandKeys([]string{"version"})
	engine.adapter.newContext.AdvanceCommand()

	var currentTimeout time.Duration
	defaultTimeout := 10 * time.Second
	timer := time.NewTimer(defaultTimeout)
	defer timer.Stop()

	// 模拟执行 ActSendCommand 副作用
	err := engine.executeSessionEffect(ActSendCommand{
		Index:   0,
		Command: "disp ver",
	}, &currentTimeout, defaultTimeout, timer)

	if err != nil {
		t.Fatalf("执行 ActSendCommand 失败: %v", err)
	}

	// 期望匹配到 CommandKey="version" 的画像超时设置 75s
	if currentTimeout != 75*time.Second {
		t.Errorf("期望通过 CommandKey 匹配到 75s 超时，实际为 %v", currentTimeout)
	}
}


