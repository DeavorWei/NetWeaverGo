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

// ErrorAction 描述了由引擎决定的继续行径
type ErrorAction int

const (
	ActionContinue ErrorAction = iota // 忽略错误，继续发送下一条
	ActionAbort                       // 立即停止该设备的后续命令
)

// SuspendHandler 定义一个回调函数，当 Executor 遇到错误时，将其抛弃给主线程引擎询问用户决策
// 引擎会阻塞在此，直到用户通过命令行或其他界面选定动作后返回，从而只影响该设备的 Goroutine 局部挂起
type SuspendHandler func(ctx context.Context, ip string, deviceLog string, failedCmd string) ErrorAction

// DeviceExecutor 封装特定设备的 SSH 数据流及命令步进下发生命周期
type DeviceExecutor struct {
	IP       string
	Port     int
	Username string
	Password string

	Matcher *matcher.StreamMatcher
	Client  *sshutil.SSHClient

	EventBus  chan report.ExecutorEvent
	OnSuspend SuspendHandler

	// SSH 算法配置
	Algorithms *config.SSHAlgorithmSettings
	LogSession *report.DeviceLogSession

	// Terminal Replayer - 实验性集成
	// 用于将 SSH 字节流正确转换为规范化逻辑文本
	replayer *terminal.Replayer
}

// NewDeviceExecutor 初始化执行器
func NewDeviceExecutor(ip string, port int, user, pass string, eb chan report.ExecutorEvent, onSuspend SuspendHandler) *DeviceExecutor {
	logger.Verbose("Executor", ip, "初始化 NewDeviceExecutor")
	return &DeviceExecutor{
		IP:        ip,
		Port:      port,
		Username:  user,
		Password:  pass,
		Matcher:   matcher.NewStreamMatcher(),
		EventBus:  eb,
		OnSuspend: onSuspend,
		replayer:  terminal.NewReplayer(80), // 实验性：使用标准终端宽度 80
	}
}

// SetAlgorithms 设置 SSH 算法配置
func (e *DeviceExecutor) SetAlgorithms(algorithms *config.SSHAlgorithmSettings) {
	e.Algorithms = algorithms
}

// SetLogSession 设置设备日志会话。
func (e *DeviceExecutor) SetLogSession(session *report.DeviceLogSession) {
	e.LogSession = session
}

// Connect 创建SSH长连接并初始化日志审计
func (e *DeviceExecutor) Connect(ctx context.Context, timeout time.Duration) error {
	return e.ConnectWithProfile(ctx, timeout, nil)
}

// ConnectWithProfile 创建SSH长连接，支持设备画像配置
func (e *DeviceExecutor) ConnectWithProfile(ctx context.Context, timeout time.Duration, profile *config.DeviceProfile) error {
	logger.Debug("Executor", e.IP, "准备建立SSH连接 (Timeout: %v)", timeout)

	// 获取 PTY 配置
	var ptyConfig *sshutil.PTYConfig
	if profile != nil {
		ptyConfig = &sshutil.PTYConfig{
			TermType: profile.PTY.TermType,
			Width:    profile.PTY.Width,
			Height:   profile.PTY.Height,
			EchoMode: profile.PTY.EchoMode,
			ISpeed:   profile.PTY.ISpeed,
			OSpeed:   profile.PTY.OSpeed,
		}
	}

	hostKeyPolicy, knownHostsPath := config.ResolveSSHHostKeyPolicy()
	cfg := sshutil.Config{
		IP:             e.IP,
		Port:           e.Port,
		Username:       e.Username,
		Password:       e.Password,
		Timeout:        timeout,
		Algorithms:     e.Algorithms,
		HostKeyPolicy:  hostKeyPolicy,
		KnownHostsPath: knownHostsPath,
		RawSink:        e.rawSink(),
		PTY:            ptyConfig,
	}

	client, err := sshutil.NewSSHClient(ctx, cfg)
	if err != nil {
		// 使用统一错误处理
		execErr := NewError(e.IP).
			WithStage(StageConnect).
			WithType(ClassifyError(err)).
			WithError(err).
			Build()
		handler := NewErrorHandler()
		handler.Handle(ctx, execErr)
		return execErr
	}
	e.Client = client
	logger.Debug("Executor", e.IP, "连接初始化成功")

	return nil
}

// ExecutePlaybook 核心引擎方法：对该设备步进发送命令队列，并支持局部阻塞等待（配合 SuspendHandler）
// 使用 StreamEngine 作为统一执行内核（Phase 2 设计要求）
func (e *DeviceExecutor) ExecutePlaybook(ctx context.Context, commands []string, cmdTimeout time.Duration) error {
	logger.Debug("Executor", e.IP, "开始执行 Playbook (%d 条)", len(commands))
	if e.Client == nil {
		return fmt.Errorf("执行器未安全建连")
	}
	defer func() {
		if err := e.flushDetailLog(); err != nil {
			logger.Warn("Executor", e.IP, "刷新详细日志失败: %v", err)
		}
	}()

	// 创建 StreamEngine
	engine := NewStreamEngine(e, e.Client, commands, 80)

	// 设置挂起处理器
	if e.OnSuspend != nil {
		engine.SetSuspendHandler(e.OnSuspend)
	}

	// 使用执行器的匹配器（已配置设备画像）
	if e.Matcher != nil {
		engine.SetErrorMatcher(e.Matcher)
	}

	// 执行 Playbook
	_, err := engine.RunPlaybook(ctx, cmdTimeout)
	return err
}

// Close 断开所有的流和连接
func (e *DeviceExecutor) Close() {
	if e.Client != nil {
		e.Client.Close()
	}
}

// ExecuteCommandSync 同步执行单条命令并读取其输出
// 直到找到下一个提示符。这绕过了 Playbook 队列系统，用于
// 像 `display startup` 这样的查询命令。
func (e *DeviceExecutor) ExecuteCommandSync(ctx context.Context, cmd string, defaultTimeout time.Duration) (string, error) {
	logger.Debug("Executor", e.IP, "进入同步交互模式: %s", cmd)
	if e.Client == nil {
		return "", fmt.Errorf("执行器未安全建连")
	}
	defer func() {
		if err := e.flushDetailLog(); err != nil {
			logger.Warn("Executor", e.IP, "刷新同步详细日志失败: %v", err)
		}
	}()

	manager := config.GetRuntimeManager()
	buf := make([]byte, manager.GetBufferSize())
	outReader := e.Client.Stdout

	var outputBuffer strings.Builder
	var streamBuffer string

	cmdToSend := cmd
	customTimeout := defaultTimeout

	// 解析内联行尾注释以获取特殊命令超时设定
	if idx := strings.Index(cmd, "// nw-timeout="); idx != -1 {
		cmdToSend = strings.TrimSpace(cmd[:idx])
		timeoutStr := strings.TrimSpace(cmd[idx+len("// nw-timeout="):])
		if pd, err := time.ParseDuration(timeoutStr); err == nil {
			customTimeout = pd
			logger.Info("Executor", e.IP, "检测到同步交互命令超时自定义控制: %s -> %v", cmdToSend, customTimeout)
		} else {
			logger.Warn("Executor", e.IP, "自定义超时时间格式无效: %s (期望格式如 30s, 1m30s), 使用默认值", timeoutStr)
		}
	}

	logger.Info("Executor", e.IP, ">>> [同步发送命令]: %s", cmdToSend)
	if err := e.writeDetailCommand(cmdToSend); err != nil {
		logger.Warn("Executor", e.IP, "写入同步命令日志失败: %v", err)
	}
	if err := e.Client.SendCommand(cmdToSend); err != nil {
		return "", fmt.Errorf("发送命令失败: %w", err)
	}

	timer := time.NewTimer(customTimeout)
	defer timer.Stop()

	type readResult struct {
		n   int
		err error
	}
	readCh := make(chan readResult, 1)

	go func() {
		defer close(readCh)
		// 添加 panic recover 防止 goroutine 泄漏
		defer func() {
			if r := recover(); r != nil {
				logger.Warn("Executor", e.IP, "同步读取协程 panic 已恢复: %v", r)
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

	// 读取循环
	for {
		select {
		case <-ctx.Done():
			return outputBuffer.String(), ctx.Err()
		case <-timer.C:
			return outputBuffer.String(), fmt.Errorf("同步命令执行超时: %s", cmd)
		case res, ok := <-readCh:
			if !ok {
				return outputBuffer.String(), fmt.Errorf("SSH 流已关闭")
			}
			n, err := res.n, res.err

			if n > 0 {
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				timer.Reset(customTimeout)

				chunk := string(buf[:n])
				if err := e.writeDetailChunk(chunk); err != nil {
					logger.Warn("Executor", e.IP, "写入同步详细日志失败: %v", err)
				}

				// 使用 terminal.Replayer 处理 chunk（与 ExecutePlaybook 保持一致）
				e.processWithReplayer(chunk)

				outputBuffer.WriteString(chunk) // 追加以作为完整的返回值
				streamBuffer += chunk

				// 查找提示符以完成执行
				lines := strings.Split(streamBuffer, "\n")

				// Optional enhancement: During sync execution, verify if errors are warned vs critical.
				for i, line := range lines {
					if i < len(lines)-1 {
						if matched, rule := e.Matcher.MatchErrorRule(line); matched {
							if rule.Severity == matcher.SeverityWarning {
								logger.Warn("Executor", e.IP, "[同步命令警告] %s: %s", rule.Name, rule.Message)
							} else {
								// Sync execution typically expects pure strings to be parsed back, but a truly critical error warrants early exit
								return outputBuffer.String(), fmt.Errorf("同步指令遇到 Critical 异常: %s", line)
							}
						}
					}
				}

				// 使用 terminal.Replayer 产出的逻辑行进行 prompt/pager 判断
				// 这是 Phase 2 设计要求的正确实现方式
				activeLine := e.replayer.ActiveLine()
				replayerLines := e.replayer.Lines()

				// 获取最后一个非空逻辑行
				var lastNonEmptyLine string
				for i := len(replayerLines) - 1; i >= 0; i-- {
					if trimmed := strings.TrimSpace(replayerLines[i]); trimmed != "" {
						lastNonEmptyLine = replayerLines[i]
						break
					}
				}

				// 对待同步交互，同样可能遇到分页符卡死
				// 基于逻辑行判断 pager（不再使用原始 lastSegment）
				if e.Matcher.IsPaginationPrompt(activeLine) || (lastNonEmptyLine != "" && e.Matcher.IsPaginationPrompt(lastNonEmptyLine)) {
					logger.Debug("Executor", e.IP, "[同步自动翻页] 截获终端分页拦截符(More)，自动下发空格放行...")
					e.Client.SendRawBytes([]byte(" "))
					streamBuffer = "" // 清除已匹配的分页符避免残留污染
					continue
				}

				// 基于逻辑行判断 prompt（不再使用原始 lastSegment）
				if e.Matcher.IsPrompt(activeLine) || (lastNonEmptyLine != "" && e.Matcher.IsPrompt(lastNonEmptyLine)) {
					// 找到提示符，命令执行完成
					_ = e.flushDetailLog()
					return outputBuffer.String(), nil
				}

				// 保留 streamBuffer 的基本管理逻辑（用于错误规则匹配）
				lastSegment := lines[len(lines)-1]
				streamBuffer = lastSegment
			}

			if err != nil {
				if err == io.EOF {
					return outputBuffer.String(), fmt.Errorf("SSH 会话已被远端安全断开")
				}
				return outputBuffer.String(), fmt.Errorf("读取SSH流时发生错误: %w", err)
			}
		}

		time.Sleep(manager.GetPaginationCheckInterval()) // 减少 CPU 忙等待
	}
}

// ExecuteCommandSyncWithResult 执行单条命令并返回完整的 CommandResult
// 这是 ExecuteCommandSync 的增强版本，返回规范化后的输出
func (e *DeviceExecutor) ExecuteCommandSyncWithResult(ctx context.Context, cmd string, timeout time.Duration) (*CommandResult, error) {
	if e.Client == nil {
		return nil, fmt.Errorf("执行器未安全建连")
	}

	// 使用 StreamEngine 执行
	engine := NewStreamEngine(e, e.Client, []string{cmd}, 80)
	result, err := engine.RunSingle(ctx, cmd, timeout)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (e *DeviceExecutor) rawSink() report.RawTranscriptSink {
	if e.LogSession == nil {
		return nil
	}
	return e.LogSession.RawSink()
}

func (e *DeviceExecutor) writeDetailCommand(command string) error {
	if e.LogSession == nil {
		return nil
	}
	return e.LogSession.WriteDetailCommand(command)
}

func (e *DeviceExecutor) writeDetailChunk(chunk string) error {
	if e.LogSession == nil {
		return nil
	}
	return e.LogSession.WriteDetailChunk(chunk)
}

func (e *DeviceExecutor) flushDetailLog() error {
	if e.LogSession == nil {
		return nil
	}
	return e.LogSession.FlushDetail()
}

// processWithReplayer 实验性方法：使用 terminal.Replayer 处理原始 chunk
// 第一版仅用于对比验证，不影响原有逻辑
func (e *DeviceExecutor) processWithReplayer(chunk string) {
	if e.replayer == nil {
		return
	}

	// 处理 chunk，获取规范化事件
	events := e.replayer.Process(chunk)

	// 记录已提交的行
	for _, event := range events {
		if event.Type == terminal.EventLineCommitted {
			logger.Verbose("Executor", e.IP, "[Replayer] 提交行: %q", event.Line)
		}
	}

	// 记录未支持的 ANSI 序列计数（用于调试）
	if count := e.replayer.UnknownCount(); count > 0 {
		logger.Verbose("Executor", e.IP, "[Replayer] 未支持的 ANSI 序列计数: %d", count)
	}
}

// GetReplayerLines 获取 Replayer 已提交的所有行（用于调试/测试）
func (e *DeviceExecutor) GetReplayerLines() []string {
	if e.replayer == nil {
		return nil
	}
	return e.replayer.Lines()
}

// GetReplayerActiveLine 获取 Replayer 当前活动行（用于调试/测试）
func (e *DeviceExecutor) GetReplayerActiveLine() string {
	if e.replayer == nil {
		return ""
	}
	return e.replayer.ActiveLine()
}
