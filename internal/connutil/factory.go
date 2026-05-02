package connutil

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/NetWeaverGo/core/internal/config"
	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/models"
	"github.com/NetWeaverGo/core/internal/report"
	"github.com/NetWeaverGo/core/internal/sshutil"
	"github.com/NetWeaverGo/core/internal/telnetutil"
)

// Protocol 协议类型常量
const (
	ProtocolSSH    = "ssh"
	ProtocolTelnet = "telnet"
)

// DefaultSSHPort 默认 SSH 端口
const DefaultSSHPort = 22

// DefaultTelnetPort 默认 Telnet 端口
const DefaultTelnetPort = 23

// ConnectionConfig 连接配置，包含建连所需的全部参数。
type ConnectionConfig struct {
	IP       string        // 设备 IP
	Port     int           // 设备端口（0 表示使用协议默认端口）
	Username string        // 用户名
	Password string        // 密码
	Protocol string        // 协议: "ssh" 或 "telnet"
	Timeout  time.Duration // 连接超时

	// SSH 专用配置（仅在 Protocol="ssh" 时生效）
	SSH *SSHOptions
}

// SSHOptions SSH 协议的额外配置选项。
type SSHOptions struct {
	// Algorithms SSH 算法配置（可选）
	Algorithms *models.SSHAlgorithmSettings
	// HostKeyPolicy 主机密钥校验策略: "strict" / "accept_new" / "insecure"
	HostKeyPolicy string
	// KnownHostsPath known_hosts 文件路径（可选）
	KnownHostsPath string
	// PTY PTY 终端配置（可选，nil 则使用默认值）
	PTY *sshutil.PTYConfig
	// RawSink 原始字节流输出（可选，用于审计日志）
	RawSink report.RawTranscriptSink
}

// ConnectionFactory 连接工厂接口。
// 根据配置创建对应协议的设备连接。
type ConnectionFactory interface {
	Connect(ctx context.Context, cfg ConnectionConfig) (DeviceConnection, error)
}

// DefaultConnectionFactory 默认连接工厂实现。
// 根据 ConnectionConfig.Protocol 分发到 SSH 或 Telnet 客户端。
type DefaultConnectionFactory struct{}

// NewDefaultConnectionFactory 创建默认连接工厂。
func NewDefaultConnectionFactory() *DefaultConnectionFactory {
	return &DefaultConnectionFactory{}
}

// Connect 根据协议类型创建设备连接。
func (f *DefaultConnectionFactory) Connect(ctx context.Context, cfg ConnectionConfig) (DeviceConnection, error) {
	protocol := strings.ToLower(strings.TrimSpace(cfg.Protocol))

	// 默认超时
	if cfg.Timeout == 0 {
		cfg.Timeout = config.GetRuntimeManager().GetConnectionTimeout()
	}

	switch protocol {
	case ProtocolSSH, "":
		return f.connectSSH(ctx, cfg)
	case ProtocolTelnet:
		return f.connectTelnet(ctx, cfg)
	default:
		return nil, NewConnectionError(protocol, cfg.IP, cfg.Port, "connect",
			fmt.Sprintf("不支持的协议: %s", protocol), ErrUnsupportedProtocol)
	}
}

// connectSSH 建立 SSH 连接。
func (f *DefaultConnectionFactory) connectSSH(ctx context.Context, cfg ConnectionConfig) (DeviceConnection, error) {
	port := cfg.Port
	if port == 0 {
		port = DefaultSSHPort
	}

	sshCfg := sshutil.Config{
		IP:       cfg.IP,
		Port:     port,
		Username: cfg.Username,
		Password: cfg.Password,
		Timeout:  cfg.Timeout,
	}

	// 应用 SSH 专用选项
	if cfg.SSH != nil {
		sshCfg.Algorithms = cfg.SSH.Algorithms
		sshCfg.HostKeyPolicy = cfg.SSH.HostKeyPolicy
		sshCfg.KnownHostsPath = cfg.SSH.KnownHostsPath
		sshCfg.PTY = cfg.SSH.PTY
		sshCfg.RawSink = cfg.SSH.RawSink
	} else {
		// 使用全局 SSH 主机密钥策略
		hostKeyPolicy, knownHostsPath := config.ResolveSSHHostKeyPolicy()
		sshCfg.HostKeyPolicy = hostKeyPolicy
		sshCfg.KnownHostsPath = knownHostsPath
	}

	client, err := sshutil.NewSSHClient(ctx, sshCfg)
	if err != nil {
		return nil, NewConnectionError(ProtocolSSH, cfg.IP, port, "connect",
			"SSH 连接失败", err)
	}

	logger.Debug("ConnFactory", cfg.IP, "SSH 连接建立成功 -> %s:%d", cfg.IP, port)
	return NewSSHConnectionAdapter(client), nil
}

// connectTelnet 建立 Telnet 连接。
func (f *DefaultConnectionFactory) connectTelnet(ctx context.Context, cfg ConnectionConfig) (DeviceConnection, error) {
	port := cfg.Port
	if port == 0 {
		port = DefaultTelnetPort
	}

	telnetCfg := telnetutil.Config{
		IP:       cfg.IP,
		Port:     port,
		Username: cfg.Username,
		Password: cfg.Password,
		Timeout:  cfg.Timeout,
	}

	client, err := telnetutil.NewClient(ctx, telnetCfg)
	if err != nil {
		return nil, NewConnectionError(ProtocolTelnet, cfg.IP, port, "connect",
			"Telnet 连接失败", err)
	}

	logger.Debug("ConnFactory", cfg.IP, "Telnet 连接建立成功 -> %s:%d", cfg.IP, port)
	return NewTelnetConnectionAdapter(client), nil
}
