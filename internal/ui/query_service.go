package ui

import (
	"context"
	"sort"
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

// ListDevices 查询设备列表（支持搜索、过滤、分页）
func (s *QueryService) ListDevices(opts QueryOptions) *QueryResult {
	// 加载所有设备
	allDevices, _, _, _, err := config.ParseOrGenerate(false)
	if err != nil {
		logger.Error("Query", "-", "加载设备列表失败: %v", err)
		return &QueryResult{
			Data:       []config.DeviceAsset{},
			Total:      0,
			Page:       opts.Page,
			PageSize:   opts.PageSize,
			TotalPages: 0,
		}
	}

	// 应用搜索过滤
	filtered := s.filterDevices(allDevices, opts)

	// 应用排序
	sorted := s.sortDevices(filtered, opts.SortBy, opts.SortOrder)

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

	// 返回结果
	return &QueryResult{
		Data:       sorted[start:end],
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}
}

// filterDevices 过滤设备列表
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
	if sortBy == "" {
		sortBy = "ip"
	}

	// 创建副本避免修改原数组
	sorted := make([]config.DeviceAsset, len(devices))
	copy(sorted, devices)

	// 排序
	switch sortBy {
	case "ip":
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].IP < sorted[j].IP
		})
	case "group":
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].Group < sorted[j].Group
		})
	case "protocol":
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].Protocol < sorted[j].Protocol
		})
	}

	// 降序
	if sortOrder == "desc" {
		for i, j := 0, len(sorted)-1; i < j; i, j = i+1, j-1 {
			sorted[i], sorted[j] = sorted[j], sorted[i]
		}
	}

	return sorted
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
	if sortBy == "" {
		sortBy = "updatedAt"
	}

	sorted := make([]config.TaskGroup, len(groups))
	copy(sorted, groups)

	switch sortBy {
	case "name":
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].Name < sorted[j].Name
		})
	case "status":
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].Status < sorted[j].Status
		})
	case "updatedAt":
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].UpdatedAt > sorted[j].UpdatedAt // 默认最新在前
		})
	case "createdAt":
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].CreatedAt > sorted[j].CreatedAt
		})
	}

	if sortOrder == "asc" {
		for i, j := 0, len(sorted)-1; i < j; i, j = i+1, j-1 {
			sorted[i], sorted[j] = sorted[j], sorted[i]
		}
	}

	return sorted
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

	// 应用排序
	sorted := s.sortCommandGroups(filtered, opts.SortBy, opts.SortOrder)

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

// sortCommandGroups 排序命令组列表
func (s *QueryService) sortCommandGroups(groups []config.CommandGroup, sortBy, sortOrder string) []config.CommandGroup {
	if sortBy == "" {
		sortBy = "updatedAt"
	}

	sorted := make([]config.CommandGroup, len(groups))
	copy(sorted, groups)

	switch sortBy {
	case "name":
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].Name < sorted[j].Name
		})
	case "updatedAt":
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].UpdatedAt > sorted[j].UpdatedAt
		})
	}

	if sortOrder == "asc" {
		for i, j := 0, len(sorted)-1; i < j; i, j = i+1, j-1 {
			sorted[i], sorted[j] = sorted[j], sorted[i]
		}
	}

	return sorted
}

// GetDeviceGroups 获取所有设备分组名称（用于前端下拉选项）
func (s *QueryService) GetDeviceGroups() []string {
	devices, _, _, _, err := config.ParseOrGenerate(false)
	if err != nil {
		return []string{}
	}

	groupSet := make(map[string]bool)
	for _, dev := range devices {
		if dev.Group != "" {
			groupSet[dev.Group] = true
		}
	}

	groups := make([]string, 0, len(groupSet))
	for group := range groupSet {
		groups = append(groups, group)
	}
	sort.Strings(groups)

	return groups
}

// GetDeviceTags 获取所有设备标签（用于前端下拉选项）
func (s *QueryService) GetDeviceTags() []string {
	devices, _, _, _, err := config.ParseOrGenerate(false)
	if err != nil {
		return []string{}
	}

	tagSet := make(map[string]bool)
	for _, dev := range devices {
		for _, tag := range dev.Tags {
			if tag != "" {
				tagSet[tag] = true
			}
		}
	}

	tags := make([]string, 0, len(tagSet))
	for tag := range tagSet {
		tags = append(tags, tag)
	}
	sort.Strings(tags)

	return tags
}
