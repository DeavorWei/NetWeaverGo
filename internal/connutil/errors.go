package connutil

import (
	"errors"
	"fmt"
)

// 预定义的连接错误，供上层使用 errors.Is 进行判断。
var (
	// ErrConnectionClosed 连接已关闭
	ErrConnectionClosed = errors.New("连接已关闭")

	// ErrConnectionTimeout 连接超时
	ErrConnectionTimeout = errors.New("连接超时")

	// ErrAuthenticationFailed 认证失败
	ErrAuthenticationFailed = errors.New("认证失败")

	// ErrReadTimeout 读取超时
	ErrReadTimeout = errors.New("读取超时")

	// ErrUnsupportedProtocol 不支持的协议
	ErrUnsupportedProtocol = errors.New("不支持的协议")

	// ErrInvalidConfig 无效的连接配置
	ErrInvalidConfig = errors.New("无效的连接配置")

	// ErrTelnetNegotiationFailed Telnet 协议协商失败
	ErrTelnetNegotiationFailed = errors.New("Telnet 协议协商失败")

	// ErrTelnetAuthPromptTimeout Telnet 认证提示等待超时
	ErrTelnetAuthPromptTimeout = errors.New("Telnet 认证提示等待超时")

	// ErrTelnetLoginFailed Telnet 登录失败
	ErrTelnetLoginFailed = errors.New("Telnet 登录失败")
)

// ConnectionError 连接错误类型，封装底层错误并附加上下文信息。
type ConnectionError struct {
	Protocol string // 协议类型: "ssh", "telnet"
	IP       string // 设备 IP
	Port     int    // 设备端口
	Stage    string // 错误阶段: "connect", "auth", "read", "write", "close"
	Message  string // 错误消息
	Err      error  // 底层错误
}

// Error 实现 error 接口。
func (e *ConnectionError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%s] %s:%d (%s): %s: %v",
			e.Protocol, e.IP, e.Port, e.Stage, e.Message, e.Err)
	}
	return fmt.Sprintf("[%s] %s:%d (%s): %s",
		e.Protocol, e.IP, e.Port, e.Stage, e.Message)
}

// Unwrap 实现 errors.Unwrap 接口，支持 errors.Is / errors.As。
func (e *ConnectionError) Unwrap() error {
	return e.Err
}

// NewConnectionError 创建一个新的连接错误。
func NewConnectionError(protocol, ip string, port int, stage, message string, err error) *ConnectionError {
	return &ConnectionError{
		Protocol: protocol,
		IP:       ip,
		Port:     port,
		Stage:    stage,
		Message:  message,
		Err:      err,
	}
}
