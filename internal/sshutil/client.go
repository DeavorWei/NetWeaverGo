package sshutil

import (
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/NetWeaverGo/core/internal/logger"
	"golang.org/x/crypto/ssh"
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

	mu sync.Mutex
}

// Config 包含了建连的基础凭证和超时参数
type Config struct {
	IP       string
	Port     int
	Username string
	Password string
	Timeout  time.Duration
}

// NewSSHClient 建立SSH连接并请求交互式 Shell 终端
func NewSSHClient(ctx context.Context, cfg Config) (*SSHClient, error) {
	logger.DebugAll("SSH", cfg.IP, "开始初始化带 Shell 的 SSH 连接 -> %s:%d", cfg.IP, cfg.Port)
	if cfg.Port == 0 {
		cfg.Port = 22
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 10 * time.Second
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
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         cfg.Timeout,
	}

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

	target := fmt.Sprintf("%s:%d", cfg.IP, cfg.Port)
	dialer := net.Dialer{Timeout: cfg.Timeout}

	conn, err := dialer.DialContext(ctx, "tcp", target)
	if err != nil {
		logger.Debug("SSH", cfg.IP, "拨号 %s 失败: %v", target, err)
		return nil, fmt.Errorf("TCP连通失败: %w", err)
	}
	logger.DebugAll("SSH", cfg.IP, "TCP %s 拨号成功", target)

	c, chans, reqs, err := ssh.NewClientConn(conn, target, sshConfig)
	if err != nil {
		logger.Debug("SSH", cfg.IP, "SSH握手(含算法协商)失败: %v", err)
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

	logger.Debug("SSH", cfg.IP, "目标 %s SSH全特性挂载(PTY/Shell)成功", target)
	return &SSHClient{
		Client:  client,
		Session: session,
		IP:      cfg.IP,
		Port:    cfg.Port,
		Stdin:   stdin,
		Stdout:  stdout,
		Stderr:  stderr,
	}, nil
}

// SendCommand 发送单条命令及回车换行符到流中
func (c *SSHClient) SendCommand(cmd string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	logger.DebugAll("SSH", c.IP, "向底层隧道投递命令行包含回车: %q", cmd)
	if c.Stdin == nil {
		return fmt.Errorf("SSHClient not fully initialized for terminal sending (Stdin is nil)")
	}
	// 恢复标准 \n。之前改为 \r\n 导致部分交换机将其解析为两下回车，严重干扰 Prompt 匹配缓冲流。
	_, err := fmt.Fprintf(c.Stdin, "%s\n", cmd)
	return err
}

// SendRawBytes 发送原生字节序列到终端流中（不附加回车换行）
func (c *SSHClient) SendRawBytes(data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	logger.DebugAll("SSH", c.IP, "向底层隧道投递 Raw Bytes: %q", string(data))
	if c.Stdin == nil {
		return fmt.Errorf("SSHClient not fully initialized for terminal sending (Stdin is nil)")
	}
	_, err := c.Stdin.Write(data)
	return err
}

// Close 断开流释放句柄
func (c *SSHClient) Close() error {
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
	return err
}

// NewRawSSHClient 建立一个最基础的SSH连接，不请求任何 Session, PTY, 或 Shell。
// 专供 SFTP 或其他只要求纯净底层子系统通道的应用（例如华为交换机的 sftp 子系统不能在包含 shell 的连接中打开）。
func NewRawSSHClient(ctx context.Context, cfg Config) (*SSHClient, error) {
	logger.DebugAll("SSH", cfg.IP, "开始初始化 Raw SSH 连接 -> %s:%d", cfg.IP, cfg.Port)
	if cfg.Port == 0 {
		cfg.Port = 22
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 10 * time.Second
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
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         cfg.Timeout,
	}

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

	target := fmt.Sprintf("%s:%d", cfg.IP, cfg.Port)
	dialer := net.Dialer{Timeout: cfg.Timeout}

	conn, err := dialer.DialContext(ctx, "tcp", target)
	if err != nil {
		logger.Debug("SSH", cfg.IP, "Raw 拨号 %s 失败: %v", target, err)
		return nil, fmt.Errorf("TCP连通失败: %w", err)
	}
	logger.DebugAll("SSH", cfg.IP, "Raw TCP %s 拨号成功", target)

	c, chans, reqs, err := ssh.NewClientConn(conn, target, sshConfig)
	if err != nil {
		logger.Debug("SSH", cfg.IP, "Raw SSH握手(含算法协商)失败: %v", err)
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
