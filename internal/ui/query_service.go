package ui

import (
	"context"
	"strings"

	"github.com/NetWeaverGo/core/internal/config"
	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// QueryService 查询服务 - 提供带条件的列表查询
// 前端无需本地过滤，直接调用后端查询接口
type QueryService struct {
	wailsApp *application.App
}

// NewQueryService 创建查询服务实例
func NewQueryService() *QueryService {
	return &QueryService{}
}

// ServiceStartup Wails 服务启动生命周期钩子
func (s *QueryService) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	s.wailsApp = application.Get()
	logger.Info("Query", "-", "查询服务已就绪")
	return nil
}

// ListDevices 查询设备列表（使用数据库分页查询）
func (s *QueryService) ListDevices(opts QueryOptions) *QueryResult {
	// 使用数据库查询替代内存过滤
	result := s.queryDevicesFromDB(opts)
	return result
}

// queryDevicesFromDB 从数据库查询设备列表
func (s *QueryService) queryDevicesFromDB(opts QueryOptions) *QueryResult {
	if config.DB == nil {
		logger.Error("Query", "-", "数据库未初始化")
		return &QueryResult{
			Data:       []config.DeviceAsset{},
			Total:      0,
			Page:       opts.Page,
			PageSize:   opts.PageSize,
			TotalPages: 0,
		}
	}

	var devices []config.DeviceAsset
	var total int64

	// 构建查询
	query := config.DB.Model(&config.DeviceAsset{})

	// 应用搜索过滤
	if opts.SearchQuery != "" {
		searchPattern := "%" + strings.ToLower(opts.SearchQuery) + "%"
		switch opts.FilterField {
		case "group":
			query = query.Where("LOWER(`group`) LIKE ?", searchPattern)
		case "ip":
			query = query.Where("LOWER(ip) LIKE ?", searchPattern)
		case "tag":
			query = query.Where("LOWER(tags) LIKE ?", searchPattern)
		default:
			// 默认搜索所有字段
			query = query.Where(
				"LOWER(ip) LIKE ? OR LOWER(`group`) LIKE ? OR LOWER(username) LIKE ? OR LOWER(tags) LIKE ?",
				searchPattern, searchPattern, searchPattern, searchPattern,
			)
		}
	}

	// 应用状态/值过滤
	if opts.FilterValue != "" {
		switch opts.FilterField {
		case "protocol":
			query = query.Where("LOWER(protocol) = ?", strings.ToLower(opts.FilterValue))
		case "group":
			query = query.Where("LOWER(`group`) = ?", strings.ToLower(opts.FilterValue))
		}
	}

	// 计算总数
	query.Count(&total)

	// 应用排序
	orderClause := s.buildOrderClause(opts.SortBy, opts.SortOrder)
	if orderClause != "" {
		query = query.Order(orderClause)
	}

	// 计算分页参数
	pageSize := opts.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}
	page := opts.Page
	if page <= 0 {
		page = 1
	}

	totalPages := int((total + int64(pageSize) - 1) / int64(pageSize))
	if totalPages < 1 {
		totalPages = 1
	}

	// 边界检查
	offset := (page - 1) * pageSize
	if offset >= int(total) {
		offset = 0
		page = 1
	}

	// 执行分页查询
	query.Offset(offset).Limit(pageSize).Find(&devices)

	return &QueryResult{
		Data:       devices,
		Total:      int(total),
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}
}

// buildOrderClause 构建排序子句
func (s *QueryService) buildOrderClause(sortBy, sortOrder string) string {
	if sortBy == "" {
		sortBy = "ip"
	}

	order := "ASC"
	if sortOrder == "desc" {
		order = "DESC"
	}

	// 处理特殊字段名（如 group 是 SQL 关键字）
	field := sortBy
	if sortBy == "group" {
		field = "`group`"
	}

	return field + " " + order
}

// filterDevices 过滤设备列表（内存过滤，作为备用方案）
func (s *QueryService) filterDevices(devices []config.DeviceAsset, opts QueryOptions) []config.DeviceAsset {
	if opts.SearchQuery == "" && opts.FilterValue == "" {
		return devices
	}

	query := strings.ToLower(strings.TrimSpace(opts.SearchQuery))
	field := opts.FilterField
	filterVal := strings.ToLower(opts.FilterValue)

	result := make([]config.DeviceAsset, 0, len(devices))

	for _, dev := range devices {
		// 搜索过滤
		if query != "" {
			var searchTarget string
			switch field {
			case "group":
				searchTarget = strings.ToLower(dev.Group)
			case "ip":
				searchTarget = strings.ToLower(dev.IP)
			case "tag":
				// 搜索标签数组
				for _, tag := range dev.Tags {
					if strings.Contains(strings.ToLower(tag), query) {
						result = append(result, dev)
						break
					}
				}
				continue
			default:
				// 默认搜索所有字段
				searchTarget = strings.ToLower(dev.IP + " " + dev.Group + " " + dev.Username)
				for _, tag := range dev.Tags {
					searchTarget += " " + strings.ToLower(tag)
				}
			}

			if !strings.Contains(searchTarget, query) {
				continue
			}
		}

		// 状态/值过滤
		if filterVal != "" {
			switch field {
			case "protocol":
				if strings.ToLower(dev.Protocol) != filterVal {
					continue
				}
			case "group":
				if strings.ToLower(dev.Group) != filterVal {
					continue
				}
			}
		}

		result = append(result, dev)
	}

	return result
}

// sortDevices 排序设备列表
func (s *QueryService) sortDevices(devices []config.DeviceAsset, sortBy, sortOrder string) []config.DeviceAsset {
	return devices // 数据库查询已排序，此方法保留用于备用
}

// ListTaskGroups 查询任务组列表（支持搜索、过滤、分页）
func (s *QueryService) ListTaskGroups(opts QueryOptions) *QueryResult {
	// 加载所有任务组
	allGroups, err := config.ListTaskGroups()
	if err != nil {
		logger.Error("Query", "-", "加载任务组列表失败: %v", err)
		return &QueryResult{
			Data:       []config.TaskGroup{},
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
func (s *QueryService) filterTaskGroups(groups []config.TaskGroup, opts QueryOptions) []config.TaskGroup {
	if opts.SearchQuery == "" && opts.FilterValue == "" {
		return groups
	}

	query := strings.ToLower(strings.TrimSpace(opts.SearchQuery))
	filterVal := strings.ToLower(opts.FilterValue)

	result := make([]config.TaskGroup, 0, len(groups))

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
func (s *QueryService) sortTaskGroups(groups []config.TaskGroup, sortBy, sortOrder string) []config.TaskGroup {
	return groups // 数据库查询已排序
}

// ListCommandGroups 查询命令组列表（支持搜索、过滤、分页）
func (s *QueryService) ListCommandGroups(opts QueryOptions) *QueryResult {
	// 加载所有命令组
	allGroups, err := config.ListCommandGroups()
	if err != nil {
		logger.Error("Query", "-", "加载命令组列表失败: %v", err)
		return &QueryResult{
			Data:       []config.CommandGroup{},
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
func (s *QueryService) filterCommandGroups(groups []config.CommandGroup, opts QueryOptions) []config.CommandGroup {
	if opts.SearchQuery == "" && opts.FilterValue == "" {
		return groups
	}

	query := strings.ToLower(strings.TrimSpace(opts.SearchQuery))

	result := make([]config.CommandGroup, 0, len(groups))

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

		result = append(result, group)
	}

	return result
}

// GetDeviceGroups 获取所有设备分组名称（用于前端下拉选项）
func (s *QueryService) GetDeviceGroups() []string {
	if config.DB == nil {
		return []string{}
	}

	var groups []string
	config.DB.Model(&config.DeviceAsset{}).
		Distinct("`group`").
		Where("`group` != ''").
		Pluck("`group`", &groups)

	return groups
}

// GetDeviceTags 获取所有设备标签（用于前端下拉选项）
func (s *QueryService) GetDeviceTags() []string {
	if config.DB == nil {
		return []string{}
	}

	var tagStrings []string
	config.DB.Model(&config.DeviceAsset{}).
		Where("tags IS NOT NULL AND tags != ''").
		Pluck("tags", &tagStrings)

	// 解析并去重标签
	tagSet := make(map[string]bool)
	for _, ts := range tagStrings {
		// 假设标签以逗号分隔或存储为 JSON 数组字符串
		tags := strings.Split(ts, ",")
		for _, tag := range tags {
			tag = strings.TrimSpace(tag)
			if tag != "" {
				tagSet[tag] = true
			}
		}
	}

	tags := make([]string, 0, len(tagSet))
	for tag := range tagSet {
		tags = append(tags, tag)
	}

	return tags
}
