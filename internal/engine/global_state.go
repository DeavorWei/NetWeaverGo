package engine

import (
	"fmt"
	"sync"
	"time"
)

// GlobalEngineState 重构后的全局引擎状态管理器
// 核心改变：持有 Engine 实例指针，而不是状态副本
type GlobalEngineState struct {
	mu           sync.RWMutex
	activeEngine *Engine // 持有实例指针
	runnerID     string
	runnerSrc    string    // 运行来源：engine, taskgroup, backup
	startedAt    time.Time // 启动时间
}

// 全局单例相关变量
var (
	globalState     *GlobalEngineState
	globalStateOnce sync.Once
)

// GetGlobalState 获取全局状态实例（线程安全）
func GetGlobalState() *GlobalEngineState {
	globalStateOnce.Do(func() {
		globalState = &GlobalEngineState{}
	})
	return globalState
}

// SetActiveEngine 设置当前活动的引擎实例
func (g *GlobalEngineState) SetActiveEngine(engine *Engine, runnerSrc, runnerID string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.activeEngine != nil && g.activeEngine.IsRunning() {
		if g.runnerSrc != "" && g.runnerID != "" {
			return fmt.Errorf("引擎正在由 %s (ID: %s) 运行中，请等待当前任务完成", g.runnerSrc, g.runnerID)
		}
		if g.runnerSrc != "" {
			return fmt.Errorf("引擎正在由 %s 运行中，请等待当前任务完成", g.runnerSrc)
		}
		return fmt.Errorf("引擎正在运行中，请等待当前任务完成")
	}

	g.activeEngine = engine
	g.runnerSrc = runnerSrc
	g.runnerID = runnerID
	g.startedAt = time.Now()
	return nil
}

// ClearActiveEngine 清除活动引擎引用
func (g *GlobalEngineState) ClearActiveEngine() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.activeEngine = nil
	g.runnerSrc = ""
	g.runnerID = ""
	g.startedAt = time.Time{}
}

// IsRunning 检查引擎是否正在运行 - 委托给实例
func (g *GlobalEngineState) IsRunning() bool {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if g.activeEngine == nil {
		return false
	}
	return g.activeEngine.IsRunning()
}

// GetEngineState 获取引擎当前状态 - 委托给实例
func (g *GlobalEngineState) GetEngineState() EngineState {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if g.activeEngine == nil {
		return StateIdle
	}
	return g.activeEngine.State()
}

// GetRunnerSource 获取当前运行来源
func (g *GlobalEngineState) GetRunnerSource() string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.runnerSrc
}

// GetRunnerID 获取当前运行实例ID
func (g *GlobalEngineState) GetRunnerID() string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.runnerID
}

// GetStartedAt 获取启动时间
func (g *GlobalEngineState) GetStartedAt() time.Time {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.startedAt
}

// GetStatus 获取完整状态信息
func (g *GlobalEngineState) GetStatus() map[string]interface{} {
	g.mu.RLock()
	defer g.mu.RUnlock()

	state := "Idle"
	isRunning := false
	// 在锁内复制指针并检查，防止并发修改
	if engine := g.activeEngine; engine != nil {
		state = engine.State().String()
		isRunning = engine.IsRunning()
	}

	return map[string]interface{}{
		"state":     state,
		"isRunning": isRunning,
		"runnerSrc": g.runnerSrc,
		"runnerID":  g.runnerID,
		"startedAt": g.startedAt,
	}
}

// 兼容性全局函数
func TryAcquireEngine(runnerSrc string) error {
	return TryAcquireEngineWithID(runnerSrc, "")
}

func TryAcquireEngineWithID(runnerSrc, runnerID string) error {
	// 检查是否有运行中的引擎
	if GetGlobalState().IsRunning() {
		g := GetGlobalState()
		g.mu.RLock()
		defer g.mu.RUnlock()
		if g.runnerSrc != "" && g.runnerID != "" {
			return fmt.Errorf("引擎正在由 %s (ID: %s) 运行中，请等待当前任务完成", g.runnerSrc, g.runnerID)
		}
		if g.runnerSrc != "" {
			return fmt.Errorf("引擎正在由 %s 运行中，请等待当前任务完成", g.runnerSrc)
		}
		return fmt.Errorf("引擎正在运行中，请等待当前任务完成")
	}
	return nil
}

func ReleaseEngine() {
	GetGlobalState().ClearActiveEngine()
}

func IsEngineRunning() bool {
	return GetGlobalState().IsRunning()
}

func GetEngineRunnerSource() string {
	return GetGlobalState().GetRunnerSource()
}

func GetEngineRunnerID() string {
	return GetGlobalState().GetRunnerID()
}

func GetEngineStatus() map[string]interface{} {
	return GetGlobalState().GetStatus()
}
