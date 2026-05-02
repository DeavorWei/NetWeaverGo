// Package telnetutil 实现 Telnet 协议客户端。
// 提供与网络设备的 Telnet 连接，支持协议协商、密码认证和命令交互。
package telnetutil

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/NetWeaverGo/core/internal/config"
	"github.com/NetWeaverGo/core/internal/logger"
)

// ============================================================================
// 连接配置
// ============================================================================

// Config Telnet 连接配置。
type Config struct {
	IP       string        // 设备 IP
	Port     int           // 设备端口（0 则使用默认端口 23）
	Username string        // 用户名
	Password string        // 密码
	Timeout  time.Duration // 连接超时

	// TerminalType 终端类型（可选，默认 "VT100"）
	TerminalType string
	// TermWidth 终端宽度（可选，默认 256）
	TermWidth int
	// TermHeight 终端高度（可选，默认 200）
	TermHeight int
}

// ============================================================================
// Telnet 客户端
// ============================================================================

// Client 实现 Telnet 协议的设备连接客户端。
// 实现 connutil.DeviceConnection 接口。
type Client struct {
	// conn 底层 TCP 连接
	conn net.Conn

	// ip 设备 IP
	ip string
	// port 设备端口
	port int

	// optHandler 协议选项协商处理器
	optHandler *OptionHandler

	// readBuf 读取缓冲区，用于存储从设备读取的已过滤数据
	// 由于 Telnet 协议需要过滤 IAC 命令，不能直接从 conn 读取
	readBuf *bytes.Buffer
	// readMu 保护 readBuf 的并发访问
	readMu sync.Mutex

	// writeMu 保护写操作的并发访问
	writeMu sync.Mutex

	// closed 连接关闭标记
	closed atomic.Bool

	// readCancel 取消读取操作的函数
	readCancel context.CancelFunc
	// readCtx 读取操作的上下文
	readCtx context.Context

	// promptDetected 提示符检测通道
	// 用于在认证阶段和命令交互中同步提示符到达事件
	promptDetected chan struct{}

	// rawReader 底层连接的原始读取器
	rawReader *bufio.Reader

	// pendingIAC 跨 Read 调用累积的不完整 IAC 序列
	// 解决 filterIAC 在数据边界处截断 IAC 命令的问题
	pendingIAC []byte
}

// NewClient 创建并初始化一个 Telnet 客户端。
// 建立 TCP 连接，完成协议协商和密码认证。
func NewClient(ctx context.Context, cfg Config) (*Client, error) {
	if cfg.Port == 0 {
		cfg.Port = config.DefaultTelnetPort
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = config.GetRuntimeManager().GetConnectionTimeout()
	}

	addr := fmt.Sprintf("%s:%d", cfg.IP, cfg.Port)
	logger.Debug("Telnet", cfg.IP, "正在建立 Telnet 连接 -> %s (超时: %v)", addr, cfg.Timeout)

	// 建立 TCP 连接
	dialer := net.Dialer{Timeout: cfg.Timeout}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("TCP 连接失败 %s: %w", addr, err)
	}
	logger.Verbose("Telnet", cfg.IP, "TCP 连接建立成功 -> %s", addr)

	// 创建客户端
	readCtx, readCancel := context.WithCancel(context.Background())
	c := &Client{
		conn:           conn,
		ip:             cfg.IP,
		port:           cfg.Port,
		optHandler:     NewOptionHandler(cfg.TerminalType, cfg.TermWidth, cfg.TermHeight),
		readBuf:        &bytes.Buffer{},
		readCancel:     readCancel,
		readCtx:        readCtx,
		promptDetected: make(chan struct{}, 1),
		rawReader:      bufio.NewReader(conn),
	}

	// 执行协议协商
	if err := c.negotiate(ctx, cfg.Timeout); err != nil {
		conn.Close()
		return nil, fmt.Errorf("Telnet 协商失败: %w", err)
	}

	// 执行密码认证
	if cfg.Username != "" || cfg.Password != "" {
		if err := c.authenticate(ctx, cfg.Username, cfg.Password, cfg.Timeout); err != nil {
			conn.Close()
			return nil, fmt.Errorf("Telnet 认证失败: %w", err)
		}
	}

	logger.Debug("Telnet", cfg.IP, "Telnet 连接初始化完成 -> %s", addr)
	return c, nil
}

// ============================================================================
// 协议协商
// ============================================================================

// negotiate 执行 Telnet 协议选项协商。
// 发送初始协商消息，处理服务器的协商请求。
func (c *Client) negotiate(ctx context.Context, timeout time.Duration) error {
	logger.Debug("Telnet", c.ip, "开始 Telnet 协议协商")

	// 发送初始协商消息
	initMsg := c.optHandler.BuildInitialNegotiation()
	if _, err := c.conn.Write(initMsg); err != nil {
		return fmt.Errorf("发送初始协商消息失败: %w", err)
	}

	// 等待并处理服务器的协商响应
	// 设置一个较短的协商超时，避免长时间阻塞
	negotiateTimeout := 5 * time.Second
	if timeout > 0 && timeout < negotiateTimeout {
		negotiateTimeout = timeout
	}

	deadline := time.Now().Add(negotiateTimeout)
	if err := c.conn.SetReadDeadline(deadline); err != nil {
		return fmt.Errorf("设置协商超时失败: %w", err)
	}

	// 读取并处理协商数据，直到超时或协商完成
	for {
		b, err := c.rawReader.ReadByte()
		if err != nil {
			// 超时或 EOF 表示协商阶段结束
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				break
			}
			if err == io.EOF {
				break
			}
			return fmt.Errorf("读取协商数据失败: %w", err)
		}

		if b == IAC {
			// 读取命令字节
			cmd, err := c.rawReader.ReadByte()
			if err != nil {
				return fmt.Errorf("读取 IAC 命令失败: %w", err)
			}

			switch cmd {
			case WILL, WONT, DO, DONT:
				// 读取选项字节
				opt, err := c.rawReader.ReadByte()
				if err != nil {
					return fmt.Errorf("读取 IAC 选项失败: %w", err)
				}
				logger.Verbose("Telnet", c.ip, "收到协商: IAC %s %d", commandName(cmd), opt)

				// 处理协商并发送响应
				responses := c.optHandler.HandleCommand(cmd, opt)
				for _, resp := range responses {
					respBytes := []byte{IAC, resp.Command, resp.Option}
					if _, err := c.conn.Write(respBytes); err != nil {
						return fmt.Errorf("发送协商响应失败: %w", err)
					}
					logger.Verbose("Telnet", c.ip, "发送协商: IAC %s %d", commandName(resp.Command), resp.Option)
				}

			case SB:
				// 子协商：读取选项字节和子协商数据直到 SE
				opt, err := c.rawReader.ReadByte()
				if err != nil {
					return fmt.Errorf("读取子协商选项失败: %w", err)
				}

				// 读取子协商数据直到 IAC SE
				subData := make([]byte, 0)
				for {
					sb, err := c.rawReader.ReadByte()
					if err != nil {
						return fmt.Errorf("读取子协商数据失败: %w", err)
					}
					if sb == IAC {
						se, err := c.rawReader.ReadByte()
						if err != nil {
							return fmt.Errorf("读取子协商结束标记失败: %w", err)
						}
						if se == SE {
							break
						}
						// 非 SE 的 IAC 后续字节，可能是转义的 0xFF
						subData = append(subData, sb, se)
					} else {
						subData = append(subData, sb)
					}
				}

				logger.Verbose("Telnet", c.ip, "收到子协商: SB %d %q", opt, subData)

				// 构建子协商响应
				response := c.optHandler.BuildSubNegotiation(opt)
				if response != nil {
					if _, err := c.conn.Write(response); err != nil {
						return fmt.Errorf("发送子协商响应失败: %w", err)
					}
					logger.Verbose("Telnet", c.ip, "发送子协商响应: opt=%d", opt)
				}

			case IAC:
				// 转义的 0xFF 字节，放入缓冲区
				c.readBuf.WriteByte(IAC)

			case GA, NOP:
				// 忽略 GA 和 NOP

			default:
				// 其他命令，忽略
				logger.Verbose("Telnet", c.ip, "忽略 IAC 命令: %d", cmd)
			}
		}
		// 非 IAC 字节在协商阶段忽略（通常是登录提示前的欢迎信息）
	}

	// 清除读取超时
	_ = c.conn.SetReadDeadline(time.Time{})
	logger.Debug("Telnet", c.ip, "Telnet 协议协商完成")
	return nil
}

// ============================================================================
// 密码认证
// ============================================================================

// authenticate 执行 Telnet 密码认证流程。
// 等待 login/username 提示 → 发送用户名 → 等待 password 提示 → 发送密码。
func (c *Client) authenticate(ctx context.Context, username, password string, timeout time.Duration) error {
	logger.Debug("Telnet", c.ip, "开始 Telnet 认证 (用户: %s)", username)

	authTimeout := 15 * time.Second
	if timeout > 0 && timeout < authTimeout {
		authTimeout = timeout
	}

	// 等待 login/username 提示
	loginPrompt, err := c.waitForPrompt(ctx, []string{"login:", "username:", "user:"}, authTimeout)
	if err != nil {
		return fmt.Errorf("等待登录提示超时: %w", err)
	}
	logger.Debug("Telnet", c.ip, "检测到登录提示: %q", loginPrompt)

	// 发送用户名
	if _, err := c.conn.Write([]byte(username + "\r\n")); err != nil {
		return fmt.Errorf("发送用户名失败: %w", err)
	}
	logger.Verbose("Telnet", c.ip, "已发送用户名")

	// 等待 password 提示
	passwordPrompt, err := c.waitForPrompt(ctx, []string{"password:"}, authTimeout)
	if err != nil {
		return fmt.Errorf("等待密码提示超时: %w", err)
	}
	logger.Debug("Telnet", c.ip, "检测到密码提示: %q", passwordPrompt)

	// 发送密码
	if _, err := c.conn.Write([]byte(password + "\r\n")); err != nil {
		return fmt.Errorf("发送密码失败: %w", err)
	}
	logger.Verbose("Telnet", c.ip, "已发送密码")

	// 等待认证完成（检测命令提示符）
	promptPatterns := []string{">", "#", "$", "]"}
	_, err = c.waitForPrompt(ctx, promptPatterns, 10*time.Second)
	if err != nil {
		return fmt.Errorf("等待认证完成超时: %w", err)
	}

	// 清除读取超时
	_ = c.conn.SetReadDeadline(time.Time{})
	logger.Debug("Telnet", c.ip, "Telnet 认证完成")
	return nil
}

// waitForPrompt 等待设备输出中出现指定的提示符之一。
// 返回匹配到的提示符文本。
func (c *Client) waitForPrompt(ctx context.Context, prompts []string, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	if err := c.conn.SetReadDeadline(deadline); err != nil {
		return "", fmt.Errorf("设置读取超时失败: %w", err)
	}
	defer func() { _ = c.conn.SetReadDeadline(time.Time{}) }()

	var accumulated []byte
	buf := make([]byte, 256)

	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}

		n, err := c.rawReader.Read(buf)
		if n > 0 {
			// 过滤 IAC 命令
			filtered := c.filterIAC(buf[:n])
			accumulated = append(accumulated, filtered...)

			accumulatedStr := strings.ToLower(string(accumulated))
			for _, prompt := range prompts {
				if strings.Contains(accumulatedStr, prompt) {
					return prompt, nil
				}
			}
		}
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				return "", fmt.Errorf("等待提示符 %v 超时", prompts)
			}
			if err == io.EOF {
				return "", fmt.Errorf("连接已关闭 (EOF)，等待提示符 %v 失败", prompts)
			}
			return "", fmt.Errorf("读取数据失败: %w", err)
		}
	}
}

// filterIAC 从数据中过滤 Telnet IAC 命令，返回纯文本数据。
// 处理跨 Read 调用的 IAC 序列边界截断问题。
func (c *Client) filterIAC(data []byte) []byte {
	// 将之前累积的不完整 IAC 序列与新数据安全合并
	// 使用 copy 避免 append 可能修改 pendingIAC 原始底层数组
	if len(c.pendingIAC) > 0 {
		merged := make([]byte, len(c.pendingIAC)+len(data))
		copy(merged, c.pendingIAC)
		copy(merged[len(c.pendingIAC):], data)
		data = merged
		c.pendingIAC = nil
	}

	var result []byte
	i := 0
	for i < len(data) {
		if data[i] == IAC {
			if i+1 >= len(data) {
				// IAC 是最后一个字节，累积等待更多数据
				c.pendingIAC = data[i:]
				break
			}

			switch data[i+1] {
			case IAC:
				// 转义的 0xFF
				result = append(result, IAC)
				i += 2
			case WILL, WONT, DO, DONT:
				if i+2 >= len(data) {
					// 不完整的协商命令，累积等待
					c.pendingIAC = data[i:]
					i = len(data)
					break
				}
				// 处理协商命令
				responses := c.optHandler.HandleCommand(data[i+1], data[i+2])
				for _, resp := range responses {
					_, _ = c.conn.Write([]byte{IAC, resp.Command, resp.Option})
				}
				i += 3
			case SB:
				// 子协商处理：跳过直到 IAC SE
				j := i + 2
				found := false
				for j < len(data)-1 {
					if data[j] == IAC && data[j+1] == SE {
						j += 2
						found = true
						break
					}
					j++
				}
				if !found {
					// 子协商不完整，累积等待
					c.pendingIAC = data[i:]
					i = len(data)
				} else {
					i = j
				}
			case GA, NOP:
				i += 2
			default:
				// 其他 IAC 命令，跳过
				i += 2
			}
		} else {
			result = append(result, data[i])
			i++
		}
	}

	return result
}

// ============================================================================
// DeviceConnection 接口实现
// ============================================================================

// Read 从 Telnet 连接读取数据（已过滤 IAC 命令）。
// 实现 io.Reader 接口。
// 阻塞直到有数据可读或连接关闭，超时由上层通过 SetReadDeadline 控制。
func (c *Client) Read(p []byte) (n int, err error) {
	if c.closed.Load() {
		return 0, net.ErrClosed
	}

	c.readMu.Lock()
	defer c.readMu.Unlock()

	// 先从缓冲区读取
	if c.readBuf.Len() > 0 {
		return c.readBuf.Read(p)
	}

	// 从底层连接读取（阻塞式，不设置短超时）
	n, err = c.conn.Read(p)
	if n > 0 {
		filtered := c.filterIAC(p[:n])
		if len(filtered) < n {
			// 有 IAC 命令被过滤，数据量减少
			c.readBuf.Write(filtered)
			return c.readBuf.Read(p)
		}
		return n, nil
	}
	return 0, err
}

// Write 向 Telnet 连接写入数据。
// 实现 io.Writer 接口。自动对 0xFF 字节进行 IAC 转义。
func (c *Client) Write(p []byte) (n int, err error) {
	if c.closed.Load() {
		return 0, net.ErrClosed
	}

	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	escaped := escapeIAC(p)
	_, err = c.conn.Write(escaped)
	return len(p), err // 返回原始数据长度
}

// Close 关闭 Telnet 连接并释放所有资源。
// 实现 io.Closer 接口。
func (c *Client) Close() error {
	if c.closed.Swap(true) {
		return nil // 已经关闭
	}

	// 取消读取操作
	if c.readCancel != nil {
		c.readCancel()
	}

	// 关闭底层连接
	err := c.conn.Close()
	logger.Debug("Telnet", c.ip, "Telnet 连接已关闭")
	return err
}

// SendCommand 发送命令到设备并读取响应。
// 发送命令后等待设备输出，直到检测到提示符或超时。
func (c *Client) SendCommand(cmd string) (string, error) {
	if c.closed.Load() {
		return "", net.ErrClosed
	}

	c.writeMu.Lock()
	_, err := fmt.Fprintf(c.conn, "%s\r\n", cmd)
	c.writeMu.Unlock()
	if err != nil {
		return "", fmt.Errorf("发送命令失败: %w", err)
	}

	logger.Verbose("Telnet", c.ip, "已发送命令: %s", cmd)

	// 等待响应
	time.Sleep(200 * time.Millisecond)

	c.readMu.Lock()
	defer c.readMu.Unlock()

	// 读取所有可用数据
	var response []byte
	buf := make([]byte, 4096)
	for {
		_ = c.conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		n, readErr := c.conn.Read(buf)
		if n > 0 {
			filtered := c.filterIAC(buf[:n])
			response = append(response, filtered...)
		}
		if readErr != nil {
			break
		}
	}
	_ = c.conn.SetReadDeadline(time.Time{})

	return string(response), nil
}

// SendRawBytes 发送原始字节数据到设备（不附加任何终止符）。
func (c *Client) SendRawBytes(data []byte) error {
	if c.closed.Load() {
		return net.ErrClosed
	}

	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	_, err := c.conn.Write(data)
	return err
}

// SetReadDeadline 设置读取操作的超时时间。
func (c *Client) SetReadDeadline(deadline time.Time) error {
	if c.closed.Load() {
		return net.ErrClosed
	}
	return c.conn.SetReadDeadline(deadline)
}

// CancelRead 强制中断当前的读取操作。
func (c *Client) CancelRead() {
	if c.readCancel != nil {
		c.readCancel()
	}
	// 设置一个过去的 deadline 来强制中断当前的读取
	if c.conn != nil {
		_ = c.conn.SetReadDeadline(time.Now().Add(-1 * time.Second))
	}
}

// IsClosed 返回连接是否已关闭。
func (c *Client) IsClosed() bool {
	return c.closed.Load()
}

// RemoteAddr 返回远程设备地址。
func (c *Client) RemoteAddr() string {
	return fmt.Sprintf("%s:%d", c.ip, c.port)
}

// ============================================================================
// 辅助函数
// ============================================================================

// commandName 返回 Telnet 命令的可读名称，用于日志输出。
func commandName(cmd byte) string {
	switch cmd {
	case WILL:
		return "WILL"
	case WONT:
		return "WONT"
	case DO:
		return "DO"
	case DONT:
		return "DONT"
	case SB:
		return "SB"
	case SE:
		return "SE"
	case IAC:
		return "IAC"
	case GA:
		return "GA"
	case NOP:
		return "NOP"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", cmd)
	}
}

// escapeIAC 对数据中的 0xFF 字节进行 IAC 转义。
// 将每个 0xFF 字节替换为 IAC IAC (0xFF 0xFF)，符合 Telnet 协议规范。
func escapeIAC(data []byte) []byte {
	// 统计需要转义的 0xFF 字节数
	count := 0
	for _, b := range data {
		if b == IAC {
			count++
		}
	}

	if count == 0 {
		return data // 无需转义
	}

	// 创建转义后的数据
	escaped := make([]byte, len(data)+count)
	j := 0
	for _, b := range data {
		escaped[j] = b
		j++
		if b == IAC {
			escaped[j] = IAC // 0xFF 转义为 IAC IAC
			j++
		}
	}
	return escaped
}
