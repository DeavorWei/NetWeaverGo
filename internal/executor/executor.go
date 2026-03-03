package executor

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/matcher"
	"github.com/NetWeaverGo/core/internal/sshutil"
)

// ErrorAction 描述了由引擎决定的继续行径
type ErrorAction int

const (
	ActionContinue ErrorAction = iota // 忽略错误，继续发送下一条
	ActionSkip                        // 跳过本条引发错误的命令，或者干脆直接完成
	ActionAbort                       // 立即停止该设备的后续命令
)

// SuspendHandler 定义一个回调函数，当 Executor 遇到错误时，将其抛弃给主线程引擎询问用户决策
// 引擎会阻塞在此，直到用户通过命令行或其他界面选定动作后返回，从而只影响该设备的 Goroutine 局部挂起
type SuspendHandler func(ip string, deviceLog string, failedCmd string) ErrorAction

// DeviceExecutor 封装特定设备的 SSH 数据流及命令步进下发生命周期
type DeviceExecutor struct {
	IP       string
	Port     int
	Username string
	Password string

	Matcher *matcher.StreamMatcher
	Client  *sshutil.SSHClient
	Log     *logger.DeviceLogger

	OnSuspend SuspendHandler
}

// NewDeviceExecutor 初始化执行器
func NewDeviceExecutor(ip string, port int, user, pass string, onSuspend SuspendHandler) *DeviceExecutor {
	return &DeviceExecutor{
		IP:        ip,
		Port:      port,
		Username:  user,
		Password:  pass,
		Matcher:   matcher.NewStreamMatcher(),
		OnSuspend: onSuspend,
	}
}

// Connect 创建SSH长连接并初始化日志审计
func (e *DeviceExecutor) Connect(ctx context.Context) error {
	cfg := sshutil.Config{
		IP:       e.IP,
		Port:     e.Port,
		Username: e.Username,
		Password: e.Password,
		Timeout:  10 * time.Second,
	}

	client, err := sshutil.NewSSHClient(ctx, cfg)
	if err != nil {
		return err
	}
	e.Client = client

	devLog, err := logger.NewDeviceLogger(e.IP)
	if err != nil {
		e.Client.Close()
		return err
	}
	e.Log = devLog

	return nil
}

// ExecutePlaybook 核心引擎方法：对该设备步进发送命令队列，并支持局部阻塞等待（配合 SuspendHandler）
func (e *DeviceExecutor) ExecutePlaybook(ctx context.Context, commands []string) error {
	if e.Client == nil || e.Log == nil {
		return fmt.Errorf("执行器未安全建连")
	}

	buf := make([]byte, 1024)
	outReader := io.TeeReader(e.Client.Stdout, e.Log)
	errReader := io.TeeReader(e.Client.Stderr, e.Log)

	// 丢弃并记录 stderr（因为 TeeReader 已经挂在日志里了）
	go func() {
		_, _ = io.Copy(io.Discard, errReader)
	}()

	currentCmdIndex := -1 // 初始化状态: -1 表示还在探测第一个提示符
	var streamBuffer string

	// Timeout duration for waiting for command prompts
	timeoutDuration := 30 * time.Second
	timer := time.NewTimer(timeoutDuration)
	defer timer.Stop()

	type readResult struct {
		n   int
		err error
	}
	readCh := make(chan readResult, 1)

	// 后台运行读操作，以便主流程能响应 timeout 和 ctx.Done()
	go func() {
		for {
			n, err := outReader.Read(buf)
			readCh <- readResult{n: n, err: err}
			if err != nil {
				close(readCh)
				return
			}
		}
	}()

	readDelay := 100 * time.Millisecond
	for {
		// Reset the timer at the start of each select iteration
		// Since we handle timer.C, we need to ensure it's drained if we reset it, but here we just leave it firing.
		// Actually, standard way is to stop/drain, or just re-create/reset cleanly.
		// A simpler approach: timer is reset after any prompt matches or data received.
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			// Timeout triggered
			e.Log.Log("====== [等待超时] 命令无回显或提示符超时 (30s) ======")

			var failedCmd string
			if currentCmdIndex >= 0 && currentCmdIndex < len(commands) {
				failedCmd = commands[currentCmdIndex]
			}

			// Report to interceptor
			action := e.OnSuspend(e.IP, "Timeout Error: No prompt received within 30 seconds", failedCmd)
			switch action {
			case ActionAbort:
				e.Log.Log("====== 用户选择中止 (Abort): 将断开连接 ======")
				return fmt.Errorf("设备 %s 的执行因超时被用户中止", e.IP)
			case ActionSkip:
				e.Log.Log("====== 用户选择跳过 (Skip): 丢弃该超时，进入下一条命令 ======")
				// Simulate prompt received to move to next command, and clear buffer
				streamBuffer = ""
				if currentCmdIndex >= 0 {
					currentCmdIndex++
				}
				timer.Reset(timeoutDuration) // Restart timer for next command or finishing up
			case ActionContinue:
				e.Log.Log("====== 用户选择继续 (Continue): 强制忽略并继续等待 ======")
				// Keep waiting
				timer.Reset(timeoutDuration)
			}
		case res, ok := <-readCh:
			if !ok {
				// The channel is closed, which means background goroutine returned (likely due to error like EOF)
				// The actual error was sent just before closing, so we normally would have caught it in the previous readCh case.
				// However, if we get here, connection died.
				e.Log.Log("SSH 会话由于连接中断或完成已结束。")
				return nil
			}

			n, err := res.n, res.err

			if n > 0 {
				// We received data, reset the idle timeout timer
				if !timer.Stop() {
					// Drain the channel if the timer had already fired but we didn't consume it yet
					select {
					case <-timer.C:
					default:
					}
				}
				timer.Reset(timeoutDuration)

				chunk := string(buf[:n])
				streamBuffer += chunk

				lines := strings.Split(streamBuffer, "\n")
				// 检查最后一行以外的完整回显行，查找 error 关键字
				for i, line := range lines {
					if i < len(lines)-1 {
						if e.Matcher.MatchError(line) {
							e.Log.Log("====== [命中异常规则] 挂起当前设备执行 ======")
							e.Log.Log("错误流内容: %s", line)

							// 触发外部回调执行暂停，将由外部引擎的通道控制该函数返回，形成单设备挂起效果
							var failedCmd string
							if currentCmdIndex >= 0 && currentCmdIndex < len(commands) {
								failedCmd = commands[currentCmdIndex] // 上一条发送的引起错误的命令
							}

							action := e.OnSuspend(e.IP, line, failedCmd)
							switch action {
							case ActionAbort:
								e.Log.Log("====== 用户选择中止 (Abort): 将断开连接 ======")
								return fmt.Errorf("设备 %s 的执行被用户手动中止", e.IP)
							case ActionSkip:
								e.Log.Log("====== 用户选择跳过 (Skip): 丢弃该错误继续当前流程 ======")
							case ActionContinue:
								e.Log.Log("====== 用户选择继续 (Continue): 强制忽略并放行 ======")
							}

							// 继续后清空缓冲区，避免二次重复报错
							streamBuffer = ""
							break
						}
					}
				}

				// 将没有换行符的最后一部分留到 Buffer 中进行提示符识别
				streamBuffer = lines[len(lines)-1]

				// 处于等待主提示符阶段
				if currentCmdIndex == -1 {
					if e.Matcher.IsPrompt(streamBuffer) {
						e.Log.Log("==== [连接成功] 获得首个提示符，准备下发配置 ====")
						currentCmdIndex = 0

						e.Client.SendCommand("") // Wakeup Line 以稳定状态
					}
				}

				// 发送队列中的命令
				if currentCmdIndex >= 0 && currentCmdIndex < len(commands) {
					if e.Matcher.IsPrompt(streamBuffer) {
						cmd := commands[currentCmdIndex]
						e.Log.Log(">>> [发送命令]: %s", cmd)
						e.Client.SendCommand(cmd)
						streamBuffer = "" // 发送命令后清空当前 Buffer，防止将上一步的提示符混到了接下来
						currentCmdIndex++
					}
				} else if currentCmdIndex >= len(commands) {
					// 任务完成，判断最后一条命令结果是否已回显出提示符
					if e.Matcher.IsPrompt(streamBuffer) {
						e.Log.Log("==== [执行完成] 所有命令已下发完毕 ====")
						return nil
					}
				}
			}

			if err != nil {
				if err == io.EOF {
					e.Log.Log("SSH 会话已被远端安全断开。")
					return nil
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
	if e.Log != nil {
		e.Log.Close()
	}
}
