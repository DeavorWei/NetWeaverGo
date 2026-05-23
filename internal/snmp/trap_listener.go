// Package snmp 提供 SNMP 核心业务功能
// trap_listener.go 实现 SNMP Trap 监听器，支持 v1/v2c/v3 Trap 接收
package snmp

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/gosnmp/gosnmp"

	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/models"
)

// ============================================================================
// 监听统计
// ============================================================================

// ListenerStats 监听统计
type ListenerStats struct {
	IsRunning    bool      `json:"isRunning"`    // 是否正在运行
	ListenAddr   string    `json:"listenAddr"`   // 监听地址
	TotalTraps   int64     `json:"totalTraps"`   // 总接收 Trap 数
	FilteredOut  int64     `json:"filteredOut"`  // 被过滤掉的 Trap 数
	LastTrapTime time.Time `json:"lastTrapTime"` // 最后一次收到 Trap 的时间
	StartTime    time.Time `json:"startTime"`    // 启动时间
}

// ============================================================================
// v3 用户配置
// ============================================================================

// V3UserConfig SNMPv3 用户配置（用于 Trap 监听器认证）
type V3UserConfig struct {
	Username      string `json:"username"`
	AuthProtocol  string `json:"authProtocol"`  // MD5/SHA/SHA224/SHA256/SHA384/SHA512
	AuthKey       string `json:"authKey"`       // 认证密钥（明文，运行时使用）
	PrivProtocol  string `json:"privProtocol"`  // DES/AES/AES192/AES256/AES192C/AES256C
	PrivKey       string `json:"privKey"`       // 加密密钥（明文，运行时使用）
	SecurityLevel string `json:"securityLevel"` // noAuthNoPriv/authNoPriv/authPriv
}

// ============================================================================
// Trap 监听器
// ============================================================================

// TrapListener SNMP Trap 监听器
// 使用 gosnmp 库监听 SNMP Trap 消息，支持 v1、v2c 和 v3
type TrapListener struct {
	handler  *TrapHandler          // Trap 处理器
	config   *models.SNMPServerConfig // 服务器配置
	notifier EventNotifier         // 事件通知器

	listener *gosnmp.TrapListener // gosnmp 监听器实例
	running  bool                 // 运行状态
	mu       sync.Mutex           // 并发控制
	stopCh   chan struct{}        // 停止信号通道

	// 配置更新通道（用于避免死锁）
	configChan chan *models.SNMPServerConfig
	configWg   sync.WaitGroup // 等待配置处理 goroutine 结束

	// v3 用户配置（userName -> V3UserConfig）
	v3Users map[string]*V3UserConfig
	v3Mu    sync.RWMutex // v3 用户配置并发控制

	stats ListenerStats // 监听统计
}

// NewTrapListener 创建 Trap 监听器
// config 可以为 nil，此时使用默认配置
func NewTrapListener(handler *TrapHandler, config *models.SNMPServerConfig, notifier EventNotifier) *TrapListener {
	// 应用默认配置
	if config == nil {
		config = &models.SNMPServerConfig{
			TrapPort:      1162, // 使用非特权端口
			TrapCommunity: "public",
		}
	}

	l := &TrapListener{
		handler:    handler,
		config:     config,
		notifier:   notifier,
		stopCh:     make(chan struct{}),
		configChan: make(chan *models.SNMPServerConfig, 8),
		v3Users:    make(map[string]*V3UserConfig),
		stats: ListenerStats{
			ListenAddr: fmt.Sprintf("0.0.0.0:%d", config.TrapPort),
		},
	}

	// 启动配置变更处理 goroutine
	l.configWg.Add(1)
	go l.configLoop()

	logger.Info("SNMP-Listener", "-", "Trap 监听器已创建: 端口=%d, Community=%s, V3Enabled=%v",
		config.TrapPort, config.TrapCommunity, config.V3Enabled)

	return l
}

// ============================================================================
// v3 用户管理
// ============================================================================

// AddV3User 添加 v3 用户配置
func (l *TrapListener) AddV3User(config *V3UserConfig) error {
	if config == nil || config.Username == "" {
		return fmt.Errorf("v3 用户配置无效")
	}

	l.v3Mu.Lock()
	defer l.v3Mu.Unlock()

	l.v3Users[config.Username] = config
	logger.Info("SNMP-Listener", "-", "添加 SNMPv3 用户: %s (安全级别: %s)", config.Username, config.SecurityLevel)
	return nil
}

// RemoveV3User 移除 v3 用户配置
func (l *TrapListener) RemoveV3User(username string) error {
	l.v3Mu.Lock()
	defer l.v3Mu.Unlock()

	if _, exists := l.v3Users[username]; !exists {
		return fmt.Errorf("v3 用户不存在: %s", username)
	}

	delete(l.v3Users, username)
	logger.Info("SNMP-Listener", "-", "移除 SNMPv3 用户: %s", username)
	return nil
}

// GetV3Users 获取所有 v3 用户配置列表
func (l *TrapListener) GetV3Users() []*V3UserConfig {
	l.v3Mu.RLock()
	defer l.v3Mu.RUnlock()

	users := make([]*V3UserConfig, 0, len(l.v3Users))
	for _, user := range l.v3Users {
		users = append(users, user)
	}
	return users
}

// ============================================================================
// 生命周期管理
// ============================================================================

// Start 启动监听
func (l *TrapListener) Start() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.running {
		logger.Warn("SNMP-Listener", "-", "监听器已在运行中")
		return fmt.Errorf("监听器已在运行中")
	}

	// 创建 gosnmp Trap 监听器
	listenAddr := fmt.Sprintf("0.0.0.0:%d", l.config.TrapPort)

	// 创建监听器
	trapListener := gosnmp.NewTrapListener()

	// 注意：gosnmp TrapListener 对 v3 的支持有限
	// v3 用户配置存储在 l.v3Users 中，用于后续验证和处理
	// 实际 v3 Trap 解密需要在 HandleTrap 回调中处理
	if l.config.V3Enabled && len(l.v3Users) > 0 {
		logger.Info("SNMP-Listener", "-", "SNMPv3 已启用，已配置 %d 个用户", len(l.v3Users))
	}

	// 设置 Trap 处理回调
	trapListener.OnNewTrap = func(p *gosnmp.SnmpPacket, addr *net.UDPAddr) {
		// 更新统计
		l.mu.Lock()
		l.stats.TotalTraps++
		l.stats.LastTrapTime = time.Now()
		l.mu.Unlock()

		// 调用处理器
		if l.handler != nil {
			l.handler.HandleTrap(p, addr)
		}
	}

	l.listener = trapListener

	// 启动监听（在独立 goroutine 中运行）
	l.stopCh = make(chan struct{})

	go func() {
		logger.Info("SNMP-Listener", "-", "正在启动 Trap 监听: %s (v1/v2c/v3)", listenAddr)

		// 使用 gosnmp 的 Listen 方法
		err := trapListener.Listen(listenAddr)
		if err != nil {
			select {
			case <-l.stopCh:
				// 正常停止，忽略错误
			default:
				logger.Error("SNMP-Listener", "-", "Trap 监听器异常退出: %v", err)
				l.setRunning(false)
			}
		}
	}()

	l.running = true
	l.stats.IsRunning = true
	l.stats.StartTime = time.Now()
	l.stats.ListenAddr = listenAddr

	// 通知监听器状态变更
	if l.notifier != nil {
		l.notifier.NotifyListenerStatus(&l.stats)
	}

	logger.Info("SNMP-Listener", "-", "Trap 监听器启动成功: %s", listenAddr)
	return nil
}

// Stop 停止监听
func (l *TrapListener) Stop() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if !l.running {
		logger.Debug("SNMP-Listener", "-", "监听器未在运行")
		return nil
	}

	logger.Info("SNMP-Listener", "-", "正在停止 Trap 监听器...")

	// 发送停止信号
	close(l.stopCh)

	// 关闭 gosnmp 监听器
	if l.listener != nil {
		l.listener.Close()
		l.listener = nil
	}

	l.running = false
	l.stats.IsRunning = false

	// 通知监听器状态变更
	if l.notifier != nil {
		l.notifier.NotifyListenerStatus(&l.stats)
	}

	logger.Info("SNMP-Listener", "-", "Trap 监听器已停止")
	return nil
}

// Close 关闭监听器并释放所有资源
// 与 Stop 不同，Close 还会关闭配置通道并等待配置处理 goroutine 结束
func (l *TrapListener) Close() {
	// 先停止监听
	l.Stop()

	// 关闭配置通道，触发 configLoop 退出
	close(l.configChan)

	// 等待配置处理 goroutine 结束
	l.configWg.Wait()
}

// IsRunning 检查是否正在运行
func (l *TrapListener) IsRunning() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.running
}

// ============================================================================
// 配置更新
// ============================================================================

// UpdateConfig 更新监听配置
// 通过 channel 异步传递配置变更，避免死锁
// 配置变更会自动触发监听器重启（如果正在运行）
func (l *TrapListener) UpdateConfig(config *models.SNMPServerConfig) {
	if config == nil {
		logger.Warn("SNMP-Listener", "-", "忽略空配置更新")
		return
	}

	select {
	case l.configChan <- config:
		logger.Info("SNMP-Listener", "-", "配置更新请求已提交: 端口=%d", config.TrapPort)
	default:
		// channel 满时丢弃旧配置，确保非阻塞
		logger.Warn("SNMP-Listener", "-", "配置更新通道已满，丢弃旧请求")
		select {
		case <-l.configChan:
			// 丢弃旧配置
		default:
		}
		l.configChan <- config
	}
}

// configLoop 配置变更处理 goroutine
// 在独立 goroutine 中处理配置变更，安全地执行停止/重启操作
// 避免了在持有锁时调用 Stop/Start 导致的死锁问题
func (l *TrapListener) configLoop() {
	defer l.configWg.Done()

	for config := range l.configChan {
		l.applyConfig(config)
	}
}

// applyConfig 应用新配置（在 configLoop goroutine 中调用）
// 安全地停止/重启监听器，无需担心锁嵌套
func (l *TrapListener) applyConfig(config *models.SNMPServerConfig) {
	// 读取当前运行状态（需要加锁）
	l.mu.Lock()
	wasRunning := l.running
	l.config = config
	l.stats.ListenAddr = fmt.Sprintf("0.0.0.0:%d", config.TrapPort)
	l.mu.Unlock()

	// 如果正在运行，先停止再重启（Stop/Start 各自管理锁，不会死锁）
	if wasRunning {
		if err := l.Stop(); err != nil {
			logger.Error("SNMP-Listener", "-", "配置更新时停止监听器失败: %v", err)
			return
		}

		if err := l.Start(); err != nil {
			logger.Error("SNMP-Listener", "-", "配置更新时重启监听器失败: %v", err)
			return
		}
	}

	logger.Info("SNMP-Listener", "-", "监听配置已应用: 端口=%d, wasRunning=%v", config.TrapPort, wasRunning)
}

// ============================================================================
// 统计方法
// ============================================================================

// GetStats 获取监听统计
func (l *TrapListener) GetStats() *ListenerStats {
	l.mu.Lock()
	defer l.mu.Unlock()

	stats := l.stats
	return &stats
}

// setRunning 设置运行状态（内部方法）
func (l *TrapListener) setRunning(running bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.running = running
	l.stats.IsRunning = running
}

// ============================================================================
// 上下文支持
// ============================================================================

// StartWithContext 带上下文启动监听
func (l *TrapListener) StartWithContext(ctx context.Context) error {
	if err := l.Start(); err != nil {
		return err
	}

	// 监听上下文取消信号
	go func() {
		select {
		case <-ctx.Done():
			logger.Info("SNMP-Listener", "-", "收到上下文取消信号，停止监听器")
			l.Stop()
		case <-l.stopCh:
			// 监听器已停止
		}
	}()

	return nil
}
