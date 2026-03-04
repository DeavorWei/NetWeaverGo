package sshutil

import (
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"

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

	sshConfig.Config.Ciphers = append(sshConfig.Config.Ciphers,
		"aes128-gcm@openssh.com", "chacha20-poly1305@openssh.com",
		"aes128-ctr", "aes192-ctr", "aes256-ctr",
		"aes128-cbc", "aes192-cbc", "aes256-cbc", "3des-cbc",
		"arcfour", "arcfour128", "arcfour256",
	)
	sshConfig.Config.KeyExchanges = append(sshConfig.Config.KeyExchanges,
		"curve25519-sha256", "curve25519-sha256@libssh.org",
		"ecdh-sha2-nistp256", "ecdh-sha2-nistp384", "ecdh-sha2-nistp521",
		"diffie-hellman-group16-sha512",
		"diffie-hellman-group14-sha256",
		"diffie-hellman-group14-sha1",
		"diffie-hellman-group-exchange-sha256",
		"diffie-hellman-group-exchange-sha1",
		"diffie-hellman-group1-sha1",
	)
	sshConfig.Config.MACs = append(sshConfig.Config.MACs,
		"hmac-sha2-256-etm@openssh.com", "hmac-sha2-256",
		"hmac-sha1", "hmac-sha1-96",
	)
	sshConfig.HostKeyAlgorithms = append(sshConfig.HostKeyAlgorithms,
		"rsa-sha2-512", "rsa-sha2-256",
		"ssh-rsa", "ssh-dss",
		"ecdsa-sha2-nistp256", "ecdsa-sha2-nistp384", "ecdsa-sha2-nistp521",
		"ssh-ed25519",
	)

	target := fmt.Sprintf("%s:%d", cfg.IP, cfg.Port)
	dialer := net.Dialer{Timeout: cfg.Timeout}

	conn, err := dialer.DialContext(ctx, "tcp", target)
	if err != nil {
		return nil, fmt.Errorf("TCP连通失败: %w", err)
	}

	c, chans, reqs, err := ssh.NewClientConn(conn, target, sshConfig)
	if err != nil {
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
		session.Close()
		client.Close()
		return nil, fmt.Errorf("Shell启动失败: %w", err)
	}

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

	sshConfig.Config.Ciphers = append(sshConfig.Config.Ciphers,
		"aes128-gcm@openssh.com", "chacha20-poly1305@openssh.com",
		"aes128-ctr", "aes192-ctr", "aes256-ctr",
		"aes128-cbc", "aes192-cbc", "aes256-cbc", "3des-cbc",
		"arcfour", "arcfour128", "arcfour256",
	)
	sshConfig.Config.KeyExchanges = append(sshConfig.Config.KeyExchanges,
		"curve25519-sha256", "curve25519-sha256@libssh.org",
		"ecdh-sha2-nistp256", "ecdh-sha2-nistp384", "ecdh-sha2-nistp521",
		"diffie-hellman-group16-sha512",
		"diffie-hellman-group14-sha256",
		"diffie-hellman-group14-sha1",
		"diffie-hellman-group-exchange-sha256",
		"diffie-hellman-group-exchange-sha1",
		"diffie-hellman-group1-sha1",
	)
	sshConfig.Config.MACs = append(sshConfig.Config.MACs,
		"hmac-sha2-256-etm@openssh.com", "hmac-sha2-256",
		"hmac-sha1", "hmac-sha1-96",
	)
	sshConfig.HostKeyAlgorithms = append(sshConfig.HostKeyAlgorithms,
		"rsa-sha2-512", "rsa-sha2-256",
		"ssh-rsa", "ssh-dss",
		"ecdsa-sha2-nistp256", "ecdsa-sha2-nistp384", "ecdsa-sha2-nistp521",
		"ssh-ed25519",
	)

	target := fmt.Sprintf("%s:%d", cfg.IP, cfg.Port)
	dialer := net.Dialer{Timeout: cfg.Timeout}

	conn, err := dialer.DialContext(ctx, "tcp", target)
	if err != nil {
		return nil, fmt.Errorf("TCP连通失败: %w", err)
	}

	c, chans, reqs, err := ssh.NewClientConn(conn, target, sshConfig)
	if err != nil {
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
