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
	logger.Debug("Executor", e.IP, "准备建立SSH连接 (Timeout: %v)", timeout)
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

	manager := config.GetRuntimeManager()
	buf := make([]byte, manager.GetBufferSize())
	outReader := e.Client.Stdout
	errReader := e.Client.Stderr

	// 丢弃并记录 stderr（因为 TeeReader 已经挂在流文件里了）
	go func() {
		defer func() {
			// 防止 panic 导致 goroutine 泄漏
			if r := recover(); r != nil {
				logger.Warn("Executor", e.IP, "stderr copier panic recovered: %v", r)
			}
		}()
		_, _ = io.Copy(io.Discard, errReader)
	}()

	currentCmdIndex := -1 // 初始化状态: -1 表示还在探测第一个提示符
	lastSentCmdIndex := -1
	var streamBuffer string
	completedAllCommands := false

	// 等待命令提示符的超时时间
	timeoutDuration := cmdTimeout
	timer := time.NewTimer(timeoutDuration)
	defer timer.Stop()

	type readResult struct {
		n   int
		err error
	}
	readCh := make(chan readResult, 1)

	// 后台运行读操作，以便主流程能响应 timeout 和 ctx.Done()
	go func() {
		defer close(readCh)
		// 添加 panic recover 防止 goroutine 泄漏
		defer func() {
			if r := recover(); r != nil {
				logger.Warn("Executor", e.IP, "reader goroutine panic recovered: %v", r)
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

	resolveFailedCmd := func() string {
		if lastSentCmdIndex >= 0 && lastSentCmdIndex < len(commands) {
			return commands[lastSentCmdIndex]
		}
		if currentCmdIndex >= 0 && currentCmdIndex < len(commands) {
			return commands[currentCmdIndex]
		}
		return ""
	}

	readDelay := manager.GetPaginationCheckInterval()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			logger.Debug("Executor", e.IP, "读取提示符超时 (index=%d)", currentCmdIndex)
			// Timeout triggered
			failedCmd := resolveFailedCmd()

			if e.EventBus != nil {
				e.EventBus <- report.ExecutorEvent{IP: e.IP, Type: report.EventDeviceError, Message: "Waiting for prompt Timeout (30s)", TotalCmd: len(commands), CmdIndex: currentCmdIndex}
			}

			// 汇报给拦截器
			action := e.OnSuspend(ctx, e.IP, "Timeout Error: No prompt received within 30 seconds", failedCmd)
			switch action {
			case ActionAbort:
				if e.EventBus != nil {
					e.EventBus <- report.ExecutorEvent{IP: e.IP, Type: report.EventDeviceAbort, Message: "因超时被手动中止", TotalCmd: len(commands)}
				}
				return fmt.Errorf("设备 %s 的执行因超时被用户中止", e.IP)
			case ActionContinue:
				if e.EventBus != nil {
					e.EventBus <- report.ExecutorEvent{IP: e.IP, Type: report.EventDeviceSkip, Message: "放行超时并继续执行", TotalCmd: len(commands)}
				}
				// 不再推进命令索引，避免误跳过下一条命令
				streamBuffer = ""
				timer.Reset(timeoutDuration)
			}
		case res, ok := <-readCh:
			if !ok {
				if completedAllCommands {
					logger.Info("Executor", e.IP, "读取流已结束，命令执行已完成。")
					logger.Verbose("Executor", e.IP, "读取流 readCh 已关闭（完成态）")
					return nil
				}
				logger.Warn("Executor", e.IP, "读取流已结束，但命令尚未确认全部完成（index=%d）", currentCmdIndex)
				return fmt.Errorf("设备 %s 执行未完成即终止（readCh closed, index=%d/%d）", e.IP, currentCmdIndex, len(commands))
			}

			n, err := res.n, res.err

			if n > 0 {
				// 接收到数据，重置空闲超时计时器
				if !timer.Stop() {
					// 如果计时器已触发但尚未消费，则排空通道
					select {
					case <-timer.C:
					default:
					}
				}
				timer.Reset(timeoutDuration)

				chunk := string(buf[:n])
				if err := e.writeDetailChunk(chunk); err != nil {
					logger.Warn("Executor", e.IP, "写入详细日志失败: %v", err)
				}
				streamBuffer += chunk
				logger.Verbose("Executor", e.IP, "Received chunk (len=%d) | streamBuffer_len=%d", n, len(streamBuffer))

				lines := strings.Split(streamBuffer, "\n")
				// 检查最后一行以外的完整回显行，查找 error 关键字
				for i, line := range lines {
					if i < len(lines)-1 {
						// 仅在命令执行阶段匹配错误规则，避免把登录横幅等初始化输出误判成命令告警/错误。
						if currentCmdIndex < 0 {
							continue
						}
						if matched, rule := e.Matcher.MatchErrorRule(line); matched {
							failedCmd := resolveFailedCmd()

							if rule.Severity == matcher.SeverityWarning {
								// 仅抛出警告事件并放行
								if e.EventBus != nil {
									e.EventBus <- report.ExecutorEvent{IP: e.IP, Type: report.EventDeviceSkip, Message: "警告: " + rule.Message + " (" + line + ")", TotalCmd: len(commands)}
								}
								logger.Warn("Executor", e.IP, "[告警放行] %s: %s", rule.Name, rule.Message)
								continue
							}

							// 如果是严重错误 (Critical)，则触发原始告警和阻断
							if e.EventBus != nil {
								e.EventBus <- report.ExecutorEvent{IP: e.IP, Type: report.EventDeviceError, Message: "执行错误: " + line, TotalCmd: len(commands)}
							}

							// 触发外部回调执行暂停，将由外部引擎的通道控制该函数返回，形成单设备挂起效果
							action := e.OnSuspend(ctx, e.IP, line, failedCmd)
							switch action {
							case ActionAbort:
								if e.EventBus != nil {
									e.EventBus <- report.ExecutorEvent{IP: e.IP, Type: report.EventDeviceAbort, Message: "执行异常被手动中止", TotalCmd: len(commands)}
								}
								return fmt.Errorf("设备 %s 的执行被用户手动中止", e.IP)
							case ActionContinue:
								if e.EventBus != nil {
									e.EventBus <- report.ExecutorEvent{IP: e.IP, Type: report.EventDeviceSkip, Message: "放行异常并继续执行", TotalCmd: len(commands)}
								}
								// 命令索引已经在发送时推进，此处只清理状态
								streamBuffer = ""
							}
							// 清空缓冲区，避免二次重复报错
							streamBuffer = ""
							break
						}
					}
				}

				// 在裁剪到 lastSegment 前先判定 prompt，避免 "prompt+\r\n" 被裁成空串导致漏判。
				lastSegment := lines[len(lines)-1]
				lastNonEmptySegment := ""
				for i := len(lines) - 1; i >= 0; i-- {
					segment := strings.TrimSpace(lines[i])
					if segment != "" {
						lastNonEmptySegment = segment
						break
					}
				}

				promptDetected := e.Matcher.IsPrompt(streamBuffer)
				if !promptDetected && lastNonEmptySegment != "" {
					promptDetected = e.Matcher.IsPrompt(lastNonEmptySegment)
				}

				// 将没有换行符的最后一部分留到 Buffer 中进行下一轮累积
				streamBuffer = lastSegment

				// 检测是否由于回显过长触发了终端分页符（如 ---- More ----）
				if e.Matcher.IsPaginationPrompt(streamBuffer) || (lastNonEmptySegment != "" && e.Matcher.IsPaginationPrompt(lastNonEmptySegment)) {
					logger.Debug("Executor", e.IP, "[自动翻页] 截获终端分页拦截符(More)，自动下发空格放行...")
					e.Client.SendRawBytes([]byte(" ")) // 向 SSH 隧道发送一个纯净空格
					streamBuffer = ""                  // 清除已匹配的分页符避免残留污染
					_ = e.flushDetailLog()
					continue
				}

				// 首个提示符仅表示登录完成，不代表交互式 shell 已完全稳定。
				// 先发送一个空回车做终端预热，等下一个提示符回来后再正式下发业务命令。
				if currentCmdIndex == -1 && promptDetected {
					logger.Debug("Executor", e.IP, "等到首个提示符，发送预热空行稳定终端")
					currentCmdIndex = -2
					_ = e.flushDetailLog()
					if err := e.Client.SendCommand(""); err != nil {
						return fmt.Errorf("发送预热空行失败: %w", err)
					}
					if !timer.Stop() {
						select {
						case <-timer.C:
						default:
						}
					}
					timer.Reset(timeoutDuration)
					logger.Verbose("Executor", e.IP, "预热空行已发送，清空 streamBuffer 等待稳定提示符返回")
					streamBuffer = ""
					continue
				}

				if currentCmdIndex == -2 {
					if promptDetected {
						logger.Debug("Executor", e.IP, "预热完成，进入命令下发阶段")
						currentCmdIndex = 0
					} else {
						logger.Verbose("Executor", e.IP, "预热空行已发出，等待稳定提示符返回")
					}
				}

				// 发送队列中的命令
				if currentCmdIndex >= 0 && currentCmdIndex < len(commands) {
					if promptDetected {
						rawCmd := commands[currentCmdIndex]
						cmdToSend := rawCmd
						customDelay := timeoutDuration

						// 解析内联行尾注释以获取特殊命令超时设定
						if idx := strings.Index(rawCmd, "// nw-timeout="); idx != -1 {
							cmdToSend = strings.TrimSpace(rawCmd[:idx])
							timeoutStr := strings.TrimSpace(rawCmd[idx+len("// nw-timeout="):])
							if pd, err := time.ParseDuration(timeoutStr); err == nil {
								customDelay = pd
								logger.Debug("Executor", e.IP, "命令拥有自定超时 %v => %s", customDelay, cmdToSend)
								logger.Info("Executor", e.IP, "=== 检测到自定义长效命令超时控制 ===: %s -> %v", cmdToSend, customDelay)
							}
						}

						if e.EventBus != nil {
							e.EventBus <- report.ExecutorEvent{IP: e.IP, Type: report.EventDeviceCmd, Message: rawCmd, CmdIndex: currentCmdIndex + 1, TotalCmd: len(commands)}
						}
						// logger.Info("[%s] >>> [发送命令]: %s", e.IP, cmd) (Moved to GUI)
						if err := e.writeDetailCommand(cmdToSend); err != nil {
							logger.Warn("Executor", e.IP, "写入命令日志失败: %v", err)
						}
						lastSentCmdIndex = currentCmdIndex
						e.Client.SendCommand(cmdToSend)

						// 重置自定义计时器并清空 Buffer
						if !timer.Stop() {
							select {
							case <-timer.C:
							default:
							}
						}
						timer.Reset(customDelay)
						logger.Verbose("Executor", e.IP, "已执行发送动作，将 streamBuffer 人为清空防止污染。原长度=%d", len(streamBuffer))
						streamBuffer = "" // 发送命令后清空当前 Buffer，防止将上一步的提示符混到了接下来
						currentCmdIndex++
					} else {
						logger.Verbose("Executor", e.IP, "currentCmd=%d，还在等待匹配 Prompt, 当前 buff 末尾：%s", currentCmdIndex, streamBuffer)
					}
				} else if currentCmdIndex >= len(commands) {
					// 任务完成，判断最后一条命令结果是否已回显出提示符
					if promptDetected {
						logger.Debug("Executor", e.IP, "命令全部下发完成")
						// logger.Info("[%s] ==== [执行完成] 所有命令已下发完毕 ====", e.IP)
						completedAllCommands = true
						_ = e.flushDetailLog()
						return nil
					}
				}
			}

			if err != nil {
				if err == io.EOF {
					if completedAllCommands {
						logger.Info("Executor", e.IP, "SSH 会话已被远端安全断开（完成态）。")
						return nil
					}
					logger.Warn("Executor", e.IP, "SSH 会话在命令完成前被远端断开（index=%d）", currentCmdIndex)
					return fmt.Errorf("设备 %s 在命令完成前断开连接（EOF, index=%d/%d）", e.IP, currentCmdIndex, len(commands))
				}
				return fmt.Errorf("读取SSH流时发生错误: %w", err)
			}
		}

		time.Sleep(readDelay)
	}
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
			logger.Info("Executor", e.IP, "=== 检测到同步交互命令超时自定义控制 ===: %s -> %v", cmdToSend, customTimeout)
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
				logger.Warn("Executor", e.IP, "sync reader goroutine panic recovered: %v", r)
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

				// 检查没有换行符的最后一段是否看起来像提示符
				lastSegment := lines[len(lines)-1]

				// 对待同步交互，同样可能遇到分页符卡死
				if e.Matcher.IsPaginationPrompt(lastSegment) {
					logger.Debug("Executor", e.IP, "[同步自动翻页] 截获终端分页拦截符(More)，自动下发空格放行...")
					e.Client.SendRawBytes([]byte(" "))
					streamBuffer = "" // 因为同步方法中 streamBuffer 的末尾就是 lastSegment
					continue
				}

				if e.Matcher.IsPrompt(lastSegment) {
					// 找到提示符，命令执行完成
					_ = e.flushDetailLog()
					return outputBuffer.String(), nil
				}

				// 可选：检查前面的行是否为提示符？但通常它出现在最后
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
