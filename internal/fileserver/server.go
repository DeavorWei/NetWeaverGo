// Package fileserver 提供内置的轻量级文件服务器功能
// 支持 SFTP、FTP、TFTP 三种协议
package fileserver

import (
	"context"
	"fmt"
	"runtime/debug"
	"sync"
	"time"

	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/models"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// Protocol 协议类型
type Protocol string

const (
	ProtocolSFTP Protocol = "sftp"
	ProtocolFTP  Protocol = "ftp"
	ProtocolTFTP Protocol = "tftp"
	ProtocolHTTP Protocol = "http"
)

// Action 操作类型
type Action string

const (
	ActionConnect    Action = "CONNECT"
	ActionDisconnect Action = "DISCONNECT"
	ActionUpload     Action = "UPLOAD"
	ActionDownload   Action = "DOWNLOAD"
	ActionDelete     Action = "DELETE"
	ActionError      Action = "ERROR"
	ActionBrowse     Action = "BROWSE" // HTTP 目录浏览专用
)

// LogLevel 日志级别
type LogLevel string

const (
	LogLevelInfo    LogLevel = "info"
	LogLevelWarn    LogLevel = "warn"
	LogLevelError   LogLevel = "error"
	LogLevelSuccess LogLevel = "success"
)

// FileServer 文件服务器接口
type FileServer interface {
	// Start 启动服务器
	Start(config *models.FileServerConfig) error
	// Stop 停止服务器
	Stop() error
	// IsRunning 检查服务器是否运行中
	IsRunning() bool
	// DisconnectAll 断开所有客户端连接
	DisconnectAll() error
	// GetProtocol 获取协议类型
	GetProtocol() Protocol
}

// LogEvent 日志事件
type LogEvent struct {
	Timestamp int64    `json:"timestamp"`
	Level     LogLevel `json:"level"`
	Protocol  Protocol `json:"protocol"`
	ClientIP  string   `json:"clientIp"`
	Action    Action   `json:"action"`
	Message   string   `json:"message"`
	File      string   `json:"file,omitempty"`
}

// ServerManager 服务器管理器
type ServerManager struct {
	mu       sync.RWMutex
	servers  map[Protocol]FileServer
	configs  map[Protocol]*models.FileServerConfig
	wailsApp *application.App
	eventMu  sync.Mutex
}

// NewServerManager 创建服务器管理器
func NewServerManager() *ServerManager {
	logger.Debug("FileServer", "-", "创建服务器管理器")
	m := &ServerManager{
		servers: make(map[Protocol]FileServer),
		configs: make(map[Protocol]*models.FileServerConfig),
	}
	// 设置全局管理器引用，用于panic恢复时发送事件
	SetGlobalManager(m)
	return m
}

// SetWailsApp 设置 Wails 应用实例（用于事件推送）
func (m *ServerManager) SetWailsApp(app *application.App) {
	logger.Debug("FileServer", "-", "设置 Wails 应用实例")
	m.wailsApp = app
}

// emitLog 发送日志事件到前端，同时写入日志文件
func (m *ServerManager) emitLog(event LogEvent) {
	m.eventMu.Lock()
	defer m.eventMu.Unlock()

	// 确保时间戳已设置
	if event.Timestamp == 0 {
		event.Timestamp = time.Now().UnixMilli()
	}

	// 构建日志模块名
	module := fmt.Sprintf("FileServer:%s", event.Protocol)

	// 同步写入日志文件（确保所有事件都落盘）
	switch event.Level {
	case LogLevelError:
		logger.Error(module, event.ClientIP, "%s", event.Message)
	case LogLevelWarn:
		logger.Warn(module, event.ClientIP, "%s", event.Message)
	case LogLevelSuccess:
		logger.Info(module, event.ClientIP, "%s", event.Message)
	case LogLevelInfo:
		logger.Info(module, event.ClientIP, "%s", event.Message)
	default:
		logger.Info(module, event.ClientIP, "%s", event.Message)
	}

	// 发送事件到前端
	if m.wailsApp == nil || m.wailsApp.Event == nil {
		logger.Warn("FileServer", "-", "Wails app 未设置，无法发送前端事件")
		return
	}

	m.wailsApp.Event.Emit("fileserver:log", event)
}

// safeGo 安全的 goroutine 启动函数，包含 panic 恢复机制
func safeGo(name string, fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Error("FileServer", "-", "[%s] Goroutine 发生 panic: %v", name, r)
				logger.Error("FileServer", "-", "[%s] Panic 堆栈:\n%s", name, string(debug.Stack()))

				// 尝试通知前端发生错误，防止程序静默崩溃
				// 通过全局管理器发送错误事件
				if manager := getGlobalManager(); manager != nil {
					manager.emitLog(LogEvent{
						Level:    LogLevelError,
						Protocol: ProtocolFTP, // 默认使用FTP协议
						Action:   ActionError,
						Message:  fmt.Sprintf("[%s] 内部错误: %v", name, r),
					})
				}
			}
		}()
		fn()
	}()
}

// globalManager 全局服务器管理器引用，用于panic恢复时发送事件
var globalManager *ServerManager

// getGlobalManager 获取全局管理器
func getGlobalManager() *ServerManager {
	return globalManager
}

// SetGlobalManager 设置全局管理器
func SetGlobalManager(m *ServerManager) {
	globalManager = m
}

// StartServer 启动指定协议的服务器
func (m *ServerManager) StartServer(protocol Protocol, config *models.FileServerConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	logger.Debug("FileServer", "-", "StartServer 被调用: protocol=%s, port=%d, homeDir=%s", protocol, config.Port, config.HomeDir)

	// 检查是否已存在运行中的服务器
	if server, exists := m.servers[protocol]; exists && server.IsRunning() {
		logger.Warn("FileServer", "-", "%s 服务器已在运行中，跳过启动", protocol)
		return fmt.Errorf("%s 服务器已在运行中", protocol)
	}

	// 创建新的服务器实例
	logger.Verbose("FileServer", "-", "正在创建 %s 服务器实例...", protocol)
	server, err := m.createServer(protocol)
	if err != nil {
		logger.Error("FileServer", "-", "创建 %s 服务器实例失败: %v", protocol, err)
		return err
	}

	// 启动服务器
	logger.Verbose("FileServer", "-", "正在启动 %s 服务器 (端口: %d, 根目录: %s)...", protocol, config.Port, config.HomeDir)
	if err := server.Start(config); err != nil {
		logger.Error("FileServer", "-", "启动 %s 服务器失败: %v", protocol, err)
		return err
	}

	m.servers[protocol] = server
	m.configs[protocol] = config

	logger.Info("FileServer", "-", "%s 服务器已成功启动，监听端口 %d", protocol, config.Port)

	m.emitLog(LogEvent{
		Level:    LogLevelInfo,
		Protocol: protocol,
		Action:   ActionConnect,
		Message:  fmt.Sprintf("%s 服务器已启动，监听端口 %d", protocol, config.Port),
	})

	return nil
}

// StopServer 停止指定协议的服务器
func (m *ServerManager) StopServer(protocol Protocol) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	logger.Debug("FileServer", "-", "StopServer 被调用: protocol=%s", protocol)

	server, exists := m.servers[protocol]
	if !exists || !server.IsRunning() {
		logger.Warn("FileServer", "-", "%s 服务器未运行，无法停止", protocol)
		return fmt.Errorf("%s 服务器未运行", protocol)
	}

	logger.Verbose("FileServer", "-", "正在停止 %s 服务器...", protocol)
	if err := server.Stop(); err != nil {
		logger.Error("FileServer", "-", "停止 %s 服务器失败: %v", protocol, err)
		return err
	}

	logger.Info("FileServer", "-", "%s 服务器已成功停止", protocol)

	m.emitLog(LogEvent{
		Level:    LogLevelInfo,
		Protocol: protocol,
		Action:   ActionDisconnect,
		Message:  fmt.Sprintf("%s 服务器已停止", protocol),
	})

	return nil
}

// IsRunning 检查指定协议的服务器是否运行中
func (m *ServerManager) IsRunning(protocol Protocol) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	server, exists := m.servers[protocol]
	if !exists {
		logger.Verbose("FileServer", "-", "IsRunning: %s 服务器实例不存在", protocol)
		return false
	}
	running := server.IsRunning()
	logger.Verbose("FileServer", "-", "IsRunning: %s 服务器运行状态=%v", protocol, running)
	return running
}

// DisconnectAll 断开指定协议服务器的所有客户端连接
func (m *ServerManager) DisconnectAll(protocol Protocol) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	logger.Debug("FileServer", "-", "DisconnectAll 被调用: protocol=%s", protocol)

	server, exists := m.servers[protocol]
	if !exists {
		logger.Warn("FileServer", "-", "%s 服务器未运行，无法断开连接", protocol)
		return fmt.Errorf("%s 服务器未运行", protocol)
	}

	return server.DisconnectAll()
}

// StopAll 停止所有运行中的服务器
func (m *ServerManager) StopAll() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	logger.Info("FileServer", "-", "正在停止所有文件服务器 (共 %d 个)", len(m.servers))

	var lastErr error
	for protocol, server := range m.servers {
		if server.IsRunning() {
			logger.Verbose("FileServer", "-", "正在停止 %s 服务器...", protocol)
			if err := server.Stop(); err != nil {
				lastErr = err
				logger.Error("FileServer", "-", "停止 %s 服务器失败: %v", protocol, err)
			} else {
				logger.Info("FileServer", "-", "%s 服务器已停止", protocol)
				m.emitLog(LogEvent{
					Level:    LogLevelInfo,
					Protocol: protocol,
					Action:   ActionDisconnect,
					Message:  fmt.Sprintf("%s 服务器已停止", protocol),
				})
			}
		}
	}
	return lastErr
}

// createServer 创建指定协议的服务器实例
func (m *ServerManager) createServer(protocol Protocol) (FileServer, error) {
	logger.Debug("FileServer", "-", "createServer: 正在创建 %s 服务器实例", protocol)
	switch protocol {
	case ProtocolSFTP:
		return NewSFTPServer(m), nil
	case ProtocolFTP:
		return NewFTPServer(m), nil
	case ProtocolTFTP:
		return NewTFTPServer(m), nil
	case ProtocolHTTP:
		return NewWebServer(m), nil
	default:
		logger.Error("FileServer", "-", "不支持的协议类型: %s", protocol)
		return nil, fmt.Errorf("不支持的协议: %s", protocol)
	}
}

// GetConfig 获取指定协议的配置
func (m *ServerManager) GetConfig(protocol Protocol) *models.FileServerConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	config := m.configs[protocol]
	logger.Verbose("FileServer", "-", "GetConfig: %s 配置=%+v", protocol, config)
	return config
}

// Shutdown 优雅关闭所有服务器
func (m *ServerManager) Shutdown(ctx context.Context) error {
	logger.Info("FileServer", "-", "正在优雅关闭所有文件服务器...")

	done := make(chan error, 1)
	go func() {
		done <- m.StopAll()
	}()

	select {
	case err := <-done:
		if err != nil {
			logger.Error("FileServer", "-", "优雅关闭文件服务器时出错: %v", err)
		} else {
			logger.Info("FileServer", "-", "所有文件服务器已优雅关闭")
		}
		return err
	case <-ctx.Done():
		logger.Error("FileServer", "-", "关闭文件服务器超时")
		return fmt.Errorf("关闭服务器超时")
	}
}
