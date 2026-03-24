package executor

import (
	"context"
	"errors"
	"fmt"
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
	ActionContinue     ErrorAction = iota // 忽略错误，继续发送下一条
	ActionAbort                           // 立即停止该设备的后续命令
	ActionAbortTimeout                    // 挂起超时后自动停止
)

// SuspendHandler 定义一个回调函数，当 Executor 遇到错误时，将其抛弃给主线程引擎询问用户决策
// 引擎会阻塞在此，直到用户通过命令行或其他界面选定动作后返回，从而只影响该设备的 Goroutine 局部挂起
type SuspendHandler func(ctx context.Context, ip string, deviceLog string, failedCmd string) ErrorAction

// ExecutorOptions 描述执行器统一构造入口的可选配置。
type ExecutorOptions struct {
	EventBus       chan report.ExecutorEvent
	SuspendHandler SuspendHandler
	LogSession     *report.DeviceLogSession
	Algorithms     *config.SSHAlgorithmSettings
	Vendor         string
	DeviceProfile  *config.DeviceProfile
}

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

	// 构造阶段注入的连接与日志配置。
	algorithms    *config.SSHAlgorithmSettings
	logSession    *report.DeviceLogSession
	deviceProfile *config.DeviceProfile

	// Terminal Replayer - 实验性集成
	// 用于将 SSH 字节流正确转换为规范化逻辑文本
	replayer *terminal.Replayer
}

// NewDeviceExecutor 初始化执行器
func NewDeviceExecutor(ip string, port int, user, pass string, opts ExecutorOptions) *DeviceExecutor {
	logger.Verbose("Executor", ip, "初始化 NewDeviceExecutor")
	profile := opts.DeviceProfile
	if profile == nil && strings.TrimSpace(opts.Vendor) != "" {
		profile = config.GetDeviceProfile(opts.Vendor)
	}
	return &DeviceExecutor{
		IP:            ip,
		Port:          port,
		Username:      user,
		Password:      pass,
		Matcher:       matcher.NewStreamMatcher(),
		EventBus:      opts.EventBus,
		OnSuspend:     opts.SuspendHandler,
		algorithms:    opts.Algorithms,
		logSession:    opts.LogSession,
		deviceProfile: profile,
		replayer:      terminal.NewReplayer(80), // 实验性：使用标准终端宽度 80
	}
}

// Connect 创建SSH长连接并初始化日志审计
func (e *DeviceExecutor) Connect(ctx context.Context, timeout time.Duration) error {
	logger.Debug("Executor", e.IP, "准备建立SSH连接 (Timeout: %v)", timeout)

	// 获取 PTY 配置
	var ptyConfig *sshutil.PTYConfig
	if e.deviceProfile != nil {
		ptyConfig = &sshutil.PTYConfig{
			TermType: e.deviceProfile.PTY.TermType,
			Width:    e.deviceProfile.PTY.Width,
			Height:   e.deviceProfile.PTY.Height,
			EchoMode: e.deviceProfile.PTY.EchoMode,
			ISpeed:   e.deviceProfile.PTY.ISpeed,
			OSpeed:   e.deviceProfile.PTY.OSpeed,
		}
	}

	hostKeyPolicy, knownHostsPath := config.ResolveSSHHostKeyPolicy()
	cfg := sshutil.Config{
		IP:             e.IP,
		Port:           e.Port,
		Username:       e.Username,
		Password:       e.Password,
		Timeout:        timeout,
		Algorithms:     e.algorithms,
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
//
// 注意：此方法现在委托给 executeInternal 统一实现
func (e *DeviceExecutor) ExecutePlaybook(ctx context.Context, commands []string, cmdTimeout time.Duration) error {
	logger.Debug("Executor", e.IP, "开始执行 Playbook (%d 条)", len(commands))
	if e.Client == nil {
		return fmt.Errorf("执行器未安全建连")
	}

	// 将命令列表转换为 PlannedCommand
	plannedCmds := make([]PlannedCommand, len(commands))
	for i, cmd := range commands {
		plannedCmds[i] = PlannedCommand{
			Key:             fmt.Sprintf("cmd_%d", i),
			Command:         cmd,
			Timeout:         cmdTimeout,
			ContinueOnError: true,
		}
	}

	plan := ExecutionPlan{
		Name:               "playbook",
		Commands:           plannedCmds,
		ContinueOnCmdError: true,
		Mode:               PlanModePlaybook,
	}

	report, err := e.executeInternal(ctx, plan, nil)
	if err != nil {
		return err
	}

	// 为了保持向后兼容，根据报告返回 error
	if !report.IsSuccess() {
		return fmt.Errorf("playbook 执行失败: %d/%d 个命令失败",
			report.FailureCount(), len(commands))
	}

	return nil
}

// ExecutePlan 执行统一执行计划
// 这是 discovery、engine 等批量路径唯一应使用的统一入口。
// 方案三实现：初始化命令与业务命令并入同一条 StreamEngine 状态机路径执行。
//
// 注意：此方法委托给 executeInternal 统一实现
func (e *DeviceExecutor) ExecutePlan(ctx context.Context, plan ExecutionPlan) (*ExecutionReport, error) {
	logger.Debug("Executor", e.IP, "开始执行 Plan: %s (%d 条命令)", plan.Name, len(plan.Commands))

	if e.Client == nil {
		return nil, fmt.Errorf("执行器未安全建连")
	}

	return e.executeInternal(ctx, plan, nil)
}

// executeInternal 统一的内部执行实现
// ExecutePlaybook 和 ExecutePlan 都委托给此方法。
// 方案三实现：初始化命令前置并与业务命令同路执行，消除 RunInit 旁路。
func (e *DeviceExecutor) executeInternal(
	ctx context.Context,
	plan ExecutionPlan,
	eventCallback func(event ExecutionEvent),
) (*ExecutionReport, error) {
	logger.Debug("Executor", e.IP, "开始执行内部计划: %s (%d 条命令)", plan.Name, len(plan.Commands))

	report := &ExecutionReport{
		PlanName:       plan.Name,
		Results:        make([]*CommandResult, 0, len(plan.Commands)),
		SessionHealthy: true,
		StartedAt:      time.Now(),
	}

	defer func() {
		report.FinishedAt = time.Now()
		report.ComputeStats()
		if err := e.flushDetailLog(); err != nil {
			logger.Warn("Executor", e.IP, "刷新详细日志失败: %v", err)
		}
	}()

	// 方案三：将初始化命令前置到统一命令队列，由同一状态机驱动。
	unifiedCommands, initCmdCount := e.buildUnifiedPlanCommands(plan.Commands)

	// 提取命令字符串列表和标识（统一队列）
	commandStrings := make([]string, len(unifiedCommands))
	commandKeys := make([]string, len(unifiedCommands))
	for i, cmd := range unifiedCommands {
		commandStrings[i] = cmd.Command
		commandKeys[i] = cmd.Key
	}

	// 创建 StreamEngine
	engine := NewStreamEngine(e, e.Client, commandStrings, 80)

	// 设置命令标识
	engine.adapter.SetCommandKeys(commandKeys)

	// 设置挂起处理器
	if e.OnSuspend != nil {
		engine.SetSuspendHandler(e.OnSuspend)
	}

	// 使用执行器的匹配器
	if e.Matcher != nil {
		engine.SetErrorMatcher(e.Matcher)
	}

	if initCmdCount > 0 {
		logger.Debug("Executor", e.IP, "统一执行模式：前置初始化命令 %d 条，业务命令 %d 条",
			initCmdCount, len(plan.Commands))
	} else {
		logger.Debug("Executor", e.IP, "统一执行模式：无前置初始化命令，业务命令 %d 条", len(plan.Commands))
	}

	// 关键修复：使用 RunPlaybook 一次性执行所有命令，而非循环调用 RunSingle
	// 这避免了结果错位和状态机推进冲突
	defaultTimeout := e.getDefaultTimeout(unifiedCommands)
	results, err := engine.RunPlaybook(ctx, defaultTimeout)

	// 方案三：剥离前置初始化结果，只对业务命令做回填与汇报。
	report.InitDuration = e.computeInitDuration(results, initCmdCount)
	bizResults := e.dropInitResults(results, initCmdCount)
	bizCommandKeys := e.dropInitKeys(commandKeys, initCmdCount)
	report.Results = e.processResultsWithKeys(bizResults, bizCommandKeys, plan.Commands)

	// 更新统计
	for _, result := range report.Results {
		if result != nil {
			report.TotalBytesRead += result.RawSize
			report.TotalLinesParsed += len(result.NormalizedLines)
		}
	}

	// 处理执行错误
	if err != nil {
		errorClass := classifyRunError(err)
		isFatal := e.isFatalError(err, plan)

		logger.Warn("Executor", e.IP, "执行过程中发生错误: class=%s, fatal=%v, err=%v",
			errorClass, isFatal, err)

		// 判断是否为致命错误
		if isFatal {
			report.FatalError = err
			report.SessionHealthy = false
			logger.Debug("Executor", e.IP, "错误被判定为致命错误: class=%s, abortOnTransport=%v, abortOnTimeout=%v, continueOnCmdError=%v",
				errorClass, plan.AbortOnTransportErr, plan.AbortOnCommandTimeout, plan.ContinueOnCmdError)
		} else {
			logger.Debug("Executor", e.IP, "错误被判定为非致命，继续执行: class=%s", errorClass)
		}
	}

	// 发送执行完成事件
	if eventCallback != nil {
		eventCallback(ExecutionEvent{
			Type:      EventComplete,
			Duration:  time.Since(report.StartedAt),
			Timestamp: time.Now(),
		})
	}

	logger.Debug("Executor", e.IP, "计划执行完成: success=%d, failed=%d, sessionHealthy=%v, 总耗时=%v",
		report.SuccessCount(), report.FailureCount(), report.SessionHealthy, time.Since(report.StartedAt))

	return report, nil
}

func (e *DeviceExecutor) buildUnifiedPlanCommands(commands []PlannedCommand) ([]PlannedCommand, int) {
	initCommands := e.buildInitCommands(commands)
	if len(initCommands) == 0 {
		merged := make([]PlannedCommand, len(commands))
		copy(merged, commands)
		return merged, 0
	}

	merged := make([]PlannedCommand, 0, len(initCommands)+len(commands))
	merged = append(merged, initCommands...)
	merged = append(merged, commands...)
	return merged, len(initCommands)
}

func (e *DeviceExecutor) buildInitCommands(commands []PlannedCommand) []PlannedCommand {
	if e.deviceProfile == nil {
		return nil
	}

	initTimeout := e.getInitTimeout(commands)
	if initTimeout <= 0 {
		initTimeout = 30 * time.Second
	}

	result := make([]PlannedCommand, 0)
	for i, cmd := range e.deviceProfile.Init.DisablePagerCommands {
		cmd = strings.TrimSpace(cmd)
		if cmd == "" {
			continue
		}
		result = append(result, PlannedCommand{
			Key:             fmt.Sprintf("__init_disable_pager_%d", i),
			Command:         cmd,
			Timeout:         initTimeout,
			ContinueOnError: false,
		})
	}
	for i, cmd := range e.deviceProfile.Init.ExtraCommands {
		cmd = strings.TrimSpace(cmd)
		if cmd == "" {
			continue
		}
		result = append(result, PlannedCommand{
			Key:             fmt.Sprintf("__init_extra_%d", i),
			Command:         cmd,
			Timeout:         initTimeout,
			ContinueOnError: false,
		})
	}

	return result
}

func (e *DeviceExecutor) computeInitDuration(results []*CommandResult, initCmdCount int) time.Duration {
	if initCmdCount <= 0 || len(results) == 0 {
		return 0
	}
	limit := initCmdCount
	if limit > len(results) {
		limit = len(results)
	}
	var total time.Duration
	for i := 0; i < limit; i++ {
		if results[i] != nil {
			total += results[i].Duration
		}
	}
	return total
}

func (e *DeviceExecutor) dropInitResults(results []*CommandResult, initCmdCount int) []*CommandResult {
	if initCmdCount <= 0 {
		return results
	}
	if len(results) <= initCmdCount {
		return []*CommandResult{}
	}
	trimmed := make([]*CommandResult, len(results)-initCmdCount)
	copy(trimmed, results[initCmdCount:])
	return trimmed
}

func (e *DeviceExecutor) dropInitKeys(keys []string, initCmdCount int) []string {
	if initCmdCount <= 0 {
		return keys
	}
	if len(keys) <= initCmdCount {
		return []string{}
	}
	trimmed := make([]string, len(keys)-initCmdCount)
	copy(trimmed, keys[initCmdCount:])
	return trimmed
}

// processResultsWithKeys 处理结果并回填 CommandKey
// 确保结果与计划命令一一对应
// P2修复：当 results==0 时按计划命令数补齐失败结果
func (e *DeviceExecutor) processResultsWithKeys(
	results []*CommandResult,
	commandKeys []string,
	plannedCmds []PlannedCommand,
) []*CommandResult {
	// P2修复：空结果时按计划命令数补齐失败结果
	if len(results) == 0 && len(plannedCmds) > 0 {
		logger.Warn("Executor", e.IP, "执行结果为空，按计划命令数(%d)补齐失败结果", len(plannedCmds))
		processed := make([]*CommandResult, 0, len(plannedCmds))
		for i, cmd := range plannedCmds {
			key := ""
			if i < len(commandKeys) {
				key = commandKeys[i]
			}
			processed = append(processed, &CommandResult{
				Index:        i,
				CommandKey:   key,
				Command:      cmd.Command,
				Success:      false,
				ErrorMessage: "missing result",
			})
		}
		return processed
	}

	if len(results) == 0 {
		logger.Warn("Executor", e.IP, "执行结果为空且无计划命令")
		return make([]*CommandResult, 0)
	}

	// P2修复：处理结果数量大于命令数量的情况（截断）
	if len(results) > len(plannedCmds) {
		logger.Warn("Executor", e.IP, "结果数量超过命令数量，截断多余结果: results=%d, commands=%d",
			len(results), len(plannedCmds))
		results = results[:len(plannedCmds)]
	}

	// 如果结果数量与命令数量不一致，记录警告
	if len(results) != len(plannedCmds) {
		logger.Warn("Executor", e.IP, "结果数量与命令数量不一致: results=%d, commands=%d",
			len(results), len(plannedCmds))
	}

	// 回填 CommandKey
	processed := make([]*CommandResult, 0, len(plannedCmds))
	for i, result := range results {
		if result == nil {
			// 补齐空结果
			result = &CommandResult{
				Index:        i,
				Success:      false,
				ErrorMessage: "missing result",
			}
		}
		// 回填 CommandKey（按索引对应）
		if i < len(commandKeys) && commandKeys[i] != "" {
			result.CommandKey = commandKeys[i]
		}
		// 回填 Command 文本（如果为空）
		if result.Command == "" && i < len(plannedCmds) {
			result.Command = plannedCmds[i].Command
		}
		processed = append(processed, result)
	}

	// 如果结果数量少于命令数量，补齐失败结果
	for i := len(results); i < len(plannedCmds); i++ {
		key := ""
		if i < len(commandKeys) {
			key = commandKeys[i]
		}
		logger.Warn("Executor", e.IP, "命令 %d (%s) 缺少结果，补齐失败记录", i, plannedCmds[i].Key)
		processed = append(processed, &CommandResult{
			Index:        i,
			CommandKey:   key,
			Command:      plannedCmds[i].Command,
			Success:      false,
			ErrorMessage: "missing result",
		})
	}

	return processed
}

// getDefaultTimeout 获取默认超时
func (e *DeviceExecutor) getDefaultTimeout(commands []PlannedCommand) time.Duration {
	if len(commands) > 0 && commands[0].Timeout > 0 {
		return commands[0].Timeout
	}
	return 30 * time.Second
}

// getInitTimeout 获取初始化超时
func (e *DeviceExecutor) getInitTimeout(commands []PlannedCommand) time.Duration {
	if len(commands) > 0 && commands[0].Timeout > 0 {
		return commands[0].Timeout
	}
	return 30 * time.Second
}

// ErrorClass 错误分类
type ErrorClass string

const (
	ErrorClassTransport     ErrorClass = "transport"      // 传输层错误（连接断开等）
	ErrorClassTimeout       ErrorClass = "timeout"        // 超时错误
	ErrorClassCommand       ErrorClass = "command"        // 命令执行错误（设备返回错误）
	ErrorClassContextCancel ErrorClass = "context_cancel" // 上下文取消
	ErrorClassUnknown       ErrorClass = "unknown"        // 未知错误
)

// classifyRunError 对执行错误进行分类
// P1修复：新增错误分类函数，支持策略语义
func classifyRunError(err error) ErrorClass {
	if err == nil {
		return ""
	}

	// 检查上下文取消
	if errors.Is(err, context.Canceled) {
		return ErrorClassContextCancel
	}

	// 检查超时错误
	if isTimeoutError(err) {
		return ErrorClassTimeout
	}

	// 检查传输层错误
	// P1修复：使用大小写不敏感匹配
	errStrLower := strings.ToLower(err.Error())
	transportKeywords := []string{
		"connection reset", "connection refused", "broken pipe",
		"network is unreachable", "no such host", "dial tcp",
		"eof", "ssh: handshake failed", "会话被远端断开",
	}
	for _, kw := range transportKeywords {
		if strings.Contains(errStrLower, kw) {
			return ErrorClassTransport
		}
	}

	// 检查命令错误（设备返回的错误提示）
	commandKeywords := []string{
		"invalid", "unknown command", "syntax error",
		"unrecognized", "not found", "失败", "错误",
	}
	for _, kw := range commandKeywords {
		if strings.Contains(errStrLower, kw) {
			return ErrorClassCommand
		}
	}

	return ErrorClassUnknown
}

// isFatalError 基于策略矩阵判断是否为致命错误
// P1修复：从"默认全fatal"改为"基于分类+策略矩阵"
func (e *DeviceExecutor) isFatalError(err error, plan ExecutionPlan) bool {
	if err == nil {
		return false
	}

	errorClass := classifyRunError(err)

	switch errorClass {
	case ErrorClassTransport:
		// 传输错误仅在 AbortOnTransportErr=true 时标记为 fatal
		return plan.AbortOnTransportErr

	case ErrorClassTimeout:
		// 超时错误仅在 AbortOnCommandTimeout=true 时标记为 fatal
		return plan.AbortOnCommandTimeout

	case ErrorClassContextCancel:
		// 上下文取消总是 fatal
		return true

	case ErrorClassCommand:
		// 命令错误遵循 ContinueOnCmdError
		// 如果 ContinueOnCmdError=true 则不 fatal，否则 fatal
		return !plan.ContinueOnCmdError

	default:
		// P1修复：unknown 错误回退到传输错误策略
		// 假设可能是未识别的传输错误，遵循 AbortOnTransportErr 策略
		logger.Warn("Executor", e.IP, "未识别错误类型，回退到传输错误策略(AbortOnTransportErr=%v): %v",
			plan.AbortOnTransportErr, err)
		return plan.AbortOnTransportErr
	}
}

// isTimeoutError 检查是否为超时错误
// 阶段C修复：优先使用标准错误检查，字符串匹配作为兜底
func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}

	// 优先检查标准错误类型
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	// 检查网络超时错误
	if netErr, ok := err.(interface{ Timeout() bool }); ok && netErr.Timeout() {
		return true
	}

	// 兜底：字符串匹配（保留但降低优先级）
	errStrLower := strings.ToLower(err.Error())
	if strings.Contains(errStrLower, "timeout") || strings.Contains(err.Error(), "超时") {
		logger.Verbose("Executor", "-", "通过字符串匹配检测到超时错误（建议使用标准错误类型）")
		return true
	}

	return false
}

// Close 断开所有的流和连接
func (e *DeviceExecutor) Close() {
	if e.Client != nil {
		e.Client.Close()
	}
}

// ExecuteCommandSync 同步执行单条命令并返回原始输出文本
// 内部使用 ExecutePlan 实现，保持与其他执行方法的一致性
func (e *DeviceExecutor) ExecuteCommandSync(ctx context.Context, cmd string, defaultTimeout time.Duration) (string, error) {
	plan := ExecutionPlan{
		Name: "single-cmd",
		Commands: []PlannedCommand{
			{Key: "cmd", Command: cmd, Timeout: defaultTimeout},
		},
	}
	report, err := e.ExecutePlan(ctx, plan)
	if err != nil {
		return "", err
	}
	if len(report.Results) > 0 && report.Results[0] != nil {
		return report.Results[0].RawText, nil
	}
	return "", fmt.Errorf("no result")
}

func (e *DeviceExecutor) rawSink() report.RawTranscriptSink {
	if e.logSession == nil {
		return nil
	}
	return e.logSession.RawSink()
}

func (e *DeviceExecutor) writeDetailCommand(command string) error {
	if e.logSession == nil {
		return nil
	}
	return e.logSession.WriteDetailCommand(command)
}

func (e *DeviceExecutor) writeDetailChunk(chunk string) error {
	if e.logSession == nil {
		return nil
	}
	return e.logSession.WriteDetailChunk(chunk)
}

func (e *DeviceExecutor) flushDetailLog() error {
	if e.logSession == nil {
		return nil
	}
	return e.logSession.FlushDetail()
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
