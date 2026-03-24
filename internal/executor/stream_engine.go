package executor

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/NetWeaverGo/core/internal/config"
	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/matcher"
	"github.com/NetWeaverGo/core/internal/report"
	"github.com/NetWeaverGo/core/internal/sshutil"
	"github.com/NetWeaverGo/core/internal/terminal"
)

// RunMode 运行模式
type RunMode int

const (
	// ModePlaybook Playbook 模式：执行整个命令队列
	ModePlaybook RunMode = iota
	// ModeSingle 单命令模式：只执行一条命令并返回结果
	ModeSingle
)

// StreamEngine 统一流处理引擎
// 为 ExecutePlaybook() 和 ExecuteCommandSync() 提供公共执行内核
// 重构后：统一使用 SessionAdapter 驱动会话状态。
type StreamEngine struct {
	// adapter 会话适配器
	adapter *SessionAdapter

	// matcher 流匹配器
	matcher *matcher.StreamMatcher

	// client SSH 客户端
	client *sshutil.SSHClient

	// executor 所属执行器（用于回调）
	executor *DeviceExecutor

	// width 终端宽度
	width int

	// suspendHandler 错误/超时挂起处理器
	suspendHandler SuspendHandler
}

// NewStreamEngine 创建新的流处理引擎
func NewStreamEngine(executor *DeviceExecutor, client *sshutil.SSHClient, commands []string, width int) *StreamEngine {
	m := matcher.NewStreamMatcher()
	adapter := NewSessionAdapter(width, commands, m)
	logger.Debug("StreamEngine", "-", "使用新会话架构 (Detector+Reducer+Driver)")

	return &StreamEngine{
		adapter:        adapter,
		matcher:        m,
		client:         client,
		executor:       executor,
		width:          width,
		suspendHandler: nil,
	}
}

// SetSuspendHandler 设置挂起处理器
func (e *StreamEngine) SetSuspendHandler(handler SuspendHandler) {
	e.suspendHandler = handler
}

// SetErrorMatcher 设置错误匹配器（使用执行器的匹配器）
func (e *StreamEngine) SetErrorMatcher(m *matcher.StreamMatcher) {
	e.matcher = m
	e.adapter.matcher = m
}

// Run 运行流处理引擎
// mode=playbook 时消费整队列
// mode=single 时只消费一条命令并返回单结果
func (e *StreamEngine) Run(ctx context.Context, mode RunMode, defaultTimeout time.Duration) ([]*CommandResult, error) {
	if e.client != nil {
		// 初始化阶段可能设置过底层 TCP read deadline，这里统一清零，避免影响正式执行流。
		_ = e.client.SetReadDeadline(time.Time{})
	}

	bufferSize := config.DefaultBufferSize
	paginationInterval := config.PaginationCheckInterval
	if manager := config.GetRuntimeManagerIfInitialized(); manager != nil {
		if size := manager.GetBufferSize(); size > 0 {
			bufferSize = size
		}
		if interval := manager.GetPaginationCheckInterval(); interval > 0 {
			paginationInterval = interval
		}
	}

	buf := make([]byte, bufferSize)
	outReader := e.client.Stdout
	errReader := e.client.Stderr

	// 丢弃并记录 stderr
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Warn("StreamEngine", "-", "stderr 协程 panic 已恢复: %v", r)
			}
		}()
		_, _ = io.Copy(io.Discard, errReader)
	}()

	// 当前超时设置
	currentTimeout := defaultTimeout
	timer := time.NewTimer(currentTimeout)
	defer timer.Stop()

	// 读取结果通道
	// 关键修复：readResult 携带数据副本，避免共享缓冲区竞态条件
	type readResult struct {
		data []byte // 存储读取数据的副本
		err  error
	}
	readCh := make(chan readResult, 1)

	// 后台读取协程
	go func() {
		defer close(readCh)
		defer func() {
			if r := recover(); r != nil {
				logger.Warn("StreamEngine", "-", "读取协程 panic 已恢复: %v", r)
			}
		}()

		for {
			n, err := outReader.Read(buf)
			// 关键修复：立即复制数据，避免竞态条件
			// 主线程稍后取数据时，buf 可能已被下一次读取覆盖
			var data []byte
			if n > 0 {
				data = make([]byte, n)
				copy(data, buf[:n])
			}
			select {
			case readCh <- readResult{data: data, err: err}:
			case <-ctx.Done():
				return
			}
			if err != nil {
				return
			}
		}
	}()

	// 主事件循环
	for {
		select {
		case <-ctx.Done():
			e.adapter.MarkFailed("上下文取消")
			return e.adapter.Results(), ctx.Err()

		case <-timer.C:
			// 超时处理
			logger.Debug("StreamEngine", "-", "读取超时，当前状态: %s", e.adapter.NewState())

			// 获取当前命令信息
			failedCmd := ""
			cmdIndex := -1
			if cmd := e.adapter.CurrentCommand(); cmd != nil {
				failedCmd = cmd.RawCommand
				cmdIndex = cmd.Index
			}

			// 发送超时事件
			if e.executor != nil && e.executor.EventBus != nil {
				e.executor.EventBus <- report.ExecutorEvent{
					IP:       e.executor.IP,
					Type:     report.EventDeviceError,
					Message:  "Timeout Error: No prompt received within timeout",
					TotalCmd: e.adapter.TotalCommands(),
					CmdIndex: cmdIndex + 1,
				}
			}

			// 调用挂起处理器等待用户决策
			if e.suspendHandler != nil {
				userAction := e.suspendHandler(ctx, e.executor.IP, "Timeout Error: No prompt received within timeout", failedCmd)
				switch userAction {
				case ActionAbort:
					fallthrough
				case ActionAbortTimeout:
					// 用户选择中止
					if e.executor != nil && e.executor.EventBus != nil {
						e.executor.EventBus <- report.ExecutorEvent{
							IP:       e.executor.IP,
							Type:     report.EventDeviceAbort,
							Message:  "因超时被手动中止",
							TotalCmd: e.adapter.TotalCommands(),
						}
					}
					e.adapter.MarkFailed("读取超时，用户中止")
					return e.adapter.Results(), fmt.Errorf("设备 %s 的执行因超时被用户中止", e.executor.IP)

				case ActionContinue:
					// 用户选择继续
					if e.executor != nil && e.executor.EventBus != nil {
						e.executor.EventBus <- report.ExecutorEvent{
							IP:       e.executor.IP,
							Type:     report.EventDeviceSkip,
							Message:  "放行超时并继续执行",
							TotalCmd: e.adapter.TotalCommands(),
						}
					}
					// 重置计时器继续执行
					timer.Reset(currentTimeout)
					logger.Debug("StreamEngine", "-", "用户选择放行超时，继续执行")
					continue
				}
			}

			e.adapter.MarkFailed("读取超时")
			return e.adapter.Results(), fmt.Errorf("读取超时")

		case res, ok := <-readCh:
			if !ok {
				// 读取通道关闭
				if e.adapter.NewState() == NewStateCompleted {
					logger.Info("StreamEngine", "-", "读取流已结束，命令执行已完成")
					return e.adapter.Results(), nil
				}
				logger.Warn("StreamEngine", "-", "读取流已结束，但命令尚未完成")
				e.adapter.MarkFailed("读取流意外关闭")
				return e.adapter.Results(), fmt.Errorf("读取流意外关闭")
			}

			// 关键修复：使用 res.data 而非共享缓冲区 buf
			data, err := res.data, res.err

			if len(data) > 0 {
				// 接收到数据，重置计时器
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				timer.Reset(currentTimeout)

				// 处理 chunk - 使用数据副本
				chunk := string(data)
				actions := e.processChunk(chunk)

				// 执行动作
				for _, action := range actions {
					if err := e.executeSessionAction(action, &currentTimeout, defaultTimeout, timer); err != nil {
						e.adapter.MarkFailed(err.Error())
						return e.adapter.Results(), err
					}
				}

				// 检查是否完成
				if e.adapter.NewState() == NewStateCompleted {
					return e.adapter.Results(), nil
				}

				// 单命令模式：检查当前命令是否完成
				if mode == ModeSingle && e.adapter.NewState() == NewStateReady {
					return e.adapter.Results(), nil
				}
			}

			if err != nil {
				if err == io.EOF {
					if e.adapter.NewState() == NewStateCompleted {
						return e.adapter.Results(), nil
					}
					e.adapter.MarkFailed("SSH 会话被远端断开")
					return e.adapter.Results(), fmt.Errorf("SSH 会话被远端断开")
				}
				e.adapter.MarkFailed(err.Error())
				return e.adapter.Results(), fmt.Errorf("读取错误: %w", err)
			}
		}

		time.Sleep(paginationInterval)
	}
}

// processChunk 处理 chunk，返回需要执行的新动作
func (e *StreamEngine) processChunk(chunk string) []SessionAction {
	actions := e.adapter.FeedSessionActions(chunk)

	// 将规范化后的行同步写入 Detail 日志
	if e.executor != nil && e.executor.logSession != nil {
		lines := e.adapter.GetNewCommittedLines()
		if len(lines) > 0 {
			if err := e.executor.logSession.Detail.WriteNormalizedLines(lines); err != nil {
				logger.Warn("StreamEngine", "-", "写入规范化日志失败: %v", err)
			}
		}
	}

	return actions
}

// executeSessionAction 执行统一的新动作模型
func (e *StreamEngine) executeSessionAction(action SessionAction, currentTimeout *time.Duration, defaultTimeout time.Duration, timer *time.Timer) error {
	switch act := action.(type) {
	case ActSendCommand:
		// 发送命令
		logger.Info("StreamEngine", "-", ">>> [发送命令]: %s", act.Command)

		// 写入命令日志
		if e.executor != nil {
			if err := e.executor.writeDetailCommand(act.Command); err != nil {
				logger.Warn("StreamEngine", "-", "写入命令日志失败: %v", err)
			}
		}

		// 发送命令
		if err := e.client.SendCommand(act.Command); err != nil {
			return fmt.Errorf("发送命令失败: %w", err)
		}

		// 设置超时
		if cmd := e.adapter.CurrentCommand(); cmd != nil && cmd.CustomTimeout > 0 {
			*currentTimeout = cmd.CustomTimeout
		} else {
			*currentTimeout = defaultTimeout
		}

		// 重置计时器
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		timer.Reset(*currentTimeout)

		// 发送事件
		if e.executor != nil && e.executor.EventBus != nil {
			cmd := e.adapter.CurrentCommand()
			if cmd != nil {
				e.executor.EventBus <- report.ExecutorEvent{
					IP:       e.executor.IP,
					Type:     report.EventDeviceCmd,
					Message:  cmd.RawCommand,
					CmdIndex: cmd.Index + 1,
					TotalCmd: e.adapter.TotalCommands(),
				}
			}
		}

	case ActSendPagerContinue:
		// 发送空格（分页）
		logger.Debug("StreamEngine", "-", "[自动翻页] 发送空格继续...")
		if err := e.client.SendRawBytes([]byte(" ")); err != nil {
			return fmt.Errorf("发送空格失败: %w", err)
		}

		// 刷新详细日志
		if e.executor != nil {
			_ = e.executor.flushDetailLog()
		}

	case ActSendWarmup:
		// 发送预热空行
		logger.Debug("StreamEngine", "-", "发送预热空行...")
		if err := e.client.SendCommand(""); err != nil {
			return fmt.Errorf("发送预热空行失败: %w", err)
		}

		// 刷新详细日志
		if e.executor != nil {
			_ = e.executor.flushDetailLog()
		}

	case ActRequestSuspendDecision:
		// 处理错误 - 调用 SuspendHandler 等待用户决策
		if act.ErrorContext == nil {
			logger.Warn("StreamEngine", "-", "ActRequestSuspendDecision 但无错误上下文")
			return nil
		}

		// 发送错误事件
		if e.executor != nil && e.executor.EventBus != nil {
			e.executor.EventBus <- report.ExecutorEvent{
				IP:       e.executor.IP,
				Type:     report.EventDeviceError,
				Message:  "执行错误: " + act.ErrorContext.Line,
				TotalCmd: e.adapter.TotalCommands(),
			}
		}

		// 调用挂起处理器等待用户决策
		if e.suspendHandler != nil {
			userAction := e.suspendHandler(context.Background(), e.executor.IP, act.ErrorContext.Line, act.ErrorContext.Cmd)
			switch userAction {
			case ActionAbort:
				// 用户选择中止
				if e.executor != nil && e.executor.EventBus != nil {
					e.executor.EventBus <- report.ExecutorEvent{
						IP:       e.executor.IP,
						Type:     report.EventDeviceAbort,
						Message:  "执行异常被手动中止",
						TotalCmd: e.adapter.TotalCommands(),
					}
				}
				followups := e.adapter.ResolveErrorActions(false)
				for _, next := range followups {
					if err := e.executeSessionAction(next, currentTimeout, defaultTimeout, timer); err != nil {
						return err
					}
				}
				return fmt.Errorf("设备 %s 的执行被用户手动中止", e.executor.IP)

			case ActionAbortTimeout:
				if e.executor != nil && e.executor.EventBus != nil {
					e.executor.EventBus <- report.ExecutorEvent{
						IP:       e.executor.IP,
						Type:     report.EventDeviceAbort,
						Message:  "执行异常挂起超时，自动中止",
						TotalCmd: e.adapter.TotalCommands(),
					}
				}
				followups := e.adapter.ReduceEvent(EvSuspendTimeout{
					CommandIndex: act.ErrorContext.CmdIndex,
					Reason:       "5分钟超时",
				})
				for _, next := range followups {
					if err := e.executeSessionAction(next, currentTimeout, defaultTimeout, timer); err != nil {
						return err
					}
				}
				return fmt.Errorf("设备 %s 的执行因挂起超时被自动中止", e.executor.IP)

			case ActionContinue:
				// 用户选择继续
				if e.executor != nil && e.executor.EventBus != nil {
					e.executor.EventBus <- report.ExecutorEvent{
						IP:       e.executor.IP,
						Type:     report.EventDeviceSkip,
						Message:  "放行异常并继续执行",
						TotalCmd: e.adapter.TotalCommands(),
					}
				}
				followups := e.adapter.ResolveErrorActions(true)
				for _, next := range followups {
					if err := e.executeSessionAction(next, currentTimeout, defaultTimeout, timer); err != nil {
						return err
					}
				}
				logger.Debug("StreamEngine", "-", "用户选择放行错误，继续执行")
			}
		} else {
			// 没有挂起处理器，默认中止
			followups := e.adapter.ResolveErrorActions(false)
			for _, next := range followups {
				if err := e.executeSessionAction(next, currentTimeout, defaultTimeout, timer); err != nil {
					return err
				}
			}
			return fmt.Errorf("设备 %s 执行错误: %s", e.executor.IP, act.ErrorContext.Line)
		}

	case ActAbortSession:
		// 中止任务
		return fmt.Errorf("任务被中止: %s", act.Reason)

	case ActResetReadTimeout:
		if act.Timeout > 0 {
			*currentTimeout = act.Timeout
		} else {
			*currentTimeout = defaultTimeout
		}
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		timer.Reset(*currentTimeout)

	case ActFlushDetailLog:
		if e.executor != nil {
			_ = e.executor.flushDetailLog()
		}

	case ActEmitCommandStart:
		if e.executor != nil && e.executor.EventBus != nil {
			e.executor.EventBus <- report.ExecutorEvent{
				IP:       e.executor.IP,
				Type:     report.EventDeviceCmd,
				Message:  act.Command,
				CmdIndex: act.Index + 1,
				TotalCmd: e.adapter.TotalCommands(),
			}
		}

	case ActEmitCommandDone:
		if e.executor != nil && e.executor.EventBus != nil {
			eventType := report.EventDeviceCmd
			message := fmt.Sprintf("命令完成 (耗时: %v)", act.Duration)
			if !act.Success {
				eventType = report.EventDeviceError
				message = fmt.Sprintf("命令执行失败 (耗时: %v)", act.Duration)
			}
			e.executor.EventBus <- report.ExecutorEvent{
				IP:       e.executor.IP,
				Type:     eventType,
				Message:  message,
				CmdIndex: act.Index + 1,
				TotalCmd: e.adapter.TotalCommands(),
			}
		}

	case ActEmitDeviceError:
		if e.executor != nil && e.executor.EventBus != nil {
			e.executor.EventBus <- report.ExecutorEvent{
				IP:       e.executor.IP,
				Type:     report.EventDeviceError,
				Message:  act.Message,
				TotalCmd: e.adapter.TotalCommands(),
			}
		}
	}

	return nil
}

// RunSingle 执行单条命令并返回结果
func (e *StreamEngine) RunSingle(ctx context.Context, cmd string, defaultTimeout time.Duration) (*CommandResult, error) {
	results, err := e.Run(ctx, ModeSingle, defaultTimeout)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("无命令结果")
	}
	return results[0], nil
}

// RunPlaybook 执行整个命令队列
func (e *StreamEngine) RunPlaybook(ctx context.Context, defaultTimeout time.Duration) ([]*CommandResult, error) {
	return e.Run(ctx, ModePlaybook, defaultTimeout)
}

// GetAdapter 获取适配器（用于调试和新架构访问）
func (e *StreamEngine) GetAdapter() *SessionAdapter {
	return e.adapter
}

// NormalizeOutput 规范化输出
// 用于测试中复用终端重放后的规范化输出。
func NormalizeOutput(chunk string, width int) string {
	replayer := terminal.NewReplayer(width)
	replayer.Process(chunk)

	lines := replayer.Lines()
	active := replayer.ActiveLine()

	result := strings.Join(lines, "\n")
	if active != "" {
		if result != "" {
			result += "\n"
		}
		result += active
	}

	return result
}
