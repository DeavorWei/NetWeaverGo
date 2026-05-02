package connutil

import (
	"fmt"
	"time"

	"github.com/NetWeaverGo/core/internal/telnetutil"
)

// TelnetConnectionAdapter 将 telnetutil.Client 适配为 DeviceConnection 接口。
// SendCommand 只发送命令不读取响应，响应由 StreamEngine 通过 Read 消费。
type TelnetConnectionAdapter struct {
	client *telnetutil.Client
}

// NewTelnetConnectionAdapter 创建 Telnet 连接适配器。
func NewTelnetConnectionAdapter(client *telnetutil.Client) *TelnetConnectionAdapter {
	return &TelnetConnectionAdapter{client: client}
}

// Read 从 Telnet 连接读取数据（实现 io.Reader）。
func (a *TelnetConnectionAdapter) Read(p []byte) (int, error) {
	return a.client.Read(p)
}

// Write 向 Telnet 连接写入数据（实现 io.Writer）。
func (a *TelnetConnectionAdapter) Write(p []byte) (int, error) {
	return a.client.Write(p)
}

// Close 关闭 Telnet 连接（实现 io.Closer）。
func (a *TelnetConnectionAdapter) Close() error {
	return a.client.Close()
}

// SendCommand 只发送命令，不读取响应（与 SSHConnectionAdapter 行为一致）。
// 响应由 StreamEngine 通过 Read 消费。
func (a *TelnetConnectionAdapter) SendCommand(cmd string) (string, error) {
	_, err := fmt.Fprintf(a.client, "%s\r\n", cmd)
	return "", err
}

// SendRawBytes 发送原始字节数据到设备（不做 IAC 转义）。
func (a *TelnetConnectionAdapter) SendRawBytes(data []byte) error {
	return a.client.SendRawBytes(data)
}

// SetReadDeadline 设置读取超时。
func (a *TelnetConnectionAdapter) SetReadDeadline(deadline time.Time) error {
	return a.client.SetReadDeadline(deadline)
}

// CancelRead 取消当前读取操作。
func (a *TelnetConnectionAdapter) CancelRead() {
	a.client.CancelRead()
}

// IsClosed 返回连接是否已关闭。
func (a *TelnetConnectionAdapter) IsClosed() bool {
	return a.client.IsClosed()
}

// RemoteAddr 返回远程地址。
func (a *TelnetConnectionAdapter) RemoteAddr() string {
	return a.client.RemoteAddr()
}

// Unwrap 返回底层的 telnetutil.Client，供需要直接访问的场景使用。
func (a *TelnetConnectionAdapter) Unwrap() *telnetutil.Client {
	return a.client
}
