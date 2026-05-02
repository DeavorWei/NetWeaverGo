package connutil

import (
	"fmt"
	"net"
	"time"

	"github.com/NetWeaverGo/core/internal/sshutil"
)

// SSHConnectionAdapter 将现有的 sshutil.SSHClient 适配为 DeviceConnection 接口。
// 这是一个薄包装层，不改变 SSHClient 的任何行为。
type SSHConnectionAdapter struct {
	client *sshutil.SSHClient
}

// NewSSHConnectionAdapter 创建 SSH 连接适配器。
func NewSSHConnectionAdapter(client *sshutil.SSHClient) *SSHConnectionAdapter {
	return &SSHConnectionAdapter{client: client}
}

// Read 从 SSH 连接读取数据。
func (a *SSHConnectionAdapter) Read(p []byte) (n int, err error) {
	return a.client.Read(p)
}

// Write 向 SSH 连接写入数据。
func (a *SSHConnectionAdapter) Write(p []byte) (n int, err error) {
	return a.client.Stdin.Write(p)
}

// Close 关闭 SSH 连接。
func (a *SSHConnectionAdapter) Close() error {
	return a.client.Close()
}

// SendCommand 发送命令到设备。
func (a *SSHConnectionAdapter) SendCommand(cmd string) (string, error) {
	err := a.client.SendCommand(cmd)
	if err != nil {
		return "", err
	}
	// SSH 的 SendCommand 只负责发送，响应由 StreamEngine 通过 Read 消费。
	// 返回空字符串，上层应通过 Read 获取响应。
	return "", nil
}

// SendRawBytes 发送原始字节。
func (a *SSHConnectionAdapter) SendRawBytes(data []byte) error {
	return a.client.SendRawBytes(data)
}

// SetReadDeadline 设置读取超时。
func (a *SSHConnectionAdapter) SetReadDeadline(deadline time.Time) error {
	return a.client.SetReadDeadline(deadline)
}

// CancelRead 取消当前读取操作。
func (a *SSHConnectionAdapter) CancelRead() {
	a.client.CancelRead()
}

// IsClosed 返回连接是否已关闭。
func (a *SSHConnectionAdapter) IsClosed() bool {
	return a.client.IsClosed()
}

// RemoteAddr 返回远程地址。
func (a *SSHConnectionAdapter) RemoteAddr() string {
	return net.JoinHostPort(a.client.IP, fmt.Sprintf("%d", a.client.Port))
}

// Unwrap 返回底层的 SSHClient，供需要直接访问 SSH 功能的场景使用。
func (a *SSHConnectionAdapter) Unwrap() *sshutil.SSHClient {
	return a.client
}
