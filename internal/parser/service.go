package parser

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/NetWeaverGo/core/internal/models"
	"github.com/NetWeaverGo/core/internal/normalize"
	"gorm.io/gorm"
)

// Service 解析服务
type Service struct {
	db     *gorm.DB
	parser *TextFSMParser
}

// NewService 创建解析服务
func NewService(db *gorm.DB) *Service {
	return &Service{
		db:     db,
		parser: NewTextFSMParser(),
	}
}

// LoadVendorTemplates 加载厂商模板
func (s *Service) LoadVendorTemplates(vendor string) error {
	return s.parser.LoadBuiltinTemplates(vendor)
}

// ParseTaskDevice 解析任务中单个设备的原始输出
func (s *Service) ParseTaskDevice(taskID, deviceIP, vendor string) (*ParsedResult, error) {
	if err := s.LoadVendorTemplates(vendor); err != nil {
		return nil, fmt.Errorf("加载模板失败: %v", err)
	}

	result := &ParsedResult{
		TaskID:   taskID,
		DeviceIP: deviceIP,
		ParsedAt: time.Now(),
	}

	var outputs []models.RawCommandOutput
	if err := s.db.
		Where("task_id = ? AND device_ip = ? AND status = ?", taskID, deviceIP, "success").
		Order("created_at ASC").
		Find(&outputs).Error; err != nil {
		return nil, fmt.Errorf("获取命令输出失败: %v", err)
	}

	mapper := GetMapper(vendor)
	identity := &DeviceIdentity{Vendor: vendor, MgmtIP: deviceIP}

	for _, output := range outputs {
		// 使用 GetParseFilePath() 获取规范化输出路径
		parseFilePath := output.GetParseFilePath()
		if parseFilePath == "" {
			s.updateRawParseStatus(output.ID, "skipped", "规范化输出文件路径为空")
			continue
		}

		rawText, err := os.ReadFile(parseFilePath)
		if err != nil {
			msg := fmt.Sprintf("读取 %s 失败: %v", output.CommandKey, err)
			result.ParseErrors = append(result.ParseErrors, msg)
			s.updateRawParseStatus(output.ID, "parse_failed", msg)
			continue
		}

		// 规范化输出已经由 terminal.Replayer 处理过，不再需要 ANSI 清理
		rows, err := s.parser.Parse(output.CommandKey, string(rawText))
		if err != nil {
			msg := fmt.Sprintf("解析 %s 失败: %v", output.CommandKey, err)
			result.ParseErrors = append(result.ParseErrors, msg)
			s.updateRawParseStatus(output.ID, "parse_failed", msg)
			continue
		}

		parseStatus := "success"
		parseErrorMsg := ""
		markMappingError := func(msg string) {
			result.ParseErrors = append(result.ParseErrors, msg)
			parseStatus = "parse_failed"
			if parseErrorMsg == "" {
				parseErrorMsg = msg
			}
		}

		switch output.CommandKey {
		case "version":
			id, mapErr := mapper.ToDeviceInfo(rows)
			if mapErr != nil {
				markMappingError(fmt.Sprintf("映射version失败: %v", mapErr))
			} else {
				mergeIdentity(identity, id)
				identity.RawRefID = fmt.Sprintf("%d", output.ID)
			}
		case "sysname":
			applyFieldRows(identity, rows, "hostname", "")
			identity.RawRefID = fmt.Sprintf("%d", output.ID)
		case "esn":
			applyFieldRows(identity, rows, "serial_number", "")
			identity.RawRefID = fmt.Sprintf("%d", output.ID)
		case "device_info":
			applyFieldRows(identity, rows, "model", "")
		case "interface_brief", "interface_detail":
			interfaces, mapErr := mapper.ToInterfaces(rows)
			if mapErr != nil {
				markMappingError(fmt.Sprintf("映射interface失败: %v", mapErr))
			} else {
				result.Interfaces = mergeInterfaceFacts(result.Interfaces, interfaces)
			}
		case "lldp_neighbor":
			neighbors, mapErr := mapper.ToLLDP(rows)
			if mapErr != nil {
				markMappingError(fmt.Sprintf("映射lldp失败: %v", mapErr))
			} else {
				rawRefID := fmt.Sprintf("%d", output.ID)
				for i := range neighbors {
					neighbors[i].CommandKey = output.CommandKey
					neighbors[i].RawRefID = rawRefID
				}
				result.LLDPNeighbors = append(result.LLDPNeighbors, neighbors...)
			}
		case "mac_address":
			fdbEntries, mapErr := mapper.ToFDB(rows)
			if mapErr != nil {
				markMappingError(fmt.Sprintf("映射mac_address失败: %v", mapErr))
			} else {
				rawRefID := fmt.Sprintf("%d", output.ID)
				for i := range fdbEntries {
					fdbEntries[i].CommandKey = output.CommandKey
					fdbEntries[i].RawRefID = rawRefID
				}
				result.FDBEntries = append(result.FDBEntries, fdbEntries...)
			}
		case "arp_all":
			arpEntries, mapErr := mapper.ToARP(rows)
			if mapErr != nil {
				markMappingError(fmt.Sprintf("映射arp失败: %v", mapErr))
			} else {
				rawRefID := fmt.Sprintf("%d", output.ID)
				for i := range arpEntries {
					arpEntries[i].CommandKey = output.CommandKey
					arpEntries[i].RawRefID = rawRefID
				}
				result.ARPEntries = append(result.ARPEntries, arpEntries...)
			}
		case "eth_trunk", "eth_trunk_verbose":
			aggregates, mapErr := mapper.ToAggregate(rows)
			if mapErr != nil {
				markMappingError(fmt.Sprintf("映射eth_trunk失败: %v", mapErr))
			} else {
				rawRefID := fmt.Sprintf("%d", output.ID)
				for i := range aggregates {
					aggregates[i].CommandKey = output.CommandKey
					aggregates[i].RawRefID = rawRefID
				}
				result.Aggregates = append(result.Aggregates, aggregates...)
			}
		}

		s.updateRawParseStatus(output.ID, parseStatus, parseErrorMsg)
	}

	identity.Vendor = normalize.NormalizeVendor(identity.Vendor)
	identity.Hostname = normalize.NormalizeDeviceName(identity.Hostname)
	result.Identity = identity
	return result, nil
}

func (s *Service) updateRawParseStatus(id uint, status, msg string) {
	if id == 0 {
		return
	}
	_ = s.db.Model(&models.RawCommandOutput{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"parse_status": status,
			"parse_error":  msg,
		}).Error
}

// ParseErrorDetail 解析错误详情
type ParseErrorDetail struct {
	DeviceIP   string `json:"deviceIp"`
	CommandKey string `json:"commandKey"`
	Error      string `json:"error"`
	ParseTime  string `json:"parseTime"`
}

// GetParseErrorsByTask 获取任务的所有解析错误
func (s *Service) GetParseErrorsByTask(taskID string) ([]ParseErrorDetail, error) {
	var outputs []models.RawCommandOutput
	if err := s.db.
		Where("task_id = ? AND parse_status = ?", taskID, "parse_failed").
		Find(&outputs).Error; err != nil {
		return nil, err
	}

	var errors []ParseErrorDetail
	for _, output := range outputs {
		errors = append(errors, ParseErrorDetail{
			DeviceIP:   output.DeviceIP,
			CommandKey: output.CommandKey,
			Error:      output.ParseError,
			ParseTime:  output.UpdatedAt.Format("2006-01-02 15:04:05"),
		})
	}
	return errors, nil
}

func applyFieldRows(identity *DeviceIdentity, rows []map[string]string, key, fallback string) {
	for _, row := range rows {
		if v := strings.TrimSpace(row[key]); v != "" {
			switch key {
			case "hostname":
				identity.Hostname = v
			case "serial_number":
				identity.SerialNumber = v
			case "model":
				identity.Model = v
			case "version":
				identity.Version = v
			case "mgmt_ip":
				identity.MgmtIP = v
			case "chassis_id":
				identity.ChassisID = v
			}
			return
		}
	}
	if fallback != "" {
		switch key {
		case "hostname":
			identity.Hostname = fallback
		case "serial_number":
			identity.SerialNumber = fallback
		case "model":
			identity.Model = fallback
		case "version":
			identity.Version = fallback
		case "mgmt_ip":
			identity.MgmtIP = fallback
		case "chassis_id":
			identity.ChassisID = fallback
		}
	}
}

func mergeIdentity(base, update *DeviceIdentity) {
	if update == nil {
		return
	}
	if update.Vendor != "" {
		base.Vendor = update.Vendor
	}
	if update.Model != "" {
		base.Model = update.Model
	}
	if update.SerialNumber != "" {
		base.SerialNumber = update.SerialNumber
	}
	if update.Version != "" {
		base.Version = update.Version
	}
	if update.Hostname != "" {
		base.Hostname = update.Hostname
	}
	if update.MgmtIP != "" {
		base.MgmtIP = update.MgmtIP
	}
	if update.ChassisID != "" {
		base.ChassisID = update.ChassisID
	}
}

func mergeInterfaceFacts(existing []InterfaceFact, incoming []InterfaceFact) []InterfaceFact {
	idx := make(map[string]int, len(existing))
	for i := range existing {
		idx[existing[i].Name] = i
	}

	for _, iface := range incoming {
		if iface.Name == "" {
			continue
		}
		if i, ok := idx[iface.Name]; ok {
			// 以更丰富字段覆盖空值，保持幂等。
			if existing[i].Description == "" {
				existing[i].Description = iface.Description
			}
			if existing[i].Status == "" {
				existing[i].Status = iface.Status
			}
			if existing[i].Protocol == "" {
				existing[i].Protocol = iface.Protocol
			}
			if existing[i].Speed == "" {
				existing[i].Speed = iface.Speed
			}
			if existing[i].Duplex == "" {
				existing[i].Duplex = iface.Duplex
			}
			if existing[i].MACAddress == "" {
				existing[i].MACAddress = iface.MACAddress
			}
			if existing[i].IPAddress == "" {
				existing[i].IPAddress = iface.IPAddress
			}
			continue
		}
		idx[iface.Name] = len(existing)
		existing = append(existing, iface)
	}
	return existing
}

// ParseAndSaveTaskDevice 解析并保存结果到数据库
func (s *Service) ParseAndSaveTaskDevice(taskID, deviceIP, vendor string) error {
	result, err := s.ParseTaskDevice(taskID, deviceIP, vendor)
	if err != nil {
		return err
	}
	return s.SaveParsedResult(result)
}

// SaveParsedResult 保存解析结果（幂等）
func (s *Service) SaveParsedResult(result *ParsedResult) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		// 幂等：先清理旧解析事实，再写入新结果。
		if err := tx.Where("task_id = ? AND device_ip = ?", result.TaskID, result.DeviceIP).Delete(&models.TopologyInterface{}).Error; err != nil {
			return fmt.Errorf("清理旧接口失败: %v", err)
		}
		if err := tx.Where("task_id = ? AND device_ip = ?", result.TaskID, result.DeviceIP).Delete(&models.TopologyLLDPNeighbor{}).Error; err != nil {
			return fmt.Errorf("清理旧LLDP失败: %v", err)
		}
		if err := tx.Where("task_id = ? AND device_ip = ?", result.TaskID, result.DeviceIP).Delete(&models.TopologyFDBEntry{}).Error; err != nil {
			return fmt.Errorf("清理旧FDB失败: %v", err)
		}
		if err := tx.Where("task_id = ? AND device_ip = ?", result.TaskID, result.DeviceIP).Delete(&models.TopologyARPEntry{}).Error; err != nil {
			return fmt.Errorf("清理旧ARP失败: %v", err)
		}
		if err := tx.Where("task_id = ? AND device_ip = ?", result.TaskID, result.DeviceIP).Delete(&models.TopologyAggregateMember{}).Error; err != nil {
			return fmt.Errorf("清理旧聚合成员失败: %v", err)
		}
		if err := tx.Where("task_id = ? AND device_ip = ?", result.TaskID, result.DeviceIP).Delete(&models.TopologyAggregateGroup{}).Error; err != nil {
			return fmt.Errorf("清理旧聚合组失败: %v", err)
		}

		if result.Identity != nil {
			updates := map[string]interface{}{
				"vendor":          result.Identity.Vendor,
				"model":           result.Identity.Model,
				"serial_number":   result.Identity.SerialNumber,
				"version":         result.Identity.Version,
				"hostname":        result.Identity.Hostname,
				"normalized_name": normalize.NormalizeDeviceName(result.Identity.Hostname),
				"mgmt_ip":         result.Identity.MgmtIP,
				"chassis_id":      result.Identity.ChassisID,
			}
			if err := tx.Model(&models.DiscoveryDevice{}).
				Where("task_id = ? AND device_ip = ?", result.TaskID, result.DeviceIP).
				Updates(updates).Error; err != nil {
				return fmt.Errorf("更新设备信息失败: %v", err)
			}
		}

		for _, iface := range result.Interfaces {
			topoIf := models.TopologyInterface{
				TaskID:        result.TaskID,
				DeviceIP:      result.DeviceIP,
				InterfaceName: iface.Name,
				Status:        iface.Status,
				Speed:         iface.Speed,
				Duplex:        iface.Duplex,
				Description:   iface.Description,
				MACAddress:    iface.MACAddress,
				IPAddress:     iface.IPAddress,
				IsAggregate:   iface.IsAggregate,
				AggregateID:   iface.AggregateID,
			}
			if err := tx.Create(&topoIf).Error; err != nil {
				return fmt.Errorf("保存接口信息失败: %v", err)
			}
		}

		for _, neighbor := range result.LLDPNeighbors {
			lldp := models.TopologyLLDPNeighbor{
				TaskID:          result.TaskID,
				DeviceIP:        result.DeviceIP,
				LocalInterface:  neighbor.LocalInterface,
				NeighborName:    neighbor.NeighborName,
				NeighborChassis: neighbor.NeighborChassis,
				NeighborPort:    neighbor.NeighborPort,
				NeighborIP:      neighbor.NeighborIP,
				NeighborDesc:    neighbor.NeighborDesc,
				CommandKey:      neighbor.CommandKey,
				RawRefID:        neighbor.RawRefID,
			}
			if err := tx.Create(&lldp).Error; err != nil {
				return fmt.Errorf("保存LLDP邻居失败: %v", err)
			}
		}

		for _, entry := range result.FDBEntries {
			fdb := models.TopologyFDBEntry{
				TaskID:     result.TaskID,
				DeviceIP:   result.DeviceIP,
				MACAddress: entry.MACAddress,
				VLAN:       entry.VLAN,
				Interface:  entry.Interface,
				Type:       entry.Type,
				CommandKey: entry.CommandKey,
				RawRefID:   entry.RawRefID,
			}
			if err := tx.Create(&fdb).Error; err != nil {
				return fmt.Errorf("保存FDB表项失败: %v", err)
			}
		}

		for _, entry := range result.ARPEntries {
			arp := models.TopologyARPEntry{
				TaskID:     result.TaskID,
				DeviceIP:   result.DeviceIP,
				IPAddress:  entry.IPAddress,
				MACAddress: entry.MACAddress,
				Interface:  entry.Interface,
				Type:       entry.Type,
				CommandKey: entry.CommandKey,
				RawRefID:   entry.RawRefID,
			}
			if err := tx.Create(&arp).Error; err != nil {
				return fmt.Errorf("保存ARP表项失败: %v", err)
			}
		}

		for _, agg := range result.Aggregates {
			group := models.TopologyAggregateGroup{
				TaskID:        result.TaskID,
				DeviceIP:      result.DeviceIP,
				AggregateName: agg.AggregateName,
				Mode:          agg.Mode,
				CommandKey:    agg.CommandKey,
				RawRefID:      agg.RawRefID,
			}
			if err := tx.Create(&group).Error; err != nil {
				return fmt.Errorf("保存聚合组失败: %v", err)
			}

			for _, member := range agg.MemberPorts {
				memberModel := models.TopologyAggregateMember{
					TaskID:        result.TaskID,
					DeviceIP:      result.DeviceIP,
					AggregateName: agg.AggregateName,
					MemberPort:    member,
					CommandKey:    agg.CommandKey,
					RawRefID:      agg.RawRefID,
				}
				if err := tx.Create(&memberModel).Error; err != nil {
					return fmt.Errorf("保存聚合成员失败: %v", err)
				}
			}
		}
		return nil
	})
}

// ParseTask 解析整个任务的所有设备
func (s *Service) ParseTask(taskID string) error {
	var task models.DiscoveryTask
	if err := s.db.First(&task, "id = ?", taskID).Error; err != nil {
		return fmt.Errorf("获取任务信息失败: %v", err)
	}

	var devices []models.DiscoveryDevice
	if err := s.db.Where("task_id = ? AND status IN ?", taskID, []string{"success", "partial"}).Find(&devices).Error; err != nil {
		return fmt.Errorf("获取设备列表失败: %v", err)
	}

	parseErrors := make([]string, 0)
	for _, device := range devices {
		vendor := device.Vendor
		if vendor == "" {
			vendor = task.Vendor
		}
		vendor = normalize.NormalizeVendor(vendor)
		if vendor == "" || vendor == "unknown" {
			vendor = "huawei"
		}
		if err := s.ParseAndSaveTaskDevice(taskID, device.DeviceIP, vendor); err != nil {
			parseErrors = append(parseErrors, fmt.Sprintf("%s: %v", device.DeviceIP, err))
			continue
		}
	}
	if len(parseErrors) > 0 {
		return fmt.Errorf("任务 %s 解析存在失败设备(%d): %s", taskID, len(parseErrors), strings.Join(parseErrors, "; "))
	}
	return nil
}

// GetParsedDeviceDetail 获取已解析设备的详细信息
func (s *Service) GetParsedDeviceDetail(taskID, deviceIP string) (*ParsedResult, error) {
	result := &ParsedResult{
		TaskID:   taskID,
		DeviceIP: deviceIP,
	}

	var device models.DiscoveryDevice
	if err := s.db.Where("task_id = ? AND device_ip = ?", taskID, deviceIP).First(&device).Error; err == nil {
		result.Identity = &DeviceIdentity{
			Vendor:       device.Vendor,
			Model:        device.Model,
			SerialNumber: device.SerialNumber,
			Version:      device.Version,
			Hostname:     device.Hostname,
			MgmtIP:       device.MgmtIP,
			ChassisID:    device.ChassisID,
		}
	}

	var interfaces []models.TopologyInterface
	if err := s.db.Where("task_id = ? AND device_ip = ?", taskID, deviceIP).Find(&interfaces).Error; err == nil {
		for _, iface := range interfaces {
			result.Interfaces = append(result.Interfaces, InterfaceFact{
				Name:        iface.InterfaceName,
				Status:      iface.Status,
				Speed:       iface.Speed,
				Duplex:      iface.Duplex,
				Description: iface.Description,
				MACAddress:  iface.MACAddress,
				IPAddress:   iface.IPAddress,
				IsAggregate: iface.IsAggregate,
				AggregateID: iface.AggregateID,
			})
		}
	}

	var lldpNeighbors []models.TopologyLLDPNeighbor
	if err := s.db.Where("task_id = ? AND device_ip = ?", taskID, deviceIP).Find(&lldpNeighbors).Error; err == nil {
		for _, neighbor := range lldpNeighbors {
			result.LLDPNeighbors = append(result.LLDPNeighbors, LLDPFact{
				LocalInterface:  neighbor.LocalInterface,
				NeighborName:    neighbor.NeighborName,
				NeighborChassis: neighbor.NeighborChassis,
				NeighborPort:    neighbor.NeighborPort,
				NeighborIP:      neighbor.NeighborIP,
				NeighborDesc:    neighbor.NeighborDesc,
				CommandKey:      neighbor.CommandKey,
				RawRefID:        neighbor.RawRefID,
			})
		}
	}

	var fdbEntries []models.TopologyFDBEntry
	if err := s.db.Where("task_id = ? AND device_ip = ?", taskID, deviceIP).Find(&fdbEntries).Error; err == nil {
		for _, entry := range fdbEntries {
			result.FDBEntries = append(result.FDBEntries, FDBFact{
				MACAddress: entry.MACAddress,
				VLAN:       entry.VLAN,
				Interface:  entry.Interface,
				Type:       entry.Type,
				CommandKey: entry.CommandKey,
				RawRefID:   entry.RawRefID,
			})
		}
	}

	var arpEntries []models.TopologyARPEntry
	if err := s.db.Where("task_id = ? AND device_ip = ?", taskID, deviceIP).Find(&arpEntries).Error; err == nil {
		for _, entry := range arpEntries {
			result.ARPEntries = append(result.ARPEntries, ARPFact{
				IPAddress:  entry.IPAddress,
				MACAddress: entry.MACAddress,
				Interface:  entry.Interface,
				Type:       entry.Type,
				CommandKey: entry.CommandKey,
				RawRefID:   entry.RawRefID,
			})
		}
	}

	var aggregates []models.TopologyAggregateGroup
	if err := s.db.Where("task_id = ? AND device_ip = ?", taskID, deviceIP).Find(&aggregates).Error; err == nil {
		for _, agg := range aggregates {
			var members []models.TopologyAggregateMember
			s.db.Where("task_id = ? AND device_ip = ? AND aggregate_name = ?", taskID, deviceIP, agg.AggregateName).Find(&members)

			memberPorts := make([]string, 0, len(members))
			for _, m := range members {
				memberPorts = append(memberPorts, m.MemberPort)
			}

			result.Aggregates = append(result.Aggregates, AggregateFact{
				AggregateName: agg.AggregateName,
				Mode:          agg.Mode,
				MemberPorts:   memberPorts,
				CommandKey:    agg.CommandKey,
				RawRefID:      agg.RawRefID,
			})
		}
	}

	return result, nil
}

// ParseTaskWithErrors 解析任务并返回详细错误列表
// 与 ParseTask 不同，此方法不返回错误，而是将所有错误收集到列表中返回
// 这允许调用者获取完整的错误详情而不会被单个错误中断
func (s *Service) ParseTaskWithErrors(taskID string) []ParseErrorDetail {
	var errors []ParseErrorDetail

	var rawOutputs []models.RawCommandOutput
	s.db.Where("task_id = ? AND status = ?", taskID, "success").Find(&rawOutputs)

	mapper := GetMapper("huawei")

	for _, output := range rawOutputs {
		if output.FilePath == "" {
			errors = append(errors, ParseErrorDetail{
				DeviceIP:   output.DeviceIP,
				CommandKey: output.CommandKey,
				Error:      "原始输出文件路径为空",
				ParseTime:  time.Now().Format("2006-01-02 15:04:05"),
			})
			s.updateRawParseStatus(output.ID, "skipped", "原始输出文件路径为空")
			continue
		}

		rawText, err := os.ReadFile(output.FilePath)
		if err != nil {
			msg := fmt.Sprintf("读取 %s 失败: %v", output.CommandKey, err)
			errors = append(errors, ParseErrorDetail{
				DeviceIP:   output.DeviceIP,
				CommandKey: output.CommandKey,
				Error:      msg,
				ParseTime:  time.Now().Format("2006-01-02 15:04:05"),
			})
			s.updateRawParseStatus(output.ID, "parse_failed", msg)
			continue
		}

		rows, err := s.parser.Parse(output.CommandKey, string(rawText))
		if err != nil {
			msg := fmt.Sprintf("解析 %s 失败: %v", output.CommandKey, err)
			errors = append(errors, ParseErrorDetail{
				DeviceIP:   output.DeviceIP,
				CommandKey: output.CommandKey,
				Error:      msg,
				ParseTime:  time.Now().Format("2006-01-02 15:04:05"),
			})
			s.updateRawParseStatus(output.ID, "parse_failed", msg)
			continue
		}

		// 根据命令类型进行映射验证
		var mapErr error
		switch output.CommandKey {
		case "interface_brief", "interface_detail":
			_, mapErr = mapper.ToInterfaces(rows)
		case "lldp_neighbor":
			_, mapErr = mapper.ToLLDP(rows)
		case "mac_address":
			_, mapErr = mapper.ToFDB(rows)
		case "arp_all":
			_, mapErr = mapper.ToARP(rows)
		case "eth_trunk", "eth_trunk_verbose":
			_, mapErr = mapper.ToAggregate(rows)
		}

		if mapErr != nil {
			msg := fmt.Sprintf("映射 %s 失败: %v", output.CommandKey, mapErr)
			errors = append(errors, ParseErrorDetail{
				DeviceIP:   output.DeviceIP,
				CommandKey: output.CommandKey,
				Error:      msg,
				ParseTime:  time.Now().Format("2006-01-02 15:04:05"),
			})
			s.updateRawParseStatus(output.ID, "parse_failed", msg)
		}
	}

	return errors
}
