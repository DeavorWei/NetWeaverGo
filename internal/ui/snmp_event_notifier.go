// Package ui 提供 Wails 暴露层服务
// snmp_event_notifier.go 实现 SNMP 事件通知器，通过 Wails Events 推送到前端
package ui

import (
	"time"

	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/models"
	"github.com/NetWeaverGo/core/internal/snmp"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// ============================================================================
// SNMP 事件常量定义
// ============================================================================

const (
	// Trap 事件
	EventSNMPTrapReceived     = "snmp:trap:received"
	EventSNMPTrapStats        = "snmp:trap:stats"
	EventSNMPListenerStatus   = "snmp:listener:status"

	// 轮询事件
	EventSNMPPollResult       = "snmp:poll:result"
	EventSNMPPollError        = "snmp:poll:error"
	EventSNMPSchedulerStatus  = "snmp:scheduler:status"
	EventSNMPPollingResult    = "snmp:polling:result"

	// MIB 事件
	EventSNMPMIBImported      = "snmp:mib:imported"
	EventSNMPMIBDeleted       = "snmp:mib:deleted"
	EventSNMPMIBImportProgress = "snmp:mib:import:progress"
)

// ============================================================================
// SNMPEventNotifier 实现
// ============================================================================

// SNMPEventNotifier SNMP 事件通知器
// 实现 snmp.EventNotifier 接口，将 SNMP 内部事件转换为 Wails 事件推送到前端
type SNMPEventNotifier struct {
	wailsApp *application.App
}

// NewSNMPEventNotifier 创建 SNMP 事件通知器实例
func NewSNMPEventNotifier() *SNMPEventNotifier {
	return &SNMPEventNotifier{}
}

// SetWailsApp 设置 Wails 应用实例
// 在应用启动后调用，用于事件推送
func (n *SNMPEventNotifier) SetWailsApp(app *application.App) {
	n.wailsApp = app
	logger.Info("SNMP", "-", "SNMP 事件通知器已绑定 Wails 应用")
}

// ============================================================================
// Trap 事件通知
// ============================================================================

// NotifyNewTrap 通知新 Trap 告警
func (n *SNMPEventNotifier) NotifyNewTrap(trap *models.SNMPTrapRecord) {
	if n.wailsApp == nil || trap == nil {
		return
	}

	// 转换为前端事件格式（轻量级）
	event := snmp.TrapEvent{
		SourceIP:   trap.SourceIP,
		SourcePort: trap.SourcePort,
		TrapOID:    trap.TrapOID,
		TrapName:   trap.TrapName,
		Severity:   trap.Severity,
		Community:  trap.Community,
		Version:    trap.Version,
		ReceivedAt: trap.ReceivedAt.UnixMilli(),
	}

	n.emitEvent(EventSNMPTrapReceived, event)
	logger.Debug("SNMP", "-", "Trap 事件已推送: %s from %s", trap.TrapOID, trap.SourceIP)
}

// NotifyTrapStats 通知 Trap 统计信息更新
func (n *SNMPEventNotifier) NotifyTrapStats(stats *snmp.TrapStats) {
	if n.wailsApp == nil || stats == nil {
		return
	}

	n.emitEvent(EventSNMPTrapStats, stats)
}

// NotifyListenerStatus 通知 Trap 监听器状态变更
// P2-13: 发送完整 ListenerStats 对象，与前端 ListenerStatus 类型匹配
func (n *SNMPEventNotifier) NotifyListenerStatus(stats *snmp.ListenerStats) {
	if n.wailsApp == nil || stats == nil {
		return
	}

	// 构建与前端 ListenerStatus 类型匹配的事件数据
	event := ListenerStatusVM{
		IsRunning:   stats.IsRunning,
		ListenAddr:  stats.ListenAddr,
		TotalTraps:  stats.TotalTraps,
		FilteredOut: stats.FilteredOut,
	}

	if !stats.LastTrapTime.IsZero() {
		event.LastTrapTime = stats.LastTrapTime.Format(time.RFC3339)
	}
	if !stats.StartTime.IsZero() {
		event.StartTime = stats.StartTime.Format(time.RFC3339)
	}

	n.emitEvent(EventSNMPListenerStatus, event)

	statusStr := "stopped"
	if stats.IsRunning {
		statusStr = "running"
	}
	logger.Info("SNMP", "-", "Trap 监听器状态变更: %s", statusStr)
}

// NotifyTrapReceived 通知 Trap 接收事件（轻量级）
func (n *SNMPEventNotifier) NotifyTrapReceived(trap snmp.TrapEvent) {
	if n.wailsApp == nil {
		return
	}

	n.emitEvent(EventSNMPTrapReceived, trap)
}

// ============================================================================
// 轮询事件通知
// ============================================================================

// NotifyPollResult 通知轮询结果
func (n *SNMPEventNotifier) NotifyPollResult(targetID uint, results []models.SNMPPollingResult) {
	if n.wailsApp == nil || len(results) == 0 {
		return
	}

	// 构建事件数据
	event := map[string]interface{}{
		"targetId": targetID,
		"count":    len(results),
		"results":  results,
	}

	n.emitEvent(EventSNMPPollResult, event)
}

// NotifyPollError 通知轮询错误
func (n *SNMPEventNotifier) NotifyPollError(targetID uint, err error) {
	if n.wailsApp == nil || err == nil {
		return
	}

	n.emitEvent(EventSNMPPollError, map[string]interface{}{
		"targetId": targetID,
		"error":    err.Error(),
	})

	logger.Warn("SNMP", "-", "轮询错误已推送: target=%d, error=%v", targetID, err)
}

// NotifySchedulerStatus 通知轮询调度器状态变更
func (n *SNMPEventNotifier) NotifySchedulerStatus(running bool) {
	if n.wailsApp == nil {
		return
	}

	statusStr := "stopped"
	if running {
		statusStr = "running"
	}
	n.emitEvent(EventSNMPSchedulerStatus, map[string]interface{}{
		"running": running,
		"status":  statusStr,
	})
	logger.Info("SNMP", "-", "轮询调度器状态变更: %s", statusStr)
}

// NotifyPollingResult 通知轮询结果事件（轻量级）
func (n *SNMPEventNotifier) NotifyPollingResult(result snmp.PollingResultEvent) {
	if n.wailsApp == nil {
		return
	}

	n.emitEvent(EventSNMPPollingResult, result)
}

// ============================================================================
// MIB 事件通知
// ============================================================================

// NotifyMIBImported 通知 MIB 模块导入完成
func (n *SNMPEventNotifier) NotifyMIBImported(module *models.MIBModule) {
	if n.wailsApp == nil || module == nil {
		return
	}

	n.emitEvent(EventSNMPMIBImported, module)
	logger.Info("SNMP", "-", "MIB 模块导入完成事件已推送: %s (%d 节点)", module.Name, module.NodeCount)
}

// NotifyMIBDeleted 通知 MIB 模块删除
func (n *SNMPEventNotifier) NotifyMIBDeleted(moduleID uint) {
	if n.wailsApp == nil {
		return
	}

	n.emitEvent(EventSNMPMIBDeleted, map[string]interface{}{
		"moduleId": moduleID,
	})

	logger.Info("SNMP", "-", "MIB 模块删除事件已推送: ID=%d", moduleID)
}

// NotifyMIBImportProgress 通知 MIB 导入进度
func (n *SNMPEventNotifier) NotifyMIBImportProgress(progress snmp.MIBImportProgress) {
	if n.wailsApp == nil {
		return
	}

	n.emitEvent(EventSNMPMIBImportProgress, progress)

	// 仅在关键阶段记录日志
	if progress.Phase == "completed" || progress.Phase == "error" {
		logger.Info("SNMP", "-", "MIB 导入进度: %s %s (%.1f%%)", 
			progress.FileName, progress.Phase, progress.Progress)
	}
}

// ============================================================================
// 辅助方法
// ============================================================================

// emitEvent 发送事件到前端
func (n *SNMPEventNotifier) emitEvent(eventName string, data interface{}) {
	if n.wailsApp == nil || n.wailsApp.Event == nil {
		logger.Warn("SNMP", "-", "Wails 应用未初始化，无法推送事件: %s", eventName)
		return
	}

	n.wailsApp.Event.Emit(eventName, data)
}

// EmitCustomEvent 发送自定义事件（供外部服务调用）
func (n *SNMPEventNotifier) EmitCustomEvent(eventName string, data interface{}) {
	n.emitEvent(eventName, data)
}