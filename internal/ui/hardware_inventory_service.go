package ui

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/NetWeaverGo/core/internal/ceas"
	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/models"
	"github.com/NetWeaverGo/core/internal/report"
	"github.com/NetWeaverGo/core/internal/taskexec"
	"gorm.io/gorm"
)

// CEASDeviceOverviewVO 硬件清单设备概览
type CEASDeviceOverviewVO struct {
	IP            string    `json:"ip"`
	GroupName     string    `json:"groupName"`
	Vendor        string    `json:"vendor"`
	Model         string    `json:"model"`
	ModelSeries   string    `json:"modelSeries"`
	PatchVersion  string    `json:"patchVersion"`
	ESN           string    `json:"esn"`
	NodeCount     int       `json:"nodeCount"`
	AlertCount    int       `json:"alertCount"`
	HasCEASData   bool      `json:"hasCeasData"`
	LastCollectAt time.Time `json:"lastCollectAt,omitempty"`
}

// HardwareInventoryService 硬件清单与批次预警UI服务
type HardwareInventoryService struct {
	db       *gorm.DB
	taskexec *taskexec.TaskExecutionService
}

// NewHardwareInventoryService 创建硬件清单管理服务
func NewHardwareInventoryService(db *gorm.DB, taskexec *taskexec.TaskExecutionService) *HardwareInventoryService {
	return &HardwareInventoryService{
		db:       db,
		taskexec: taskexec,
	}
}

// SetTaskExecutionService 注入统一任务执行服务
func (s *HardwareInventoryService) SetTaskExecutionService(svc *taskexec.TaskExecutionService) {
	s.taskexec = svc
}

// ListCEASDevices 列出所有设备及其硬件采集概况
func (s *HardwareInventoryService) ListCEASDevices() ([]CEASDeviceOverviewVO, error) {
	if s.db == nil {
		return []CEASDeviceOverviewVO{}, nil
	}

	var devices []models.DeviceAsset
	if err := s.db.Order("group_name asc, ip asc").Find(&devices).Error; err != nil {
		return nil, fmt.Errorf("查询设备资产失败: %w", err)
	}

	// 1. 获取所有启用的 BOM Watchlist
	var watchlist []models.BOMWatchlistItem
	_ = s.db.Where("enabled = ?", true).Find(&watchlist).Error
	watchItemMap := make(map[string]bool, len(watchlist))
	for _, w := range watchlist {
		watchItemMap[strings.ToUpper(strings.TrimSpace(w.Item))] = true
	}

	// 2. 统计已采集设备硬件节点与预警信息
	type nodeStat struct {
		DeviceIP      string    `gorm:"column:device_ip"`
		NodeCount     int       `gorm:"column:node_count"`
		LastCollectAt time.Time `gorm:"column:last_collect_at"`
	}
	var stats []nodeStat
	_ = s.db.Model(&models.TaskCEASNode{}).
		Select("device_ip, count(*) as node_count, max(created_at) as last_collect_at").
		Group("device_ip").
		Scan(&stats).Error

	statMap := make(map[string]nodeStat, len(stats))
	for _, st := range stats {
		statMap[st.DeviceIP] = st
	}

	// 3. 统计预警数（从 node 中匹配）
	var itemNodes []struct {
		DeviceIP string `gorm:"column:device_ip"`
		Item     string `gorm:"column:item"`
	}
	_ = s.db.Model(&models.TaskCEASNode{}).
		Select("device_ip, item").
		Where("item != ''").
		Scan(&itemNodes).Error

	alertCountMap := make(map[string]int)
	for _, in := range itemNodes {
		norm := strings.ToUpper(strings.TrimSpace(in.Item))
		if watchItemMap[norm] {
			alertCountMap[in.DeviceIP]++
		}
	}

	// 4. 组装概览列表
	result := make([]CEASDeviceOverviewVO, 0, len(devices))
	for _, d := range devices {
		st, hasData := statMap[d.IP]
		nodeCount := 0
		var lastAt time.Time
		if hasData {
			nodeCount = st.NodeCount
			lastAt = st.LastCollectAt
		}

		vo := CEASDeviceOverviewVO{
			IP:            d.IP,
			GroupName:     d.Group,
			Vendor:        d.Vendor,
			Model:         d.Model,
			ModelSeries:   d.ModelSeries,
			PatchVersion:  d.PatchVersion,
			ESN:           d.ESN,
			NodeCount:     nodeCount,
			AlertCount:    alertCountMap[d.IP],
			HasCEASData:   hasData && nodeCount > 0,
			LastCollectAt: lastAt,
		}
		result = append(result, vo)
	}

	return result, nil
}

// loadDeviceCEASNodes 加载指定设备最近一次采集的硬件节点（领域模型）与 ESN
func (s *HardwareInventoryService) loadDeviceCEASNodes(deviceIP string) ([]*ceas.Node, string, error) {
	if s.db == nil {
		return nil, "", fmt.Errorf("数据库未初始化")
	}

	var dbNodes []models.TaskCEASNode
	var latestRunNode models.TaskCEASNode
	if err := s.db.Where("device_ip = ?", deviceIP).Order("created_at desc").First(&latestRunNode).Error; err == nil && latestRunNode.TaskRunID != "" {
		if err := s.db.Where("device_ip = ? AND task_run_id = ?", deviceIP, latestRunNode.TaskRunID).Order("level asc, id asc").Find(&dbNodes).Error; err != nil {
			return nil, "", fmt.Errorf("查询设备硬件节点失败: %w", err)
		}
	} else {
		if err := s.db.Where("device_ip = ?", deviceIP).Order("level asc, id asc").Find(&dbNodes).Error; err != nil {
			return nil, "", fmt.Errorf("查询设备硬件节点失败: %w", err)
		}
	}

	var device models.DeviceAsset
	_ = s.db.Where("ip = ?", deviceIP).First(&device).Error

	nodes := make([]*ceas.Node, 0, len(dbNodes))
	for _, dn := range dbNodes {
		var attrs map[string]string
		if dn.AttrsJSON != "" {
			_ = json.Unmarshal([]byte(dn.AttrsJSON), &attrs)
		}
		nodes = append(nodes, &ceas.Node{
			ID:           dn.NodeID,
			ParentID:     dn.ParentID,
			Level:        dn.Level,
			Type:         dn.Type,
			Name:         dn.Name,
			Path:         dn.Path,
			Slot:         dn.Slot,
			Item:         dn.Item,
			BarCode:      dn.BarCode,
			Description:  dn.Description,
			Manufactured: dn.Manufactured,
			VendorName:   dn.VendorName,
			BoardType:    dn.BoardType,
			Attrs:        attrs,
			Children:     make([]*ceas.Node, 0),
		})
	}
	return nodes, device.ESN, nil
}

// emptyHardwareTreeVO 构造空硬件树视图
func emptyHardwareTreeVO(deviceIP, esn string) *ceas.HardwareTreeVO {
	return &ceas.HardwareTreeVO{
		DeviceIP:   deviceIP,
		ChassisESN: esn,
		TotalNodes: 0,
		Roots:      []*ceas.NodeVO{},
	}
}

// GetHardwareTree 获取指定设备的完整硬件树拓扑（带层级与属性）
func (s *HardwareInventoryService) GetHardwareTree(deviceIP string) (*ceas.HardwareTreeVO, error) {
	nodes, esn, err := s.loadDeviceCEASNodes(deviceIP)
	if err != nil {
		return nil, err
	}
	if len(nodes) == 0 {
		return emptyHardwareTreeVO(deviceIP, esn), nil
	}
	return ceas.ConvertTreeToVO(ceas.BuildHardwareTree(deviceIP, esn, nodes)), nil
}

// GetHardwareTreeFiltered 获取硬件树并按 Item / BarCode 白名单过滤（规划方案 §7.2 P3-1）。
// 白名单为空时等价于 GetHardwareTree。
func (s *HardwareInventoryService) GetHardwareTreeFiltered(deviceIP string, itemWhitelist, barcodeWhitelist []string) (*ceas.HardwareTreeVO, error) {
	if len(itemWhitelist) == 0 && len(barcodeWhitelist) == 0 {
		return s.GetHardwareTree(deviceIP)
	}
	nodes, esn, err := s.loadDeviceCEASNodes(deviceIP)
	if err != nil {
		return nil, err
	}
	if len(nodes) == 0 {
		return emptyHardwareTreeVO(deviceIP, esn), nil
	}
	filtered := ceas.FilterTreeByWhitelist(ceas.BuildHardwareTree(deviceIP, esn, nodes), itemWhitelist, barcodeWhitelist)
	return ceas.ConvertTreeToVO(filtered), nil
}

// GetHardwareNodeChildren 获取指定父节点下的直接子节点（用于按需懒加载）
func (s *HardwareInventoryService) GetHardwareNodeChildren(deviceIP string, parentID string) ([]*ceas.NodeVO, error) {
	if s.db == nil {
		return nil, fmt.Errorf("数据库未初始化")
	}

	var dbNodes []models.TaskCEASNode
	query := s.db.Where("device_ip = ?", deviceIP)
	var latestRunNode models.TaskCEASNode
	if err := s.db.Where("device_ip = ?", deviceIP).Order("created_at desc").First(&latestRunNode).Error; err == nil && latestRunNode.TaskRunID != "" {
		query = query.Where("task_run_id = ?", latestRunNode.TaskRunID)
	}
	if parentID == "" {
		query = query.Where("parent_id = '' OR level = 1")
	} else {
		query = query.Where("parent_id = ?", parentID)
	}

	if err := query.Order("level asc, id asc").Find(&dbNodes).Error; err != nil {
		return nil, fmt.Errorf("查询子节点失败: %w", err)
	}

	result := make([]*ceas.NodeVO, 0, len(dbNodes))
	for _, dn := range dbNodes {
		var attrs map[string]string
		if dn.AttrsJSON != "" {
			_ = json.Unmarshal([]byte(dn.AttrsJSON), &attrs)
		}
		result = append(result, &ceas.NodeVO{
			ID:           dn.NodeID,
			ParentID:     dn.ParentID,
			Level:        dn.Level,
			Type:         dn.Type,
			Name:         dn.Name,
			Path:         dn.Path,
			Slot:         dn.Slot,
			Item:         dn.Item,
			BarCode:      dn.BarCode,
			Description:  dn.Description,
			Manufactured: dn.Manufactured,
			VendorName:   dn.VendorName,
			BoardType:    dn.BoardType,
			Attrs:        attrs,
		})
	}
	return result, nil
}

// ListBOMWatchlist 列出所有 BOM 观察清单条目
func (s *HardwareInventoryService) ListBOMWatchlist() ([]models.BOMWatchlistItem, error) {
	if s.db == nil {
		return []models.BOMWatchlistItem{}, nil
	}
	var items []models.BOMWatchlistItem
	if err := s.db.Order("severity desc, item asc").Find(&items).Error; err != nil {
		return nil, fmt.Errorf("查询BOM观察清单失败: %w", err)
	}
	return items, nil
}

// SaveBOMWatchlistItem 创建或更新 BOM 观察条目
func (s *HardwareInventoryService) SaveBOMWatchlistItem(item models.BOMWatchlistItem) error {
	if s.db == nil {
		return fmt.Errorf("数据库未初始化")
	}
	item.Item = strings.ToUpper(strings.TrimSpace(item.Item))
	if item.Item == "" {
		return fmt.Errorf("BOM 编码不能为空")
	}
	if item.Severity == "" {
		item.Severity = "warning"
	}

	if item.ID > 0 {
		return s.db.Model(&models.BOMWatchlistItem{}).Where("id = ?", item.ID).Updates(map[string]interface{}{
			"item":        item.Item,
			"category":    item.Category,
			"model":       item.Model,
			"description": item.Description,
			"severity":    item.Severity,
			"enabled":     item.Enabled,
			"batch_no":    item.BatchNo,
			"updated_at":  time.Now(),
		}).Error
	}

	item.CreatedAt = time.Now()
	item.UpdatedAt = time.Now()
	return s.db.Create(&item).Error
}

// DeleteBOMWatchlistItem 删除 BOM 观察条目
func (s *HardwareInventoryService) DeleteBOMWatchlistItem(id uint) error {
	if s.db == nil {
		return fmt.Errorf("数据库未初始化")
	}
	return s.db.Delete(&models.BOMWatchlistItem{}, id).Error
}

// ToggleBOMWatchlistEnabled 启用/禁用 BOM 观察条目
func (s *HardwareInventoryService) ToggleBOMWatchlistEnabled(id uint, enabled bool) error {
	if s.db == nil {
		return fmt.Errorf("数据库未初始化")
	}
	return s.db.Model(&models.BOMWatchlistItem{}).Where("id = ?", id).Update("enabled", enabled).Error
}

// ListBOMAlerts 查询 BOM 批次预警命中明细
func (s *HardwareInventoryService) ListBOMAlerts(deviceIP string, severity string) ([]ceas.BOMAlertVO, error) {
	if s.db == nil {
		return []ceas.BOMAlertVO{}, nil
	}

	// 1. 获取生效的 Watchlist
	var watchlist []models.BOMWatchlistItem
	if err := s.db.Where("enabled = ?", true).Find(&watchlist).Error; err != nil {
		return nil, fmt.Errorf("查询观察清单失败: %w", err)
	}
	if len(watchlist) == 0 {
		return []ceas.BOMAlertVO{}, nil
	}

	watchMap := make(map[string]models.BOMWatchlistItem, len(watchlist))
	for _, w := range watchlist {
		watchMap[strings.ToUpper(strings.TrimSpace(w.Item))] = w
	}

	// 2. 查询硬件节点
	query := s.db.Model(&models.TaskCEASNode{}).Where("item != ''")
	if deviceIP != "" {
		query = query.Where("device_ip = ?", deviceIP)
	}

	var nodes []models.TaskCEASNode
	if err := query.Find(&nodes).Error; err != nil {
		return nil, fmt.Errorf("查询硬件节点失败: %w", err)
	}

	// 3. 匹配生成预警明细
	alerts := make([]ceas.BOMAlertVO, 0)
	for _, n := range nodes {
		norm := strings.ToUpper(strings.TrimSpace(n.Item))
		if w, ok := watchMap[norm]; ok {
			if severity != "" && w.Severity != severity {
				continue
			}
			alerts = append(alerts, ceas.BOMAlertVO{
				DeviceIP:    n.DeviceIP,
				Slot:        n.Slot,
				Path:        n.Path,
				NodeType:    n.Type,
				NodeName:    n.Name,
				Item:        n.Item,
				BarCode:     n.BarCode,
				Description: w.Description,
				Severity:    w.Severity,
				BatchNo:     w.BatchNo,
			})
		}
	}

	// 4. 按严重度排序 (critical > danger > warning)
	severityRank := map[string]int{"critical": 1, "danger": 2, "warning": 3}
	sort.Slice(alerts, func(i, j int) bool {
		rI := severityRank[alerts[i].Severity]
		if rI == 0 {
			rI = 99
		}
		rJ := severityRank[alerts[j].Severity]
		if rJ == 0 {
			rJ = 99
		}
		if rI != rJ {
			return rI < rJ
		}
		if alerts[i].DeviceIP != alerts[j].DeviceIP {
			return alerts[i].DeviceIP < alerts[j].DeviceIP
		}
		return alerts[i].Slot < alerts[j].Slot
	})

	return alerts, nil
}

// ExportBOMAlertsCSV 导出 BOM 批次预警明细为 CSV 文本
func (s *HardwareInventoryService) ExportBOMAlertsCSV(deviceIP string, severity string) (string, error) {
	alerts, err := s.ListBOMAlerts(deviceIP, severity)
	if err != nil {
		return "", err
	}

	items := make([]ceas.BOMAlertItem, 0, len(alerts))
	for _, a := range alerts {
		items = append(items, ceas.BOMAlertItem{
			DeviceIP:    a.DeviceIP,
			Slot:        a.Slot,
			Path:        a.Path,
			NodeType:    a.NodeType,
			NodeName:    a.NodeName,
			Item:        a.Item,
			BarCode:     a.BarCode,
			Description: a.Description,
			Severity:    a.Severity,
			BatchNo:     a.BatchNo,
		})
	}
	csvText, err := ceas.ExportBOMAlertsToCSV(items)
	if err != nil {
		return "", err
	}
	// 导出前脱敏自检：命中未脱敏敏感内容则阻断导出（规划方案 §5.2 P1-6）
	if err := report.ValidateExportContent(csvText); err != nil {
		return "", err
	}
	return csvText, nil
}

// ExportHardwareInventoryCSV 导出指定设备的硬件清单为 CSV 文本
func (s *HardwareInventoryService) ExportHardwareInventoryCSV(deviceIP string) (string, error) {
	if s.db == nil {
		return "", fmt.Errorf("数据库未初始化")
	}

	var nodes []models.TaskCEASNode
	query := s.db.Where("device_ip = ?", deviceIP)
	var latestRunNode models.TaskCEASNode
	if err := s.db.Where("device_ip = ?", deviceIP).Order("created_at desc").First(&latestRunNode).Error; err == nil && latestRunNode.TaskRunID != "" {
		query = query.Where("task_run_id = ?", latestRunNode.TaskRunID)
	}
	if err := query.Order("level asc, id asc").Find(&nodes).Error; err != nil {
		return "", fmt.Errorf("查询设备硬件节点失败: %w", err)
	}

	var buf bytes.Buffer
	// 写入 UTF-8 BOM，防止 Excel 打开乱码
	buf.WriteString("\xEF\xBB\xBF")

	writer := csv.NewWriter(&buf)
	headers := []string{"设备IP", "层级", "节点类型", "节点名称", "物理位置(Path)", "所属槽位", "BOM编码", "序列号/条形码", "型号/单板", "物料描述", "生产日期", "厂商", "采集时间"}
	if err := writer.Write(headers); err != nil {
		return "", err
	}

	for _, n := range nodes {
		row := []string{
			n.DeviceIP,
			fmt.Sprintf("%d", n.Level),
			n.Type,
			n.Name,
			n.Path,
			n.Slot,
			n.Item,
			n.BarCode,
			n.BoardType,
			n.Description,
			n.Manufactured,
			n.VendorName,
			n.CreatedAt.Format("2006-01-02 15:04:05"),
		}
		if err := writer.Write(row); err != nil {
			return "", err
		}
	}
	writer.Flush()
	csvText := buf.String()
	// 导出前脱敏自检：命中未脱敏敏感内容则阻断导出（规划方案 §5.2 P1-6）
	if err := report.ValidateExportContent(csvText); err != nil {
		return "", err
	}
	return csvText, nil
}

// TriggerCEASCollect 触发 CEAS 硬件清单采集任务
func (s *HardwareInventoryService) TriggerCEASCollect(deviceIPs []string) (string, error) {
	// 过滤与去重
	cleanIPs := make([]string, 0, len(deviceIPs))
	seen := make(map[string]bool)
	for _, ip := range deviceIPs {
		ip = strings.TrimSpace(ip)
		if ip != "" && !seen[ip] {
			seen[ip] = true
			cleanIPs = append(cleanIPs, ip)
		}
	}

	if len(cleanIPs) == 0 {
		return "", fmt.Errorf("至少需要选择一台设备进行硬件采集")
	}

	if s.taskexec == nil {
		return "", fmt.Errorf("任务执行服务未初始化")
	}

	taskName := fmt.Sprintf("CEAS硬件清单采集-%s", time.Now().Format("20060102-150405"))
	def, err := s.taskexec.CreateCEASTask(taskName, &ceas.CEASTaskConfig{
		DeviceIPs:   cleanIPs,
		Concurrency: 10,
		TimeoutSec:  60,
	})
	if err != nil {
		logger.Error("HardwareInventoryService", "-", "创建CEAS任务失败: %v", err)
		return "", fmt.Errorf("创建任务失败: %w", err)
	}

	runID, err := s.taskexec.StartTask(context.Background(), def)
	if err != nil {
		logger.Error("HardwareInventoryService", "-", "启动CEAS任务失败: %v", err)
		return "", fmt.Errorf("启动任务失败: %w", err)
	}

	logger.Info("HardwareInventoryService", runID, "已触发CEAS采集任务: devices=%d", len(cleanIPs))
	return runID, nil
}
