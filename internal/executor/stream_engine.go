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
type StreamEngine struct {
	// machine 会话状态机
	machine *SessionMachine

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
	return &StreamEngine{
		machine:        NewSessionMachine(width, commands, m),
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
	e.machine.matcher = m
}

// Run 运行流处理引擎
// mode=playbook 时消费整队列
// mode=single 时只消费一条命令并返回单结果
func (e *StreamEngine) Run(ctx context.Context, mode RunMode, defaultTimeout time.Duration) ([]*CommandResult, error) {
	manager := config.GetRuntimeManager()
	buf := make([]byte, manager.GetBufferSize())
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
	type readResult struct {
		n   int
		err error
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
			select {
			case readCh <- readResult{n: n, err: err}:
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
			e.machine.MarkFailed("上下文取消")
			return e.machine.Results(), ctx.Err()

		case <-timer.C:
			// 超时处理
			logger.Debug("StreamEngine", "-", "读取超时，当前状态: %s", e.machine.State())

			// 获取当前命令信息
			failedCmd := ""
			cmdIndex := -1
			if e.machine.current != nil {
				failedCmd = e.machine.current.RawCommand
				cmdIndex = e.machine.current.Index
			}

			// 发送超时事件
			if e.executor != nil && e.executor.EventBus != nil {
				e.executor.EventBus <- report.ExecutorEvent{
					IP:       e.executor.IP,
					Type:     report.EventDeviceError,
					Message:  "Timeout Error: No prompt received within timeout",
					TotalCmd: len(e.machine.queue),
					CmdIndex: cmdIndex + 1,
				}
			}

			// 调用挂起处理器等待用户决策
			if e.suspendHandler != nil {
				userAction := e.suspendHandler(ctx, e.executor.IP, "Timeout Error: No prompt received within timeout", failedCmd)
				switch userAction {
				case ActionAbort:
					// 用户选择中止
					if e.executor != nil && e.executor.EventBus != nil {
						e.executor.EventBus <- report.ExecutorEvent{
							IP:       e.executor.IP,
							Type:     report.EventDeviceAbort,
							Message:  "因超时被手动中止",
							TotalCmd: len(e.machine.queue),
						}
					}
					e.machine.MarkFailed("读取超时，用户中止")
					return e.machine.Results(), fmt.Errorf("设备 %s 的执行因超时被用户中止", e.executor.IP)

				case ActionContinue:
					// 用户选择继续
					if e.executor != nil && e.executor.EventBus != nil {
						e.executor.EventBus <- report.ExecutorEvent{
							IP:       e.executor.IP,
							Type:     report.EventDeviceSkip,
							Message:  "放行超时并继续执行",
							TotalCmd: len(e.machine.queue),
						}
					}
					// 重置计时器继续执行
					timer.Reset(currentTimeout)
					logger.Debug("StreamEngine", "-", "用户选择放行超时，继续执行")
					continue
				}
			}

			e.machine.MarkFailed("读取超时")
			return e.machine.Results(), fmt.Errorf("读取超时")

		case res, ok := <-readCh:
			if !ok {
				// 读取通道关闭
				if e.machine.State() == StateCompleted {
					logger.Info("StreamEngine", "-", "读取流已结束，命令执行已完成")
					return e.machine.Results(), nil
				}
				logger.Warn("StreamEngine", "-", "读取流已结束，但命令尚未完成")
				e.machine.MarkFailed("读取流意外关闭")
				return e.machine.Results(), fmt.Errorf("读取流意外关闭")
			}

			n, err := res.n, res.err

			if n > 0 {
				// 接收到数据，重置计时器
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				timer.Reset(currentTimeout)

				// 处理 chunk
				chunk := string(buf[:n])
				actions := e.processChunk(chunk)

				// 执行动作
				for _, action := range actions {
					actionData := e.machine.GetActionData(action)
					if err := e.executeAction(actionData, &currentTimeout, defaultTimeout, timer); err != nil {
						e.machine.MarkFailed(err.Error())
						return e.machine.Results(), err
					}
				}

				// 检查是否完成
				if e.machine.State() == StateCompleted {
					return e.machine.Results(), nil
				}

				// 单命令模式：检查当前命令是否完成
				if mode == ModeSingle && e.machine.State() == StateReady {
					return e.machine.Results(), nil
				}
			}

			if err != nil {
				if err == io.EOF {
					if e.machine.State() == StateCompleted {
						return e.machine.Results(), nil
					}
					e.machine.MarkFailed("SSH 会话被远端断开")
					return e.machine.Results(), fmt.Errorf("SSH 会话被远端断开")
				}
				e.machine.MarkFailed(err.Error())
				return e.machine.Results(), fmt.Errorf("读取错误: %w", err)
			}
		}

		time.Sleep(manager.GetPaginationCheckInterval())
	}
}

// processChunk 处理 chunk，返回需要执行的动作
func (e *StreamEngine) processChunk(chunk string) []Action {
	// 写入详细日志（使用原有逻辑）
	if e.executor != nil {
		if err := e.executor.writeDetailChunk(chunk); err != nil {
			logger.Warn("StreamEngine", "-", "写入详细日志失败: %v", err)
		}
	}

	// 喂给状态机
	return e.machine.Feed(chunk)
}

// executeAction 执行动作
func (e *StreamEngine) executeAction(action ActionData, currentTimeout *time.Duration, defaultTimeout time.Duration, timer *time.Timer) error {
	switch action.Type {
	case ActionSendCommand:
		// 发送命令
		logger.Info("StreamEngine", "-", ">>> [发送命令]: %s", action.Command)

		// 写入命令日志
		if e.executor != nil {
			if err := e.executor.writeDetailCommand(action.Command); err != nil {
				logger.Warn("StreamEngine", "-", "写入命令日志失败: %v", err)
			}
		}

		// 发送命令
		if err := e.client.SendCommand(action.Command); err != nil {
			return fmt.Errorf("发送命令失败: %w", err)
		}

		// 设置超时
		if action.Timeout > 0 {
			*currentTimeout = action.Timeout
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
			cmd := e.machine.CurrentCommand()
			if cmd != nil {
				e.executor.EventBus <- report.ExecutorEvent{
					IP:       e.executor.IP,
					Type:     report.EventDeviceCmd,
					Message:  cmd.RawCommand,
					CmdIndex: cmd.Index + 1,
					TotalCmd: len(e.machine.queue),
				}
			}
		}

	case ActionSendSpace:
		// 发送空格（分页）
		logger.Debug("StreamEngine", "-", "[自动翻页] 发送空格继续...")
		if err := e.client.SendRawBytes([]byte(" ")); err != nil {
			return fmt.Errorf("发送空格失败: %w", err)
		}

		// 刷新详细日志
		if e.executor != nil {
			_ = e.executor.flushDetailLog()
		}

	case ActionSendWarmup:
		// 发送预热空行
		logger.Debug("StreamEngine", "-", "发送预热空行...")
		if err := e.client.SendCommand(""); err != nil {
			return fmt.Errorf("发送预热空行失败: %w", err)
		}

		// 刷新详细日志
		if e.executor != nil {
			_ = e.executor.flushDetailLog()
		}

	case ActionHandleError:
		// 处理错误 - 调用 SuspendHandler 等待用户决策
		if action.ErrorCtx == nil {
			logger.Warn("StreamEngine", "-", "ActionHandleError 但无错误上下文")
			return nil
		}

		// 发送错误事件
		if e.executor != nil && e.executor.EventBus != nil {
			e.executor.EventBus <- report.ExecutorEvent{
				IP:       e.executor.IP,
				Type:     report.EventDeviceError,
				Message:  "执行错误: " + action.ErrorCtx.Line,
				TotalCmd: len(e.machine.queue),
			}
		}

		// 调用挂起处理器等待用户决策
		if e.suspendHandler != nil {
			userAction := e.suspendHandler(context.Background(), e.executor.IP, action.ErrorCtx.Line, action.ErrorCtx.Cmd)
			switch userAction {
			case ActionAbort:
				// 用户选择中止
				if e.executor != nil && e.executor.EventBus != nil {
					e.executor.EventBus <- report.ExecutorEvent{
						IP:       e.executor.IP,
						Type:     report.EventDeviceAbort,
						Message:  "执行异常被手动中止",
						TotalCmd: len(e.machine.queue),
					}
				}
				e.machine.ResolveError(false) // 中止
				return fmt.Errorf("设备 %s 的执行被用户手动中止", e.executor.IP)

			case ActionContinue:
				// 用户选择继续
				if e.executor != nil && e.executor.EventBus != nil {
					e.executor.EventBus <- report.ExecutorEvent{
						IP:       e.executor.IP,
						Type:     report.EventDeviceSkip,
						Message:  "放行异常并继续执行",
						TotalCmd: len(e.machine.queue),
					}
				}
				e.machine.ResolveError(true) // 继续
				logger.Debug("StreamEngine", "-", "用户选择放行错误，继续执行")
			}
		} else {
			// 没有挂起处理器，默认中止
			e.machine.ResolveError(false)
			return fmt.Errorf("设备 %s 执行错误: %s", e.executor.IP, action.ErrorCtx.Line)
		}

	case ActionAbortTask:
		// 中止任务
		return fmt.Errorf("任务被中止")

	case ActionSkipError:
		// 跳过错误继续执行
		logger.Debug("StreamEngine", "-", "跳过错误继续执行")
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

// GetMachine 获取状态机（用于调试）
func (e *StreamEngine) GetMachine() *SessionMachine {
	return e.machine
}

// NormalizeOutput 规范化输出
// 用于对比验证新旧实现的输出一致性
func NormalizeOutput(chunk string, width int) string {
	replayer := NewSessionMachine(width, nil, matcher.NewStreamMatcher())
	replayer.Feed(chunk)

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
