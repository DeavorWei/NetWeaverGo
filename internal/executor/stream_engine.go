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
// 重构后：使用 SessionAdapter 支持新旧架构切换
type StreamEngine struct {
	// adapter 会话适配器（支持新旧架构切换）
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

	// profile 设备画像（用于初始化）
	profile *config.DeviceProfile
}

// NewStreamEngine 创建新的流处理引擎
func NewStreamEngine(executor *DeviceExecutor, client *sshutil.SSHClient, commands []string, width int) *StreamEngine {
	m := matcher.NewStreamMatcher()
	adapter := NewSessionAdapter(width, commands, m)

	// 从运行时配置读取灰度开关
	if mgr := config.GetRuntimeManagerIfInitialized(); mgr != nil {
		cfg := mgr.GetConfig()
		adapter.SetUseNewArchitecture(cfg.Engine.UseNewSessionArchitecture)
		if cfg.Engine.UseNewSessionArchitecture {
			logger.Debug("StreamEngine", "-", "使用新会话架构 (Detector+Reducer+Driver)")
		}
	}

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

// SetProfile 设置设备画像
func (e *StreamEngine) SetProfile(profile *config.DeviceProfile) {
	e.profile = profile
}

// RunInit 执行初始化流程
// 在执行命令前调用，发送禁分页命令等
func (e *StreamEngine) RunInit(ctx context.Context, timeout time.Duration) error {
	if e.profile == nil {
		// 关键修复：无设备画像时也执行简化初始化，清理登录残留
		logger.Debug("StreamEngine", "-", "无设备画像，执行简化初始化...")
		if err := e.waitAndClearInitResidual(ctx, timeout); err != nil {
			logger.Warn("StreamEngine", "-", "简化初始化失败: %v（继续执行）", err)
			// 不阻断执行，继续尝试
		}
		return nil
	}

	// 创建初始化器
	initializer := NewInitializerWithMatcher(e.profile, e.matcher)

	// 执行初始化
	logger.Info("StreamEngine", "-", "开始执行初始化流程...")
	result := initializer.RunWithResult(ctx, e.client, e.adapter.machine)
	if !result.Success {
		return fmt.Errorf("初始化失败: %s", result.ErrorMessage)
	}

	// 清空初始化残留
	e.adapter.ClearInitResiduals()

	logger.Info("StreamEngine", "-", "初始化流程完成 (禁分页: %v, 耗时: %v)", result.PagerDisabled, result.Duration)
	return nil
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
			logger.Debug("StreamEngine", "-", "读取超时，当前状态: %s", e.adapter.State())

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
					TotalCmd: len(e.adapter.machine.queue),
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
							TotalCmd: len(e.adapter.machine.queue),
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
							TotalCmd: len(e.adapter.machine.queue),
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
				if e.adapter.State() == StateCompleted {
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
					actionData := e.adapter.GetActionData(action)
					if err := e.executeAction(actionData, &currentTimeout, defaultTimeout, timer); err != nil {
						e.adapter.MarkFailed(err.Error())
						return e.adapter.Results(), err
					}
				}

				// 检查是否完成
				if e.adapter.State() == StateCompleted {
					return e.adapter.Results(), nil
				}

				// 单命令模式：检查当前命令是否完成
				if mode == ModeSingle && e.adapter.State() == StateReady {
					return e.adapter.Results(), nil
				}
			}

			if err != nil {
				if err == io.EOF {
					if e.adapter.State() == StateCompleted {
						return e.adapter.Results(), nil
					}
					e.adapter.MarkFailed("SSH 会话被远端断开")
					return e.adapter.Results(), fmt.Errorf("SSH 会话被远端断开")
				}
				e.adapter.MarkFailed(err.Error())
				return e.adapter.Results(), fmt.Errorf("读取错误: %w", err)
			}
		}

		time.Sleep(manager.GetPaginationCheckInterval())
	}
}

// processChunk 处理 chunk，返回需要执行的动作
func (e *StreamEngine) processChunk(chunk string) []Action {
	// 喂给状态机（状态机内部使用 Replayer 处理 chunk）
	actions := e.adapter.Feed(chunk)

	// 将规范化后的行同步写入 Detail 日志
	if e.executor != nil && e.executor.LogSession != nil {
		lines := e.adapter.GetNewCommittedLines()
		if len(lines) > 0 {
			logger.Debug("StreamEngine", "-", "[修复调试] 准备写入 %d 行规范化行到 Detail 日志", len(lines))
			if err := e.executor.LogSession.Detail.WriteNormalizedLines(lines); err != nil {
				logger.Warn("StreamEngine", "-", "写入规范化日志失败: %v", err)
			} else {
				logger.Debug("StreamEngine", "-", "[修复调试] 成功写入 %d 行到 Detail 日志", len(lines))
			}
		}
	} else {
		logger.Debug("StreamEngine", "-", "[修复调试] 无法写入 Detail 日志: executor=%v, LogSession=%v", e.executor != nil, e.executor != nil && e.executor.LogSession != nil)
	}

	return actions
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
			cmd := e.adapter.CurrentCommand()
			if cmd != nil {
				e.executor.EventBus <- report.ExecutorEvent{
					IP:       e.executor.IP,
					Type:     report.EventDeviceCmd,
					Message:  cmd.RawCommand,
					CmdIndex: cmd.Index + 1,
					TotalCmd: len(e.adapter.machine.queue),
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
				TotalCmd: len(e.adapter.machine.queue),
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
						TotalCmd: len(e.adapter.machine.queue),
					}
				}
				e.adapter.ResolveError(false) // 中止
				return fmt.Errorf("设备 %s 的执行被用户手动中止", e.executor.IP)

			case ActionContinue:
				// 用户选择继续
				if e.executor != nil && e.executor.EventBus != nil {
					e.executor.EventBus <- report.ExecutorEvent{
						IP:       e.executor.IP,
						Type:     report.EventDeviceSkip,
						Message:  "放行异常并继续执行",
						TotalCmd: len(e.adapter.machine.queue),
					}
				}
				e.adapter.ResolveError(true) // 继续
				logger.Debug("StreamEngine", "-", "用户选择放行错误，继续执行")
			}
		} else {
			// 没有挂起处理器，默认中止
			e.adapter.ResolveError(false)
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
	return e.adapter.machine
}

// GetAdapter 获取适配器（用于调试和新架构访问）
func (e *StreamEngine) GetAdapter() *SessionAdapter {
	return e.adapter
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

// waitAndClearInitResidual 等待并清理初始化残留（无设备画像时使用）
// 关键修复：即使没有设备画像，也需要等待稳定提示符并清理登录欢迎信息
func (e *StreamEngine) waitAndClearInitResidual(ctx context.Context, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	buf := make([]byte, 4096)

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// 设置读取超时
		if err := e.client.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
			continue
		}

		n, err := e.client.Read(buf)
		if err != nil {
			if netErr, ok := err.(interface{ Timeout() bool }); ok && netErr.Timeout() {
				// 检查状态机是否已经进入 Ready 状态且 pendingLines 为空
				if e.adapter.State() == StateReady {
					// 关键修复：清空残留数据后再返回
					e.adapter.ClearInitResiduals()
					logger.Debug("StreamEngine", "-", "简化初始化完成，状态机已就绪")
					return nil
				}
				continue
			}
			continue
		}

		if n > 0 {
			chunk := string(buf[:n])
			// 喂给状态机
			actions := e.adapter.Feed(chunk)

			// 处理状态机返回的动作
			for _, action := range actions {
				switch action {
				case ActionSendWarmup:
					// 发送预热空行
					logger.Debug("StreamEngine", "-", "简化初始化发送预热空行...")
					if err := e.client.SendCommand(""); err != nil {
						logger.Warn("StreamEngine", "-", "发送预热空行失败: %v", err)
					}
					// 发送预热后，继续读取响应并驱动状态机
					// 设置短暂超时读取响应
					if err := e.client.SetReadDeadline(time.Now().Add(500 * time.Millisecond)); err == nil {
						if n2, err2 := e.client.Read(buf); err2 == nil && n2 > 0 {
							e.adapter.Feed(string(buf[:n2]))
						}
					}
				case ActionSendCommand:
					// 不应该在初始化阶段发送命令，忽略
					logger.Debug("StreamEngine", "-", "简化初始化忽略 ActionSendCommand")
				}
			}

			// 检查状态机是否已经进入 Ready 状态
			if e.adapter.State() == StateReady {
				// 关键修复：清空残留数据后再返回
				e.adapter.ClearInitResiduals()
				logger.Debug("StreamEngine", "-", "简化初始化完成，状态机已就绪")
				return nil
			}
		}
	}

	// 超时也清理残留，不阻断执行
	logger.Warn("StreamEngine", "-", "简化初始化超时，当前状态: %s", e.adapter.State())
	e.adapter.ClearInitResiduals()
	return fmt.Errorf("等待初始化提示符超时")
}
