package engine

import (
	"fmt"
	"sync"
	"time"
)

// GlobalEngineState 全局引擎状态管理器
// 用于统一管理所有引擎实例的运行状态，防止并发执行冲突
// 使用读写锁优化并发读取性能
type GlobalEngineState struct {
	mu        sync.RWMutex // 改为读写锁，优化只读操作性能
	isRunning bool
	runnerID  string
	runnerSrc string    // 运行来源：engine, taskgroup, backup
	startedAt time.Time // 启动时间
}

// globalEngine 全局单例实例
var globalEngine = &GlobalEngineState{}

// TryAcquire 尝试获取引擎运行锁
// 如果引擎已在运行，返回错误；否则获取锁并返回 nil
func (g *GlobalEngineState) TryAcquire(runnerSrc, runnerID string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.isRunning {
		if g.runnerSrc != "" && g.runnerID != "" {
			return fmt.Errorf("引擎正在由 %s (ID: %s) 运行中，请等待当前任务完成", g.runnerSrc, g.runnerID)
		}
		if g.runnerSrc != "" {
			return fmt.Errorf("引擎正在由 %s 运行中，请等待当前任务完成", g.runnerSrc)
		}
		return fmt.Errorf("引擎正在运行中，请等待当前任务完成")
	}

	g.isRunning = true
	g.runnerSrc = runnerSrc
	g.runnerID = runnerID
	g.startedAt = time.Now()
	return nil
}

// Release 释放引擎运行锁
func (g *GlobalEngineState) Release() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.isRunning = false
	g.runnerSrc = ""
	g.runnerID = ""
	g.startedAt = time.Time{}
}

// IsRunning 检查引擎是否正在运行
func (g *GlobalEngineState) IsRunning() bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.isRunning
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
	return map[string]interface{}{
		"isRunning": g.isRunning,
		"runnerSrc": g.runnerSrc,
		"runnerID":  g.runnerID,
		"startedAt": g.startedAt,
	}
}

// TryAcquireEngine 全局函数：尝试获取引擎锁（兼容旧接口）
func TryAcquireEngine(runnerSrc string) error {
	return globalEngine.TryAcquire(runnerSrc, "")
}

// TryAcquireEngineWithID 全局函数：尝试获取引擎锁（带实例ID）
func TryAcquireEngineWithID(runnerSrc, runnerID string) error {
	return globalEngine.TryAcquire(runnerSrc, runnerID)
}

// ReleaseEngine 全局函数：释放引擎锁
func ReleaseEngine() {
	globalEngine.Release()
}

// IsEngineRunning 全局函数：检查引擎是否运行中
func IsEngineRunning() bool {
	return globalEngine.IsRunning()
}

// GetEngineRunnerSource 全局函数：获取当前运行来源
func GetEngineRunnerSource() string {
	return globalEngine.GetRunnerSource()
}

// GetEngineRunnerID 全局函数：获取当前运行实例ID
func GetEngineRunnerID() string {
	return globalEngine.GetRunnerID()
}

// GetEngineStatus 全局函数：获取完整状态信息
func GetEngineStatus() map[string]interface{} {
	return globalEngine.GetStatus()
}
