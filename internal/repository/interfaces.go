// Package repository 提供数据访问层的抽象接口和实现
// 该包负责将数据访问逻辑从 config 包和 UI Service 层解耦
package repository

import (
	"context"
	"time"

	"github.com/NetWeaverGo/core/internal/models"
	"gorm.io/gorm"
)

// ============================================================================
// 设备资产相关接口
// ============================================================================

// DeviceRepository 设备资产数据访问接口
// 提供设备资产的 CRUD 操作和查询功能
type DeviceRepository interface {
	// 查询操作
	FindAll() ([]models.DeviceAsset, error)
	FindByID(id uint) (*models.DeviceAsset, error)
	FindByIPs(ips []string) ([]models.DeviceAsset, error)
	FindByIP(ip string) (*models.DeviceAsset, error)
	Count() (int64, error)
	ExistsByIP(ip string) (bool, error)

	// 分页查询（支持条件过滤）
	Query(opts DeviceQueryOptions) (*DeviceQueryResult, error)

	// 写入操作
	Create(device *models.DeviceAsset) error
	CreateBatch(devices []models.DeviceAsset) error
	Update(device *models.DeviceAsset) error
	UpdateBatch(devices []models.DeviceAsset) error
	Delete(id uint) error
	DeleteBatch(ids []uint) error

	// 事务支持
	WithTx(tx *gorm.DB) DeviceRepository
	BeginTx() *gorm.DB

	// 聚合查询
	GetDistinctGroups() ([]string, error)
	GetDistinctTags() ([]string, error)
}

// DeviceQueryOptions 设备查询选项
type DeviceQueryOptions struct {
	SearchQuery string   // 搜索关键词
	FilterField string   // 过滤字段 (如 group, ip, tag, protocol)
	FilterValue string   // 过滤值
	Page        int      // 页码 (1-based)
	PageSize    int      // 每页条数
	SortBy      string   // 排序字段
	SortOrder   string   // 排序方向: asc/desc
	IPFilter    []string // IP 过滤列表（用于筛选指定设备）
	IDFilter    []uint   // ID 过滤列表
}

// DeviceQueryResult 设备查询结果
type DeviceQueryResult struct {
	Data       []models.DeviceAssetListItem // 数据列表（不含密码）
	Total      int64                        // 总记录数
	Page       int                          // 当前页码
	PageSize   int                          // 每页条数
	TotalPages int                          // 总页数
}

// ============================================================================
// 命令组相关接口（预留）
// ============================================================================

// CommandGroupRepository 命令组数据访问接口
type CommandGroupRepository interface {
	FindAll() ([]models.CommandGroup, error)
	FindByID(id uint) (*models.CommandGroup, error)
	FindByName(name string) (*models.CommandGroup, error)
	FindDefault() (*models.CommandGroup, error)
	Create(group *models.CommandGroup) error
	Update(group *models.CommandGroup) error
	Delete(id uint) error
}

// ============================================================================
// 任务组相关接口（预留）
// ============================================================================

// TaskGroupRepository 任务组数据访问接口
type TaskGroupRepository interface {
	FindAll() ([]models.TaskGroup, error)
	FindByID(id uint) (*models.TaskGroup, error)
	Create(group *models.TaskGroup) error
	Update(group *models.TaskGroup) error
	UpdateStatus(id uint, status string) error
	Delete(id uint) error
}

// ============================================================================
// MIB 管理相关接口
// ============================================================================

// MIBRepository MIB 数据访问接口
// 提供 MIB 模块和节点的 CRUD 操作
// 查询方法在记录不存在时返回 nil, nil
type MIBRepository interface {
	// 模块管理
	GetAllModules() ([]models.MIBModule, error)
	GetModuleByID(id uint) (*models.MIBModule, error)
	GetModuleByName(name string) (*models.MIBModule, error)
	SaveModule(module *models.MIBModule) error
	DeleteModule(id uint) error

	// 文件夹管理
	GetAllFolders() ([]models.MIBFolder, error)
	GetFolderByID(id uint) (*models.MIBFolder, error)
	GetFolderByName(name string) (*models.MIBFolder, error)
	SaveFolder(folder *models.MIBFolder) error
	DeleteFolder(id uint) error
	MoveModuleToFolder(moduleID uint, folderID *uint) error

	// 节点管理
	GetNodeByID(id uint) (*models.MIBNode, error)
	GetNodeByOID(oid string) (*models.MIBNode, error)
	GetNodeByName(name string) (*models.MIBNode, error)
	GetNodesByModule(moduleID uint) ([]models.MIBNode, error)
	GetNodesByOIDs(oids []string) ([]models.MIBNode, error)
	GetChildNodes(parentOID string) ([]models.MIBNode, error)
	CountChildNodes(parentOID string) (int64, error)
	SaveNode(node *models.MIBNode) error
	SaveNodes(nodes []models.MIBNode) error
	DeleteNode(id uint) error
	DeleteNodesByModule(moduleID uint) error
	SearchNodes(query string) ([]models.MIBNode, error)
	SearchNodesInModule(moduleID uint, query string) ([]models.MIBNode, error)
	GetAllNodes() ([]models.MIBNode, error)
	GetNodesBatch(offset, limit int) ([]models.MIBNode, error)

	// 批量查询
	CountChildNodesBatch(parentOIDs []string) (map[string]int64, error)

	// 事务支持
	WithTx(tx *gorm.DB) MIBRepository
	BeginTx() *gorm.DB
}

// ============================================================================
// SNMP Trap 相关接口
// ============================================================================

// TrapRepository Trap 记录仓库接口
// 提供 Trap 记录、过滤规则和服务器配置的数据访问
type TrapRepository interface {
	// Trap 记录 CRUD
	CreateTrap(ctx context.Context, trap *models.SNMPTrapRecord) error
	GetTrap(ctx context.Context, id uint) (*models.SNMPTrapRecord, error)
	ListTraps(ctx context.Context, filter TrapFilter, page, pageSize int) ([]*models.SNMPTrapRecord, int64, error)
	DeleteTrap(ctx context.Context, id uint) error
	DeleteTrapsBefore(ctx context.Context, before time.Time) (int64, error)
	AcknowledgeTrap(ctx context.Context, id uint) error
	BatchAcknowledgeTraps(ctx context.Context, ids []uint) error
	GetTrapStats(ctx context.Context) (*TrapStats, error)

	// 过滤规则 CRUD
	CreateFilterRule(ctx context.Context, rule *models.SNMPTrapFilterRule) error
	GetFilterRule(ctx context.Context, id uint) (*models.SNMPTrapFilterRule, error)
	ListFilterRules(ctx context.Context) ([]*models.SNMPTrapFilterRule, error)
	UpdateFilterRule(ctx context.Context, rule *models.SNMPTrapFilterRule) error
	DeleteFilterRule(ctx context.Context, id uint) error
	ReorderFilterRules(ctx context.Context, ids []uint) error

	// 服务器配置 CRUD
	CreateServerConfig(ctx context.Context, config *models.SNMPServerConfig) error
	GetServerConfig(ctx context.Context, id uint) (*models.SNMPServerConfig, error)
	GetActiveServerConfig(ctx context.Context) (*models.SNMPServerConfig, error)
	ListServerConfigs(ctx context.Context) ([]*models.SNMPServerConfig, error)
	UpdateServerConfig(ctx context.Context, config *models.SNMPServerConfig) error
	DeleteServerConfig(ctx context.Context, id uint) error
}

// TrapFilter Trap 过滤条件
type TrapFilter struct {
	SourceIP     string     // 来源 IP 过滤
	TrapOID      string     // Trap OID 过滤
	Severity     string     // 严重级别过滤
	StartTime    *time.Time // 开始时间
	EndTime      *time.Time // 结束时间
	Acknowledged *bool      // 确认状态
	SearchQuery  string     // 搜索关键词
}

// TrapStats Trap 统计信息
type TrapStats struct {
	TotalCount     int64 `json:"totalCount"`
	Unacknowledged int64 `json:"unacknowledged"`
	CriticalCount  int64 `json:"criticalCount"`
	MajorCount     int64 `json:"majorCount"`
	MinorCount     int64 `json:"minorCount"`
	InfoCount      int64 `json:"infoCount"`
	TodayCount     int64 `json:"todayCount"`
	LastHourCount  int64 `json:"lastHourCount"`
}

// ============================================================================
// SNMP 轮询相关接口
// ============================================================================

// PollingRepository 轮询配置仓库接口
// 提供凭据、模板、目标和结果的 CRUD 操作
type PollingRepository interface {
	// 凭据管理
	CreateCredential(ctx context.Context, cred *models.SNMPCredential) error
	GetCredential(ctx context.Context, id uint) (*models.SNMPCredential, error)
	ListCredentials(ctx context.Context) ([]*models.SNMPCredential, error)
	UpdateCredential(ctx context.Context, cred *models.SNMPCredential) error
	DeleteCredential(ctx context.Context, id uint) error

	// 轮询模板管理
	CreatePollingTemplate(ctx context.Context, template *models.SNMPPollingTemplate) error
	GetPollingTemplate(ctx context.Context, id uint) (*models.SNMPPollingTemplate, error)
	ListPollingTemplates(ctx context.Context) ([]*models.SNMPPollingTemplate, error)
	UpdatePollingTemplate(ctx context.Context, template *models.SNMPPollingTemplate) error
	DeletePollingTemplate(ctx context.Context, id uint) error

	// 轮询目标管理
	CreatePollingTarget(ctx context.Context, target *models.SNMPPollingTarget) error
	GetPollingTarget(ctx context.Context, id uint) (*models.SNMPPollingTarget, error)
	ListPollingTargets(ctx context.Context, filter PollingTargetFilter) ([]*models.SNMPPollingTarget, int64, error)
	UpdatePollingTarget(ctx context.Context, target *models.SNMPPollingTarget) error
	DeletePollingTarget(ctx context.Context, id uint) error

	// 轮询结果管理
	CreatePollingResult(ctx context.Context, result *models.SNMPPollingResult) error
	CreatePollingResults(ctx context.Context, results []*models.SNMPPollingResult) error
	GetPollingResult(ctx context.Context, id uint) (*models.SNMPPollingResult, error)
	ListPollingResults(ctx context.Context, filter PollingResultFilter, page, pageSize int) ([]*models.SNMPPollingResult, int64, error)
	DeletePollingResultsBefore(ctx context.Context, before time.Time) (int64, error)

	// 统计查询
	GetPollingStats(ctx context.Context, targetID uint) (*PollingStats, error)
}

// PollingTargetFilter 轮询目标过滤条件
type PollingTargetFilter struct {
	TemplateID *uint  // 模板 ID 过滤
	Enabled    *bool  // 启用状态过滤
	SearchIP   string // IP 地址搜索
}

// PollingResultFilter 轮询结果过滤条件
type PollingResultFilter struct {
	TargetID  *uint      // 目标 ID 过滤
	OID       string     // OID 过滤
	StartTime *time.Time // 开始时间
	EndTime   *time.Time // 结束时间
	BatchID   string     // 批次 ID 过滤
}

// PollingStats 轮询统计
type PollingStats struct {
	TotalPolls   int64   `json:"totalPolls"`   // 总轮询次数
	SuccessCount int64   `json:"successCount"` // 成功次数
	FailCount    int64   `json:"failCount"`    // 失败次数
	AvgLatencyMs float64 `json:"avgLatencyMs"` // 平均延迟（毫秒）
	LastPollTime *time.Time `json:"lastPollTime"` // 最后轮询时间
}
