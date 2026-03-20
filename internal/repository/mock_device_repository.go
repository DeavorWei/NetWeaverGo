package repository

import (
	"fmt"
	"sort"
	"strings"

	"github.com/NetWeaverGo/core/internal/models"
	"gorm.io/gorm"
)

// MockDeviceRepository 用于单元测试的 Mock 实现
type MockDeviceRepository struct {
	Devices     map[uint]models.DeviceAsset
	NextID      uint
	QueryError  error
	CreateError error
	UpdateError error
	DeleteError error
}

// NewMockDeviceRepository 创建 Mock 设备 Repository
func NewMockDeviceRepository() *MockDeviceRepository {
	return &MockDeviceRepository{
		Devices: make(map[uint]models.DeviceAsset),
		NextID:  1,
	}
}

// ============================================================================
// 查询操作
// ============================================================================

func (m *MockDeviceRepository) FindAll() ([]models.DeviceAsset, error) {
	if m.QueryError != nil {
		return nil, m.QueryError
	}

	devices := make([]models.DeviceAsset, 0, len(m.Devices))
	for _, d := range m.Devices {
		devices = append(devices, d)
	}

	// 按 IP 排序
	sort.Slice(devices, func(i, j int) bool {
		return devices[i].IP < devices[j].IP
	})

	return devices, nil
}

func (m *MockDeviceRepository) FindByID(id uint) (*models.DeviceAsset, error) {
	if m.QueryError != nil {
		return nil, m.QueryError
	}

	device, ok := m.Devices[id]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return &device, nil
}

func (m *MockDeviceRepository) FindByIPs(ips []string) ([]models.DeviceAsset, error) {
	if m.QueryError != nil {
		return nil, m.QueryError
	}

	ipSet := make(map[string]bool)
	for _, ip := range ips {
		ipSet[ip] = true
	}

	devices := make([]models.DeviceAsset, 0)
	for _, d := range m.Devices {
		if ipSet[d.IP] {
			devices = append(devices, d)
		}
	}

	return devices, nil
}

func (m *MockDeviceRepository) FindByIP(ip string) (*models.DeviceAsset, error) {
	if m.QueryError != nil {
		return nil, m.QueryError
	}

	for _, d := range m.Devices {
		if d.IP == ip {
			return &d, nil
		}
	}

	return nil, gorm.ErrRecordNotFound
}

func (m *MockDeviceRepository) Count() (int64, error) {
	if m.QueryError != nil {
		return 0, m.QueryError
	}

	return int64(len(m.Devices)), nil
}

func (m *MockDeviceRepository) ExistsByIP(ip string) (bool, error) {
	if m.QueryError != nil {
		return false, m.QueryError
	}

	for _, d := range m.Devices {
		if d.IP == ip {
			return true, nil
		}
	}

	return false, nil
}

// ============================================================================
// 分页查询
// ============================================================================

func (m *MockDeviceRepository) Query(opts DeviceQueryOptions) (*DeviceQueryResult, error) {
	if m.QueryError != nil {
		return nil, m.QueryError
	}

	// 获取所有设备
	allDevices, _ := m.FindAll()

	// 应用搜索过滤
	var filtered []models.DeviceAsset
	for _, d := range allDevices {
		if m.matchSearch(d, opts.SearchQuery, opts.FilterField) {
			filtered = append(filtered, d)
		}
	}

	// 应用值过滤
	if opts.FilterValue != "" {
		var valueFiltered []models.DeviceAsset
		for _, d := range filtered {
			if m.matchFilterValue(d, opts.FilterField, opts.FilterValue) {
				valueFiltered = append(valueFiltered, d)
			}
		}
		filtered = valueFiltered
	}

	// 应用 IP 过滤
	if len(opts.IPFilter) > 0 {
		ipSet := make(map[string]bool)
		for _, ip := range opts.IPFilter {
			ipSet[ip] = true
		}
		var ipFiltered []models.DeviceAsset
		for _, d := range filtered {
			if ipSet[d.IP] {
				ipFiltered = append(ipFiltered, d)
			}
		}
		filtered = ipFiltered
	}

	// 应用 ID 过滤
	if len(opts.IDFilter) > 0 {
		idSet := make(map[uint]bool)
		for _, id := range opts.IDFilter {
			idSet[id] = true
		}
		var idFiltered []models.DeviceAsset
		for _, d := range filtered {
			if idSet[d.ID] {
				idFiltered = append(idFiltered, d)
			}
		}
		filtered = idFiltered
	}

	total := int64(len(filtered))

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

	// 分页
	end := offset + pageSize
	if end > int(total) {
		end = int(total)
	}

	paged := filtered[offset:end]

	// 转换为列表项
	items := make([]models.DeviceAssetListItem, len(paged))
	for i, d := range paged {
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

func (m *MockDeviceRepository) matchSearch(d models.DeviceAsset, query, field string) bool {
	if query == "" {
		return true
	}

	query = strings.ToLower(query)
	switch strings.ToLower(field) {
	case "group":
		return strings.Contains(strings.ToLower(d.Group), query)
	case "ip":
		return strings.Contains(strings.ToLower(d.IP), query)
	case "tag":
		for _, tag := range d.Tags {
			if strings.Contains(strings.ToLower(tag), query) {
				return true
			}
		}
		return false
	default:
		return strings.Contains(strings.ToLower(d.IP), query) ||
			strings.Contains(strings.ToLower(d.Group), query) ||
			strings.Contains(strings.ToLower(d.Username), query)
	}
}

func (m *MockDeviceRepository) matchFilterValue(d models.DeviceAsset, field, value string) bool {
	value = strings.ToLower(value)
	switch strings.ToLower(field) {
	case "protocol":
		return strings.ToLower(d.Protocol) == value
	case "group":
		return strings.ToLower(d.Group) == value
	default:
		return true
	}
}

// ============================================================================
// 写入操作
// ============================================================================

func (m *MockDeviceRepository) Create(device *models.DeviceAsset) error {
	if m.CreateError != nil {
		return m.CreateError
	}

	device.ID = m.NextID
	m.NextID++
	m.Devices[device.ID] = *device
	return nil
}

func (m *MockDeviceRepository) CreateBatch(devices []models.DeviceAsset) error {
	if m.CreateError != nil {
		return m.CreateError
	}

	for _, device := range devices {
		d := device
		d.ID = m.NextID
		m.NextID++
		m.Devices[d.ID] = d
	}
	return nil
}

func (m *MockDeviceRepository) Update(device *models.DeviceAsset) error {
	if m.UpdateError != nil {
		return m.UpdateError
	}

	if _, ok := m.Devices[device.ID]; !ok {
		return gorm.ErrRecordNotFound
	}

	m.Devices[device.ID] = *device
	return nil
}

func (m *MockDeviceRepository) UpdateBatch(devices []models.DeviceAsset) error {
	if m.UpdateError != nil {
		return m.UpdateError
	}

	for _, device := range devices {
		if _, ok := m.Devices[device.ID]; !ok {
			return fmt.Errorf("设备不存在: %d", device.ID)
		}
		m.Devices[device.ID] = device
	}
	return nil
}

func (m *MockDeviceRepository) Delete(id uint) error {
	if m.DeleteError != nil {
		return m.DeleteError
	}

	if _, ok := m.Devices[id]; !ok {
		return fmt.Errorf("未找到设备: %d", id)
	}

	delete(m.Devices, id)
	return nil
}

func (m *MockDeviceRepository) DeleteBatch(ids []uint) error {
	if m.DeleteError != nil {
		return m.DeleteError
	}
	if len(ids) == 0 {
		return nil
	}

	deleted := 0
	for _, id := range ids {
		if _, ok := m.Devices[id]; ok {
			delete(m.Devices, id)
			deleted++
		}
	}
	if deleted == 0 {
		return fmt.Errorf("未找到可删除的设备")
	}
	return nil
}

// ============================================================================
// 事务支持
// ============================================================================

func (m *MockDeviceRepository) WithTx(tx *gorm.DB) DeviceRepository {
	// Mock 不支持事务，返回自身
	return m
}

func (m *MockDeviceRepository) BeginTx() *gorm.DB {
	// Mock 不支持事务
	return nil
}

// ============================================================================
// 聚合查询
// ============================================================================

// GetDistinctGroups 获取所有不重复的分组名称
func (m *MockDeviceRepository) GetDistinctGroups() ([]string, error) {
	if m.QueryError != nil {
		return nil, m.QueryError
	}

	groupSet := make(map[string]bool)
	for _, d := range m.Devices {
		if d.Group != "" {
			groupSet[d.Group] = true
		}
	}

	groups := make([]string, 0, len(groupSet))
	for group := range groupSet {
		groups = append(groups, group)
	}
	sort.Strings(groups)
	return groups, nil
}

// GetDistinctTags 获取所有不重复的标签
func (m *MockDeviceRepository) GetDistinctTags() ([]string, error) {
	if m.QueryError != nil {
		return nil, m.QueryError
	}

	tagSet := make(map[string]bool)
	for _, d := range m.Devices {
		for _, tag := range d.Tags {
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

// ============================================================================
// 测试辅助方法
// ============================================================================

// AddDevice 添加测试设备
func (m *MockDeviceRepository) AddDevice(device models.DeviceAsset) {
	if device.ID == 0 {
		device.ID = m.NextID
		m.NextID++
	}
	m.Devices[device.ID] = device
}

// Reset 重置 Mock 状态
func (m *MockDeviceRepository) Reset() {
	m.Devices = make(map[uint]models.DeviceAsset)
	m.NextID = 1
	m.QueryError = nil
	m.CreateError = nil
	m.UpdateError = nil
	m.DeleteError = nil
}

// SetQueryError 设置查询错误
func (m *MockDeviceRepository) SetQueryError(err error) {
	m.QueryError = err
}

// SetCreateError 设置创建错误
func (m *MockDeviceRepository) SetCreateError(err error) {
	m.CreateError = err
}

// SetUpdateError 设置更新错误
func (m *MockDeviceRepository) SetUpdateError(err error) {
	m.UpdateError = err
}

// SetDeleteError 设置删除错误
func (m *MockDeviceRepository) SetDeleteError(err error) {
	m.DeleteError = err
}
