package engine

import (
	"errors"
	"sync"
)

// EngineState 引擎状态枚举（重构后）
type EngineState int

const (
	StateIdle EngineState = iota
	StateStarting
	StateRunning
	StateClosing
	StateClosed
)

// String 返回状态名称
func (s EngineState) String() string {
	switch s {
	case StateIdle:
		return "Idle"
	case StateStarting:
		return "Starting"
	case StateRunning:
		return "Running"
	case StateClosing:
		return "Closing"
	case StateClosed:
		return "Closed"
	default:
		return "Unknown"
	}
}

// stateTransitionMatrix 状态转移校验矩阵
// fromState -> toState = valid?
var stateTransitionMatrix = map[EngineState]map[EngineState]bool{
	StateIdle: {
		StateStarting: true,
		StateClosing:  true,
	},
	StateStarting: {
		StateRunning: true,
		StateClosing: true,
	},
	StateRunning: {
		StateClosing: true,
	},
	StateClosing: {
		StateClosed: true,
	},
	StateClosed: {}, // 终态，不可转移
}

// EngineStateManager 引擎状态管理器
type EngineStateManager struct {
	mu    sync.RWMutex
	state EngineState
}

// NewEngineStateManager 创建状态管理器
func NewEngineStateManager() *EngineStateManager {
	return &EngineStateManager{
		state: StateIdle,
	}
}

// TransitionTo 带校验的状态转移
func (sm *EngineStateManager) TransitionTo(newState EngineState) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	validStates, exists := stateTransitionMatrix[sm.state]
	if !exists || !validStates[newState] {
		return errors.New("invalid state transition from " + sm.state.String() + " to " + newState.String())
	}

	sm.state = newState
	return nil
}

// State 获取当前状态
func (sm *EngineStateManager) State() EngineState {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.state
}

// IsRunning 检查是否运行中
func (sm *EngineStateManager) IsRunning() bool {
	return sm.State() == StateRunning
}

// IsClosing 检查是否正在关闭
func (sm *EngineStateManager) IsClosing() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	// 显式检查，不依赖枚举值顺序
	return sm.state == StateClosing || sm.state == StateClosed
}
