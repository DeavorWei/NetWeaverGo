package alarm

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/NetWeaverGo/core/internal/models"
)

// MergerConfig 归并引擎配置
type MergerConfig struct {
	// TimeWindow 同设备关联告警聚合时间窗口（默认 10 分钟）
	TimeWindow time.Duration
}

// DefaultMergerConfig 默认配置
func DefaultMergerConfig() MergerConfig {
	return MergerConfig{
		TimeWindow: 10 * time.Minute,
	}
}

// AlarmMerger 告警归并引擎
type AlarmMerger struct {
	config MergerConfig
}

// NewAlarmMerger 创建告警归并引擎实例
func NewAlarmMerger(cfg ...MergerConfig) *AlarmMerger {
	c := DefaultMergerConfig()
	if len(cfg) > 0 {
		c = cfg[0]
	}
	return &AlarmMerger{config: c}
}

// Merge 将一批原始告警记录聚合并分析生成故障现象
func (m *AlarmMerger) Merge(records []models.AlarmRecord) []models.MergedPhenomenon {
	if len(records) == 0 {
		return nil
	}

	// 1. 按设备 IP 分组
	groupedByDevice := make(map[string][]models.AlarmRecord)
	for _, rec := range records {
		ip := strings.TrimSpace(rec.DeviceIP)
		if ip == "" {
			ip = "unknown"
		}
		groupedByDevice[ip] = append(groupedByDevice[ip], rec)
	}

	var allPhenomena []models.MergedPhenomenon

	// 2. 逐设备做时间窗口与故障语义关联归并
	for deviceIP, devRecords := range groupedByDevice {
		phenomena := m.mergeDeviceRecords(deviceIP, devRecords)
		allPhenomena = append(allPhenomena, phenomena...)
	}

	// 3. 排序：按严重性与发生时间排序
	sort.Slice(allPhenomena, func(i, j int) bool {
		sevOrder := map[string]int{
			"critical": 4,
			"major":    3,
			"minor":    2,
			"warning":  1,
			"info":     0,
		}
		if sevOrder[allPhenomena[i].Severity] != sevOrder[allPhenomena[j].Severity] {
			return sevOrder[allPhenomena[i].Severity] > sevOrder[allPhenomena[j].Severity]
		}
		return allPhenomena[i].CreatedAt.After(allPhenomena[j].CreatedAt)
	})

	return allPhenomena
}

// mergeDeviceRecords 归并单台设备的告警（按 TimeWindow 进行时间窗口分桶）
func (m *AlarmMerger) mergeDeviceRecords(deviceIP string, records []models.AlarmRecord) []models.MergedPhenomenon {
	if len(records) == 0 {
		return nil
	}

	// 1. 按发生时间排序
	sortedRecords := make([]models.AlarmRecord, len(records))
	copy(sortedRecords, records)
	sort.Slice(sortedRecords, func(i, j int) bool {
		return sortedRecords[i].OccurredAt.Before(sortedRecords[j].OccurredAt)
	})

	// 2. 根据 TimeWindow 切片分桶
	window := m.config.TimeWindow
	if window <= 0 {
		window = 10 * time.Minute
	}

	var buckets [][]models.AlarmRecord
	var currentBucket []models.AlarmRecord
	var bucketStart time.Time

	for _, rec := range sortedRecords {
		if len(currentBucket) == 0 {
			currentBucket = append(currentBucket, rec)
			bucketStart = rec.OccurredAt
			continue
		}

		// 若当前记录与当前桶起始时间之差超过时间窗口，则开启新桶
		if !rec.OccurredAt.IsZero() && !bucketStart.IsZero() && rec.OccurredAt.Sub(bucketStart) > window {
			buckets = append(buckets, currentBucket)
			currentBucket = []models.AlarmRecord{rec}
			bucketStart = rec.OccurredAt
		} else {
			currentBucket = append(currentBucket, rec)
		}
	}
	if len(currentBucket) > 0 {
		buckets = append(buckets, currentBucket)
	}

	// 3. 逐桶执行规则归并
	var allResults []models.MergedPhenomenon
	for _, b := range buckets {
		phenomena := m.mergeDeviceBucket(deviceIP, b)
		allResults = append(allResults, phenomena...)
	}
	return allResults
}

// mergeDeviceBucket 归并单台设备单一时间窗口内的告警
func (m *AlarmMerger) mergeDeviceBucket(deviceIP string, records []models.AlarmRecord) []models.MergedPhenomenon {
	if len(records) == 0 {
		return nil
	}

	// 记录哪些告警已被归并
	mergedIDs := make(map[uint]bool)
	var result []models.MergedPhenomenon
	runID := records[0].RunID
	family := records[0].Family

	// 分类桶
	var powerAlarms, fanAlarms, tempAlarms, opticalAlarms, routingAlarms, resAlarms []models.AlarmRecord

	for _, r := range records {
		cat := strings.ToLower(r.Category)
		name := strings.ToUpper(r.AlarmName)
		switch {
		case cat == "power" || strings.Contains(name, "POWER") || strings.Contains(name, "PWR") || strings.Contains(name, "POE"):
			powerAlarms = append(powerAlarms, r)
		case cat == "fan" || strings.Contains(name, "FAN"):
			fanAlarms = append(fanAlarms, r)
		case strings.Contains(name, "TEMP"):
			tempAlarms = append(tempAlarms, r)
		case cat == "interface" || strings.Contains(name, "OPTICAL") || strings.Contains(name, "IF_") || strings.Contains(name, "LINK"):
			opticalAlarms = append(opticalAlarms, r)
		case cat == "routing" || strings.Contains(name, "BGP") || strings.Contains(name, "OSPF"):
			routingAlarms = append(routingAlarms, r)
		case cat == "resource" || strings.Contains(name, "CPU") || strings.Contains(name, "MEM") || strings.Contains(name, "SESSION"):
			resAlarms = append(resAlarms, r)
		}
	}

	// ===== 规则 A：物理光路/接口故障引发路由邻居中断（衍生抑制与根因锁定） =====
	if len(opticalAlarms) > 0 && len(routingAlarms) > 0 {
		var combinedIDs []uint
		for _, a := range opticalAlarms {
			combinedIDs = append(combinedIDs, a.ID)
			mergedIDs[a.ID] = true
		}
		for _, a := range routingAlarms {
			combinedIDs = append(combinedIDs, a.ID)
			mergedIDs[a.ID] = true
		}

		result = append(result, models.MergedPhenomenon{
			RunID:       runID,
			DeviceIP:    deviceIP,
			Family:      family,
			Title:       fmt.Sprintf("物理链路/光模块故障引发路由邻居中断 (聚合 %d 条告警)", len(combinedIDs)),
			Severity:    "critical",
			Category:    "routing_infrastructure",
			RootCause:   "根因定位为底层物理接口/光链路异常断开，直接导致上层 BGP/OSPF 协议邻居超时断连（属于级联衍生告警）",
			Advice:      "优先排查本端与对端物理光路收发光功率及光模块连接，物理链路恢复后协议邻居将自动协商重建",
			ImpactScope: "该节点承载的业务路由转发中断，可能触发全网路由重新收敛或流量旁路",
			RecordCount: len(combinedIDs),
			RecordIDs:   combinedIDs,
			CreatedAt:   time.Now(),
		})
	}

	// ===== 规则 B：风道散热与温度耦合告警 =====
	if (len(fanAlarms) > 0 && len(tempAlarms) > 0) || len(fanAlarms) >= 2 {
		var fanTempIDs []uint
		for _, a := range fanAlarms {
			if !mergedIDs[a.ID] {
				fanTempIDs = append(fanTempIDs, a.ID)
				mergedIDs[a.ID] = true
			}
		}
		for _, a := range tempAlarms {
			if !mergedIDs[a.ID] {
				fanTempIDs = append(fanTempIDs, a.ID)
				mergedIDs[a.ID] = true
			}
		}

		if len(fanTempIDs) > 0 {
			result = append(result, models.MergedPhenomenon{
				RunID:       runID,
				DeviceIP:    deviceIP,
				Family:      family,
				Title:       fmt.Sprintf("整机风道散热异常与温度升高综合预警 (聚合 %d 条告警)", len(fanTempIDs)),
				Severity:    "critical", // 严重度提升
				Category:    "hardware_environment",
				RootCause:   "风扇模块转速失常或多模块故障，导致机箱风道气流受阻引起关键温感探针超标",
				Advice:      "请立即确认机房制冷空调运行状态，检查机箱防尘滤网并更换停转的风扇框，避免芯片高温降频或热停机保护",
				ImpactScope: "主控与板卡计算芯片温度超阈值，长时间未处理将引发设备自我关机保护",
				RecordCount: len(fanTempIDs),
				RecordIDs:   fanTempIDs,
				CreatedAt:   time.Now(),
			})
		}
	}

	// ===== 规则 C：电源供电系统冗余失效或全面故障 =====
	unmergedPower := make([]models.AlarmRecord, 0)
	for _, a := range powerAlarms {
		if !mergedIDs[a.ID] {
			unmergedPower = append(unmergedPower, a)
		}
	}
	if len(unmergedPower) >= 2 {
		var pwrIDs []uint
		for _, a := range unmergedPower {
			pwrIDs = append(pwrIDs, a.ID)
			mergedIDs[a.ID] = true
		}
		result = append(result, models.MergedPhenomenon{
			RunID:       runID,
			DeviceIP:    deviceIP,
			Family:      family,
			Title:       fmt.Sprintf("设备双路电源全面异常或双模组断电 (聚合 %d 条告警)", len(pwrIDs)),
			Severity:    "critical",
			Category:    "power_system",
			RootCause:   "机箱全部/多个供电模块发生硬件损坏或外部市电/PDU输入断电",
			Advice:      "立即核查机架 A/B 路双电源 PDU 供电开关，若市电正常请紧急更换电源模块以防整机掉电",
			ImpactScope: "设备失去全部供电保障，面临断电停机事故风险",
			RecordCount: len(pwrIDs),
			RecordIDs:   pwrIDs,
			CreatedAt:   time.Now(),
		})
	} else if len(unmergedPower) == 1 {
		p := unmergedPower[0]
		mergedIDs[p.ID] = true
		result = append(result, models.MergedPhenomenon{
			RunID:       runID,
			DeviceIP:    deviceIP,
			Family:      family,
			Title:       fmt.Sprintf("设备单路电源失效，供电冗余降级 (告警: %s)", p.AlarmName),
			Severity:    "major",
			Category:    "power_system",
			RootCause:   fmt.Sprintf("设备供电系统单一模块故障: %s", p.Summary),
			Advice:      "目前依靠单路电源维持工作，1+1/N+1 容错已失效，请尽快更换备件恢复双路供电",
			ImpactScope: "供电冗余丧失，剩余单电源如再发生波动将直接停机",
			RecordCount: 1,
			RecordIDs:   []uint{p.ID},
			CreatedAt:   time.Now(),
		})
	}

	// ===== 规则 D：核心计算与内存双过载 =====
	unmergedRes := make([]models.AlarmRecord, 0)
	for _, a := range resAlarms {
		if !mergedIDs[a.ID] {
			unmergedRes = append(unmergedRes, a)
		}
	}
	if len(unmergedRes) >= 2 {
		var resIDs []uint
		for _, a := range unmergedRes {
			resIDs = append(resIDs, a.ID)
			mergedIDs[a.ID] = true
		}
		result = append(result, models.MergedPhenomenon{
			RunID:       runID,
			DeviceIP:    deviceIP,
			Family:      family,
			Title:       fmt.Sprintf("控制平面 CPU 与内存资源双重严重过载 (聚合 %d 条告警)", len(resIDs)),
			Severity:    "critical",
			Category:    "system_resource",
			RootCause:   "控制面高负荷运算（如路由重算/协议泛洪）伴随内存激增，资源池逼近枯竭",
			Advice:      "排查上送控制面报文速率，检查是否存在突发流量或内存泄漏，必要时限制管理通道速率",
			ImpactScope: "设备 CLI 响应迟缓，协议心跳报文可能因调度不及时产生误判超时",
			RecordCount: len(resIDs),
			RecordIDs:   resIDs,
			CreatedAt:   time.Now(),
		})
	}

	// ===== 剩余独立未归并告警：逐条转化为独立现象 =====
	for _, r := range records {
		if mergedIDs[r.ID] {
			continue
		}
		mergedIDs[r.ID] = true
		result = append(result, models.MergedPhenomenon{
			RunID:       runID,
			DeviceIP:    deviceIP,
			Family:      family,
			Title:       fmt.Sprintf("告警事件: %s (%s)", r.AlarmName, r.Severity),
			Severity:    r.Severity,
			Category:    r.Category,
			RootCause:   r.Summary,
			Advice:      "按照对应厂商告警处置指南进行排障处理",
			ImpactScope: "局部网元单项功能受影响",
			RecordCount: 1,
			RecordIDs:   []uint{r.ID},
			CreatedAt:   time.Now(),
		})
	}

	return result
}
