package executor

import (
	"context"
	"fmt"
	"time"

	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/report"
	"github.com/NetWeaverGo/core/internal/sshutil"
)

// ============================================================================
// Session Driver (Phase 2 重构)
// ============================================================================
// 负责执行动作，协调 client / logger / eventbus
// 只负责"要做什么"，不参与状态决策

// ClientInterface 客户端接口（用于依赖注入）
type ClientInterface interface {
	SendCommand(cmd string) error
	SendRawBytes(data []byte) error
}

// EventBusInterface 事件总线接口
type EventBusInterface interface {
	Emit(event report.ExecutorEvent)
}

// LogSessionInterface 日志会话接口
type LogSessionInterface interface {
	WriteNormalizedLines(lines []string) error
	WriteCommand(cmd string) error
	Flush() error
}

// SuspendHandlerInterface 挂起处理器接口
type SuspendHandlerInterface interface {
	HandleSuspend(ctx context.Context, ip string, message string, cmd string) UserAction
}

// UserAction 用户动作
type UserAction int

const (
	UserActionNone     UserAction = iota
	UserActionAbort               // 中止
	UserActionContinue            // 继续
)

// SessionDriver 动作执行器
type SessionDriver struct {
	client         ClientInterface
	eventBus       EventBusInterface
	logSession     LogSessionInterface
	suspendHandler SuspendHandlerInterface
	deviceIP       string
	totalCommands  int
}

// NewSessionDriver 创建新的 Driver
func NewSessionDriver(client ClientInterface, eventBus EventBusInterface, logSession LogSessionInterface) *SessionDriver {
	return &SessionDriver{
		client:     client,
		eventBus:   eventBus,
		logSession: logSession,
	}
}

// SetDeviceIP 设置设备 IP（用于事件发送）
func (d *SessionDriver) SetDeviceIP(ip string) {
	d.deviceIP = ip
}

// SetTotalCommands 设置总命令数（用于事件发送）
func (d *SessionDriver) SetTotalCommands(total int) {
	d.totalCommands = total
}

// SetSuspendHandler 设置挂起处理器
func (d *SessionDriver) SetSuspendHandler(handler SuspendHandlerInterface) {
	d.suspendHandler = handler
}

// Execute 执行单个动作
func (d *SessionDriver) Execute(action SessionAction) error {
	switch a := action.(type) {
	case ActSendWarmup:
		return d.executeSendWarmup()

	case ActSendCommand:
		return d.executeSendCommand(a.Index, a.Command, 0)

	case ActSendPagerContinue:
		return d.executeSendPagerContinue()

	case ActEmitCommandStart:
		return d.executeEmitCommandStart(a.Index, a.Command)

	case ActEmitCommandDone:
		return d.executeEmitCommandDone(a.Index, a.Success, a.Duration)

	case ActEmitDeviceError:
		return d.executeEmitDeviceError(a.Message)

	case ActRequestSuspendDecision:
		return d.executeRequestSuspendDecision(a.ErrorContext)

	case ActAbortSession:
		return d.executeAbortSession(a.Reason)

	case ActResetReadTimeout:
		// 超时重置由外部处理
		logger.Debug("SessionDriver", "-", "重置读取超时: %v", a.Timeout)
		return nil

	case ActFlushDetailLog:
		return d.executeFlushDetailLog()

	case ActClearInitResiduals:
		logger.Debug("SessionDriver", "-", "清空初始化残留")
		return nil

	default:
		logger.Debug("SessionDriver", "-", "未知动作类型: %s", action.ActionType())
		return nil
	}
}

// ExecuteAll 执行动作列表
func (d *SessionDriver) ExecuteAll(actions []SessionAction) error {
	for _, action := range actions {
		if err := d.Execute(action); err != nil {
			return err
		}
	}
	return nil
}

// executeSendWarmup 执行发送预热空行
func (d *SessionDriver) executeSendWarmup() error {
	logger.Debug("SessionDriver", "-", "发送预热空行...")
	if err := d.client.SendCommand(""); err != nil {
		return fmt.Errorf("发送预热空行失败: %w", err)
	}

	// 刷新详细日志
	if d.logSession != nil {
		_ = d.logSession.Flush()
	}
	return nil
}

// executeSendCommand 执行发送命令
func (d *SessionDriver) executeSendCommand(index int, command string, timeout time.Duration) error {
	logger.Info("SessionDriver", "-", ">>> [发送命令]: %s", command)

	// 写入命令日志
	if d.logSession != nil {
		if err := d.logSession.WriteCommand(command); err != nil {
			logger.Warn("SessionDriver", "-", "写入命令日志失败: %v", err)
		}
	}

	// 发送命令
	if err := d.client.SendCommand(command); err != nil {
		return fmt.Errorf("发送命令失败: %w", err)
	}

	// 发送事件
	if d.eventBus != nil {
		d.eventBus.Emit(report.ExecutorEvent{
			IP:       d.deviceIP,
			Type:     report.EventDeviceCmd,
			Message:  command,
			CmdIndex: index + 1,
			TotalCmd: d.totalCommands,
		})
	}

	return nil
}

// executeSendPagerContinue 执行发送分页续页
func (d *SessionDriver) executeSendPagerContinue() error {
	logger.Debug("SessionDriver", "-", "[自动翻页] 发送空格继续...")
	if err := d.client.SendRawBytes([]byte(" ")); err != nil {
		return fmt.Errorf("发送空格失败: %w", err)
	}

	// 刷新详细日志
	if d.logSession != nil {
		_ = d.logSession.Flush()
	}
	return nil
}

// executeEmitCommandStart 执行发送命令开始事件
func (d *SessionDriver) executeEmitCommandStart(index int, command string) error {
	if d.eventBus != nil {
		d.eventBus.Emit(report.ExecutorEvent{
			IP:       d.deviceIP,
			Type:     report.EventDeviceCmd,
			Message:  command,
			CmdIndex: index + 1,
			TotalCmd: d.totalCommands,
		})
	}
	return nil
}

// executeEmitCommandDone 执行发送命令完成事件
func (d *SessionDriver) executeEmitCommandDone(index int, success bool, duration time.Duration) error {
	if d.eventBus != nil {
		eventType := report.EventDeviceCmd
		message := fmt.Sprintf("命令完成 (耗时: %v)", duration)
		if !success {
			eventType = report.EventDeviceError
			message = fmt.Sprintf("命令执行失败 (耗时: %v)", duration)
		}
		d.eventBus.Emit(report.ExecutorEvent{
			IP:       d.deviceIP,
			Type:     eventType,
			Message:  message,
			CmdIndex: index + 1,
			TotalCmd: d.totalCommands,
		})
	}
	return nil
}

// executeEmitDeviceError 执行发送设备错误事件
func (d *SessionDriver) executeEmitDeviceError(message string) error {
	if d.eventBus != nil {
		d.eventBus.Emit(report.ExecutorEvent{
			IP:       d.deviceIP,
			Type:     report.EventDeviceError,
			Message:  message,
			TotalCmd: d.totalCommands,
		})
	}
	return nil
}

// executeRequestSuspendDecision 执行请求挂起决策
func (d *SessionDriver) executeRequestSuspendDecision(ctx *ErrorContext) error {
	if ctx == nil {
		logger.Warn("SessionDriver", "-", "请求挂起决策但无错误上下文")
		return nil
	}

	// 发送错误事件
	if d.eventBus != nil {
		d.eventBus.Emit(report.ExecutorEvent{
			IP:       d.deviceIP,
			Type:     report.EventDeviceError,
			Message:  "执行错误: " + ctx.Line,
			TotalCmd: d.totalCommands,
		})
	}

	// 调用挂起处理器等待用户决策
	if d.suspendHandler != nil {
		userAction := d.suspendHandler.HandleSuspend(context.Background(), d.deviceIP, ctx.Line, ctx.Cmd)
		switch userAction {
		case UserActionAbort:
			// 用户选择中止
			if d.eventBus != nil {
				d.eventBus.Emit(report.ExecutorEvent{
					IP:       d.deviceIP,
					Type:     report.EventDeviceAbort,
					Message:  "执行异常被手动中止",
					TotalCmd: d.totalCommands,
				})
			}
			return fmt.Errorf("设备 %s 的执行被用户手动中止", d.deviceIP)

		case UserActionContinue:
			// 用户选择继续
			if d.eventBus != nil {
				d.eventBus.Emit(report.ExecutorEvent{
					IP:       d.deviceIP,
					Type:     report.EventDeviceSkip,
					Message:  "放行异常并继续执行",
					TotalCmd: d.totalCommands,
				})
			}
			logger.Debug("SessionDriver", "-", "用户选择放行错误，继续执行")
			return nil
		}
	}

	// 没有挂起处理器，默认中止
	return fmt.Errorf("设备 %s 执行错误: %s", d.deviceIP, ctx.Line)
}

// executeAbortSession 执行中止会话
func (d *SessionDriver) executeAbortSession(reason string) error {
	if d.eventBus != nil {
		d.eventBus.Emit(report.ExecutorEvent{
			IP:       d.deviceIP,
			Type:     report.EventDeviceAbort,
			Message:  reason,
			TotalCmd: d.totalCommands,
		})
	}
	return fmt.Errorf("任务被中止: %s", reason)
}

// executeFlushDetailLog 执行刷新详细日志
func (d *SessionDriver) executeFlushDetailLog() error {
	if d.logSession != nil {
		return d.logSession.Flush()
	}
	return nil
}

// ============================================================================
// 适配器实现
// ============================================================================

// SSHClientAdapter SSH 客户端适配器
type SSHClientAdapter struct {
	Client *sshutil.SSHClient
}

func (a *SSHClientAdapter) SendCommand(cmd string) error {
	return a.Client.SendCommand(cmd)
}

func (a *SSHClientAdapter) SendRawBytes(data []byte) error {
	return a.Client.SendRawBytes(data)
}

// EventBusAdapter 事件总线适配器
type EventBusAdapter struct {
	Bus chan report.ExecutorEvent
}

func (a *EventBusAdapter) Emit(event report.ExecutorEvent) {
	if a.Bus != nil {
		a.Bus <- event
	}
}
