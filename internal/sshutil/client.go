package sshutil

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/NetWeaverGo/core/internal/config"
	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/report"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// SSHClient 表示一个到设备的活跃 SSH 连接
type SSHClient struct {
	Client  *ssh.Client
	Session *ssh.Session
	IP      string
	Port    int

	Stdin  io.WriteCloser
	Stdout io.Reader
	Stderr io.Reader

	// transcriptSink 保存原始交互流，用于问题排查和执行回显审计。
	transcriptSink report.RawTranscriptSink

	// conn 保存底层的 TCP 连接，用于设置 deadline
	conn net.Conn

	// 读写锁：保护 Stdin/Stdout 的并发访问
	mu sync.RWMutex

	// closed 标记连接是否已关闭
	closed atomic.Bool

	// 新增：读取中断控制
	readCancel context.CancelFunc
	readCtx    context.Context
}

// Config 包含了建连的基础凭证和超时参数
type Config struct {
	IP       string
	Port     int
	Username string
	Password string
	Timeout  time.Duration

	// SSH 算法配置（可选）
	Algorithms *config.SSHAlgorithmSettings
	// 主机密钥校验策略: strict / accept_new / insecure
	HostKeyPolicy string
	// known_hosts 文件路径（可选）
	KnownHostsPath string

	// RawSink 为可选的原始 SSH 字节流输出。
	RawSink report.RawTranscriptSink
}

const (
	hostKeyPolicyStrict    = "strict"
	hostKeyPolicyAcceptNew = "accept_new"
	hostKeyPolicyInsecure  = "insecure"
)

var knownHostsWriteMu sync.Mutex

// logSSHConfig 记录SSH配置信息用于调试
func logSSHConfig(ip string, sshConfig *ssh.ClientConfig, cfg Config) {
	if !logger.EnableVerbose && !logger.EnableDebug {
		return
	}

	// 获取算法信息
	ciphers := sshConfig.Config.Ciphers
	keyExchanges := sshConfig.Config.KeyExchanges
	macs := sshConfig.Config.MACs
	hostKeyAlgorithms := sshConfig.HostKeyAlgorithms

	// 获取预设模式
	presetMode := "default"
	if cfg.Algorithms != nil {
		presetMode = cfg.Algorithms.PresetMode
	}

	logger.Verbose("SSH", ip, "准备SSH握手，算法配置:")
	logger.Verbose("SSH", ip, "  - 预设模式: %s", presetMode)
	logger.Verbose("SSH", ip, "  - 用户名: %s", cfg.Username)
	logger.Verbose("SSH", ip, "  - 目标地址: %s:%d", cfg.IP, cfg.Port)
	logger.Verbose("SSH", ip, "  - 超时时间: %v", cfg.Timeout)

	if len(ciphers) > 0 {
		logger.Verbose("SSH", ip, "  - Ciphers(%d): %v", len(ciphers), ciphers)
	} else {
		logger.Verbose("SSH", ip, "  - Ciphers: 使用Go默认配置")
	}

	if len(keyExchanges) > 0 {
		logger.Verbose("SSH", ip, "  - KeyExchanges(%d): %v", len(keyExchanges), keyExchanges)
	} else {
		logger.Verbose("SSH", ip, "  - KeyExchanges: 使用Go默认配置")
	}

	if len(macs) > 0 {
		logger.Verbose("SSH", ip, "  - MACs(%d): %v", len(macs), macs)
	} else {
		logger.Verbose("SSH", ip, "  - MACs: 使用Go默认配置")
	}

	if len(hostKeyAlgorithms) > 0 {
		logger.Verbose("SSH", ip, "  - HostKeyAlgorithms(%d): %v", len(hostKeyAlgorithms), hostKeyAlgorithms)
	} else {
		logger.Verbose("SSH", ip, "  - HostKeyAlgorithms: 使用Go默认配置")
	}
}

// logSSHHandshakeError 记录SSH握手失败的详细信息
func logSSHHandshakeError(ip string, err error, sshConfig *ssh.ClientConfig, cfg Config) {
	// 获取算法信息
	ciphers := sshConfig.Config.Ciphers
	keyExchanges := sshConfig.Config.KeyExchanges
	macs := sshConfig.Config.MACs
	hostKeyAlgorithms := sshConfig.HostKeyAlgorithms

	// 获取预设模式
	presetMode := "default"
	if cfg.Algorithms != nil {
		presetMode = cfg.Algorithms.PresetMode
	}

	logger.Error("SSH", ip, "SSH握手失败详情:")
	logger.Error("SSH", ip, "  - 错误信息: %v", err)
	logger.Error("SSH", ip, "  - 预设模式: %s", presetMode)
	logger.Error("SSH", ip, "  - 用户名: %s", cfg.Username)
	logger.Error("SSH", ip, "  - 目标地址: %s:%d", cfg.IP, cfg.Port)

	// 分析错误类型并给出建议
	errStr := err.Error()
	switch {
	case strings.Contains(errStr, "unable to authenticate"):
		logger.Error("SSH", ip, "  - 错误类型: 认证失败")
		logger.Error("SSH", ip, "  - 可能原因:")
		logger.Error("SSH", ip, "    1. 用户名或密码错误")
		logger.Error("SSH", ip, "    2. 设备配置了仅公钥认证(ssh server authentication-type publickey)")
		logger.Error("SSH", ip, "    3. 设备禁用了密码认证")
		logger.Error("SSH", ip, "  - 建议操作:")
		logger.Error("SSH", ip, "    1. 检查用户名和密码是否正确")
		logger.Error("SSH", ip, "    2. 使用命令行测试: ssh %s@%s", cfg.Username, cfg.IP)
		logger.Error("SSH", ip, "    3. 检查设备SSH配置，确认允许密码认证")
		logger.Error("SSH", ip, "    4. 如使用密钥认证，需在设备上配置公钥")

	case strings.Contains(errStr, "handshake failed"):
		logger.Error("SSH", ip, "  - 错误类型: 握手失败")
		if strings.Contains(errStr, "no common algorithm") {
			logger.Error("SSH", ip, "  - 可能原因: 客户端与服务器算法不匹配")
			logger.Error("SSH", ip, "  - 建议操作:")
			logger.Error("SSH", ip, "    1. 在设置中切换SSH算法预设为'兼容模式'")
			logger.Error("SSH", ip, "    2. 使用自定义模式，勾选更多算法选项")
		} else if strings.Contains(errStr, "exhausted") {
			logger.Error("SSH", ip, "  - 可能原因: 密钥交换算法协商失败")
			logger.Error("SSH", ip, "  - 建议操作:")
			logger.Error("SSH", ip, "    1. 检查设备支持的密钥交换算法")
			logger.Error("SSH", ip, "    2. 在设置中添加更多密钥交换算法")
		} else {
			logger.Error("SSH", ip, "  - 可能原因: 协议不兼容或网络问题")
			logger.Error("SSH", ip, "  - 建议操作:")
			logger.Error("SSH", ip, "    1. 检查设备SSH服务是否正常运行")
			logger.Error("SSH", ip, "    2. 尝试使用标准SSH客户端连接测试")
		}

	case strings.Contains(errStr, "connection refused"):
		logger.Error("SSH", ip, "  - 错误类型: 连接被拒绝")
		logger.Error("SSH", ip, "  - 可能原因: SSH服务未运行或端口错误")
		logger.Error("SSH", ip, "  - 建议操作:")
		logger.Error("SSH", ip, "    1. 确认设备IP和端口(%d)正确", cfg.Port)
		logger.Error("SSH", ip, "    2. 检查设备SSH服务是否已启动")
		logger.Error("SSH", ip, "    3. 检查防火墙/ACL是否允许连接")

	case strings.Contains(errStr, "forcibly closed") || strings.Contains(errStr, "reset"):
		logger.Error("SSH", ip, "  - 错误类型: 连接被强制关闭")
		logger.Error("SSH", ip, "  - 可能原因:")
		logger.Error("SSH", ip, "    1. 设备SSH连接数限制")
		logger.Error("SSH", ip, "    2. 设备安全策略阻止连接")
		logger.Error("SSH", ip, "    3. 网络设备中间阻断")
		logger.Error("SSH", ip, "  - 建议操作:")
		logger.Error("SSH", ip, "    1. 减少并发连接数")
		logger.Error("SSH", ip, "    2. 检查设备SSH会话限制配置")
		logger.Error("SSH", ip, "    3. 检查网络安全策略")

	case strings.Contains(errStr, "timeout"):
		logger.Error("SSH", ip, "  - 错误类型: 连接超时")
		logger.Error("SSH", ip, "  - 可能原因: 网络不可达或设备响应慢")
		logger.Error("SSH", ip, "  - 建议操作:")
		logger.Error("SSH", ip, "    1. 检查网络连通性: ping %s", cfg.IP)
		logger.Error("SSH", ip, "    2. 增加连接超时时间")
		logger.Error("SSH", ip, "    3. 检查设备负载是否过高")

	default:
		logger.Error("SSH", ip, "  - 错误类型: 未知错误")
		logger.Error("SSH", ip, "  - 建议操作:")
		logger.Error("SSH", ip, "    1. 开启Verbose日志获取更多信息")
		logger.Error("SSH", ip, "    2. 使用标准SSH客户端测试连接")
	}

	// 输出算法配置（便于排查算法问题）
	if len(ciphers) > 0 {
		logger.Error("SSH", ip, "  - 客户端Ciphers(%d): %v", len(ciphers), ciphers)
	}
	if len(keyExchanges) > 0 {
		logger.Error("SSH", ip, "  - 客户端KeyExchanges(%d): %v", len(keyExchanges), keyExchanges)
	}
	if len(macs) > 0 {
		logger.Error("SSH", ip, "  - 客户端MACs(%d): %v", len(macs), macs)
	}
	if len(hostKeyAlgorithms) > 0 {
		logger.Error("SSH", ip, "  - 客户端HostKeyAlgorithms(%d): %v", len(hostKeyAlgorithms), hostKeyAlgorithms)
	}

	// 记录认证方法
	logger.Error("SSH", ip, "  - 认证方法: [password keyboard-interactive]")
}

// applyAlgorithmConfig 应用 SSH 算法配置到 ssh.ClientConfig
// 如果提供了自定义算法配置，使用自定义配置；否则使用内置的兼容性配置
func applyAlgorithmConfig(sshConfig *ssh.ClientConfig, algoSettings *config.SSHAlgorithmSettings) {
	// 获取有效的算法配置
	var ciphers, keyExchanges, macs, hostKeyAlgorithms []string

	if algoSettings != nil {
		ciphers, keyExchanges, macs, hostKeyAlgorithms = GetEffectiveAlgorithms(*algoSettings)
	}

	// 如果有配置，使用配置的算法；否则使用内置默认（兼容性）配置
	if len(ciphers) > 0 {
		sshConfig.Config.Ciphers = ciphers
	} else {
		// 使用内置的兼容性配置
		sshConfig.Config.Ciphers = []string{
			// 1. 官方推荐的最安全的现代算法（AEAD）
			ssh.CipherAES128GCM,
			ssh.CipherAES256GCM,
			ssh.CipherChaCha20Poly1305,

			// 2. 安全的传统对称加密算法（CTR 模式）
			ssh.CipherAES128CTR,
			ssh.CipherAES192CTR,
			ssh.CipherAES256CTR,

			// 3. 为了兼容老旧网络设备添加的不安全算法（CBC 模式及 RC4）
			ssh.InsecureCipherAES128CBC,
			"aes192-cbc", // golang.org/x/crypto/ssh 默认未公开此常量
			"aes256-cbc",
			ssh.InsecureCipherTripleDESCBC,
			ssh.InsecureCipherRC4,
			ssh.InsecureCipherRC4128,
			ssh.InsecureCipherRC4256,
		}
	}

	if len(keyExchanges) > 0 {
		sshConfig.Config.KeyExchanges = keyExchanges
	} else {
		sshConfig.Config.KeyExchanges = []string{
			// 1. 官方推荐的最安全的现代算法（包含抗量子和椭圆曲线）
			ssh.KeyExchangeMLKEM768X25519,
			ssh.KeyExchangeCurve25519,
			ssh.KeyExchangeECDHP256,
			ssh.KeyExchangeECDHP384,
			ssh.KeyExchangeECDHP521,

			// 2. 安全的传统 DH 算法
			ssh.KeyExchangeDH14SHA256,
			ssh.KeyExchangeDH16SHA512,
			ssh.KeyExchangeDHGEXSHA256,

			// 3. 为了兼容老旧网络设备（如旧 Cisco/H3C）添加的不安全算法
			ssh.InsecureKeyExchangeDH14SHA1,  // 官方默认包含了这个
			ssh.InsecureKeyExchangeDH1SHA1,   // diffie-hellman-group1-sha1
			ssh.InsecureKeyExchangeDHGEXSHA1, // diffie-hellman-group-exchange-sha1
		}
	}

	if len(macs) > 0 {
		sshConfig.Config.MACs = macs
	} else {
		sshConfig.Config.MACs = []string{
			// 1. 官方推荐的最安全的现代算法（AEAD 模式不需要 MAC，但以防万一也可配置）
			ssh.HMACSHA256ETM,
			ssh.HMACSHA512ETM,

			// 2. 安全的传统哈希算法
			ssh.HMACSHA256,
			ssh.HMACSHA512,

			// 3. 为了兼容老旧网络设备添加的不安全算法（SHA1）
			ssh.HMACSHA1,
			ssh.InsecureHMACSHA196,
		}
	}

	if len(hostKeyAlgorithms) > 0 {
		sshConfig.HostKeyAlgorithms = hostKeyAlgorithms
	} else {
		sshConfig.HostKeyAlgorithms = []string{
			// 1. 官方推荐的最安全的现代算法（椭圆曲线和 ED25519）
			ssh.KeyAlgoED25519,
			ssh.KeyAlgoECDSA256,
			ssh.KeyAlgoECDSA384,
			ssh.KeyAlgoECDSA521,

			// 2. 基于硬件安全密钥 (SK, FIDO2/U2F) 的现代算法
			ssh.KeyAlgoSKED25519,
			ssh.KeyAlgoSKECDSA256,

			// 3. 基于 OpenSSH 证书签发的现代/传统算法
			ssh.CertAlgoED25519v01,
			ssh.CertAlgoECDSA256v01,
			ssh.CertAlgoECDSA384v01,
			ssh.CertAlgoECDSA521v01,
			ssh.CertAlgoSKED25519v01,
			ssh.CertAlgoSKECDSA256v01,
			ssh.CertAlgoRSASHA512v01,
			ssh.CertAlgoRSASHA256v01,
			ssh.CertAlgoRSAv01,

			// 4. 安全的传统 RSA 算法（使用 SHA2）
			ssh.KeyAlgoRSASHA512,
			ssh.KeyAlgoRSASHA256,

			// 5. 为了兼容老旧网络设备添加的不安全算法或弃用证书（SHA1 和 DSS）
			ssh.KeyAlgoRSA,
			ssh.InsecureKeyAlgoDSA,
			ssh.InsecureCertAlgoDSAv01,
		}
	}
}

func resolveHostKeyPolicy(cfg Config) (string, string) {
	policy := strings.ToLower(strings.TrimSpace(cfg.HostKeyPolicy))
	if policy == "" {
		policy = hostKeyPolicyAcceptNew
	}
	knownHostsPath := strings.TrimSpace(cfg.KnownHostsPath)
	if knownHostsPath == "" {
		knownHostsPath = config.GetPathManager().GetSSHKnownHostsPath()
	}
	return policy, knownHostsPath
}

func buildHostKeyCallback(cfg Config) (ssh.HostKeyCallback, error) {
	policy, knownHostsPath := resolveHostKeyPolicy(cfg)
	switch policy {
	case hostKeyPolicyStrict:
		return strictKnownHostsCallback(knownHostsPath)
	case hostKeyPolicyAcceptNew:
		return acceptNewKnownHostsCallback(knownHostsPath)
	case hostKeyPolicyInsecure:
		logger.Warn("SSH", cfg.IP, "HostKeyPolicy=insecure，将跳过主机密钥校验（不推荐）")
		return ssh.InsecureIgnoreHostKey(), nil
	default:
		return nil, fmt.Errorf("未知 HostKeyPolicy: %s", policy)
	}
}

func strictKnownHostsCallback(path string) (ssh.HostKeyCallback, error) {
	if err := ensureKnownHostsFile(path); err != nil {
		return nil, err
	}
	cb, err := knownhosts.New(path)
	if err != nil {
		return nil, err
	}
	return cb, nil
}

func acceptNewKnownHostsCallback(path string) (ssh.HostKeyCallback, error) {
	if err := ensureKnownHostsFile(path); err != nil {
		return nil, err
	}
	strictCB, err := knownhosts.New(path)
	if err != nil {
		return nil, err
	}

	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		err := strictCB(hostname, remote, key)
		if err == nil {
			return nil
		}

		var keyErr *knownhosts.KeyError
		if !errors.As(err, &keyErr) {
			return err
		}

		// Want 为空表示未记录该主机，按 accept_new 策略写入并放行。
		if len(keyErr.Want) == 0 {
			if appendErr := appendKnownHost(path, hostname, key); appendErr != nil {
				return fmt.Errorf("新增 known_hosts 失败: %w", appendErr)
			}
			logger.Info("SSH", hostname, "检测到新主机密钥，已写入 known_hosts")
			return nil
		}
		return fmt.Errorf("主机密钥不匹配: %w", err)
	}, nil
}

func ensureKnownHostsFile(path string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("known_hosts 路径为空")
	}
	if !filepath.IsAbs(path) {
		path = filepath.Clean(path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	return file.Close()
}

func appendKnownHost(path string, hostname string, key ssh.PublicKey) error {
	knownHostsWriteMu.Lock()
	defer knownHostsWriteMu.Unlock()

	normalizedHost := knownhosts.Normalize(hostname)
	line := knownhosts.Line([]string{normalizedHost}, key)

	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	writer := bufio.NewWriter(file)
	if _, err := writer.WriteString(line + "\n"); err != nil {
		return err
	}
	return writer.Flush()
}

// NewSSHClient 建立SSH连接并请求交互式 Shell 终端
func NewSSHClient(ctx context.Context, cfg Config) (*SSHClient, error) {
	logger.Verbose("SSH", cfg.IP, "开始初始化带 Shell 的 SSH 连接 -> %s:%d", cfg.IP, cfg.Port)
	if cfg.Port == 0 {
		cfg.Port = config.DefaultSSHPort
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = config.GetRuntimeManager().GetConnectionTimeout()
	}
	hostKeyCallback, err := buildHostKeyCallback(cfg)
	if err != nil {
		return nil, fmt.Errorf("初始化主机密钥校验失败: %w", err)
	}

	sshConfig := &ssh.ClientConfig{
		User: cfg.Username,
		Auth: []ssh.AuthMethod{
			ssh.Password(cfg.Password),
			ssh.KeyboardInteractive(func(user, instruction string, questions []string, echos []bool) (answers []string, err error) {
				answers = make([]string, len(questions))
				for i, q := range questions {
					if strings.Contains(strings.ToLower(q), "password") {
						answers[i] = cfg.Password
					}
				}
				return answers, nil
			}),
		},
		HostKeyCallback: hostKeyCallback,
		Timeout:         cfg.Timeout,
	}

	// 应用算法配置（使用传入的配置或内置默认配置）
	applyAlgorithmConfig(sshConfig, cfg.Algorithms)

	// 记录SSH配置信息（用于调试）
	logSSHConfig(cfg.IP, sshConfig, cfg)

	target := fmt.Sprintf("%s:%d", cfg.IP, cfg.Port)
	dialer := net.Dialer{Timeout: cfg.Timeout}

	conn, err := dialer.DialContext(ctx, "tcp", target)
	if err != nil {
		logger.Debug("SSH", cfg.IP, "拨号 %s 失败: %v", target, err)
		return nil, fmt.Errorf("TCP连通失败: %w", err)
	}
	logger.Verbose("SSH", cfg.IP, "TCP %s 拨号成功", target)

	c, chans, reqs, err := ssh.NewClientConn(conn, target, sshConfig)
	if err != nil {
		// 记录详细的握手失败信息
		logSSHHandshakeError(cfg.IP, err, sshConfig, cfg)
		conn.Close()
		return nil, fmt.Errorf("SSH握手失败: %w", err)
	}
	client := ssh.NewClient(c, chans, reqs)

	session, err := client.NewSession()
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("会话创建失败: %w", err)
	}

	modes := ssh.TerminalModes{
		ssh.ECHO:          0,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	if err := session.RequestPty("vt100", 80, 40, modes); err != nil {
		session.Close()
		client.Close()
		return nil, fmt.Errorf("挂载伪终端(PTY)失败: %w", err)
	}

	stdin, err := session.StdinPipe()
	if err != nil {
		session.Close()
		client.Close()
		return nil, err
	}
	stdout, err := session.StdoutPipe()
	if err != nil {
		session.Close()
		client.Close()
		return nil, err
	}
	stderr, err := session.StderrPipe()
	if err != nil {
		session.Close()
		client.Close()
		return nil, err
	}

	if err := session.Shell(); err != nil {
		logger.Debug("SSH", cfg.IP, "目标 %s Shell启动失败: %v", target, err)
		session.Close()
		client.Close()
		return nil, fmt.Errorf("Shell启动失败: %w", err)
	}

	stdoutReader := io.Reader(stdout)
	stderrReader := io.Reader(stderr)
	sink := cfg.RawSink
	if sink != nil {
		sink.WriteMarker("========== SESSION START %s %s:%d ==========\n", time.Now().Format(time.RFC3339), cfg.IP, cfg.Port)
		stdoutReader = io.TeeReader(stdout, sink)
		stderrReader = io.TeeReader(stderr, sink)
	}

	// 初始化读取上下文，用于控制读取中断
	readCtx, readCancel := context.WithCancel(context.Background())

	logger.Debug("SSH", cfg.IP, "目标 %s SSH全特性挂载(PTY/Shell)成功", target)
	return &SSHClient{
		Client:         client,
		Session:        session,
		IP:             cfg.IP,
		Port:           cfg.Port,
		Stdin:          stdin,
		Stdout:         stdoutReader,
		Stderr:         stderrReader,
		transcriptSink: sink,
		conn:           conn,
		readCtx:        readCtx,
		readCancel:     readCancel,
	}, nil
}

// SendCommand 发送单条命令及回车换行符到流中
func (c *SSHClient) SendCommand(cmd string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 检查是否已关闭
	if c.closed.Load() {
		return fmt.Errorf("SSH 连接已关闭")
	}

	logger.Verbose("SSH", c.IP, "向底层隧道投递命令行包含回车: %q", cmd)
	if c.Stdin == nil {
		return fmt.Errorf("SSHClient not fully initialized for terminal sending (Stdin is nil)")
	}
	if c.transcriptSink != nil {
		c.transcriptSink.WriteMarker("\n[%s] >>> %s\n", time.Now().Format("15:04:05"), cmd)
	}
	// 恢复标准 \n。之前改为 \r\n 导致部分交换机将其解析为两下回车，严重干扰 Prompt 匹配缓冲流。
	_, err := fmt.Fprintf(c.Stdin, "%s\n", cmd)
	return err
}

// SendRawBytes 发送原生字节序列到终端流中（不附加回车换行）
func (c *SSHClient) SendRawBytes(data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 检查是否已关闭
	if c.closed.Load() {
		return fmt.Errorf("SSH 连接已关闭")
	}

	logger.Verbose("SSH", c.IP, "向底层隧道投递 Raw Bytes: %q", string(data))
	if c.Stdin == nil {
		return fmt.Errorf("SSHClient not fully initialized for terminal sending (Stdin is nil)")
	}
	if c.transcriptSink != nil {
		c.transcriptSink.WriteMarker("\n[%s] >>> [RAW] %q\n", time.Now().Format("15:04:05"), string(data))
	}
	_, err := c.Stdin.Write(data)
	return err
}

// Read 从 Stdout 读取数据（线程安全）
func (c *SSHClient) Read(p []byte) (n int, err error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// 检查是否已关闭
	if c.closed.Load() {
		return 0, net.ErrClosed
	}

	if c.Stdout == nil {
		return 0, fmt.Errorf("SSHClient not fully initialized (Stdout is nil)")
	}

	return c.Stdout.Read(p)
}

// Close 断开流释放句柄
func (c *SSHClient) Close() error {
	// 原子性地标记为已关闭
	if c.closed.Swap(true) {
		return nil // 已经关闭
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	var err error
	if c.Session != nil {
		if c.Stdin != nil {
			c.Stdin.Close()
		}
		err = c.Session.Close()
	}
	if c.Client != nil {
		clientErr := c.Client.Close()
		if err == nil {
			err = clientErr
		}
	}
	if c.transcriptSink != nil {
		c.transcriptSink.WriteMarker("\n========== SESSION END %s ==========\n", time.Now().Format(time.RFC3339))
	}
	return err
}

// IsClosed 检查连接是否已关闭
func (c *SSHClient) IsClosed() bool {
	return c.closed.Load()
}

// SetReadDeadline 设置读取超时时间，用于实现非阻塞读取
// 注意：SSH Session 的 Stdout 是通过管道实现的，不支持直接设置 deadline
// 此方法通过底层 TCP 连接来设置 deadline 以中断阻塞的读取操作
func (c *SSHClient) SetReadDeadline(t time.Time) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// 检查是否已关闭
	if c.closed.Load() {
		return net.ErrClosed
	}

	// 使用保存的底层 TCP 连接设置 deadline
	if c.conn != nil {
		if err := c.conn.SetReadDeadline(t); err != nil {
			return err
		}
	}

	return nil
}

// CancelRead 强制中断当前的读取操作
// 用于紧急情况下快速终止阻塞的读取
func (c *SSHClient) CancelRead() {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// 取消读取上下文
	if c.readCancel != nil {
		c.readCancel()
	}

	// 设置一个过去的 deadline 来强制中断当前的读取
	if c.conn != nil {
		c.conn.SetReadDeadline(time.Now().Add(-1 * time.Second))
	}
}

// NewRawSSHClient 建立一个最基础的SSH连接，不请求任何 Session, PTY, 或 Shell。
// 专供 SFTP 或其他只要求纯净底层子系统通道的应用（例如华为交换机的 sftp 子系统不能在包含 shell 的连接中打开）。
func NewRawSSHClient(ctx context.Context, cfg Config) (*SSHClient, error) {
	logger.Verbose("SSH", cfg.IP, "开始初始化 Raw SSH 连接 -> %s:%d", cfg.IP, cfg.Port)
	if cfg.Port == 0 {
		cfg.Port = config.DefaultSSHPort
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = config.GetRuntimeManager().GetConnectionTimeout()
	}
	hostKeyCallback, err := buildHostKeyCallback(cfg)
	if err != nil {
		return nil, fmt.Errorf("初始化主机密钥校验失败: %w", err)
	}

	sshConfig := &ssh.ClientConfig{
		User: cfg.Username,
		Auth: []ssh.AuthMethod{
			ssh.Password(cfg.Password),
			ssh.KeyboardInteractive(func(user, instruction string, questions []string, echos []bool) (answers []string, err error) {
				answers = make([]string, len(questions))
				for i, q := range questions {
					if strings.Contains(strings.ToLower(q), "password") {
						answers[i] = cfg.Password
					}
				}
				return answers, nil
			}),
		},
		HostKeyCallback: hostKeyCallback,
		Timeout:         cfg.Timeout,
	}

	// 应用算法配置（使用传入的配置或内置默认配置）
	applyAlgorithmConfig(sshConfig, cfg.Algorithms)

	// 记录SSH配置信息（用于调试）
	logSSHConfig(cfg.IP, sshConfig, cfg)

	target := fmt.Sprintf("%s:%d", cfg.IP, cfg.Port)
	dialer := net.Dialer{Timeout: cfg.Timeout}

	conn, err := dialer.DialContext(ctx, "tcp", target)
	if err != nil {
		logger.Debug("SSH", cfg.IP, "Raw 拨号 %s 失败: %v", target, err)
		return nil, fmt.Errorf("TCP连通失败: %w", err)
	}
	logger.Verbose("SSH", cfg.IP, "Raw TCP %s 拨号成功", target)

	c, chans, reqs, err := ssh.NewClientConn(conn, target, sshConfig)
	if err != nil {
		// 记录详细的握手失败信息
		logSSHHandshakeError(cfg.IP, err, sshConfig, cfg)
		conn.Close()
		return nil, fmt.Errorf("SSH握手失败: %w", err)
	}
	client := ssh.NewClient(c, chans, reqs)

	return &SSHClient{
		Client: client, // 仅带有底层client，没有挂载任何终端特性 Session
		IP:     cfg.IP,
		Port:   cfg.Port,
	}, nil
}
