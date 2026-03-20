// Package repository 提供数据访问层的抽象接口和实现
// 该包负责将数据访问逻辑从 config 包和 UI Service 层解耦
package repository

import (
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
