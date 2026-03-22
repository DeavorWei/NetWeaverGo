package executor

import (
	"context"
	"testing"
	"time"

	"github.com/NetWeaverGo/core/internal/report"
)

// MockClient 用于测试的 Mock 客户端
type MockClient struct {
	lastCommand    string
	lastRawBytes   []byte
	sendCmdError   error
	sendBytesError error
}

func (m *MockClient) SendCommand(cmd string) error {
	m.lastCommand = cmd
	return m.sendCmdError
}

func (m *MockClient) SendRawBytes(data []byte) error {
	m.lastRawBytes = data
	return m.sendBytesError
}

// MockEventBus 用于测试的 Mock 事件总线
type MockEventBus struct {
	events []report.ExecutorEvent
}

func NewMockEventBus() *MockEventBus {
	return &MockEventBus{
		events: make([]report.ExecutorEvent, 0),
	}
}

func (m *MockEventBus) Emit(event report.ExecutorEvent) {
	m.events = append(m.events, event)
}

func (m *MockEventBus) LastEvent() report.ExecutorEvent {
	if len(m.events) == 0 {
		return report.ExecutorEvent{}
	}
	return m.events[len(m.events)-1]
}

// MockLogSession 用于测试的 Mock 日志会话
type MockLogSession struct {
	commands        []string
	normalizedLines []string
	flushCount      int
}

func NewMockLogSession() *MockLogSession {
	return &MockLogSession{
		commands:        make([]string, 0),
		normalizedLines: make([]string, 0),
	}
}

func (m *MockLogSession) WriteNormalizedLines(lines []string) error {
	m.normalizedLines = append(m.normalizedLines, lines...)
	return nil
}

func (m *MockLogSession) WriteCommand(cmd string) error {
	m.commands = append(m.commands, cmd)
	return nil
}

func (m *MockLogSession) Flush() error {
	m.flushCount++
	return nil
}

// TestDriverSendWarmup 测试发送预热空行
func TestDriverSendWarmup(t *testing.T) {
	mockClient := &MockClient{}
	mockEventBus := NewMockEventBus()
	mockLogSession := NewMockLogSession()

	driver := NewSessionDriver(mockClient, mockEventBus, mockLogSession)

	err := driver.Execute(ActSendWarmup{})
	if err != nil {
		t.Fatalf("执行 ActSendWarmup 失败: %v", err)
	}

	if mockClient.lastCommand != "" {
		t.Errorf("期望发送空命令，得到 '%s'", mockClient.lastCommand)
	}

	if mockLogSession.flushCount != 1 {
		t.Errorf("期望刷新 1 次，得到 %d 次", mockLogSession.flushCount)
	}
}

// TestDriverSendCommand 测试发送命令
func TestDriverSendCommand(t *testing.T) {
	mockClient := &MockClient{}
	mockEventBus := NewMockEventBus()
	mockLogSession := NewMockLogSession()

	driver := NewSessionDriver(mockClient, mockEventBus, mockLogSession)
	driver.SetDeviceIP("192.168.1.1")
	driver.SetTotalCommands(5)

	err := driver.Execute(ActSendCommand{
		Index:   0,
		Command: "display version",
	})
	if err != nil {
		t.Fatalf("执行 ActSendCommand 失败: %v", err)
	}

	if mockClient.lastCommand != "display version" {
		t.Errorf("期望发送 'display version'，得到 '%s'", mockClient.lastCommand)
	}

	if len(mockLogSession.commands) != 1 {
		t.Errorf("期望写入 1 条命令日志，得到 %d 条", len(mockLogSession.commands))
	}

	lastEvent := mockEventBus.LastEvent()
	if lastEvent.Type != report.EventDeviceCmd {
		t.Errorf("期望事件类型 EventDeviceCmd，得到 %v", lastEvent.Type)
	}
	if lastEvent.Message != "display version" {
		t.Errorf("期望事件消息 'display version'，得到 '%s'", lastEvent.Message)
	}
}

// TestDriverSendPagerContinue 测试发送分页续页
func TestDriverSendPagerContinue(t *testing.T) {
	mockClient := &MockClient{}
	mockEventBus := NewMockEventBus()
	mockLogSession := NewMockLogSession()

	driver := NewSessionDriver(mockClient, mockEventBus, mockLogSession)

	err := driver.Execute(ActSendPagerContinue{})
	if err != nil {
		t.Fatalf("执行 ActSendPagerContinue 失败: %v", err)
	}

	if string(mockClient.lastRawBytes) != " " {
		t.Errorf("期望发送空格，得到 '%s'", string(mockClient.lastRawBytes))
	}

	if mockLogSession.flushCount != 1 {
		t.Errorf("期望刷新 1 次，得到 %d 次", mockLogSession.flushCount)
	}
}

// TestDriverEmitCommandStart 测试发送命令开始事件
func TestDriverEmitCommandStart(t *testing.T) {
	mockClient := &MockClient{}
	mockEventBus := NewMockEventBus()
	mockLogSession := NewMockLogSession()

	driver := NewSessionDriver(mockClient, mockEventBus, mockLogSession)
	driver.SetDeviceIP("192.168.1.1")
	driver.SetTotalCommands(3)

	err := driver.Execute(ActEmitCommandStart{
		Index:   1,
		Command: "display interface",
	})
	if err != nil {
		t.Fatalf("执行 ActEmitCommandStart 失败: %v", err)
	}

	lastEvent := mockEventBus.LastEvent()
	if lastEvent.Type != report.EventDeviceCmd {
		t.Errorf("期望事件类型 EventDeviceCmd，得到 %v", lastEvent.Type)
	}
	if lastEvent.CmdIndex != 2 {
		t.Errorf("期望命令索引 2，得到 %d", lastEvent.CmdIndex)
	}
}

// TestDriverEmitCommandDone 测试发送命令完成事件
func TestDriverEmitCommandDone(t *testing.T) {
	mockClient := &MockClient{}
	mockEventBus := NewMockEventBus()
	mockLogSession := NewMockLogSession()

	driver := NewSessionDriver(mockClient, mockEventBus, mockLogSession)
	driver.SetDeviceIP("192.168.1.1")

	err := driver.Execute(ActEmitCommandDone{
		Index:    0,
		Success:  true,
		Duration: time.Second * 5,
	})
	if err != nil {
		t.Fatalf("执行 ActEmitCommandDone 失败: %v", err)
	}

	lastEvent := mockEventBus.LastEvent()
	if lastEvent.Type != report.EventDeviceCmd {
		t.Errorf("期望事件类型 EventDeviceCmd，得到 %v", lastEvent.Type)
	}
}

// TestDriverEmitDeviceError 测试发送设备错误事件
func TestDriverEmitDeviceError(t *testing.T) {
	mockClient := &MockClient{}
	mockEventBus := NewMockEventBus()
	mockLogSession := NewMockLogSession()

	driver := NewSessionDriver(mockClient, mockEventBus, mockLogSession)
	driver.SetDeviceIP("192.168.1.1")

	err := driver.Execute(ActEmitDeviceError{
		Index:   0,
		Message: "命令执行失败",
	})
	if err != nil {
		t.Fatalf("执行 ActEmitDeviceError 失败: %v", err)
	}

	lastEvent := mockEventBus.LastEvent()
	if lastEvent.Type != report.EventDeviceError {
		t.Errorf("期望事件类型 EventDeviceError，得到 %v", lastEvent.Type)
	}
	if lastEvent.Message != "命令执行失败" {
		t.Errorf("期望消息 '命令执行失败'，得到 '%s'", lastEvent.Message)
	}
}

// TestDriverAbortSession 测试中止会话
func TestDriverAbortSession(t *testing.T) {
	mockClient := &MockClient{}
	mockEventBus := NewMockEventBus()
	mockLogSession := NewMockLogSession()

	driver := NewSessionDriver(mockClient, mockEventBus, mockLogSession)
	driver.SetDeviceIP("192.168.1.1")

	err := driver.Execute(ActAbortSession{
		Reason: "用户手动中止",
	})
	if err == nil {
		t.Fatal("期望返回错误，得到 nil")
	}

	lastEvent := mockEventBus.LastEvent()
	if lastEvent.Type != report.EventDeviceAbort {
		t.Errorf("期望事件类型 EventDeviceAbort，得到 %v", lastEvent.Type)
	}
}

// TestDriverExecuteAll 测试执行多个动作
func TestDriverExecuteAll(t *testing.T) {
	mockClient := &MockClient{}
	mockEventBus := NewMockEventBus()
	mockLogSession := NewMockLogSession()

	driver := NewSessionDriver(mockClient, mockEventBus, mockLogSession)
	driver.SetDeviceIP("192.168.1.1")
	driver.SetTotalCommands(2)

	actions := []SessionAction{
		ActSendWarmup{},
		ActSendCommand{Index: 0, Command: "display version"},
	}

	err := driver.ExecuteAll(actions)
	if err != nil {
		t.Fatalf("ExecuteAll 失败: %v", err)
	}

	if mockClient.lastCommand != "display version" {
		t.Errorf("期望最后发送 'display version'，得到 '%s'", mockClient.lastCommand)
	}

	if len(mockEventBus.events) != 1 {
		t.Errorf("期望 1 个事件，得到 %d 个", len(mockEventBus.events))
	}
}

// MockSuspendHandler 用于测试的 Mock 挂起处理器
type MockSuspendHandler struct {
	action UserAction
}

func (m *MockSuspendHandler) HandleSuspend(ctx context.Context, ip string, message string, cmd string) UserAction {
	return m.action
}

// TestDriverRequestSuspendDecisionAbort 测试挂起决策-中止
func TestDriverRequestSuspendDecisionAbort(t *testing.T) {
	mockClient := &MockClient{}
	mockEventBus := NewMockEventBus()
	mockLogSession := NewMockLogSession()

	driver := NewSessionDriver(mockClient, mockEventBus, mockLogSession)
	driver.SetDeviceIP("192.168.1.1")
	driver.SetSuspendHandler(&MockSuspendHandler{action: UserActionAbort})

	err := driver.Execute(ActRequestSuspendDecision{
		ErrorContext: &ErrorContext{
			Line:     "Error: Invalid command",
			CmdIndex: 0,
			Cmd:      "test",
		},
	})
	if err == nil {
		t.Fatal("期望返回错误，得到 nil")
	}

	lastEvent := mockEventBus.LastEvent()
	if lastEvent.Type != report.EventDeviceAbort {
		t.Errorf("期望事件类型 EventDeviceAbort，得到 %v", lastEvent.Type)
	}
}

// TestDriverRequestSuspendDecisionContinue 测试挂起决策-继续
func TestDriverRequestSuspendDecisionContinue(t *testing.T) {
	mockClient := &MockClient{}
	mockEventBus := NewMockEventBus()
	mockLogSession := NewMockLogSession()

	driver := NewSessionDriver(mockClient, mockEventBus, mockLogSession)
	driver.SetDeviceIP("192.168.1.1")
	driver.SetSuspendHandler(&MockSuspendHandler{action: UserActionContinue})

	err := driver.Execute(ActRequestSuspendDecision{
		ErrorContext: &ErrorContext{
			Line:     "Error: Invalid command",
			CmdIndex: 0,
			Cmd:      "test",
		},
	})
	if err != nil {
		t.Fatalf("期望返回 nil，得到错误: %v", err)
	}

	// 检查是否有跳过事件
	found := false
	for _, e := range mockEventBus.events {
		if e.Type == report.EventDeviceSkip {
			found = true
			break
		}
	}
	if !found {
		t.Error("期望找到 EventDeviceSkip 事件")
	}
}
