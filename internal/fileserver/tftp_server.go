package fileserver

import (
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/models"
	"github.com/pin/tftp/v3"
)

// TFTPServer TFTP 服务器实现
type TFTPServer struct {
	mu      sync.RWMutex
	config  *models.FileServerConfig
	server  *tftp.Server
	running bool
	manager *ServerManager
	connMap sync.Map // 存储活动连接
	connWg  sync.WaitGroup
}

// NewTFTPServer 创建 TFTP 服务器实例
func NewTFTPServer(manager *ServerManager) *TFTPServer {
	logger.Debug("FileServer:TFTP", "-", "创建 TFTP 服务器实例")
	return &TFTPServer{
		manager: manager,
	}
}

// Start 启动 TFTP 服务器
func (s *TFTPServer) Start(config *models.FileServerConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	logger.Debug("FileServer:TFTP", "-", "Start 方法被调用")

	if s.running {
		logger.Warn("FileServer:TFTP", "-", "TFTP 服务器已在运行中")
		return fmt.Errorf("TFTP 服务器已在运行中")
	}

	// 验证配置
	logger.Verbose("FileServer:TFTP", "-", "开始验证配置: Port=%d, HomeDir=%s", config.Port, config.HomeDir)
	if err := s.validateConfig(config); err != nil {
		logger.Error("FileServer:TFTP", "-", "配置验证失败: %v", err)
		return err
	}
	logger.Verbose("FileServer:TFTP", "-", "配置验证通过")

	s.config = config

	// 创建 TFTP 服务器
	logger.Verbose("FileServer:TFTP", "-", "正在创建 TFTP 服务器实例...")
	s.server = tftp.NewServer(s.handleRead, s.handleWrite)

	// 启动服务器（在 goroutine 中）
	addr := fmt.Sprintf(":%d", config.Port)
	logger.Info("FileServer:TFTP", "-", "正在启动 TFTP 服务器，监听端口 %d...", config.Port)

	s.running = true

	safeGo("TFTP-Serve", func() {
		// 监听 UDP 端口
		logger.Verbose("FileServer:TFTP", "-", "正在监听 UDP 端口: %s", addr)
		conn, err := net.ListenPacket("udp", addr)
		if err != nil {
			s.mu.Lock()
			s.running = false
			s.mu.Unlock()
			logger.Error("FileServer:TFTP", "-", "TFTP 服务器启动失败，无法监听端口 %d: %v", config.Port, err)
			s.manager.emitLog(LogEvent{
				Level:    LogLevelError,
				Protocol: ProtocolTFTP,
				Action:   ActionError,
				Message:  fmt.Sprintf("TFTP 服务器启动失败: %v", err),
			})
			return
		}

		logger.Info("FileServer:TFTP", "-", "TFTP 服务器已成功启动，监听端口 %d", config.Port)
		s.manager.emitLog(LogEvent{
			Level:    LogLevelSuccess,
			Protocol: ProtocolTFTP,
			Action:   ActionConnect,
			Message:  fmt.Sprintf("TFTP 服务器已启动，监听端口 %d", config.Port),
		})

		// 服务器会阻塞直到停止
		logger.Verbose("FileServer:TFTP", "-", "TFTP 服务器开始处理请求...")
		s.server.Serve(conn)
		logger.Info("FileServer:TFTP", "-", "TFTP 服务器 Serve 已退出")
	})

	return nil
}

// Stop 停止 TFTP 服务器
func (s *TFTPServer) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	logger.Debug("FileServer:TFTP", "-", "Stop 方法被调用")

	if !s.running {
		logger.Verbose("FileServer:TFTP", "-", "TFTP 服务器未运行，无需停止")
		return nil
	}

	// TFTP 服务器没有直接的 Stop 方法，需要关闭底层连接
	// 通过设置 running = false 来阻止新的请求处理
	s.running = false

	logger.Verbose("FileServer:TFTP", "-", "等待所有连接处理完成...")
	s.connWg.Wait()

	logger.Info("FileServer:TFTP", "-", "TFTP 服务器已停止")

	s.manager.emitLog(LogEvent{
		Level:    LogLevelInfo,
		Protocol: ProtocolTFTP,
		Action:   ActionDisconnect,
		Message:  "TFTP 服务器已停止",
	})

	return nil
}

// IsRunning 检查服务器是否运行中
func (s *TFTPServer) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	logger.Verbose("FileServer:TFTP", "-", "IsRunning: %v", s.running)
	return s.running
}

// DisconnectAll 断开所有客户端连接
func (s *TFTPServer) DisconnectAll() error {
	logger.Debug("FileServer:TFTP", "-", "DisconnectAll 方法被调用")

	// TFTP 是无状态协议，每个请求独立处理
	// 这里我们等待所有正在处理的请求完成
	s.connWg.Wait()

	logger.Info("FileServer:TFTP", "-", "所有 TFTP 连接已断开")

	s.manager.emitLog(LogEvent{
		Level:    LogLevelInfo,
		Protocol: ProtocolTFTP,
		Action:   ActionDisconnect,
		Message:  "所有 TFTP 连接已断开",
	})

	return nil
}

// GetProtocol 获取协议类型
func (s *TFTPServer) GetProtocol() Protocol {
	return ProtocolTFTP
}

// handleRead 处理读请求（下载）
func (s *TFTPServer) handleRead(filename string, rf io.ReaderFrom) error {
	s.mu.RLock()
	config := s.config
	running := s.running
	s.mu.RUnlock()

	logger.Verbose("FileServer:TFTP", "-", "handleRead 被调用: filename=%s, running=%v", filename, running)

	if !running {
		logger.Warn("FileServer:TFTP", "-", "handleRead: 服务器未运行，拒绝请求")
		return fmt.Errorf("服务器未运行")
	}

	// 增加连接计数
	s.connWg.Add(1)
	defer s.connWg.Done()

	// 获取客户端地址
	clientAddr := "unknown"
	if raddr := rf.(tftp.OutgoingTransfer).RemoteAddr(); raddr.IP != nil {
		clientAddr = raddr.String()
	}

	logger.Debug("FileServer:TFTP", clientAddr, "处理读请求: %s", filename)

	// 安全检查：防止路径穿越
	safePath, err := s.safePath(config.HomeDir, filename)
	if err != nil {
		logger.Warn("FileServer:TFTP", clientAddr, "拒绝访问文件 %s: %v", filename, err)
		s.manager.emitLog(LogEvent{
			Level:    LogLevelError,
			Protocol: ProtocolTFTP,
			ClientIP: clientAddr,
			Action:   ActionError,
			Message:  fmt.Sprintf("拒绝访问文件 %s: %v", filename, err),
			File:     filename,
		})
		return err
	}

	logger.Verbose("FileServer:TFTP", clientAddr, "安全路径: %s", safePath)

	// 打开文件
	file, err := os.Open(safePath)
	if err != nil {
		logger.Warn("FileServer:TFTP", clientAddr, "无法打开文件 %s: %v", filename, err)
		s.manager.emitLog(LogEvent{
			Level:    LogLevelError,
			Protocol: ProtocolTFTP,
			ClientIP: clientAddr,
			Action:   ActionError,
			Message:  fmt.Sprintf("无法打开文件 %s: %v", filename, err),
			File:     filename,
		})
		return err
	}
	defer file.Close()

	// 获取文件信息
	fileInfo, _ := file.Stat()

	logger.Info("FileServer:TFTP", clientAddr, "开始发送文件 %s (%d bytes)", filename, fileInfo.Size())
	s.manager.emitLog(LogEvent{
		Level:    LogLevelInfo,
		Protocol: ProtocolTFTP,
		ClientIP: clientAddr,
		Action:   ActionDownload,
		Message:  fmt.Sprintf("开始发送文件 %s (%d bytes)", filename, fileInfo.Size()),
		File:     filename,
	})

	// 发送文件
	n, err := rf.ReadFrom(file)
	if err != nil {
		logger.Error("FileServer:TFTP", clientAddr, "发送文件 %s 失败: %v", filename, err)
		s.manager.emitLog(LogEvent{
			Level:    LogLevelError,
			Protocol: ProtocolTFTP,
			ClientIP: clientAddr,
			Action:   ActionError,
			Message:  fmt.Sprintf("发送文件 %s 失败: %v", filename, err),
			File:     filename,
		})
		return err
	}

	logger.Info("FileServer:TFTP", clientAddr, "成功发送文件 %s (%d bytes)", filename, n)
	s.manager.emitLog(LogEvent{
		Level:    LogLevelSuccess,
		Protocol: ProtocolTFTP,
		ClientIP: clientAddr,
		Action:   ActionDownload,
		Message:  fmt.Sprintf("成功发送文件 %s (%d bytes)", filename, n),
		File:     filename,
	})

	return nil
}

// handleWrite 处理写请求（上传）
func (s *TFTPServer) handleWrite(filename string, wt io.WriterTo) error {
	s.mu.RLock()
	config := s.config
	running := s.running
	s.mu.RUnlock()

	logger.Verbose("FileServer:TFTP", "-", "handleWrite 被调用: filename=%s, running=%v", filename, running)

	if !running {
		logger.Warn("FileServer:TFTP", "-", "handleWrite: 服务器未运行，拒绝请求")
		return fmt.Errorf("服务器未运行")
	}

	// 增加连接计数
	s.connWg.Add(1)
	defer s.connWg.Done()

	// 获取客户端地址
	clientAddr := "unknown"
	if raddr := wt.(tftp.IncomingTransfer).RemoteAddr(); raddr.IP != nil {
		clientAddr = raddr.String()
	}

	logger.Debug("FileServer:TFTP", clientAddr, "处理写请求: %s", filename)

	// 安全检查：防止路径穿越
	safePath, err := s.safePath(config.HomeDir, filename)
	if err != nil {
		logger.Warn("FileServer:TFTP", clientAddr, "拒绝写入文件 %s: %v", filename, err)
		s.manager.emitLog(LogEvent{
			Level:    LogLevelError,
			Protocol: ProtocolTFTP,
			ClientIP: clientAddr,
			Action:   ActionError,
			Message:  fmt.Sprintf("拒绝写入文件 %s: %v", filename, err),
			File:     filename,
		})
		return err
	}

	logger.Verbose("FileServer:TFTP", clientAddr, "安全路径: %s", safePath)

	// 确保目录存在
	dir := filepath.Dir(safePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		logger.Error("FileServer:TFTP", clientAddr, "无法创建目录 %s: %v", dir, err)
		s.manager.emitLog(LogEvent{
			Level:    LogLevelError,
			Protocol: ProtocolTFTP,
			ClientIP: clientAddr,
			Action:   ActionError,
			Message:  fmt.Sprintf("无法创建目录 %s: %v", dir, err),
			File:     filename,
		})
		return err
	}

	// 创建文件
	file, err := os.Create(safePath)
	if err != nil {
		logger.Error("FileServer:TFTP", clientAddr, "无法创建文件 %s: %v", filename, err)
		s.manager.emitLog(LogEvent{
			Level:    LogLevelError,
			Protocol: ProtocolTFTP,
			ClientIP: clientAddr,
			Action:   ActionError,
			Message:  fmt.Sprintf("无法创建文件 %s: %v", filename, err),
			File:     filename,
		})
		return err
	}
	defer file.Close()

	logger.Info("FileServer:TFTP", clientAddr, "开始接收文件 %s", filename)
	s.manager.emitLog(LogEvent{
		Level:    LogLevelInfo,
		Protocol: ProtocolTFTP,
		ClientIP: clientAddr,
		Action:   ActionUpload,
		Message:  fmt.Sprintf("开始接收文件 %s", filename),
		File:     filename,
	})

	// 接收文件
	n, err := wt.WriteTo(file)
	if err != nil {
		logger.Error("FileServer:TFTP", clientAddr, "接收文件 %s 失败: %v", filename, err)
		s.manager.emitLog(LogEvent{
			Level:    LogLevelError,
			Protocol: ProtocolTFTP,
			ClientIP: clientAddr,
			Action:   ActionError,
			Message:  fmt.Sprintf("接收文件 %s 失败: %v", filename, err),
			File:     filename,
		})
		return err
	}

	logger.Info("FileServer:TFTP", clientAddr, "成功接收文件 %s (%d bytes)", filename, n)
	s.manager.emitLog(LogEvent{
		Level:    LogLevelSuccess,
		Protocol: ProtocolTFTP,
		ClientIP: clientAddr,
		Action:   ActionUpload,
		Message:  fmt.Sprintf("成功接收文件 %s (%d bytes)", filename, n),
		File:     filename,
	})

	return nil
}

// safePath 安全路径处理，防止路径穿越
func (s *TFTPServer) safePath(baseDir, filename string) (string, error) {
	logger.Verbose("FileServer:TFTP", "-", "safePath: baseDir=%s, filename=%s", baseDir, filename)

	// 清理路径
	filename = filepath.Clean(filename)

	// 移除开头的斜杠
	filename = strings.TrimPrefix(filename, string(os.PathSeparator))
	filename = strings.TrimPrefix(filename, "/")

	// 构建完整路径
	fullPath := filepath.Join(baseDir, filename)

	// 获取绝对路径
	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		logger.Error("FileServer:TFTP", "-", "safePath: 无法获取基础目录绝对路径: %v", err)
		return "", fmt.Errorf("无法获取基础目录绝对路径")
	}

	absPath, err := filepath.Abs(fullPath)
	if err != nil {
		logger.Error("FileServer:TFTP", "-", "safePath: 无法获取目标路径绝对路径: %v", err)
		return "", fmt.Errorf("无法获取目标路径绝对路径")
	}

	// 检查目标路径是否在基础目录内
	if !strings.HasPrefix(absPath, absBase+string(os.PathSeparator)) && absPath != absBase {
		logger.Warn("FileServer:TFTP", "-", "safePath: 路径穿越攻击被阻止: filename=%s, absPath=%s, absBase=%s", filename, absPath, absBase)
		return "", fmt.Errorf("路径穿越攻击被阻止")
	}

	return absPath, nil
}

// validateConfig 验证配置
func (s *TFTPServer) validateConfig(config *models.FileServerConfig) error {
	logger.Verbose("FileServer:TFTP", "-", "验证端口号: %d", config.Port)
	if config.Port <= 0 || config.Port > 65535 {
		logger.Error("FileServer:TFTP", "-", "无效的端口号: %d", config.Port)
		return fmt.Errorf("无效的端口号: %d", config.Port)
	}

	logger.Verbose("FileServer:TFTP", "-", "验证根目录: %s", config.HomeDir)
	if config.HomeDir == "" {
		logger.Error("FileServer:TFTP", "-", "根目录不能为空")
		return fmt.Errorf("根目录不能为空")
	}

	// 检查目录是否存在，不存在则创建
	logger.Verbose("FileServer:TFTP", "-", "检查/创建根目录: %s", config.HomeDir)
	if err := os.MkdirAll(config.HomeDir, 0755); err != nil {
		logger.Error("FileServer:TFTP", "-", "无法创建根目录 %s: %v", config.HomeDir, err)
		return fmt.Errorf("无法创建根目录: %v", err)
	}

	logger.Verbose("FileServer:TFTP", "-", "配置验证通过")
	return nil
}
