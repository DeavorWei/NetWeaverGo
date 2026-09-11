package taskexec

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/NetWeaverGo/core/internal/executor"
	"github.com/NetWeaverGo/core/internal/logger"
)

// SuspendRequest 挂起决策请求
type SuspendRequest struct {
	ID        string    `json:"id"`
	RunID     string    `json:"runId"`
	DeviceIP  string    `json:"deviceIp"`
	Prompt    string    `json:"prompt"`
	Command   string    `json:"command"`
	CreatedAt time.Time `json:"createdAt"`
	decision  chan executor.ErrorAction
}

// SuspendManager 挂起决策管理器，管理运行中设备的挂起状态与用户审批
type SuspendManager struct {
	mu       sync.RWMutex
	requests map[string]*SuspendRequest
}

var (
	globalSuspendManager *SuspendManager
	onceSuspendManager   sync.Once
)

// GetGlobalSuspendManager 获取全局挂起管理器实例
func GetGlobalSuspendManager() *SuspendManager {
	onceSuspendManager.Do(func() {
		globalSuspendManager = &SuspendManager{
			requests: make(map[string]*SuspendRequest),
		}
	})
	return globalSuspendManager
}

// Register 注册新的挂起请求
func (m *SuspendManager) Register(req *SuspendRequest) {
	if m == nil || req == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	req.decision = make(chan executor.ErrorAction, 1)
	m.requests[req.ID] = req
	logger.Info("SuspendManager", req.RunID, "设备 %s 挂起等待决策 [RequestID: %s, Command: %s, Reason: %s]",
		req.DeviceIP, req.ID, req.Command, req.Prompt)
}

// Resolve 响应挂起请求
func (m *SuspendManager) Resolve(requestID string, action executor.ErrorAction) bool {
	if m == nil {
		return false
	}
	m.mu.Lock()
	req, found := m.requests[requestID]
	if found {
		delete(m.requests, requestID)
	}
	m.mu.Unlock()

	if !found || req == nil {
		return false
	}

	select {
	case req.decision <- action:
		logger.Info("SuspendManager", req.RunID, "已决议挂起请求 %s: action=%v", requestID, action)
		return true
	default:
		return false
	}
}

// ListPending 获取所有待处理的挂起请求
func (m *SuspendManager) ListPending() []*SuspendRequest {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()

	list := make([]*SuspendRequest, 0, len(m.requests))
	for _, req := range m.requests {
		list = append(list, req)
	}
	return list
}

// WaitDecision 等待决策输入或超时
func (m *SuspendManager) WaitDecision(ctx context.Context, req *SuspendRequest, timeout time.Duration) executor.ErrorAction {
	if req == nil {
		return executor.ActionAbort
	}
	defer func() {
		m.mu.Lock()
		delete(m.requests, req.ID)
		m.mu.Unlock()
	}()

	if timeout <= 0 {
		timeout = 5 * time.Minute
	}

	select {
	case act := <-req.decision:
		return act
	case <-time.After(timeout):
		logger.Warn("SuspendManager", req.RunID, "挂起请求 %s 等待决策超时 (5分钟)，自动中止", req.ID)
		return executor.ActionAbortTimeout
	case <-ctx.Done():
		logger.Warn("SuspendManager", req.RunID, "挂起请求 %s 上下文已取消", req.ID)
		return executor.ActionAbort
	}
}

// BuildDefaultSuspendHandler 为任务执行构建生产 SuspendHandler
func BuildDefaultSuspendHandler(runID string, errorMode string) executor.SuspendHandler {
	return func(ctx context.Context, ip string, deviceLog string, failedCmd string) executor.ErrorAction {
		// 1. 若配置为忽略错误并继续
		if errorMode == "skip" {
			logger.Info("TaskExec", runID, "[策略放行] 设备 %s 遇到挂起提示，根据 errorMode=skip 自动放行: %s", ip, deviceLog)
			return executor.ActionContinue
		}

		// 2. 若配置为遇到错误直接终止
		if errorMode == "abort" {
			logger.Warn("TaskExec", runID, "[策略终止] 设备 %s 遇到挂起提示，根据 errorMode=abort 直接终止: %s", ip, deviceLog)
			return executor.ActionAbort
		}

		// 3. 默认 pause：进入全局挂起审批通道
		reqID := fmt.Sprintf("%s_%s_%d", runID, ip, time.Now().UnixNano())
		req := &SuspendRequest{
			ID:        reqID,
			RunID:     runID,
			DeviceIP:  ip,
			Prompt:    deviceLog,
			Command:   failedCmd,
			CreatedAt: time.Now(),
		}

		mgr := GetGlobalSuspendManager()
		mgr.Register(req)

		// 等待工程师决策或 5 分钟超时
		return mgr.WaitDecision(ctx, req, 5*time.Minute)
	}
}
