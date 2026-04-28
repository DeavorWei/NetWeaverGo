package ui

import (
	"context"
	"fmt"

	"github.com/NetWeaverGo/core/internal/config"
	"github.com/NetWeaverGo/core/internal/fileserver"
	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/models"
	"github.com/wailsapp/wails/v3/pkg/application"
	"gorm.io/gorm"
)

// FileServerService 文件服务器管理服务
type FileServerService struct {
	wailsApp *application.App
	manager  *fileserver.ServerManager
	db       *gorm.DB
}

// NewFileServerService 创建文件服务器服务实例
func NewFileServerService() *FileServerService {
	logger.Debug("FileServerService", "-", "创建文件服务器服务实例")
	return &FileServerService{
		manager: fileserver.NewServerManager(),
		db:      config.GetDB(),
	}
}

// ServiceStartup Wails 服务启动生命周期钩子
func (s *FileServerService) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	logger.Debug("FileServerService", "-", "ServiceStartup 被调用")
	s.wailsApp = application.Get()
	s.manager.SetWailsApp(s.wailsApp)
	logger.Info("FileServerService", "-", "文件服务器服务已启动")
	return nil
}

// GetServerConfig 获取指定协议的服务器配置
func (s *FileServerService) GetServerConfig(protocol string) (*models.FileServerConfig, error) {
	logger.Debug("FileServerService", "-", "GetServerConfig 被调用: protocol=%s", protocol)

	var cfg models.FileServerConfig
	result := s.db.Where("protocol = ?", protocol).First(&cfg)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			logger.Verbose("FileServerService", "-", "未找到 %s 配置，返回默认配置", protocol)
			defaultCfg := s.getDefaultConfig(protocol)
			logger.Verbose("FileServerService", "-", "默认配置: Port=%d, HomeDir=%s, Username=%s", defaultCfg.Port, defaultCfg.HomeDir, defaultCfg.Username)
			return defaultCfg, nil
		}
		logger.Error("FileServerService", "-", "查询 %s 配置失败: %v", protocol, result.Error)
		return nil, result.Error
	}

	logger.Verbose("FileServerService", "-", "找到 %s 配置: Port=%d, HomeDir=%s", protocol, cfg.Port, cfg.HomeDir)
	return &cfg, nil
}

// SaveServerConfig 保存服务器配置
func (s *FileServerService) SaveServerConfig(cfg models.FileServerConfig) error {
	logger.Debug("FileServerService", "-", "SaveServerConfig 被调用: protocol=%s, port=%d, homeDir=%s", cfg.Protocol, cfg.Port, cfg.HomeDir)

	// 验证协议类型
	if !isValidProtocol(cfg.Protocol) {
		logger.Error("FileServerService", "-", "无效的协议类型: %s", cfg.Protocol)
		return fmt.Errorf("无效的协议类型: %s", cfg.Protocol)
	}

	// 检查是否已存在配置
	var existing models.FileServerConfig
	result := s.db.Where("protocol = ?", cfg.Protocol).First(&existing)

	if result.Error == gorm.ErrRecordNotFound {
		logger.Verbose("FileServerService", "-", "创建新配置: %s", cfg.Protocol)
		result = s.db.Create(&cfg)
		if result.Error != nil {
			logger.Error("FileServerService", "-", "创建配置失败: %v", result.Error)
			return result.Error
		}
		logger.Info("FileServerService", "-", "配置已创建: %s", cfg.Protocol)
	} else {
		logger.Verbose("FileServerService", "-", "更新现有配置: %s (ID=%d)", cfg.Protocol, existing.ID)
		cfg.ID = existing.ID
		result = s.db.Save(&cfg)
		if result.Error != nil {
			logger.Error("FileServerService", "-", "更新配置失败: %v", result.Error)
			return result.Error
		}
		logger.Info("FileServerService", "-", "配置已更新: %s", cfg.Protocol)
	}

	return nil
}

// ToggleServer 启动/停止服务器
func (s *FileServerService) ToggleServer(protocol string, start bool) error {
	logger.Debug("FileServerService", "-", "ToggleServer 被调用: protocol=%s, start=%v", protocol, start)

	// 验证协议类型
	if !isValidProtocol(protocol) {
		logger.Error("FileServerService", "-", "无效的协议类型: %s", protocol)
		return fmt.Errorf("无效的协议类型: %s", protocol)
	}

	// 获取配置
	logger.Verbose("FileServerService", "-", "正在获取 %s 配置...", protocol)
	cfg, err := s.GetServerConfig(protocol)
	if err != nil {
		logger.Error("FileServerService", "-", "获取 %s 配置失败: %v", protocol, err)
		return fmt.Errorf("获取配置失败: %v", err)
	}

	// 执行操作
	if start {
		logger.Info("FileServerService", "-", "正在启动 %s 服务器...", protocol)
		err = s.manager.StartServer(fileserver.Protocol(protocol), cfg)
		if err != nil {
			logger.Error("FileServerService", "-", "启动 %s 服务器失败: %v", protocol, err)
			return err
		}
		logger.Info("FileServerService", "-", "%s 服务器已成功启动", protocol)
	} else {
		logger.Info("FileServerService", "-", "正在停止 %s 服务器...", protocol)
		err = s.manager.StopServer(fileserver.Protocol(protocol))
		if err != nil {
			logger.Error("FileServerService", "-", "停止 %s 服务器失败: %v", protocol, err)
			return err
		}
		logger.Info("FileServerService", "-", "%s 服务器已成功停止", protocol)
	}

	return nil
}

// GetServerStatus 获取服务器运行状态
func (s *FileServerService) GetServerStatus(protocol string) (bool, error) {
	logger.Debug("FileServerService", "-", "GetServerStatus 被调用: protocol=%s", protocol)

	if !isValidProtocol(protocol) {
		logger.Error("FileServerService", "-", "无效的协议类型: %s", protocol)
		return false, fmt.Errorf("无效的协议类型: %s", protocol)
	}

	running := s.manager.IsRunning(fileserver.Protocol(protocol))
	logger.Verbose("FileServerService", "-", "%s 服务器运行状态: %v", protocol, running)
	return running, nil
}

// DisconnectAll 断开所有客户端连接
func (s *FileServerService) DisconnectAll(protocol string) error {
	logger.Debug("FileServerService", "-", "DisconnectAll 被调用: protocol=%s", protocol)

	if !isValidProtocol(protocol) {
		logger.Error("FileServerService", "-", "无效的协议类型: %s", protocol)
		return fmt.Errorf("无效的协议类型: %s", protocol)
	}

	logger.Info("FileServerService", "-", "正在断开 %s 所有客户端连接...", protocol)
	err := s.manager.DisconnectAll(fileserver.Protocol(protocol))
	if err != nil {
		logger.Error("FileServerService", "-", "断开 %s 连接失败: %v", protocol, err)
		return err
	}
	logger.Info("FileServerService", "-", "%s 所有客户端连接已断开", protocol)
	return nil
}

// GetAllServerStatus 获取所有服务器状态
func (s *FileServerService) GetAllServerStatus() (map[string]bool, error) {
	logger.Debug("FileServerService", "-", "GetAllServerStatus 被调用")

	status := make(map[string]bool)
	protocols := []string{"sftp", "ftp", "tftp"}

	for _, p := range protocols {
		running := s.manager.IsRunning(fileserver.Protocol(p))
		status[p] = running
		logger.Verbose("FileServerService", "-", "%s 服务器状态: %v", p, running)
	}

	logger.Debug("FileServerService", "-", "所有服务器状态: sftp=%v, ftp=%v, tftp=%v", status["sftp"], status["ftp"], status["tftp"])
	return status, nil
}

// getDefaultConfig 获取默认配置
func (s *FileServerService) getDefaultConfig(protocol string) *models.FileServerConfig {
	logger.Debug("FileServerService", "-", "getDefaultConfig 被调用: protocol=%s", protocol)

	// 获取默认数据目录
	pm := config.GetPathManager()
	defaultHome := pm.GetStorageRoot()
	logger.Verbose("FileServerService", "-", "默认根目录: %s", defaultHome)

	switch protocol {
	case "sftp":
		logger.Verbose("FileServerService", "-", "返回 SFTP 默认配置: Port=2222")
		return &models.FileServerConfig{
			Protocol:    "sftp",
			Port:        2222,
			HomeDir:     defaultHome,
			Username:    "admin",
			Password:    "admin",
			AllowGet:    true,
			AllowPut:    true,
			AllowDel:    true,
			AllowRename: true,
		}
	case "ftp":
		logger.Verbose("FileServerService", "-", "返回 FTP 默认配置: Port=2121")
		return &models.FileServerConfig{
			Protocol:    "ftp",
			Port:        2121,
			HomeDir:     defaultHome,
			Username:    "admin",
			Password:    "admin",
			AllowGet:    true,
			AllowPut:    true,
			AllowDel:    true,
			AllowRename: true,
		}
	case "tftp":
		logger.Verbose("FileServerService", "-", "返回 TFTP 默认配置: Port=6969")
		return &models.FileServerConfig{
			Protocol: "tftp",
			Port:     6969,
			HomeDir:  defaultHome,
		}
	default:
		logger.Warn("FileServerService", "-", "未知协议类型 %s，返回通用默认配置", protocol)
		return &models.FileServerConfig{
			Protocol: protocol,
			Port:     2121,
			HomeDir:  defaultHome,
		}
	}
}

// isValidProtocol 验证协议类型
func isValidProtocol(protocol string) bool {
	switch protocol {
	case "sftp", "ftp", "tftp":
		return true
	default:
		return false
	}
}

// StopAll 停止所有服务器（应用退出时调用）
func (s *FileServerService) StopAll() error {
	logger.Info("FileServerService", "-", "正在停止所有文件服务器...")
	return s.manager.StopAll()
}
