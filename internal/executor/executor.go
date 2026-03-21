package executor

import (
	"context"
	"fmt"
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

// ExecuteCommandSync 同步执行单条命令并返回原始输出文本
// 这是 ExecuteCommandSyncWithResult 的兼容性包装器，内部使用 StreamEngine 实现
// 保持返回 string 类型以向后兼容现有调用方
func (e *DeviceExecutor) ExecuteCommandSync(ctx context.Context, cmd string, defaultTimeout time.Duration) (string, error) {
	result, err := e.ExecuteCommandSyncWithResult(ctx, cmd, defaultTimeout)
	if err != nil {
		return "", err
	}
	// 返回原始文本以保持向后兼容
	return result.RawText, nil
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
