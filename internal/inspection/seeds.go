package inspection

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	seedOnce sync.Once
)

// DefaultTemplates 内置巡检模板
var DefaultTemplates = []models.InspectionTemplate{
	{
		ID:          "tpl-huawei-general",
		Name:        "华为数通设备通用日常健康巡检模板",
		Vendor:      "huawei",
		Category:    "general",
		Description: "适用于各类华为路由器与交换机的通用关键指标巡检（CPU/内存/温度/风扇/电源/接口等）",
		Enabled:     true,
	},
	{
		ID:          "tpl-huawei-ce",
		Name:        "华为 CloudEngine 数据中心交换机深度巡检模板",
		Vendor:      "huawei",
		Category:    "ce",
		Description: "面向金融/IDC数据中心的核心高可用巡检（堆叠/BGP/转发面/时钟）",
		Enabled:     true,
	},
	{
		ID:          "tpl-huawei-s",
		Name:        "华为 S 系列园区接入与汇聚交换机巡检模板",
		Vendor:      "huawei",
		Category:    "s",
		Description: "面向园区网的业务承载与端口状态巡检（PoE/STP防环/端口链路等）",
		Enabled:     true,
	},
	{
		ID:          "tpl-huawei-ar",
		Name:        "华为 AR 系列企业广域网路由器巡检模板",
		Vendor:      "huawei",
		Category:    "ar",
		Description: "面向企业总部与分支出口的广域链路与协议巡检（WAN口/路由邻居/VPN等）",
		Enabled:     true,
	},
}

// DefaultItems 内置检查项种子清单
var DefaultItems = []models.InspectionItem{
	// ================= 通用模板检查项 =================
	{
		TemplateID:   "tpl-huawei-general",
		Code:         "GEN_CPU_USAGE",
		Name:         "主控 CPU 使用率检查",
		Category:     "system",
		CommandKey:   "display cpu-usage",
		IsPreCollect: true,
		CheckType:    "threshold",
		Field:        "cpu_usage",
		Severity:     "major",
		Order:        1,
		Description:  "检查设备主控板 CPU 使用率是否低于警戒阈值 (80%)",
		Problem:      "主控板 CPU 利用率超过安全警戒线 (80%)",
		Advice:       "排查导致 CPU 高企的任务进程，检查是否存在二层广播风暴、OSPF/BGP路由频繁震荡或微突发大流量",
		ThresholdsJSON: mustJSON([]models.Threshold{
			{Name: "cpu_usage", DataType: "float", DefaultValue: "80.0", MaxValue: "80.0", RangeType: "lte", Unit: "%"},
		}),
		Enabled: true,
	},
	{
		TemplateID:   "tpl-huawei-general",
		Code:         "GEN_MEM_USAGE",
		Name:         "内存使用率检查",
		Category:     "system",
		CommandKey:   "display memory-usage",
		IsPreCollect: true,
		CheckType:    "threshold",
		Field:        "memory_usage",
		Severity:     "major",
		Order:        2,
		Description:  "检查设备物理内存使用率是否低于安全警戒线 (85%)",
		Problem:      "设备内存占用率超过安全警戒线 (85%)",
		Advice:       "执行 display memory-usage 排查占用内存最高模块，检查是否存在路由转发表项过大或进程内存泄漏",
		ThresholdsJSON: mustJSON([]models.Threshold{
			{Name: "memory_usage", DataType: "float", DefaultValue: "85.0", MaxValue: "85.0", RangeType: "lte", Unit: "%"},
		}),
		Enabled: true,
	},
	{
		TemplateID:   "tpl-huawei-general",
		Code:         "GEN_TEMPERATURE",
		Name:         "单板环境温度检查",
		Category:     "environment",
		CommandKey:   "display temperature",
		IsPreCollect: false,
		CheckType:    "threshold",
		Field:        "temperature",
		Severity:     "major",
		Order:        3,
		Description:  "检查单板温度传感器数值是否处于安全运行区间 (≤ 65°C)",
		Problem:      "设备单板温度过高，存在高温保护自动关机风险",
		Advice:       "检查机房空调与温湿度环境，清理设备防尘网与进出风口障碍物，核验机架散热风道",
		ThresholdsJSON: mustJSON([]models.Threshold{
			{Name: "temperature", DataType: "float", DefaultValue: "65.0", MaxValue: "65.0", RangeType: "lte", Unit: "°C"},
		}),
		Enabled: true,
	},
	{
		TemplateID:   "tpl-huawei-general",
		Code:         "GEN_FAN_STATUS",
		Name:         "风扇运行状态检查",
		Category:     "environment",
		CommandKey:   "display fan",
		IsPreCollect: false,
		CheckType:    "must_not_contain",
		Field:        "status",
		Severity:     "blocker",
		Order:        4,
		Description:  "检查设备各风扇框与模块状态，不得包含 Abnormal/Fail 异常",
		Problem:      "设备风扇模块状态异常或转速停转",
		Advice:       "检查风扇托架是否插好，风扇叶片是否有机械阻挡或积灰卡死，必要时更换故障风扇模块",
		ThresholdsJSON: mustJSON([]models.Threshold{
			{Name: "fan_abnormal", DataType: "string", DefaultValue: "Abnormal"},
		}),
		Enabled: true,
	},
	{
		TemplateID:   "tpl-huawei-general",
		Code:         "GEN_POWER_STATUS",
		Name:         "电源模块供电状态检查",
		Category:     "environment",
		CommandKey:   "display power",
		IsPreCollect: false,
		CheckType:    "must_not_contain",
		Field:        "status",
		Severity:     "blocker",
		Order:        5,
		Description:  "检查双电源或冗余电源状态，严禁出现供电中断或故障",
		Problem:      "电源模块供电异常，双电源冗余失效",
		Advice:       "检查电源 PDU 供电线路输入电压是否稳定，检查电源卡扣接触是否紧固",
		ThresholdsJSON: mustJSON([]models.Threshold{
			{Name: "power_fail", DataType: "string", DefaultValue: "Abnormal"},
		}),
		Enabled: true,
	},
	{
		TemplateID:   "tpl-huawei-general",
		Code:         "GEN_DEVICE_UPTIME",
		Name:         "设备连续运行时间健康检查",
		Category:     "system",
		CommandKey:   "display version",
		IsPreCollect: true,
		CheckType:    "must_contain",
		Field:        "uptime",
		Severity:     "minor",
		Order:        6,
		Description:  "确认系统正常持续开机时间（排查非预期重启）",
		Problem:      "无法获取系统持续运行时间或设备发生异常重启",
		Advice:       "执行 display reboot-info 查阅设备最近一次重启原因，判断是否为硬件看门狗复位或突发掉电",
		ThresholdsJSON: mustJSON([]models.Threshold{
			{Name: "uptime_keyword", DataType: "string", DefaultValue: "uptime is"},
		}),
		Enabled: true,
	},
	{
		TemplateID:   "tpl-huawei-general",
		Code:         "GEN_INTERFACE_ERRORS",
		Name:         "接口错包与丢包检查",
		Category:     "interface",
		CommandKey:   "display interface brief",
		IsPreCollect: false,
		CheckType:    "must_not_contain",
		Field:        "error_state",
		Severity:     "minor",
		Order:        7,
		Description:  "排查物理接口是否存在 CRC、Input Errors 或频繁 Flap 错包",
		Problem:      "接口存在误码率高或丢包隐患",
		Advice:       "使用光功率计检查光模块接收与发送光衰，清洁光纤端面或更换跳线",
		ThresholdsJSON: mustJSON([]models.Threshold{
			{Name: "error_state", DataType: "string", DefaultValue: "Error-Down"},
		}),
		Enabled: true,
	},
	{
		TemplateID:   "tpl-huawei-general",
		Code:         "GEN_NTP_SYNC",
		Name:         "NTP 时钟同步状态检查",
		Category:     "system",
		CommandKey:   "display ntp status",
		IsPreCollect: false,
		CheckType:    "must_contain",
		Field:        "clock_status",
		Severity:     "minor",
		Order:        8,
		Description:  "验证设备与全网基准时钟服务器是否保持精确同步 (synchronized)",
		Problem:      "NTP 时钟未同步，可能导致全网日志关联审计与证书认证失败",
		Advice:       "检查上游 NTP 服务器 IP 可达性，核验 UDP 123 端口放通策略及时区配置",
		ThresholdsJSON: mustJSON([]models.Threshold{
			{Name: "sync_keyword", DataType: "string", DefaultValue: "synchronized"},
		}),
		Enabled: true,
	},

	// ================= CE 数据中心交换机专属 =================
	{
		TemplateID:   "tpl-huawei-ce",
		Code:         "CE_CPU_USAGE",
		Name:         "CE 数据中心核心 CPU 状态检查",
		Category:     "system",
		CommandKey:   "display cpu-usage",
		IsPreCollect: true,
		CheckType:    "threshold",
		Field:        "cpu_usage",
		Severity:     "major",
		Order:        1,
		Description:  "数据中心交换机核心主控与业务板 CPU 运行监测",
		Problem:      "CE 核心交换机 CPU 超负荷",
		Advice:       "排查 EVPN/VXLAN 控制面收敛状态或控制面微突发流",
		ThresholdsJSON: mustJSON([]models.Threshold{
			{Name: "cpu_usage", DataType: "float", DefaultValue: "75.0", MaxValue: "75.0", RangeType: "lte", Unit: "%"},
		}),
		Enabled: true,
	},
	{
		TemplateID:   "tpl-huawei-ce",
		Code:         "CE_BGP_PEER",
		Name:         "BGP/EVPN 邻居会话状态检查",
		Category:     "routing",
		CommandKey:   "display bgp peer",
		IsPreCollect: false,
		CheckType:    "must_contain",
		Field:        "state",
		Severity:     "blocker",
		Order:        2,
		Description:  "验证数据中心 Spine-Leaf 间 BGP 邻居建立状态为 Established",
		Problem:      "部分 BGP 邻居未进入 Established 状态，路由同步异常",
		Advice:       "检查 BGP Router-ID、AS 号配置及直连端口连通性",
		ThresholdsJSON: mustJSON([]models.Threshold{
			{Name: "bgp_state", DataType: "string", DefaultValue: "Established"},
		}),
		Enabled: true,
	},

	// ================= S 园区交换机专属 =================
	{
		TemplateID:   "tpl-huawei-s",
		Code:         "S_STP_STATUS",
		Name:         "STP/RSTP 环路保护状态检查",
		Category:     "interface",
		CommandKey:   "display stp brief",
		IsPreCollect: false,
		CheckType:    "must_not_contain",
		Field:        "role",
		Severity:     "major",
		Order:        1,
		Description:  "检查园区网二层边缘接入接口防环与生成树收敛状态",
		Problem:      "生成树协议发生频繁拓扑变化 (Topology Change)",
		Advice:       "开启边缘端口 BPDU 保护，排查接入层是否存在私接交换机引发的环路震荡",
		ThresholdsJSON: mustJSON([]models.Threshold{
			{Name: "stp_disrupt", DataType: "string", DefaultValue: "DISCARDING"},
		}),
		Enabled: true,
	},

	// ================= AR 路由器专属 =================
	{
		TemplateID:   "tpl-huawei-ar",
		Code:         "AR_OSPF_NEIGHBOR",
		Name:         "OSPF 邻居建立状态检查",
		Category:     "routing",
		CommandKey:   "display ospf peer brief",
		IsPreCollect: false,
		CheckType:    "must_contain",
		Field:        "state",
		Severity:     "blocker",
		Order:        1,
		Description:  "检查广域网分支与骨干核心 OSPF 邻居状态为 Full",
		Problem:      "OSPF 邻居状态异常（处于 2-Way 或 Init）",
		Advice:       "检查接口 MTU、Area ID、Hello/Dead 计时器以及认证密钥配置是否一致",
		ThresholdsJSON: mustJSON([]models.Threshold{
			{Name: "ospf_state", DataType: "string", DefaultValue: "Full"},
		}),
		Enabled: true,
	},
}

// EnsureInspectionSeeds 初始化巡检模板与检查项内置种子数据（幂等保证）
// textSeedOnce 多语言文案种子的幂等控制（与 EnsureInspectionSeeds 独立）
var textSeedOnce sync.Once

// EnsureInspectionItemTextSeeds 初始化巡检多语言文案种子。
//
// 定位（方案 §5.3.6）：服务于海外/涉外局点的**英文巡检报告导出**，
// 不铺开前端 UI 国际化。种子覆盖内置模板的全部检查项，Key 与 InspectionItem.Code 一一对应。
func EnsureInspectionItemTextSeeds(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	var err error
	textSeedOnce.Do(func() {
		now := time.Now()
		texts := make([]models.InspectionItemText, len(DefaultItemTexts))
		for i, t := range DefaultItemTexts {
			texts[i] = t
			texts[i].CreatedAt = now
			texts[i].UpdatedAt = now
		}
		if txErr := db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "key"}, {Name: "locale"}},
			DoUpdates: clause.AssignmentColumns([]string{"name", "description", "advice", "updated_at"}),
		}).Create(&texts).Error; txErr != nil {
			logger.Warn("Inspection", "-", "初始化巡检多语言文案种子失败: %v", txErr)
			err = txErr
			return
		}
	})
	return err
}

// DefaultItemTexts 内置多语言文案。
// Key 必须等于 InspectionItem.Code（如 GEN_CPU_USAGE），否则导出时无法命中。
// 覆盖内置模板的全部检查项；中文导出直接复用结果原文，不在此重复维护。
var DefaultItemTexts = []models.InspectionItemText{
	// ===== tpl-huawei-general =====
	{
		Key:         "GEN_CPU_USAGE",
		Locale:      "en-US",
		Name:        "Main control CPU usage check",
		Description: "Check whether the main control board CPU usage is below the threshold (80%).",
		Advice:      "Identify top CPU-consuming processes with display cpu-usage; check for L2 broadcast storms, OSPF/BGP route flapping or micro-burst traffic.",
	},
	{
		Key:         "GEN_MEM_USAGE",
		Locale:      "en-US",
		Name:        "Memory usage check",
		Description: "Check whether the physical memory usage is below the threshold (85%).",
		Advice:      "Run display memory-usage to locate the largest memory consumers; watch for oversized routing tables or process memory leaks.",
	},
	{
		Key:         "GEN_TEMPERATURE",
		Locale:      "en-US",
		Name:        "Board temperature check",
		Description: "Check whether board temperature sensors are within the safe range (<= 65C).",
		Advice:      "Verify room air conditioning and humidity, clean dust filters and intake/exhaust vents, and check rack airflow.",
	},
	{
		Key:         "GEN_FAN_STATUS",
		Locale:      "en-US",
		Name:        "Fan status check",
		Description: "Check that no fan tray or module reports Abnormal/Fail state.",
		Advice:      "Reseat fan trays, remove mechanical obstructions or dust from fan blades, and replace faulty fan modules if necessary.",
	},
	{
		Key:         "GEN_POWER_STATUS",
		Locale:      "en-US",
		Name:        "Power module status check",
		Description: "Check dual or redundant power supply state; no outage or fault is allowed.",
		Advice:      "Verify PDU input voltage stability and that power module latches are securely seated.",
	},
	{
		Key:         "GEN_DEVICE_UPTIME",
		Locale:      "en-US",
		Name:        "Device uptime health check",
		Description: "Confirm a normal continuous uptime to detect unexpected reboots.",
		Advice:      "Run display reboot-info to inspect the last reboot reason and determine whether a watchdog reset or power loss occurred.",
	},
	{
		Key:         "GEN_INTERFACE_ERRORS",
		Locale:      "en-US",
		Name:        "Interface error and packet loss check",
		Description: "Detect physical interfaces with CRC errors, input errors or frequent flaps.",
		Advice:      "Use an optical power meter to check Rx/Tx levels, clean fiber end faces, or replace patch cords.",
	},
	{
		Key:         "GEN_NTP_SYNC",
		Locale:      "en-US",
		Name:        "NTP clock synchronization check",
		Description: "Verify the device is precisely synchronized (synchronized) with the reference clock server.",
		Advice:      "Verify upstream NTP server reachability, UDP 123 policy and timezone configuration.",
	},
	// ===== tpl-huawei-ce =====
	{
		Key:         "CE_CPU_USAGE",
		Locale:      "en-US",
		Name:        "CE data center core CPU status check",
		Description: "Monitor CPU usage of the CE core main control and service boards.",
		Advice:      "Inspect EVPN/VXLAN control-plane convergence state or control-plane micro-burst traffic.",
	},
	{
		Key:         "CE_BGP_PEER",
		Locale:      "en-US",
		Name:        "BGP/EVPN peer session status check",
		Description: "Verify that BGP peers between data center Spine-Leaf nodes are Established.",
		Advice:      "Verify BGP Router-ID, AS number configuration and direct-link connectivity.",
	},
	// ===== tpl-huawei-s =====
	{
		Key:         "S_STP_STATUS",
		Locale:      "en-US",
		Name:        "STP/RSTP loop protection status check",
		Description: "Check loop protection and spanning tree convergence on campus edge access ports.",
		Advice:      "Enable BPDU protection on edge ports and check for unauthorized switches causing topology flapping.",
	},
	// ===== tpl-huawei-ar =====
	{
		Key:         "AR_OSPF_NEIGHBOR",
		Locale:      "en-US",
		Name:        "OSPF neighbor adjacency status check",
		Description: "Check that OSPF neighbors between WAN branches and the core are in Full state.",
		Advice:      "Verify interface MTU, Area ID, Hello/Dead timers and authentication key consistency.",
	},
}

func EnsureInspectionSeeds(db *gorm.DB) error {
	var err error
	seedOnce.Do(func() {
		now := time.Now()

		// 1. 初始化模板
		templates := make([]models.InspectionTemplate, len(DefaultTemplates))
		for i, t := range DefaultTemplates {
			templates[i] = t
			templates[i].CreatedAt = now
			templates[i].UpdatedAt = now
		}
		if txErr := db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "id"}},
			DoNothing: true,
		}).Create(&templates).Error; txErr != nil {
			logger.Warn("Inspection", "-", "初始化巡检模板种子失败: %v", txErr)
			err = txErr
			return
		}

		// 2. 初始化检查项
		items := make([]models.InspectionItem, len(DefaultItems))
		for i, it := range DefaultItems {
			items[i] = it
			items[i].CreatedAt = now
			items[i].UpdatedAt = now
		}
		if txErr := db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "template_id"}, {Name: "code"}},
			DoNothing: true,
		}).Create(&items).Error; txErr != nil {
			logger.Warn("Inspection", "-", "初始化巡检检查项种子失败: %v", txErr)
			err = txErr
			return
		}

		logger.Info("Inspection", "-", "成功初始化内置巡检模板 (%d 个) 与检查项 (%d 项)", len(templates), len(items))
	})
	return err
}

func mustJSON(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}
