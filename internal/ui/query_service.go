package ui

import (
	"context"
	"strings"

	"github.com/NetWeaverGo/core/internal/config"
	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/models"
	"github.com/NetWeaverGo/core/internal/repository"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// QueryService 查询服务 - 提供带条件的列表查询
// 前端无需本地过滤，直接调用后端查询接口
type QueryService struct {
	wailsApp *application.App
	repo     repository.DeviceRepository
}

// NewQueryService 创建查询服务实例
func NewQueryService() *QueryService {
	return &QueryService{
		repo: repository.NewDeviceRepository(),
	}
}

// NewQueryServiceWithRepo 使用指定 Repository 创建查询服务实例（用于测试）
func NewQueryServiceWithRepo(repo repository.DeviceRepository) *QueryService {
	return &QueryService{
		repo: repo,
	}
}

// ServiceStartup Wails 服务启动生命周期钩子
func (s *QueryService) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	s.wailsApp = application.Get()
	logger.Info("Query", "-", "查询服务已就绪")
	return nil
}

// ListDevices 查询设备列表（使用 Repository 分页查询）
func (s *QueryService) ListDevices(opts QueryOptions) *QueryResult {
	return s.queryDevicesFromRepo(opts)
}

// queryDevicesFromRepo 从 Repository 查询设备列表
func (s *QueryService) queryDevicesFromRepo(opts QueryOptions) *QueryResult {
	// 转换查询选项
	repoOpts := repository.DeviceQueryOptions{
		SearchQuery: opts.SearchQuery,
		FilterField: opts.FilterField,
		FilterValue: opts.FilterValue,
		Page:        opts.Page,
		PageSize:    opts.PageSize,
		SortBy:      opts.SortBy,
		SortOrder:   opts.SortOrder,
	}

	result, err := s.repo.Query(repoOpts)
	if err != nil {
		logger.Error("Query", "-", "查询设备失败: %v", err)
		return emptyDeviceQueryResult(opts)
	}

	return &QueryResult{
		Data:       result.Data,
		Total:      int(result.Total),
		Page:       result.Page,
		PageSize:   result.PageSize,
		TotalPages: result.TotalPages,
	}
}

func emptyDeviceQueryResult(opts QueryOptions) *QueryResult {
	pageSize := opts.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}
	page := opts.Page
	if page <= 0 {
		page = 1
	}

	return &QueryResult{
		Data:       []models.DeviceAssetListItem{},
		Total:      0,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: 0,
	}
}

var deviceSortFieldMap = map[string]string{
	"id":         "id",
	"ip":         "ip",
	"group":      "group_name",
	"group_name": "group_name",
	"protocol":   "protocol",
	"port":       "port",
	"username":   "username",
	"tags":       "tags",
	"createdat":  "created_at",
	"created_at": "created_at",
	"updatedat":  "updated_at",
	"updated_at": "updated_at",
}

func normalizeDeviceFilterField(field string) string {
	switch strings.ToLower(strings.TrimSpace(field)) {
	case "group", "group_name":
		return "group"
	case "ip":
		return "ip"
	case "tag", "tags":
		return "tag"
	case "protocol":
		return "protocol"
	default:
		return ""
	}
}

// buildOrderClause 构建排序子句
func (s *QueryService) buildOrderClause(sortBy, sortOrder string) string {
	field, ok := deviceSortFieldMap[strings.ToLower(strings.TrimSpace(sortBy))]
	if !ok {
		field = "ip"
	}

	order := "ASC"
	if strings.EqualFold(sortOrder, "desc") {
		order = "DESC"
	}

	return field + " " + order
}

// ListTaskGroups 查询任务组列表（支持搜索、过滤、分页）
func (s *QueryService) ListTaskGroups(opts QueryOptions) *QueryResult {
	// 加载所有任务组
	allGroups, err := config.ListTaskGroups()
	if err != nil {
		logger.Error("Query", "-", "加载任务组列表失败: %v", err)
		return &QueryResult{
			Data:       []models.TaskGroup{},
			Total:      0,
			Page:       opts.Page,
			PageSize:   opts.PageSize,
			TotalPages: 0,
		}
	}

	// 应用搜索过滤
	filtered := s.filterTaskGroups(allGroups, opts)

	// 应用排序
	sorted := s.sortTaskGroups(filtered, opts.SortBy, opts.SortOrder)

	// 计算分页
	total := len(sorted)
	pageSize := opts.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}
	page := opts.Page
	if page <= 0 {
		page = 1
	}

	totalPages := (total + pageSize - 1) / pageSize
	if totalPages < 1 {
		totalPages = 1
	}

	// 边界检查
	start := (page - 1) * pageSize
	if start >= total {
		start = 0
		page = 1
	}
	end := start + pageSize
	if end > total {
		end = total
	}

	return &QueryResult{
		Data:       sorted[start:end],
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}
}

// filterTaskGroups 过滤任务组列表
func (s *QueryService) filterTaskGroups(groups []models.TaskGroup, opts QueryOptions) []models.TaskGroup {
	if opts.SearchQuery == "" && opts.FilterValue == "" {
		return groups
	}

	query := strings.ToLower(strings.TrimSpace(opts.SearchQuery))
	filterVal := strings.ToLower(opts.FilterValue)

	result := make([]models.TaskGroup, 0, len(groups))

	for _, group := range groups {
		// 搜索过滤
		if query != "" {
			searchTarget := strings.ToLower(group.Name + " " + group.Description)
			for _, tag := range group.Tags {
				searchTarget += " " + strings.ToLower(tag)
			}

			if !strings.Contains(searchTarget, query) {
				continue
			}
		}

		// 状态过滤
		if filterVal != "" {
			if strings.ToLower(group.Status) != filterVal {
				continue
			}
		}

		// 模式过滤
		if opts.FilterField == "mode" && filterVal != "" {
			if group.Mode != filterVal {
				continue
			}
		}

		result = append(result, group)
	}

	return result
}

// sortTaskGroups 排序任务组列表
func (s *QueryService) sortTaskGroups(groups []models.TaskGroup, sortBy, sortOrder string) []models.TaskGroup {
	return groups // 数据库查询已排序
}

// ListCommandGroups 查询命令组列表（支持搜索、过滤、分页）
func (s *QueryService) ListCommandGroups(opts QueryOptions) *QueryResult {
	// 加载所有命令组
	allGroups, err := config.ListCommandGroups()
	if err != nil {
		logger.Error("Query", "-", "加载命令组列表失败: %v", err)
		return &QueryResult{
			Data:       []models.CommandGroup{},
			Total:      0,
			Page:       opts.Page,
			PageSize:   opts.PageSize,
			TotalPages: 0,
		}
	}

	// 应用搜索过滤
	filtered := s.filterCommandGroups(allGroups, opts)

	// 计算分页
	total := len(filtered)
	pageSize := opts.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}
	page := opts.Page
	if page <= 0 {
		page = 1
	}

	totalPages := (total + pageSize - 1) / pageSize
	if totalPages < 1 {
		totalPages = 1
	}

	start := (page - 1) * pageSize
	if start >= total {
		start = 0
		page = 1
	}
	end := start + pageSize
	if end > total {
		end = total
	}

	return &QueryResult{
		Data:       filtered[start:end],
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}
}

// filterCommandGroups 过滤命令组列表
func (s *QueryService) filterCommandGroups(groups []models.CommandGroup, opts QueryOptions) []models.CommandGroup {
	if opts.SearchQuery == "" && opts.FilterValue == "" {
		return groups
	}

	query := strings.ToLower(strings.TrimSpace(opts.SearchQuery))

	result := make([]models.CommandGroup, 0, len(groups))

	for _, group := range groups {
		// 搜索过滤
		if query != "" {
			searchTarget := strings.ToLower(group.Name + " " + group.Description)

			if !strings.Contains(searchTarget, query) {
				continue
			}
		}

		result = append(result, group)
	}

	return result
}

// GetDeviceGroups 获取所有设备分组名称（用于前端下拉选项）
func (s *QueryService) GetDeviceGroups() []string {
	groups, err := s.repo.GetDistinctGroups()
	if err != nil {
		logger.Error("Query", "-", "加载设备分组失败: %v", err)
		return []string{}
	}
	return groups
}

// GetDeviceTags 获取所有设备标签（用于前端下拉选项）
func (s *QueryService) GetDeviceTags() []string {
	tags, err := s.repo.GetDistinctTags()
	if err != nil {
		logger.Error("Query", "-", "加载设备标签失败: %v", err)
		return []string{}
	}
	return tags
}
