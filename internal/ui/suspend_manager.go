package ui

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/NetWeaverGo/core/internal/executor"
	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// SuspendSession 挂起会话信息
type SuspendSession struct {
	ID        string
	IP        string
	CreatedAt time.Time
	ActionCh  chan executor.ErrorAction
	timedOut  atomic.Bool // 超时标记，防止超时后前端响应向已关闭channel发送
	resolved  atomic.Bool // 已响应标记，防止重复响应
}

// SuspendManager 全局挂起会话管理器
type SuspendManager struct {
	mu           sync.Mutex
	sessions     map[string]*SuspendSession
	sessionsByIP map[string]string
	sessionIDGen int64
	wailsApp     *application.App
}

// globalSuspendManager 全局单例
var globalSuspendManager = &SuspendManager{
	sessions:     make(map[string]*SuspendSession),
	sessionsByIP: make(map[string]string),
}

// GetSuspendManager 获取全局管理器
func GetSuspendManager() *SuspendManager {
	return globalSuspendManager
}

// SetWailsApp 设置 Wails 应用实例
func (m *SuspendManager) SetWailsApp(app *application.App) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.wailsApp = app
}

// CreateHandler 创建 SuspendHandler（供 Engine/TaskGroupService 使用）
func (m *SuspendManager) CreateHandler() executor.SuspendHandler {
	return func(ctx context.Context, ip string, logLine string, cmd string) executor.ErrorAction {
		sessionID := m.generateSessionID()
		actionCh := make(chan executor.ErrorAction, 1)

		session := &SuspendSession{
			ID:        sessionID,
			IP:        ip,
			CreatedAt: time.Now(),
			ActionCh:  actionCh,
		}

		m.mu.Lock()
		// 清理该 IP 的旧会话
		if oldSessionID, exists := m.sessionsByIP[ip]; exists {
			if oldSession, ok := m.sessions[oldSessionID]; ok {
				select {
				case oldSession.ActionCh <- executor.ActionAbort:
					logger.Debug("SuspendManager", ip, "旧的挂起会话 %s 已被终止", oldSessionID)
				default:
				}
			}
		}
		m.sessions[sessionID] = session
		m.sessionsByIP[ip] = sessionID
		app := m.wailsApp
		m.mu.Unlock()

		defer func() {
			m.mu.Lock()
			delete(m.sessions, sessionID)
			if m.sessionsByIP[ip] == sessionID {
				delete(m.sessionsByIP, ip)
			}
			m.mu.Unlock()
			close(actionCh)
		}()

		// 发射事件到前端
		if app != nil {
			app.Event.Emit("engine:suspend_required", map[string]interface{}{
				"sessionId": sessionID,
				"ip":        ip,
				"error":     logLine,
				"command":   cmd,
			})
		}

		logger.Warn("SuspendManager", ip, "挂起会话创建 (sessionID: %s)，等待用户操作...", sessionID)

		select {
		case action := <-actionCh:
			logger.Debug("SuspendManager", ip, "挂起会话 %s 已收到用户响应", sessionID)
			return action
		case <-ctx.Done():
			logger.Warn("SuspendManager", ip, "引擎任务结束，强制释放挂起的会话 (sessionID: %s)", sessionID)
			return executor.ActionAbort
		case <-time.After(5 * time.Minute):
			// 设置超时标记，防止前端后续响应
			session.timedOut.Store(true)

			// 发射超时事件到前端，让前端关闭弹窗
			if app != nil {
				app.Event.Emit("engine:suspend_timeout", map[string]interface{}{
					"sessionId": sessionID,
					"ip":        ip,
					"message":   "挂起超时（5分钟），已自动终止设备连接",
				})
			}

			logger.Warn("SuspendManager", ip, "挂起超时（5分钟），自动 Abort")
			return executor.ActionAbort
		}
	}
}

// Resolve 解除挂起（供前端调用）
func (m *SuspendManager) Resolve(sessionIDOrIP string, action string) {
	m.mu.Lock()

	var session *SuspendSession
	var exists bool

	// 先尝试 sessionID
	session, exists = m.sessions[sessionIDOrIP]
	if !exists {
		// 再尝试 IP
		if sessionID, ok := m.sessionsByIP[sessionIDOrIP]; ok {
			session, exists = m.sessions[sessionID]
		}
	}

	m.mu.Unlock()

	if !exists || session == nil {
		logger.Warn("SuspendManager", sessionIDOrIP, "找不到挂起会话，可能任务已结束或超时")
		return
	}

	// 检查是否已超时
	if session.timedOut.Load() {
		logger.Warn("SuspendManager", session.IP, "挂起会话已超时，忽略用户响应")
		return
	}

	// 检查是否已响应（防止重复响应）
	if session.resolved.Swap(true) {
		logger.Warn("SuspendManager", session.IP, "挂起会话已处理，忽略重复响应")
		return
	}

	var errAction executor.ErrorAction
	switch action {
	case "C":
		errAction = executor.ActionContinue
	case "S":
		errAction = executor.ActionSkip
	case "A":
		errAction = executor.ActionAbort
	default:
		logger.Warn("SuspendManager", session.IP, "未知的挂起动作: %s", action)
		// 重置 resolved 标记，允许重新响应
		session.resolved.Store(false)
		return
	}

	select {
	case session.ActionCh <- errAction:
		logger.Debug("SuspendManager", session.IP, "挂起会话 %s 已收到用户响应: %s", session.ID, action)
	default:
		logger.Warn("SuspendManager", session.IP, "挂起信号通道已满，会话可能已结束")
	}
}

// generateSessionID 生成唯一的会话ID
func (m *SuspendManager) generateSessionID() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessionIDGen++
	return fmt.Sprintf("suspend_%d_%d", time.Now().UnixNano(), m.sessionIDGen)
}

// GetActiveSessionCount 获取当前活跃的挂起会话数量
func (m *SuspendManager) GetActiveSessionCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.sessions)
}
