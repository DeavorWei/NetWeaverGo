package repository

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/NetWeaverGo/core/internal/config"
	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/models"
	"gorm.io/gorm"
)

// deviceRepository DeviceRepository 的实现
type deviceRepository struct {
	db *gorm.DB
}

// NewDeviceRepository 创建设备资产 Repository
func NewDeviceRepository() DeviceRepository {
	return &deviceRepository{
		db: config.GetDB(),
	}
}

// NewDeviceRepositoryWithDB 使用指定 DB 创建 Repository（用于测试）
func NewDeviceRepositoryWithDB(db *gorm.DB) DeviceRepository {
	return &deviceRepository{db: db}
}

// ============================================================================
// 查询操作
// ============================================================================

func (r *deviceRepository) FindAll() ([]models.DeviceAsset, error) {
	if r.db == nil {
		return nil, fmt.Errorf("数据库未初始化")
	}

	var devices []models.DeviceAsset
	if err := r.db.Order("ip ASC").Find(&devices).Error; err != nil {
		return nil, err
	}
	return devices, nil
}

func (r *deviceRepository) FindByID(id uint) (*models.DeviceAsset, error) {
	if r.db == nil {
		return nil, fmt.Errorf("数据库未初始化")
	}

	var device models.DeviceAsset
	if err := r.db.First(&device, id).Error; err != nil {
		return nil, err
	}
	return &device, nil
}

func (r *deviceRepository) FindByIPs(ips []string) ([]models.DeviceAsset, error) {
	if r.db == nil {
		return nil, fmt.Errorf("数据库未初始化")
	}

	var devices []models.DeviceAsset
	if err := r.db.Where("ip IN ?", ips).Find(&devices).Error; err != nil {
		return nil, err
	}
	return devices, nil
}

func (r *deviceRepository) FindByIP(ip string) (*models.DeviceAsset, error) {
	if r.db == nil {
		return nil, fmt.Errorf("数据库未初始化")
	}

	var device models.DeviceAsset
	if err := r.db.Where("ip = ?", ip).First(&device).Error; err != nil {
		return nil, err
	}
	return &device, nil
}

func (r *deviceRepository) Count() (int64, error) {
	if r.db == nil {
		return 0, fmt.Errorf("数据库未初始化")
	}

	var count int64
	if err := r.db.Model(&models.DeviceAsset{}).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (r *deviceRepository) ExistsByIP(ip string) (bool, error) {
	if r.db == nil {
		return false, fmt.Errorf("数据库未初始化")
	}

	var count int64
	if err := r.db.Model(&models.DeviceAsset{}).Where("ip = ?", ip).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// ============================================================================
// 分页查询
// ============================================================================

func (r *deviceRepository) Query(opts DeviceQueryOptions) (*DeviceQueryResult, error) {
	if r.db == nil {
		return nil, fmt.Errorf("数据库未初始化")
	}

	var devices []models.DeviceAsset
	var total int64
	filterField := normalizeDeviceFilterField(opts.FilterField)

	// 构建查询
	query := r.db.Model(&models.DeviceAsset{})

	// 应用搜索过滤
	if opts.SearchQuery != "" {
		searchPattern := "%" + strings.ToLower(opts.SearchQuery) + "%"
		switch filterField {
		case "group":
			query = query.Where("LOWER(group_name) LIKE ?", searchPattern)
		case "ip":
			query = query.Where("LOWER(ip) LIKE ?", searchPattern)
		case "tag":
			query = query.Where("LOWER(tags) LIKE ?", searchPattern)
		default:
			// 默认搜索所有字段
			query = query.Where(
				"LOWER(ip) LIKE ? OR LOWER(group_name) LIKE ? OR LOWER(username) LIKE ? OR LOWER(tags) LIKE ?",
				searchPattern, searchPattern, searchPattern, searchPattern,
			)
		}
	}

	// 应用状态/值过滤
	if opts.FilterValue != "" {
		switch filterField {
		case "protocol":
			query = query.Where("LOWER(protocol) = ?", strings.ToLower(opts.FilterValue))
		case "group":
			query = query.Where("LOWER(group_name) = ?", strings.ToLower(opts.FilterValue))
		}
	}

	// 应用 IP 过滤
	if len(opts.IPFilter) > 0 {
		query = query.Where("ip IN ?", opts.IPFilter)
	}

	// 应用 ID 过滤
	if len(opts.IDFilter) > 0 {
		query = query.Where("id IN ?", opts.IDFilter)
	}

	// 计算总数
	if err := query.Count(&total).Error; err != nil {
		logger.Error("Repository", "-", "统计设备总数失败: %v", err)
		return nil, fmt.Errorf("统计设备总数失败: %w", err)
	}

	// 应用排序
	orderClause := r.buildOrderClause(opts.SortBy, opts.SortOrder)
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
	if err := query.Offset(offset).Limit(pageSize).Find(&devices).Error; err != nil {
		logger.Error("Repository", "-", "分页查询设备失败: %v", err)
		return nil, fmt.Errorf("分页查询设备失败: %w", err)
	}

	// 转换为列表项，清除密码字段
	items := make([]models.DeviceAssetListItem, len(devices))
	for i, d := range devices {
		items[i] = d.ToListItem()
	}

	return &DeviceQueryResult{
		Data:       items,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}, nil
}

func (r *deviceRepository) buildOrderClause(sortBy, sortOrder string) string {
	if sortBy == "" {
		return "ip ASC"
	}

	order := "ASC"
	if strings.ToLower(sortOrder) == "desc" {
		order = "DESC"
	}

	// 映射前端字段到数据库字段
	fieldMap := map[string]string{
		"ip":          "ip",
		"port":        "port",
		"protocol":    "protocol",
		"group":       "group_name",
		"displayName": "display_name",
		"vendor":      "vendor",
		"role":        "role",
		"site":        "site",
		"createdAt":   "created_at",
		"updatedAt":   "updated_at",
	}

	dbField, ok := fieldMap[sortBy]
	if !ok {
		return "ip ASC"
	}

	return fmt.Sprintf("%s %s", dbField, order)
}

func emptyDeviceQueryResult(opts DeviceQueryOptions) *DeviceQueryResult {
	pageSize := opts.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}
	page := opts.Page
	if page <= 0 {
		page = 1
	}

	return &DeviceQueryResult{
		Data:       []models.DeviceAssetListItem{},
		Total:      0,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: 1,
	}
}

func normalizeDeviceFilterField(field string) string {
	switch strings.ToLower(field) {
	case "group":
		return "group"
	case "ip":
		return "ip"
	case "tag":
		return "tag"
	case "protocol":
		return "protocol"
	default:
		return ""
	}
}

// ============================================================================
// 写入操作
// ============================================================================

func (r *deviceRepository) Create(device *models.DeviceAsset) error {
	if r.db == nil {
		return fmt.Errorf("数据库未初始化")
	}

	device.ID = 0
	return r.db.Create(device).Error
}

func (r *deviceRepository) CreateBatch(devices []models.DeviceAsset) error {
	if r.db == nil {
		return fmt.Errorf("数据库未初始化")
	}
	if len(devices) == 0 {
		return nil
	}

	// IP 展开逻辑已移至 Service 层，此处直接批量创建
	return r.db.Create(&devices).Error
}

func (r *deviceRepository) Update(device *models.DeviceAsset) error {
	if r.db == nil {
		return fmt.Errorf("数据库未初始化")
	}

	return r.db.Save(device).Error
}

func (r *deviceRepository) UpdateBatch(devices []models.DeviceAsset) error {
	if r.db == nil {
		return fmt.Errorf("数据库未初始化")
	}
	if len(devices) == 0 {
		return nil
	}

	return r.db.Transaction(func(tx *gorm.DB) error {
		for i := range devices {
			if err := tx.Save(&devices[i]).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *deviceRepository) Delete(id uint) error {
	if r.db == nil {
		return fmt.Errorf("数据库未初始化")
	}
	if id == 0 {
		return fmt.Errorf("无效的设备 ID")
	}

	result := r.db.Delete(&models.DeviceAsset{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("未找到设备: %d", id)
	}
	return nil
}

func (r *deviceRepository) DeleteBatch(ids []uint) error {
	if r.db == nil {
		return fmt.Errorf("数据库未初始化")
	}
	if len(ids) == 0 {
		return nil
	}

	result := r.db.Where("id IN ?", ids).Delete(&models.DeviceAsset{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("未找到可删除的设备")
	}
	return nil
}

// ============================================================================
// 事务支持
// ============================================================================

func (r *deviceRepository) WithTx(tx *gorm.DB) DeviceRepository {
	return &deviceRepository{db: tx}
}

func (r *deviceRepository) BeginTx() *gorm.DB {
	return r.db.Begin()
}

// ============================================================================
// 聚合查询
// ============================================================================

// GetDistinctGroups 获取所有不重复的分组名称
func (r *deviceRepository) GetDistinctGroups() ([]string, error) {
	if r.db == nil {
		return nil, fmt.Errorf("数据库未初始化")
	}

	var groups []string
	err := r.db.Model(&models.DeviceAsset{}).
		Where("group_name != ''").
		Distinct("group_name").
		Pluck("group_name", &groups).Error
	if err != nil {
		return nil, err
	}

	sort.Strings(groups)
	return groups, nil
}

// GetDistinctTags 获取所有不重复的标签
func (r *deviceRepository) GetDistinctTags() ([]string, error) {
	if r.db == nil {
		return nil, fmt.Errorf("数据库未初始化")
	}

	var tagStrings []string
	err := r.db.Model(&models.DeviceAsset{}).
		Where("tags IS NOT NULL AND tags != ''").
		Pluck("tags", &tagStrings).Error
	if err != nil {
		return nil, err
	}

	// 解析并去重标签
	tagSet := make(map[string]bool)
	for _, ts := range tagStrings {
		tags := extractTagsFromJSON(ts)
		for _, tag := range tags {
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
	return tags, nil
}

// extractTagsFromJSON 从 JSON 字符串或逗号分隔字符串中提取标签
func extractTagsFromJSON(raw string) []string {
	var tags []string
	if err := json.Unmarshal([]byte(raw), &tags); err == nil {
		return tags
	}

	// 回退到逗号分隔解析
	fallback := strings.Split(raw, ",")
	tags = make([]string, 0, len(fallback))
	for _, tag := range fallback {
		tag = strings.TrimSpace(tag)
		if tag != "" {
			tags = append(tags, tag)
		}
	}
	return tags
}
