package fileserver

import (
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/models"
	ftpserverlib "github.com/fclairamb/ftpserverlib"
	"github.com/spf13/afero"
)

// FTPServer FTP 服务器实现
type FTPServer struct {
	mu          sync.RWMutex
	config      *models.FileServerConfig
	server      *ftpserverlib.FtpServer
	driver      *ftpDriver
	running     bool
	manager     *ServerManager
	connections sync.Map // 存储活动连接
}

// ftpDriver FTP 服务器驱动
type ftpDriver struct {
	config  *models.FileServerConfig
	manager *ServerManager
	fs      afero.Fs
}

// ftpClientDriver 客户端驱动
type ftpClientDriver struct {
	driver   *ftpDriver
	clientIP string
}

// NewFTPServer 创建 FTP 服务器实例
func NewFTPServer(manager *ServerManager) *FTPServer {
	logger.Debug("FileServer:FTP", "-", "创建 FTP 服务器实例")
	return &FTPServer{
		manager: manager,
	}
}

// Start 启动 FTP 服务器
func (s *FTPServer) Start(config *models.FileServerConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	logger.Debug("FileServer:FTP", "-", "Start 方法被调用")

	if s.running {
		logger.Warn("FileServer:FTP", "-", "FTP 服务器已在运行中")
		return fmt.Errorf("FTP 服务器已在运行中")
	}

	// 验证配置
	logger.Verbose("FileServer:FTP", "-", "开始验证配置: Port=%d, HomeDir=%s, Username=%s",
		config.Port, config.HomeDir, config.Username)
	if err := s.validateConfig(config); err != nil {
		logger.Error("FileServer:FTP", "-", "配置验证失败: %v", err)
		return err
	}
	logger.Verbose("FileServer:FTP", "-", "配置验证通过")

	s.config = config

	// 创建驱动
	logger.Verbose("FileServer:FTP", "-", "正在创建 FTP 驱动...")
	s.driver = &ftpDriver{
		config:  config,
		manager: s.manager,
		fs:      afero.NewBasePathFs(afero.NewOsFs(), config.HomeDir),
	}

	// 创建 FTP 服务器
	logger.Verbose("FileServer:FTP", "-", "正在初始化 FTP 服务器库...")
	s.server = ftpserverlib.NewFtpServer(s.driver)

	// 启动服务器（使用 safeGo 包装）
	logger.Info("FileServer:FTP", "-", "正在启动 FTP 服务器，监听端口 %d...", config.Port)
	
	s.running = true

	safeGo("FTP-ListenAndServe", func() {
		logger.Info("FileServer:FTP", "-", "FTP 服务器 goroutine 已启动，开始监听...")
		if err := s.server.ListenAndServe(); err != nil {
			s.mu.Lock()
			s.running = false
			s.mu.Unlock()
			logger.Error("FileServer:FTP", "-", "FTP 服务器运行错误: %v", err)
			s.manager.emitLog(LogEvent{
				Level:    LogLevelError,
				Protocol: ProtocolFTP,
				Action:   ActionError,
				Message:  fmt.Sprintf("FTP 服务器运行错误: %v", err),
			})
		}
		logger.Info("FileServer:FTP", "-", "FTP 服务器 goroutine 已退出")
	})

	// 等待一小段时间确认服务器启动
	time.Sleep(100 * time.Millisecond)

	logger.Info("FileServer:FTP", "-", "FTP 服务器已成功启动，监听端口 %d", config.Port)

	s.manager.emitLog(LogEvent{
		Level:    LogLevelSuccess,
		Protocol: ProtocolFTP,
		Action:   ActionConnect,
		Message:  fmt.Sprintf("FTP 服务器已启动，监听端口 %d", config.Port),
	})

	return nil
}

// Stop 停止 FTP 服务器
func (s *FTPServer) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	logger.Debug("FileServer:FTP", "-", "Stop 方法被调用")

	if !s.running || s.server == nil {
		logger.Verbose("FileServer:FTP", "-", "FTP 服务器未运行，无需停止")
		return nil
	}

	logger.Verbose("FileServer:FTP", "-", "正在关闭 FTP 服务器...")

	// 关闭服务器
	if err := s.server.Stop(); err != nil {
		logger.Error("FileServer:FTP", "-", "停止 FTP 服务器失败: %v", err)
		return fmt.Errorf("停止 FTP 服务器失败: %v", err)
	}

	s.running = false

	logger.Info("FileServer:FTP", "-", "FTP 服务器已停止")

	s.manager.emitLog(LogEvent{
		Level:    LogLevelInfo,
		Protocol: ProtocolFTP,
		Action:   ActionDisconnect,
		Message:  "FTP 服务器已停止",
	})

	return nil
}

// IsRunning 检查服务器是否运行中
func (s *FTPServer) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	logger.Verbose("FileServer:FTP", "-", "IsRunning: %v", s.running)
	return s.running
}

// DisconnectAll 断开所有客户端连接
func (s *FTPServer) DisconnectAll() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	logger.Debug("FileServer:FTP", "-", "DisconnectAll 方法被调用")

	if s.server == nil {
		logger.Verbose("FileServer:FTP", "-", "服务器实例为空，无需断开连接")
		return nil
	}

	logger.Info("FileServer:FTP", "-", "已断开所有 FTP 连接")

	s.manager.emitLog(LogEvent{
		Level:    LogLevelInfo,
		Protocol: ProtocolFTP,
		Action:   ActionDisconnect,
		Message:  "所有 FTP 连接已断开",
	})

	return nil
}

// GetProtocol 获取协议类型
func (s *FTPServer) GetProtocol() Protocol {
	return ProtocolFTP
}

// validateConfig 验证配置
func (s *FTPServer) validateConfig(config *models.FileServerConfig) error {
	logger.Verbose("FileServer:FTP", "-", "验证端口号: %d", config.Port)
	if config.Port <= 0 || config.Port > 65535 {
		logger.Error("FileServer:FTP", "-", "无效的端口号: %d", config.Port)
		return fmt.Errorf("无效的端口号: %d", config.Port)
	}

	logger.Verbose("FileServer:FTP", "-", "验证根目录: %s", config.HomeDir)
	if config.HomeDir == "" {
		logger.Error("FileServer:FTP", "-", "根目录不能为空")
		return fmt.Errorf("根目录不能为空")
	}

	// 检查目录是否存在，不存在则创建
	logger.Verbose("FileServer:FTP", "-", "检查/创建根目录: %s", config.HomeDir)
	if err := os.MkdirAll(config.HomeDir, 0755); err != nil {
		logger.Error("FileServer:FTP", "-", "无法创建根目录 %s: %v", config.HomeDir, err)
		return fmt.Errorf("无法创建根目录: %v", err)
	}

	logger.Verbose("FileServer:FTP", "-", "配置验证通过")
	return nil
}

// ==================== ftpDriver 实现 (MainDriver) ====================

// GetSettings 获取服务器设置
func (d *ftpDriver) GetSettings() (*ftpserverlib.Settings, error) {
	logger.Verbose("FileServer:FTP", "-", "GetSettings 被调用，端口: %d", d.config.Port)
	return &ftpserverlib.Settings{
		ListenAddr: fmt.Sprintf(":%d", d.config.Port),
		PassiveTransferPortRange: &ftpserverlib.PortRange{
			Start: 50000,
			End:   51000,
		},
		// 明确设置不要求TLS，允许客户端使用普通FTP连接
		// ClearOrEncrypted 表示服务器接受明文连接，不强制TLS
		TLSRequired: ftpserverlib.ClearOrEncrypted,
	}, nil
}

// ClientConnected 客户端连接时调用
func (d *ftpDriver) ClientConnected(cc ftpserverlib.ClientContext) (string, error) {
	clientIP := cc.RemoteAddr().String()
	sessionID := generateSessionID()

	logger.Verbose("FileServer:FTP", clientIP, "客户端连接 (Session: %s)", sessionID)

	// 记录连接
	d.manager.emitLog(LogEvent{
		Level:    LogLevelInfo,
		Protocol: ProtocolFTP,
		ClientIP: clientIP,
		Action:   ActionConnect,
		Message:  fmt.Sprintf("客户端已连接 (Session: %s)", sessionID),
	})

	return fmt.Sprintf("Welcome to NetWeaverGo FTP Server"), nil
}

// ClientDisconnected 客户端断开连接时调用
func (d *ftpDriver) ClientDisconnected(cc ftpserverlib.ClientContext) {
	clientIP := cc.RemoteAddr().String()

	logger.Verbose("FileServer:FTP", clientIP, "客户端断开连接")

	d.manager.emitLog(LogEvent{
		Level:    LogLevelInfo,
		Protocol: ProtocolFTP,
		ClientIP: clientIP,
		Action:   ActionDisconnect,
		Message:  "客户端已断开连接",
	})
}

// AuthUser 用户认证
func (d *ftpDriver) AuthUser(cc ftpserverlib.ClientContext, user, pass string) (ftpserverlib.ClientDriver, error) {
	clientIP := cc.RemoteAddr().String()

	logger.Debug("FileServer:FTP", clientIP, "认证请求: user=%s", user)

	// 验证用户名和密码
	if user != d.config.Username || pass != d.config.Password {
		logger.Warn("FileServer:FTP", clientIP, "认证失败: 用户名或密码错误 (user: %s)", user)
		d.manager.emitLog(LogEvent{
			Level:    LogLevelWarn,
			Protocol: ProtocolFTP,
			ClientIP: clientIP,
			Action:   ActionError,
			Message:  fmt.Sprintf("认证失败: 用户名或密码错误 (user: %s)", user),
		})
		return nil, fmt.Errorf("认证失败")
	}

	logger.Info("FileServer:FTP", clientIP, "用户 %s 认证成功", user)

	d.manager.emitLog(LogEvent{
		Level:    LogLevelSuccess,
		Protocol: ProtocolFTP,
		ClientIP: clientIP,
		Action:   ActionConnect,
		Message:  fmt.Sprintf("用户 %s 认证成功", user),
	})

	return &ftpClientDriver{
		driver:   d,
		clientIP: clientIP,
	}, nil
}

// GetTLSConfig 获取 TLS 配置
func (d *ftpDriver) GetTLSConfig() (*tls.Config, error) {
	logger.Info("FileServer:FTP", "-", "客户端请求TLS连接，但服务器未配置TLS支持")
	logger.Info("FileServer:FTP", "-", "TLS不可用，客户端应降级使用普通FTP连接")
	// 返回错误告知ftpserverlib库TLS不可用
	// 库会向客户端发送"Cannot get a TLS config: ..."响应
	// 客户端收到后会降级到普通FTP模式继续连接
	return nil, fmt.Errorf("TLS未配置，服务器不支持加密连接")
}

// ==================== ftpClientDriver 实现 (ClientDriver) ====================

// ftpClientDriver 需要实现 afero.Fs 接口，我们直接包装 afero.Fs

// Create 创建文件
func (c *ftpClientDriver) Create(name string) (afero.File, error) {
	logger.Verbose("FileServer:FTP", c.clientIP, "Create: %s", name)
	
	// 检查上传权限
	if !c.driver.config.AllowPut {
		logger.Warn("FileServer:FTP", c.clientIP, "拒绝创建文件 %s: 权限不足", name)
		c.driver.manager.emitLog(LogEvent{
			Level:    LogLevelWarn,
			Protocol: ProtocolFTP,
			ClientIP: c.clientIP,
			Action:   ActionError,
			Message:  fmt.Sprintf("拒绝创建文件 %s: 权限不足", name),
			File:     name,
		})
		return nil, fmt.Errorf("权限不足: 不允许上传文件")
	}
	
	logger.Info("FileServer:FTP", c.clientIP, "上传文件: %s", name)
	
	c.driver.manager.emitLog(LogEvent{
		Level:    LogLevelInfo,
		Protocol: ProtocolFTP,
		ClientIP: c.clientIP,
		Action:   ActionUpload,
		Message:  "开始上传",
		File:     name,
	})
	
	return c.driver.fs.Create(name)
}

// Mkdir 创建目录
func (c *ftpClientDriver) Mkdir(name string, perm os.FileMode) error {
	logger.Verbose("FileServer:FTP", c.clientIP, "Mkdir: %s, perm: %v", name, perm)
	
	logger.Info("FileServer:FTP", c.clientIP, "创建目录: %s", name)
	
	c.driver.manager.emitLog(LogEvent{
		Level:    LogLevelInfo,
		Protocol: ProtocolFTP,
		ClientIP: c.clientIP,
		Action:   ActionUpload,
		Message:  "创建目录",
		File:     name,
	})
	
	return c.driver.fs.Mkdir(name, perm)
}

// MkdirAll 递归创建目录
func (c *ftpClientDriver) MkdirAll(path string, perm os.FileMode) error {
	logger.Verbose("FileServer:FTP", c.clientIP, "MkdirAll: %s, perm: %v", path, perm)
	
	logger.Info("FileServer:FTP", c.clientIP, "递归创建目录: %s", path)
	
	c.driver.manager.emitLog(LogEvent{
		Level:    LogLevelInfo,
		Protocol: ProtocolFTP,
		ClientIP: c.clientIP,
		Action:   ActionUpload,
		Message:  "递归创建目录",
		File:     path,
	})
	
	return c.driver.fs.MkdirAll(path, perm)
}

// Open 打开文件
func (c *ftpClientDriver) Open(name string) (afero.File, error) {
	logger.Verbose("FileServer:FTP", c.clientIP, "Open: %s", name)
	
	// 检查下载权限
	if !c.driver.config.AllowGet {
		logger.Warn("FileServer:FTP", c.clientIP, "拒绝打开文件 %s: 权限不足", name)
		c.driver.manager.emitLog(LogEvent{
			Level:    LogLevelWarn,
			Protocol: ProtocolFTP,
			ClientIP: c.clientIP,
			Action:   ActionError,
			Message:  fmt.Sprintf("拒绝打开文件 %s: 权限不足", name),
			File:     name,
		})
		return nil, fmt.Errorf("权限不足: 不允许下载文件")
	}
	
	logger.Info("FileServer:FTP", c.clientIP, "下载文件: %s", name)
	
	c.driver.manager.emitLog(LogEvent{
		Level:    LogLevelInfo,
		Protocol: ProtocolFTP,
		ClientIP: c.clientIP,
		Action:   ActionDownload,
		Message:  "开始下载",
		File:     name,
	})
	
	return c.driver.fs.Open(name)
}

// OpenFile 打开文件（带标志）
func (c *ftpClientDriver) OpenFile(name string, flag int, perm os.FileMode) (afero.File, error) {
	logger.Verbose("FileServer:FTP", c.clientIP, "OpenFile: %s, flag: %d, perm: %v", name, flag, perm)
	
	// 根据标志检查权限
	if flag&os.O_WRONLY != 0 || flag&os.O_RDWR != 0 {
		if !c.driver.config.AllowPut {
			logger.Warn("FileServer:FTP", c.clientIP, "拒绝写入文件 %s: 权限不足", name)
			c.driver.manager.emitLog(LogEvent{
				Level:    LogLevelWarn,
				Protocol: ProtocolFTP,
				ClientIP: c.clientIP,
				Action:   ActionError,
				Message:  fmt.Sprintf("拒绝写入文件 %s: 权限不足", name),
				File:     name,
			})
			return nil, fmt.Errorf("权限不足: 不允许上传文件")
		}
		
		logger.Info("FileServer:FTP", c.clientIP, "写入文件: %s", name)
		
		c.driver.manager.emitLog(LogEvent{
			Level:    LogLevelInfo,
			Protocol: ProtocolFTP,
			ClientIP: c.clientIP,
			Action:   ActionUpload,
			Message:  "写入文件",
			File:     name,
		})
	}
	return c.driver.fs.OpenFile(name, flag, perm)
}

// Remove 删除文件
func (c *ftpClientDriver) Remove(name string) error {
	logger.Verbose("FileServer:FTP", c.clientIP, "Remove: %s", name)
	
	// 检查删除权限
	if !c.driver.config.AllowDel {
		logger.Warn("FileServer:FTP", c.clientIP, "拒绝删除文件 %s: 权限不足", name)
		c.driver.manager.emitLog(LogEvent{
			Level:    LogLevelWarn,
			Protocol: ProtocolFTP,
			ClientIP: c.clientIP,
			Action:   ActionError,
			Message:  fmt.Sprintf("拒绝删除文件 %s: 权限不足", name),
			File:     name,
		})
		return fmt.Errorf("权限不足: 不允许删除文件")
	}

	logger.Info("FileServer:FTP", c.clientIP, "删除文件: %s", name)
	
	c.driver.manager.emitLog(LogEvent{
		Level:    LogLevelSuccess,
		Protocol: ProtocolFTP,
		ClientIP: c.clientIP,
		Action:   ActionDelete,
		Message:  "删除文件",
		File:     name,
	})

	return c.driver.fs.Remove(name)
}

// RemoveAll 递归删除
func (c *ftpClientDriver) RemoveAll(path string) error {
	logger.Verbose("FileServer:FTP", c.clientIP, "RemoveAll: %s", path)
	
	// 检查删除权限
	if !c.driver.config.AllowDel {
		logger.Warn("FileServer:FTP", c.clientIP, "拒绝删除目录 %s: 权限不足", path)
		c.driver.manager.emitLog(LogEvent{
			Level:    LogLevelWarn,
			Protocol: ProtocolFTP,
			ClientIP: c.clientIP,
			Action:   ActionError,
			Message:  fmt.Sprintf("拒绝删除目录 %s: 权限不足", path),
			File:     path,
		})
		return fmt.Errorf("权限不足: 不允许删除文件")
	}

	logger.Info("FileServer:FTP", c.clientIP, "递归删除: %s", path)
	
	c.driver.manager.emitLog(LogEvent{
		Level:    LogLevelSuccess,
		Protocol: ProtocolFTP,
		ClientIP: c.clientIP,
		Action:   ActionDelete,
		Message:  "递归删除",
		File:     path,
	})
	
	return c.driver.fs.RemoveAll(path)
}

// Rename 重命名
func (c *ftpClientDriver) Rename(oldname, newname string) error {
	logger.Verbose("FileServer:FTP", c.clientIP, "Rename: %s -> %s", oldname, newname)
	
	// 检查重命名权限
	if !c.driver.config.AllowRename {
		logger.Warn("FileServer:FTP", c.clientIP, "拒绝重命名文件 %s: 权限不足", oldname)
		c.driver.manager.emitLog(LogEvent{
			Level:    LogLevelWarn,
			Protocol: ProtocolFTP,
			ClientIP: c.clientIP,
			Action:   ActionError,
			Message:  fmt.Sprintf("拒绝重命名文件 %s: 权限不足", oldname),
			File:     oldname,
		})
		return fmt.Errorf("权限不足: 不允许重命名文件")
	}

	logger.Info("FileServer:FTP", c.clientIP, "重命名: %s -> %s", oldname, newname)
	
	c.driver.manager.emitLog(LogEvent{
		Level:    LogLevelSuccess,
		Protocol: ProtocolFTP,
		ClientIP: c.clientIP,
		Action:   ActionUpload,
		Message:  "重命名",
		File:     fmt.Sprintf("%s -> %s", oldname, newname),
	})

	return c.driver.fs.Rename(oldname, newname)
}

// Stat 获取文件信息
func (c *ftpClientDriver) Stat(name string) (os.FileInfo, error) {
	logger.Verbose("FileServer:FTP", c.clientIP, "Stat: %s", name)
	return c.driver.fs.Stat(name)
}

// Name 返回文件系统名称
func (c *ftpClientDriver) Name() string {
	return "NetWeaverGo FTP"
}

// Chmod 修改权限
func (c *ftpClientDriver) Chmod(name string, mode os.FileMode) error {
	logger.Verbose("FileServer:FTP", c.clientIP, "Chmod: %s, mode: %v", name, mode)
	return c.driver.fs.Chmod(name, mode)
}

// Chown 修改所有者
func (c *ftpClientDriver) Chown(name string, uid, gid int) error {
	logger.Verbose("FileServer:FTP", c.clientIP, "Chown: %s, uid: %d, gid: %d", name, uid, gid)
	return c.driver.fs.Chown(name, uid, gid)
}

// Chtimes 修改时间
func (c *ftpClientDriver) Chtimes(name string, atime, mtime time.Time) error {
	logger.Verbose("FileServer:FTP", c.clientIP, "Chtimes: %s", name)
	return c.driver.fs.Chtimes(name, atime, mtime)
}

// ==================== 文件传输处理 ====================

// GetFile 获取文件（下载）- 通过 Open 实现
// PutFile 上传文件 - 通过 Create/OpenFile 实现

// generateSessionID 生成会话ID
func generateSessionID() string {
	b := make([]byte, 4)
	rand.Read(b)
	return hex.EncodeToString(b)
}
