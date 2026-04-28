package fileserver

import (
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/models"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// SFTPServer SFTP 服务器实现
type SFTPServer struct {
	mu          sync.RWMutex
	config      *models.FileServerConfig
	sshListener net.Listener
	sshConfig   *ssh.ServerConfig
	running     bool
	manager     *ServerManager
	connections sync.Map
	connWg      sync.WaitGroup
}

// NewSFTPServer 创建 SFTP 服务器实例
func NewSFTPServer(manager *ServerManager) *SFTPServer {
	logger.Debug("FileServer:SFTP", "-", "创建 SFTP 服务器实例")
	return &SFTPServer{
		manager: manager,
	}
}

// Start 启动 SFTP 服务器
func (s *SFTPServer) Start(config *models.FileServerConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	logger.Debug("FileServer:SFTP", "-", "Start 方法被调用")

	if s.running {
		logger.Warn("FileServer:SFTP", "-", "SFTP 服务器已在运行中")
		return fmt.Errorf("SFTP 服务器已在运行中")
	}

	logger.Verbose("FileServer:SFTP", "-", "开始验证配置: Port=%d, HomeDir=%s, Username=%s",
		config.Port, config.HomeDir, config.Username)
	if err := s.validateConfig(config); err != nil {
		logger.Error("FileServer:SFTP", "-", "配置验证失败: %v", err)
		return err
	}
	logger.Verbose("FileServer:SFTP", "-", "配置验证通过")

	s.config = config

	logger.Verbose("FileServer:SFTP", "-", "正在生成 SSH 主机密钥...")
	hostKey, err := s.generateHostKey(config.HomeDir)
	if err != nil {
		logger.Error("FileServer:SFTP", "-", "生成主机密钥失败: %v", err)
		return fmt.Errorf("生成主机密钥失败: %v", err)
	}
	logger.Verbose("FileServer:SFTP", "-", "SSH 主机密钥生成成功")

	s.sshConfig = &ssh.ServerConfig{
		PasswordCallback: s.passwordCallback,
	}
	s.sshConfig.AddHostKey(hostKey)

	addr := fmt.Sprintf(":%d", config.Port)
	logger.Verbose("FileServer:SFTP", "-", "正在监听 TCP 端口: %s", addr)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		logger.Error("FileServer:SFTP", "-", "监听端口 %d 失败: %v", config.Port, err)
		return fmt.Errorf("SFTP 服务器启动失败: %v", err)
	}

	s.sshListener = listener
	s.running = true

	logger.Info("FileServer:SFTP", "-", "SFTP 服务器已启动，监听端口 %d", config.Port)

	// 使用 safeGo 启动连接接受循环
	safeGo("SFTP-acceptConnections", func() {
		s.acceptConnections()
	})

	s.manager.emitLog(LogEvent{
		Level:    LogLevelSuccess,
		Protocol: ProtocolSFTP,
		Action:   ActionConnect,
		Message:  fmt.Sprintf("SFTP 服务器已启动，监听端口 %d", config.Port),
	})

	return nil
}

// Stop 停止 SFTP 服务器
func (s *SFTPServer) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	logger.Debug("FileServer:SFTP", "-", "Stop 方法被调用")

	if !s.running {
		logger.Verbose("FileServer:SFTP", "-", "SFTP 服务器未运行，无需停止")
		return nil
	}

	s.running = false

	if s.sshListener != nil {
		logger.Verbose("FileServer:SFTP", "-", "正在关闭 SSH 监听器...")
		s.sshListener.Close()
	}

	logger.Verbose("FileServer:SFTP", "-", "正在关闭所有客户端连接...")
	s.connections.Range(func(key, value interface{}) bool {
		if conn, ok := value.(net.Conn); ok {
			conn.Close()
		}
		return true
	})

	logger.Verbose("FileServer:SFTP", "-", "等待所有连接处理完成...")
	s.connWg.Wait()

	logger.Info("FileServer:SFTP", "-", "SFTP 服务器已停止")

	s.manager.emitLog(LogEvent{
		Level:    LogLevelInfo,
		Protocol: ProtocolSFTP,
		Action:   ActionDisconnect,
		Message:  "SFTP 服务器已停止",
	})

	return nil
}

// IsRunning 检查服务器是否运行中
func (s *SFTPServer) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	logger.Verbose("FileServer:SFTP", "-", "IsRunning: %v", s.running)
	return s.running
}

// DisconnectAll 断开所有客户端连接
func (s *SFTPServer) DisconnectAll() error {
	logger.Debug("FileServer:SFTP", "-", "DisconnectAll 方法被调用")

	s.connections.Range(func(key, value interface{}) bool {
		if conn, ok := value.(net.Conn); ok {
			logger.Verbose("FileServer:SFTP", "-", "关闭连接: %v", key)
			conn.Close()
		}
		s.connections.Delete(key)
		return true
	})

	logger.Info("FileServer:SFTP", "-", "所有 SFTP 连接已断开")

	s.manager.emitLog(LogEvent{
		Level:    LogLevelInfo,
		Protocol: ProtocolSFTP,
		Action:   ActionDisconnect,
		Message:  "所有 SFTP 连接已断开",
	})

	return nil
}

// GetProtocol 获取协议类型
func (s *SFTPServer) GetProtocol() Protocol {
	return ProtocolSFTP
}

// acceptConnections 接受客户端连接
func (s *SFTPServer) acceptConnections() {
	logger.Info("FileServer:SFTP", "-", "连接接受循环已启动")

	for {
		s.mu.RLock()
		running := s.running
		s.mu.RUnlock()

		if !running {
			logger.Info("FileServer:SFTP", "-", "服务器已停止，退出连接接受循环")
			return
		}

		conn, err := s.sshListener.Accept()
		if err != nil {
			s.mu.RLock()
			running := s.running
			s.mu.RUnlock()
			if !running {
				logger.Info("FileServer:SFTP", "-", "服务器已停止，退出连接接受循环")
				return
			}
			logger.Error("FileServer:SFTP", "-", "接受连接失败: %v", err)
			s.manager.emitLog(LogEvent{
				Level:    LogLevelError,
				Protocol: ProtocolSFTP,
				Action:   ActionError,
				Message:  fmt.Sprintf("接受连接失败: %v", err),
			})
			continue
		}

		clientIP := conn.RemoteAddr().String()
		logger.Verbose("FileServer:SFTP", clientIP, "接受新 TCP 连接")

		s.connWg.Add(1)
		safeGo(fmt.Sprintf("SFTP-handleConnection-%s", clientIP), func() {
			s.handleConnection(conn)
		})
	}
}

// handleConnection 处理单个 SSH 连接
func (s *SFTPServer) handleConnection(conn net.Conn) {
	defer s.connWg.Done()
	defer conn.Close()

	clientIP := conn.RemoteAddr().String()
	connID := conn.RemoteAddr().String()
	s.connections.Store(connID, conn)
	defer s.connections.Delete(connID)

	logger.Verbose("FileServer:SFTP", clientIP, "开始处理 SSH 连接")

	sshConn, chans, reqs, err := ssh.NewServerConn(conn, s.sshConfig)
	if err != nil {
		logger.Error("FileServer:SFTP", clientIP, "SSH 握手失败: %v", err)
		s.manager.emitLog(LogEvent{
			Level:    LogLevelError,
			Protocol: ProtocolSFTP,
			ClientIP: clientIP,
			Action:   ActionError,
			Message:  fmt.Sprintf("SSH 握手失败: %v", err),
		})
		return
	}
	defer sshConn.Close()

	logger.Info("FileServer:SFTP", clientIP, "用户 %s 已连接", sshConn.User())

	s.manager.emitLog(LogEvent{
		Level:    LogLevelInfo,
		Protocol: ProtocolSFTP,
		ClientIP: clientIP,
		Action:   ActionConnect,
		Message:  fmt.Sprintf("用户 %s 已连接", sshConn.User()),
	})

	go ssh.DiscardRequests(reqs)

	logger.Verbose("FileServer:SFTP", clientIP, "开始处理 SSH 通道")

	for newChannel := range chans {
		channelType := newChannel.ChannelType()
		logger.Verbose("FileServer:SFTP", clientIP, "收到通道请求: %s", channelType)

		if channelType != "session" {
			logger.Warn("FileServer:SFTP", clientIP, "拒绝未知通道类型: %s", channelType)
			newChannel.Reject(ssh.UnknownChannelType, "未知的通道类型")
			continue
		}

		channel, requests, err := newChannel.Accept()
		if err != nil {
			logger.Error("FileServer:SFTP", clientIP, "无法接受通道: %v", err)
			s.manager.emitLog(LogEvent{
				Level:    LogLevelError,
				Protocol: ProtocolSFTP,
				ClientIP: clientIP,
				Action:   ActionError,
				Message:  fmt.Sprintf("无法接受通道: %v", err),
			})
			continue
		}

		logger.Verbose("FileServer:SFTP", clientIP, "会话通道已建立")
		go s.handleSession(channel, requests, clientIP)
	}

	logger.Verbose("FileServer:SFTP", clientIP, "SSH 连接处理完成")
}

// handleSession 处理 SFTP 会话
func (s *SFTPServer) handleSession(channel ssh.Channel, requests <-chan *ssh.Request, clientIP string) {
	defer channel.Close()

	logger.Verbose("FileServer:SFTP", clientIP, "开始处理 SFTP 会话")

	for req := range requests {
		logger.Verbose("FileServer:SFTP", clientIP, "收到会话请求: type=%s, wantReply=%v", req.Type, req.WantReply)

		switch req.Type {
		case "subsystem":
			payload := struct{ Subsystem string }{}
			if err := ssh.Unmarshal(req.Payload, &payload); err != nil {
				logger.Warn("FileServer:SFTP", clientIP, "解析 subsystem 请求失败: %v", err)
				if req.WantReply {
					req.Reply(false, nil)
				}
				continue
			}

			logger.Verbose("FileServer:SFTP", clientIP, "subsystem 请求: %s", payload.Subsystem)

			if payload.Subsystem == "sftp" {
				if req.WantReply {
					req.Reply(true, nil)
				}

				s.mu.RLock()
				homeDir := s.config.HomeDir
				s.mu.RUnlock()

				logger.Debug("FileServer:SFTP", clientIP, "启动 SFTP 服务器，根目录: %s", homeDir)

				handler := &sftpHandler{
					homeDir:  homeDir,
					clientIP: clientIP,
					manager:  s.manager,
				}

				handlers := sftp.Handlers{
					FileGet:  handler,
					FilePut:  handler,
					FileCmd:  handler,
					FileList: handler,
				}

				server := sftp.NewRequestServer(channel, handlers)

				logger.Info("FileServer:SFTP", clientIP, "SFTP 会话已开始")

				if err := server.Serve(); err != nil && err != io.EOF {
					logger.Error("FileServer:SFTP", clientIP, "SFTP 会话错误: %v", err)
					s.manager.emitLog(LogEvent{
						Level:    LogLevelError,
						Protocol: ProtocolSFTP,
						ClientIP: clientIP,
						Action:   ActionError,
						Message:  fmt.Sprintf("SFTP 会话错误: %v", err),
					})
				}

				logger.Info("FileServer:SFTP", clientIP, "SFTP 会话已结束")
				s.manager.emitLog(LogEvent{
					Level:    LogLevelInfo,
					Protocol: ProtocolSFTP,
					ClientIP: clientIP,
					Action:   ActionDisconnect,
					Message:  "SFTP 会话已结束",
				})
				return
			}

			logger.Warn("FileServer:SFTP", clientIP, "未知的 subsystem: %s", payload.Subsystem)
			if req.WantReply {
				req.Reply(false, nil)
			}
		default:
			logger.Verbose("FileServer:SFTP", clientIP, "忽略请求类型: %s", req.Type)
			if req.WantReply {
				req.Reply(false, nil)
			}
		}
	}
}

// passwordCallback SSH 密码认证回调
func (s *SFTPServer) passwordCallback(conn ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
	s.mu.RLock()
	config := s.config
	s.mu.RUnlock()

	clientIP := conn.RemoteAddr().String()
	user := conn.User()

	logger.Debug("FileServer:SFTP", clientIP, "认证请求: user=%s", user)

	if user != config.Username || string(password) != config.Password {
		logger.Warn("FileServer:SFTP", clientIP, "认证失败: 用户名或密码错误 (user: %s)", user)
		s.manager.emitLog(LogEvent{
			Level:    LogLevelWarn,
			Protocol: ProtocolSFTP,
			ClientIP: clientIP,
			Action:   ActionError,
			Message:  fmt.Sprintf("认证失败: 用户名或密码错误 (user: %s)", user),
		})
		return nil, fmt.Errorf("认证失败")
	}

	logger.Info("FileServer:SFTP", clientIP, "用户 %s 认证成功", user)
	s.manager.emitLog(LogEvent{
		Level:    LogLevelSuccess,
		Protocol: ProtocolSFTP,
		ClientIP: clientIP,
		Action:   ActionConnect,
		Message:  fmt.Sprintf("用户 %s 认证成功", user),
	})

	return nil, nil
}

// generateHostKey 生成 SSH 主机密钥
func (s *SFTPServer) generateHostKey(homeDir string) (ssh.Signer, error) {
	keyPath := filepath.Join(homeDir, ".sftp_host_key")
	logger.Verbose("FileServer:SFTP", "-", "检查主机密钥文件: %s", keyPath)

	if data, err := os.ReadFile(keyPath); err == nil {
		logger.Verbose("FileServer:SFTP", "-", "找到已有主机密钥文件，尝试解析...")
		signer, err := ssh.ParsePrivateKey(data)
		if err == nil {
			logger.Verbose("FileServer:SFTP", "-", "已有主机密钥解析成功")
			return signer, nil
		}
		logger.Warn("FileServer:SFTP", "-", "已有主机密钥解析失败: %v，将重新生成", err)
	} else {
		logger.Verbose("FileServer:SFTP", "-", "未找到已有主机密钥文件: %v", err)
	}

	logger.Verbose("FileServer:SFTP", "-", "正在生成新的 RSA 密钥 (2048 bit)...")
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		logger.Error("FileServer:SFTP", "-", "生成 RSA 密钥失败: %v", err)
		return nil, fmt.Errorf("生成 RSA 密钥失败: %v", err)
	}

	signer, err := ssh.NewSignerFromKey(privateKey)
	if err != nil {
		logger.Error("FileServer:SFTP", "-", "创建签名器失败: %v", err)
		return nil, fmt.Errorf("创建签名器失败: %v", err)
	}

	keyDir := filepath.Dir(keyPath)
	if err := os.MkdirAll(keyDir, 0700); err != nil {
		logger.Warn("FileServer:SFTP", "-", "创建密钥目录失败: %v", err)
	} else {
		logger.Verbose("FileServer:SFTP", "-", "密钥目录已就绪: %s", keyDir)
	}

	logger.Verbose("FileServer:SFTP", "-", "新主机密钥生成成功")
	return signer, nil
}

// validateConfig 验证配置
func (s *SFTPServer) validateConfig(config *models.FileServerConfig) error {
	logger.Verbose("FileServer:SFTP", "-", "验证端口号: %d", config.Port)
	if config.Port <= 0 || config.Port > 65535 {
		logger.Error("FileServer:SFTP", "-", "无效的端口号: %d", config.Port)
		return fmt.Errorf("无效的端口号: %d", config.Port)
	}

	logger.Verbose("FileServer:SFTP", "-", "验证根目录: %s", config.HomeDir)
	if config.HomeDir == "" {
		logger.Error("FileServer:SFTP", "-", "根目录不能为空")
		return fmt.Errorf("根目录不能为空")
	}

	logger.Verbose("FileServer:SFTP", "-", "检查/创建根目录: %s", config.HomeDir)
	if err := os.MkdirAll(config.HomeDir, 0755); err != nil {
		logger.Error("FileServer:SFTP", "-", "无法创建根目录 %s: %v", config.HomeDir, err)
		return fmt.Errorf("无法创建根目录: %v", err)
	}

	logger.Verbose("FileServer:SFTP", "-", "配置验证通过")
	return nil
}

// sftpHandler SFTP 文件系统处理器
type sftpHandler struct {
	homeDir  string
	clientIP string
	manager  *ServerManager
}

// Fileread 打开文件用于读取
func (h *sftpHandler) Fileread(r *sftp.Request) (io.ReaderAt, error) {
	logger.Verbose("FileServer:SFTP", h.clientIP, "Fileread: %s", r.Filepath)

	safePath, err := h.safePath(r.Filepath)
	if err != nil {
		logger.Warn("FileServer:SFTP", h.clientIP, "Fileread 路径安全检查失败: %s, err: %v", r.Filepath, err)
		return nil, sftp.ErrSSHFxPermissionDenied
	}

	file, err := os.Open(safePath)
	if err != nil {
		logger.Warn("FileServer:SFTP", h.clientIP, "Fileread 打开文件失败: %s, err: %v", r.Filepath, err)
		return nil, sftp.ErrSSHFxNoSuchFile
	}

	logger.Debug("FileServer:SFTP", h.clientIP, "读取文件: %s", r.Filepath)
	h.manager.emitLog(LogEvent{
		Level:    LogLevelInfo,
		Protocol: ProtocolSFTP,
		ClientIP: h.clientIP,
		Action:   ActionDownload,
		Message:  fmt.Sprintf("读取文件 %s", r.Filepath),
		File:     r.Filepath,
	})

	return file, nil
}

// Filewrite 打开文件用于写入
func (h *sftpHandler) Filewrite(r *sftp.Request) (io.WriterAt, error) {
	logger.Verbose("FileServer:SFTP", h.clientIP, "Filewrite: %s", r.Filepath)

	safePath, err := h.safePath(r.Filepath)
	if err != nil {
		logger.Warn("FileServer:SFTP", h.clientIP, "Filewrite 路径安全检查失败: %s, err: %v", r.Filepath, err)
		return nil, sftp.ErrSSHFxPermissionDenied
	}

	dir := filepath.Dir(safePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		logger.Error("FileServer:SFTP", h.clientIP, "Filewrite 创建目录失败: %s, err: %v", dir, err)
		return nil, sftp.ErrSSHFxFailure
	}

	file, err := os.OpenFile(safePath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		logger.Error("FileServer:SFTP", h.clientIP, "Filewrite 打开文件失败: %s, err: %v", r.Filepath, err)
		return nil, sftp.ErrSSHFxFailure
	}

	logger.Debug("FileServer:SFTP", h.clientIP, "写入文件: %s", r.Filepath)
	h.manager.emitLog(LogEvent{
		Level:    LogLevelInfo,
		Protocol: ProtocolSFTP,
		ClientIP: h.clientIP,
		Action:   ActionUpload,
		Message:  fmt.Sprintf("写入文件 %s", r.Filepath),
		File:     r.Filepath,
	})

	return file, nil
}

// Filecmd 处理文件命令
func (h *sftpHandler) Filecmd(r *sftp.Request) error {
	logger.Verbose("FileServer:SFTP", h.clientIP, "Filecmd: method=%s, path=%s", r.Method, r.Filepath)

	safePath, err := h.safePath(r.Filepath)
	if err != nil {
		logger.Warn("FileServer:SFTP", h.clientIP, "Filecmd 路径安全检查失败: %s, err: %v", r.Filepath, err)
		return sftp.ErrSSHFxPermissionDenied
	}

	switch r.Method {
	case "Setstat":
		logger.Verbose("FileServer:SFTP", h.clientIP, "Setstat: %s (忽略)", r.Filepath)
		return nil
	case "Rename":
		safeTarget, err := h.safePath(r.Target)
		if err != nil {
			logger.Warn("FileServer:SFTP", h.clientIP, "Rename 目标路径安全检查失败: %s, err: %v", r.Target, err)
			return sftp.ErrSSHFxPermissionDenied
		}
		if err := os.Rename(safePath, safeTarget); err != nil {
			logger.Error("FileServer:SFTP", h.clientIP, "Rename 失败: %s -> %s, err: %v", r.Filepath, r.Target, err)
			return sftp.ErrSSHFxFailure
		}
		logger.Info("FileServer:SFTP", h.clientIP, "重命名: %s -> %s", r.Filepath, r.Target)
		h.manager.emitLog(LogEvent{
			Level:    LogLevelSuccess,
			Protocol: ProtocolSFTP,
			ClientIP: h.clientIP,
			Action:   ActionUpload,
			Message:  fmt.Sprintf("重命名 %s -> %s", r.Filepath, r.Target),
			File:     r.Filepath,
		})
	case "Rmdir":
		if err := os.Remove(safePath); err != nil {
			logger.Error("FileServer:SFTP", h.clientIP, "Rmdir 失败: %s, err: %v", r.Filepath, err)
			return sftp.ErrSSHFxFailure
		}
		logger.Info("FileServer:SFTP", h.clientIP, "删除目录: %s", r.Filepath)
		h.manager.emitLog(LogEvent{
			Level:    LogLevelSuccess,
			Protocol: ProtocolSFTP,
			ClientIP: h.clientIP,
			Action:   ActionDelete,
			Message:  fmt.Sprintf("删除目录 %s", r.Filepath),
			File:     r.Filepath,
		})
	case "Remove":
		if err := os.Remove(safePath); err != nil {
			logger.Error("FileServer:SFTP", h.clientIP, "Remove 失败: %s, err: %v", r.Filepath, err)
			return sftp.ErrSSHFxFailure
		}
		logger.Info("FileServer:SFTP", h.clientIP, "删除文件: %s", r.Filepath)
		h.manager.emitLog(LogEvent{
			Level:    LogLevelSuccess,
			Protocol: ProtocolSFTP,
			ClientIP: h.clientIP,
			Action:   ActionDelete,
			Message:  fmt.Sprintf("删除文件 %s", r.Filepath),
			File:     r.Filepath,
		})
	case "Mkdir":
		if err := os.MkdirAll(safePath, 0755); err != nil {
			logger.Error("FileServer:SFTP", h.clientIP, "Mkdir 失败: %s, err: %v", r.Filepath, err)
			return sftp.ErrSSHFxFailure
		}
		logger.Info("FileServer:SFTP", h.clientIP, "创建目录: %s", r.Filepath)
		h.manager.emitLog(LogEvent{
			Level:    LogLevelSuccess,
			Protocol: ProtocolSFTP,
			ClientIP: h.clientIP,
			Action:   ActionUpload,
			Message:  fmt.Sprintf("创建目录 %s", r.Filepath),
			File:     r.Filepath,
		})
	case "Symlink":
		safeTarget, err := h.safePath(r.Target)
		if err != nil {
			logger.Warn("FileServer:SFTP", h.clientIP, "Symlink 目标路径安全检查失败: %s, err: %v", r.Target, err)
			return sftp.ErrSSHFxPermissionDenied
		}
		if err := os.Symlink(safeTarget, safePath); err != nil {
			logger.Error("FileServer:SFTP", h.clientIP, "Symlink 失败: %s -> %s, err: %v", safeTarget, safePath, err)
			return sftp.ErrSSHFxFailure
		}
		logger.Info("FileServer:SFTP", h.clientIP, "创建符号链接: %s -> %s", r.Filepath, r.Target)
	}

	return nil
}

// PosixRename POSIX 重命名
func (h *sftpHandler) PosixRename(r *sftp.Request) error {
	logger.Verbose("FileServer:SFTP", h.clientIP, "PosixRename: %s -> %s", r.Filepath, r.Target)

	safePath, err := h.safePath(r.Filepath)
	if err != nil {
		logger.Warn("FileServer:SFTP", h.clientIP, "PosixRename 源路径安全检查失败: %s, err: %v", r.Filepath, err)
		return sftp.ErrSSHFxPermissionDenied
	}
	safeTarget, err := h.safePath(r.Target)
	if err != nil {
		logger.Warn("FileServer:SFTP", h.clientIP, "PosixRename 目标路径安全检查失败: %s, err: %v", r.Target, err)
		return sftp.ErrSSHFxPermissionDenied
	}

	if err := os.Rename(safePath, safeTarget); err != nil {
		logger.Error("FileServer:SFTP", h.clientIP, "PosixRename 失败: %s -> %s, err: %v", r.Filepath, r.Target, err)
		return sftp.ErrSSHFxFailure
	}

	logger.Info("FileServer:SFTP", h.clientIP, "POSIX 重命名: %s -> %s", r.Filepath, r.Target)
	h.manager.emitLog(LogEvent{
		Level:    LogLevelSuccess,
		Protocol: ProtocolSFTP,
		ClientIP: h.clientIP,
		Action:   ActionUpload,
		Message:  fmt.Sprintf("重命名 %s -> %s", r.Filepath, r.Target),
		File:     r.Filepath,
	})

	return nil
}

// Stat 获取文件信息
func (h *sftpHandler) Stat(r *sftp.Request) (sftp.ListerAt, error) {
	logger.Verbose("FileServer:SFTP", h.clientIP, "Stat: %s", r.Filepath)

	safePath, err := h.safePath(r.Filepath)
	if err != nil {
		logger.Warn("FileServer:SFTP", h.clientIP, "Stat 路径安全检查失败: %s, err: %v", r.Filepath, err)
		return nil, sftp.ErrSSHFxPermissionDenied
	}

	stat, err := os.Stat(safePath)
	if err != nil {
		logger.Verbose("FileServer:SFTP", h.clientIP, "Stat 文件不存在: %s, err: %v", r.Filepath, err)
		return nil, sftp.ErrSSHFxNoSuchFile
	}

	return listerAt([]os.FileInfo{stat}), nil
}

// Readlink 读取符号链接
func (h *sftpHandler) Readlink(r *sftp.Request) (sftp.ListerAt, error) {
	logger.Verbose("FileServer:SFTP", h.clientIP, "Readlink: %s", r.Filepath)

	safePath, err := h.safePath(r.Filepath)
	if err != nil {
		logger.Warn("FileServer:SFTP", h.clientIP, "Readlink 路径安全检查失败: %s, err: %v", r.Filepath, err)
		return nil, sftp.ErrSSHFxPermissionDenied
	}

	target, err := os.Readlink(safePath)
	if err != nil {
		logger.Warn("FileServer:SFTP", h.clientIP, "Readlink 失败: %s, err: %v", r.Filepath, err)
		return nil, sftp.ErrSSHFxFailure
	}

	logger.Verbose("FileServer:SFTP", h.clientIP, "Readlink 结果: %s -> %s", r.Filepath, target)

	// 返回符号链接目标路径
	fi := &symlinkFileInfo{name: target}
	return listerAt([]os.FileInfo{fi}), nil
}

// Filelist 列出目录内容 (FileLister 接口)
func (h *sftpHandler) Filelist(r *sftp.Request) (sftp.ListerAt, error) {
	logger.Verbose("FileServer:SFTP", h.clientIP, "Filelist: %s", r.Filepath)

	safePath, err := h.safePath(r.Filepath)
	if err != nil {
		logger.Warn("FileServer:SFTP", h.clientIP, "Filelist 路径安全检查失败: %s, err: %v", r.Filepath, err)
		return nil, sftp.ErrSSHFxPermissionDenied
	}

	dir, err := os.Open(safePath)
	if err != nil {
		logger.Warn("FileServer:SFTP", h.clientIP, "Filelist 打开目录失败: %s, err: %v", r.Filepath, err)
		return nil, sftp.ErrSSHFxNoSuchFile
	}
	defer dir.Close()

	files, err := dir.Readdir(-1)
	if err != nil {
		logger.Error("FileServer:SFTP", h.clientIP, "Filelist 读取目录失败: %s, err: %v", r.Filepath, err)
		return nil, sftp.ErrSSHFxFailure
	}

	logger.Debug("FileServer:SFTP", h.clientIP, "列出目录: %s (%d 个文件)", r.Filepath, len(files))
	h.manager.emitLog(LogEvent{
		Level:    LogLevelInfo,
		Protocol: ProtocolSFTP,
		ClientIP: h.clientIP,
		Action:   ActionDownload,
		Message:  fmt.Sprintf("列出目录 %s (%d 个文件)", r.Filepath, len(files)),
		File:     r.Filepath,
	})

	return listerAt(files), nil
}

// safePath 安全路径处理，防止路径穿越
func (h *sftpHandler) safePath(name string) (string, error) {
	name = filepath.Clean(name)
	name = strings.TrimPrefix(name, string(os.PathSeparator))
	name = strings.TrimPrefix(name, "/")

	fullPath := filepath.Join(h.homeDir, name)

	absBase, err := filepath.Abs(h.homeDir)
	if err != nil {
		logger.Error("FileServer:SFTP", h.clientIP, "safePath: 无法获取基础目录绝对路径: %v", err)
		return "", fmt.Errorf("无法获取基础目录绝对路径")
	}

	absPath, err := filepath.Abs(fullPath)
	if err != nil {
		logger.Error("FileServer:SFTP", h.clientIP, "safePath: 无法获取目标路径绝对路径: %v", err)
		return "", fmt.Errorf("无法获取目标路径绝对路径")
	}

	if !strings.HasPrefix(absPath, absBase+string(os.PathSeparator)) && absPath != absBase {
		logger.Warn("FileServer:SFTP", h.clientIP, "safePath: 路径穿越攻击被阻止: name=%s, absPath=%s, absBase=%s", name, absPath, absBase)
		return "", fmt.Errorf("路径穿越攻击被阻止")
	}

	return absPath, nil
}

// listerAt 实现 sftp.ListerAt 接口
type listerAt []os.FileInfo

func (l listerAt) ListAt(f []os.FileInfo, offset int64) (int, error) {
	n := len(f)
	if offset >= int64(len(l)) {
		return 0, io.EOF
	}
	copied := 0
	for i := offset; i < int64(len(l)) && copied < n; i++ {
		f[copied] = l[i]
		copied++
	}
	return copied, nil
}

// symlinkFileInfo 符号链接目标路径信息
type symlinkFileInfo struct {
	name string
}

func (fi *symlinkFileInfo) Name() string       { return fi.name }
func (fi *symlinkFileInfo) Size() int64        { return 0 }
func (fi *symlinkFileInfo) Mode() os.FileMode  { return os.ModeSymlink }
func (fi *symlinkFileInfo) ModTime() time.Time { return time.Time{} }
func (fi *symlinkFileInfo) IsDir() bool        { return false }
func (fi *symlinkFileInfo) Sys() interface{}   { return nil }
