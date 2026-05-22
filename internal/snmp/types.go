// Package snmp 提供 SNMP 核心业务功能
// types.go 定义核心类型和事件通知接口
package snmp

import (
	"context"

	"github.com/NetWeaverGo/core/internal/models"
)

// ============================================================================
// 事件通知接口（解耦 Wails 依赖）
// ============================================================================

// EventNotifier 事件通知接口
// SNMP 核心模块通过此接口推送事件，不直接依赖 Wails 框架
// 实现：internal/ui/snmp_event_notifier.go（Wails Events 推送）
type EventNotifier interface {
	// Trap 事件
	NotifyNewTrap(trap *models.SNMPTrapRecord)
	NotifyTrapStats(stats *TrapStats)
	NotifyListenerStatus(running bool)

	// 轮询事件
	NotifyPollResult(targetID uint, results []models.SNMPPollingResult)
	NotifyPollError(targetID uint, err error)
	NotifySchedulerStatus(running bool)

	// MIB 事件
	NotifyMIBImported(module *models.MIBModule)
	NotifyMIBDeleted(moduleID uint)
}

// ============================================================================
// Trap 相关类型
// ============================================================================

// TrapStats Trap 告警统计信息
type TrapStats struct {
	TotalCount        int64 `json:"totalCount"`
	Unacknowledged    int64 `json:"unacknowledged"`
	CriticalCount     int64 `json:"criticalCount"`
	MajorCount        int64 `json:"majorCount"`
	MinorCount        int64 `json:"minorCount"`
	InfoCount         int64 `json:"infoCount"`
	TodayCount        int64 `json:"todayCount"`
	LastHourCount     int64 `json:"lastHourCount"`
}

// TrapQueryOptions Trap 查询选项
type TrapQueryOptions struct {
	SearchQuery   string   `json:"searchQuery"`   // 搜索关键词（OID/IP/名称）
	Severity      string   `json:"severity"`      // 严重级别过滤
	SourceIP      string   `json:"sourceIP"`      // 来源 IP 过滤
	StartTime     string   `json:"startTime"`     // 开始时间
	EndTime       string   `json:"endTime"`       // 结束时间
	Acknowledged  *bool    `json:"acknowledged"`  // 确认状态过滤
	TrapOIDPrefix string   `json:"trapOIDPrefix"` // OID 前缀过滤
	Page          int      `json:"page"`          // 页码（1-based）
	PageSize      int      `json:"pageSize"`      // 每页条数
	SortBy        string   `json:"sortBy"`        // 排序字段
	SortOrder     string   `json:"sortOrder"`     // 排序方向（asc/desc）
}

// TrapQueryResult Trap 查询结果
type TrapQueryResult struct {
	Data       []models.SNMPTrapRecord `json:"data"`
	Total      int64                   `json:"total"`
	Page       int                     `json:"page"`
	PageSize   int                     `json:"pageSize"`
	TotalPages int                     `json:"totalPages"`
}

// ============================================================================
// 轮询相关类型
// ============================================================================

// PollingStatus 轮询目标状态
type PollingStatus struct {
	TargetID      uint   `json:"targetId"`
	TargetIP      string `json:"targetIP"`
	DisplayName   string `json:"displayName"`
	Enabled       bool   `json:"enabled"`
	LastPollAt    string `json:"lastPollAt"`
	LastPollStatus string `json:"lastPollStatus"`
	NextPollAt    string `json:"nextPollAt"`
	PollInterval  int    `json:"pollInterval"`
}

// PollingResultQueryOpts 轮询结果查询选项
type PollingResultQueryOpts struct {
	TargetID  uint   `json:"targetId"`
	TargetIP  string `json:"targetIP"`
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime"`
	OID       string `json:"oid"`
	Page      int    `json:"page"`
	PageSize  int    `json:"pageSize"`
}

// PollingResultQueryResult 轮询结果查询结果
type PollingResultQueryResult struct {
	Data       []models.SNMPPollingResult `json:"data"`
	Total      int64                      `json:"total"`
	Page       int                        `json:"page"`
	PageSize   int                        `json:"pageSize"`
	TotalPages int                        `json:"totalPages"`
}

// SNMPResult SNMP 操作结果
type SNMPResult struct {
	OID       string `json:"oid"`
	OIDName   string `json:"oidName"`
	Value     string `json:"value"`
	ValueType string `json:"valueType"`
	Error     string `json:"error,omitempty"`
}

// ============================================================================
// MIB 相关类型
// ============================================================================

// MIBTreeNode MIB 树节点（用于前端树形展示）
type MIBTreeNode struct {
	ID          uint         `json:"id"`
	OID         string       `json:"oid"`
	Name        string       `json:"name"`
	NodeType    string       `json:"nodeType"`
	Syntax      string       `json:"syntax"`
	Access      string       `json:"access"`
	Status      string       `json:"status"`
	Description string       `json:"description"`
	Children    []MIBTreeNode `json:"children,omitempty"`
	HasChildren bool         `json:"hasChildren"`
}

// MIBImportResult MIB 导入结果
type MIBImportResult struct {
	Module      *models.MIBModule `json:"module"`
	Nodes       []models.MIBNode  `json:"-"` // 内部使用，不序列化到 JSON
	NodeCount   int               `json:"nodeCount"`
	ErrorCount  int               `json:"errorCount"`
	Errors      []MIBParseError   `json:"errors,omitempty"`
}

// MIBParseError MIB 解析错误详情
type MIBParseError struct {
	LineNumber int    `json:"lineNumber,omitempty"`
	NodeName   string `json:"nodeName,omitempty"`
	Message    string `json:"message"`
}

// NodeQueryOptions MIB 节点查询选项
type NodeQueryOptions struct {
	ModuleID  *uint  `json:"moduleId"`
	ParentOID string `json:"parentOid"`
	NodeType  string `json:"nodeType"`
	Search    string `json:"search"`
	Page      int    `json:"page"`
	PageSize  int    `json:"pageSize"`
}

// ============================================================================
// OID 翻译相关类型
// ============================================================================

// OIDTranslation OID 翻译结果
type OIDTranslation struct {
	OID         string `json:"oid"`
	Name        string `json:"name"`
	Module      string `json:"module"`
	Description string `json:"description"`
	Found       bool   `json:"found"`
}

// ============================================================================
// 凭据加密相关类型
// ============================================================================

// CredentialCrypto SNMP 凭据加密管理器
// 使用 AES-256-GCM 对敏感字段加密
type CredentialCrypto struct {
	key []byte // 32 bytes AES-256 key
}

// ============================================================================
// 上下文支持
// ============================================================================

// CancellableOperation 可取消的操作接口
type CancellableOperation interface {
	Start(ctx context.Context) error
	Stop() error
	IsRunning() bool
}