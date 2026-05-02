// Package connutil 定义了设备连接的统一接口和工厂。
// 它为 SSH 和 Telnet 等不同协议提供统一的抽象层，
// 使上层执行器可以透明地使用不同协议的连接。
package connutil

import (
	"io"
	"time"
)

// DeviceConnection 定义了设备连接的统一接口。
// 继承 io.Reader、io.Writer、io.Closer，提供标准的流式读写能力。
// 所有实现必须是并发安全的。
type DeviceConnection interface {
	io.Reader
	io.Writer
	io.Closer

	// SendCommand 发送一条命令到设备并等待响应。
	// 返回设备的响应文本和可能的错误。
	// 实现应负责处理命令终止符（如回车换行）和响应等待。
	SendCommand(cmd string) (string, error)

	// SendRawBytes 发送原始字节数据到设备（不附加任何终止符）。
	SendRawBytes(data []byte) error

	// SetReadDeadline 设置读取操作的超时时间。
	// 用于实现非阻塞读取和超时控制。
	SetReadDeadline(deadline time.Time) error

	// CancelRead 强制中断当前正在进行的读取操作。
	// 用于紧急情况下快速终止阻塞的读取。
	CancelRead()

	// IsClosed 返回连接是否已关闭。
	IsClosed() bool

	// RemoteAddr 返回远程设备的地址（格式为 "ip:port"）。
	RemoteAddr() string
}
