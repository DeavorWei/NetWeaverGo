package alarm

import (
	"regexp"
	"strings"
	"sync"

	"github.com/NetWeaverGo/core/internal/models"
)

// CompiledAlarmRule 带预编译正则的告警规则
type CompiledAlarmRule struct {
	models.AlarmRule
	Regex *regexp.Regexp
}

// RuleRegistry 告警规则注册表
type RuleRegistry struct {
	mu            sync.RWMutex
	rulesByFamily map[string][]*CompiledAlarmRule
	allRules      []*CompiledAlarmRule
}

var (
	defaultRegistry     *RuleRegistry
	defaultRegistryOnce sync.Once
)

// GetDefaultRegistry 获取全局单例告警规则注册表
func GetDefaultRegistry() *RuleRegistry {
	defaultRegistryOnce.Do(func() {
		defaultRegistry = NewRuleRegistry()
		defaultRegistry.RegisterBatch(GetBuiltinAlarmRules())
	})
	return defaultRegistry
}

// NewRuleRegistry 创建规则注册表
func NewRuleRegistry() *RuleRegistry {
	return &RuleRegistry{
		rulesByFamily: make(map[string][]*CompiledAlarmRule),
		allRules:      make([]*CompiledAlarmRule, 0),
	}
}

// Register 注册单条规则（含正则编译）
func (r *RuleRegistry) Register(rule models.AlarmRule) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	re, err := regexp.Compile(rule.Pattern)
	if err != nil {
		return err
	}

	cr := &CompiledAlarmRule{
		AlarmRule: rule,
		Regex:     re,
	}

	familyKey := strings.ToUpper(strings.TrimSpace(rule.Family))
	r.rulesByFamily[familyKey] = append(r.rulesByFamily[familyKey], cr)
	r.allRules = append(r.allRules, cr)
	return nil
}

// RegisterBatch 批量注册规则
func (r *RuleRegistry) RegisterBatch(rules []models.AlarmRule) []error {
	var errs []error
	for _, rule := range rules {
		if err := r.Register(rule); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

// GetRules 查询适用于特定产品族的规则列表（包含产品族规则与通用通用规则）
func (r *RuleRegistry) GetRules(family string) []*CompiledAlarmRule {
	r.mu.RLock()
	defer r.mu.RUnlock()

	fam := strings.ToUpper(strings.TrimSpace(family))
	var matched []*CompiledAlarmRule

	// 先加入该产品族专属规则
	if famRules, ok := r.rulesByFamily[fam]; ok {
		matched = append(matched, famRules...)
	}

	// 补充通用规则（family 为 COMMON 或 *）
	if fam != "COMMON" && fam != "*" {
		if commonRules, ok := r.rulesByFamily["COMMON"]; ok {
			matched = append(matched, commonRules...)
		}
		if anyRules, ok := r.rulesByFamily["*"]; ok {
			matched = append(matched, anyRules...)
		}
	}

	return matched
}

// MatchLine 对单行或文本片段匹配首条或多条规则
func (r *RuleRegistry) MatchLine(family, line string) []*CompiledAlarmRule {
	rules := r.GetRules(family)
	var hits []*CompiledAlarmRule
	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}
		if rule.Regex.MatchString(line) {
			hits = append(hits, rule)
		}
	}
	return hits
}

// GetBuiltinAlarmRules 返回内置各产品族高频经典告警规则
func GetBuiltinAlarmRules() []models.AlarmRule {
	return []models.AlarmRule{
		// ==================== CE 系列（数据中心交换机） ====================
		{
			Vendor: "huawei", Family: "CE", AlarmName: "CE_POWER_FAIL",
			Pattern:  `(?i)(Power\s+supply\s+([A-Za-z0-9/_-]+)\s+failed|Power\s+module\s+\d+\s+is\s+faulty|POWER_MODULE_FAIL)`,
			Severity: "critical", Category: "power",
			Description: "数据中心交换机电源模块发生硬件故障或输入异常",
			Advice:      "检查对应槽位电源供电线缆与输入电压，若无外部异常请更换电源模块",
			Enabled:     true,
		},
		{
			Vendor: "huawei", Family: "CE", AlarmName: "CE_POWER_ABSENT",
			Pattern:  `(?i)(Power\s+module\s+\d+\s+uninstalled|Power\s+module\s+is\s+absent|POWER_ABSENT)`,
			Severity: "major", Category: "power",
			Description: "数据中心交换机电源模块被拔出或未在位",
			Advice:      "确认是否有维护拔插操作，如为非计划离线请重新插紧电源模块",
			Enabled:     true,
		},
		{
			Vendor: "huawei", Family: "CE", AlarmName: "CE_FAN_FAIL",
			Pattern:  `(?i)(Fan\s+module\s+\d+\s+failed|Fan\s+speed\s+abnormal|FAN_MODULE_FAIL)`,
			Severity: "major", Category: "fan",
			Description: "风扇框工作异常或转速失常，影响散热风道",
			Advice:      "清点风道是否有异物阻塞，若停转请及时更换风扇模块",
			Enabled:     true,
		},
		{
			Vendor: "huawei", Family: "CE", AlarmName: "CE_OPTICAL_LOS",
			Pattern:  `(?i)(Optical\s+module\s+([A-Za-z0-9/_-]+)\s+RX\s+power\s+too\s+low|OPTICAL_LOS|Optical\s+transceiver\s+loss\s+of\s+signal)`,
			Severity: "major", Category: "interface",
			Description: "光模块接收光功率极低或断光（LOS）",
			Advice:      "检查对应接口光纤跳线、尾纤及对端发光状态，清洁光纤端面",
			Enabled:     true,
		},
		{
			Vendor: "huawei", Family: "CE", AlarmName: "CE_IF_FLAPPING",
			Pattern:  `(?i)(Interface\s+([A-Za-z0-9/_-]+)\s+physical\s+status\s+flapping|LINK_FLAPPING)`,
			Severity: "major", Category: "interface",
			Description: "物理接口频繁反复 Up/Down 震荡",
			Advice:      "排查链路衰减、光模块接触不良或对端接口协商模式",
			Enabled:     true,
		},
		{
			Vendor: "huawei", Family: "CE", AlarmName: "CE_BGP_PEER_DOWN",
			Pattern:  `(?i)(BGP_STATE_CHANGE.*DOWN|BGP.*peer\s+([0-9\.]+)\s+down)`,
			Severity: "critical", Category: "routing",
			Description: "BGP 邻居连接中断",
			Advice:      "检查底层物理连通性、TCP 179 端口及 HoldTime 超时原因",
			Enabled:     true,
		},
		{
			Vendor: "huawei", Family: "CE", AlarmName: "CE_STACK_SPLIT",
			Pattern:  `(?i)(Stack\s+split|Stack\s+member\s+\d+\s+left|iStack\s+topology\s+changed)`,
			Severity: "critical", Category: "system",
			Description: "交换机堆叠裂化或成员脱离",
			Advice:      "检查堆叠物理连接线与专用堆叠口配置，避免发生双主脑裂",
			Enabled:     true,
		},

		// ==================== Router 系列（NE/AR 路由器） ====================
		{
			Vendor: "huawei", Family: "Router", AlarmName: "ROUTER_LPU_OFFLINE",
			Pattern:  `(?i)(LPU\s+\d+\s+is\s+offline|Board\s+\d+\s+offline|LPU_OFFLINE)`,
			Severity: "critical", Category: "system",
			Description: "路由器业务板卡下线脱机",
			Advice:      "查看系统日志定位板卡复位原因，排查供电与总线通信",
			Enabled:     true,
		},
		{
			Vendor: "huawei", Family: "Router", AlarmName: "ROUTER_MPU_SWITCHOVER",
			Pattern:  `(?i)(MPU\s+switchover\s+occurred|Active/standby\s+switchover|MPU_SWITCHOVER)`,
			Severity: "major", Category: "system",
			Description: "主控板发生主备倒换",
			Advice:      "收集原主控黑匣子诊断日志，分析主备倒换触发诱因",
			Enabled:     true,
		},
		{
			Vendor: "huawei", Family: "Router", AlarmName: "ROUTER_OSPF_NBR_DOWN",
			Pattern:  `(?i)(OSPF.*Neighbor\s+([0-9\.]+)\s+status\s+changed\s+to\s+Down|OSPF_NBR_DOWN)`,
			Severity: "major", Category: "routing",
			Description: "OSPF 路由邻居状态切换为 Down",
			Advice:      "排查链路 MTU、Hello/Dead 间隔、网络类型及底层误码",
			Enabled:     true,
		},
		{
			Vendor: "huawei", Family: "Router", AlarmName: "ROUTER_CPU_HIGH",
			Pattern:  `(?i)(CPU\s+usage\s+exceeded\s+threshold|CPU_OVER_THRESHOLD|CPU\s+usage\s+is\s+higher\s+than\s+\d+%)`,
			Severity: "major", Category: "resource",
			Description: "路由器主控或转发引擎 CPU 持续高于安全门限",
			Advice:      "排查是否存在协议报文上送泛洪（如 ARP/STP）或路由表大面积重算",
			Enabled:     true,
		},

		// ==================== S 系列（园区园区交换机） ====================
		{
			Vendor: "huawei", Family: "S", AlarmName: "S_LOOPBACK_DETECTED",
			Pattern:  `(?i)(Loopback\s+detected\s+on\s+interface|LOOP_DETECT|Loop\s+exists)`,
			Severity: "critical", Category: "interface",
			Description: "园区接入/汇聚层发生二层二层环路风暴",
			Advice:      "立即检查下游接入交换机或集线器自环，开启 STP/LDT 破环阻断",
			Enabled:     true,
		},
		{
			Vendor: "huawei", Family: "S", AlarmName: "S_POE_OVERLOAD",
			Pattern:  `(?i)(PoE\s+power\s+overload|PoE\s+power\s+supply\s+insufficient|POE_OVERLOAD)`,
			Severity: "major", Category: "power",
			Description: "交换机 PoE 供电功率不足或超出额定载荷",
			Advice:      "调整 AP/IP话机 等受电设备功率优先级，或扩充外置 PoE 电源模块",
			Enabled:     true,
		},
		{
			Vendor: "huawei", Family: "S", AlarmName: "S_UPLINK_DOWN",
			Pattern:  `(?i)(Uplink\s+port\s+([A-Za-z0-9/_-]+)\s+down|Eth-Trunk\d+\s+is\s+down)`,
			Severity: "critical", Category: "interface",
			Description: "园区核心或汇聚上行聚合链路整体中断",
			Advice:      "优先检查汇聚上联光纤收发光，排查上行端口链路聚合协议状态",
			Enabled:     true,
		},

		// ==================== USG 系列（安全防火墙） ====================
		{
			Vendor: "huawei", Family: "USG", AlarmName: "USG_HRP_HEARTBEAT_LOST",
			Pattern:  `(?i)(HRP\s+heartbeat\s+link\s+is\s+down|HRP_HEARTBEAT_LOST|Dual-system\s+hot\s+backup\s+broken)`,
			Severity: "critical", Category: "system",
			Description: "防火墙双机热备心跳中断，存在双主分流风险",
			Advice:      "检查心跳接口物理连通性、VGMP 与 HRP 对等体配置",
			Enabled:     true,
		},
		{
			Vendor: "huawei", Family: "USG", AlarmName: "USG_SESSION_FULL",
			Pattern:  `(?i)(Session\s+table\s+is\s+full|Concurrent\s+session\s+exceeded\s+license|SESSION_TABLE_OVERFLOW)`,
			Severity: "critical", Category: "resource",
			Description: "防火墙并发连接会话表耗尽，新业务连接被阻断丢弃",
			Advice:      "排查内网病毒异常大连接、优化 TCP/UDP 垃圾会话老化时间",
			Enabled:     true,
		},

		// ==================== VNE 系列（虚拟化网元） ====================
		{
			Vendor: "huawei", Family: "VNE", AlarmName: "VNE_VLINK_DISCONNECT",
			Pattern:  `(?i)(Virtual\s+link\s+disconnected|VNE_VLINK_DOWN|vNIC\s+link\s+down)`,
			Severity: "major", Category: "interface",
			Description: "虚拟化网元内部虚拟网卡或软件总线链路失联",
			Advice:      "检查宿主机 vSwitch、OVS 网桥状态及虚拟硬件驱动",
			Enabled:     true,
		},

		// ==================== COMMON 通用底座规则 ====================
		{
			Vendor: "generic", Family: "COMMON", AlarmName: "MEM_OVER_THRESHOLD",
			Pattern:  `(?i)(Memory\s+usage\s+exceeded\s+threshold|Memory\s+usage\s+is\s+higher\s+than\s+\d+%|MEMORY_THRESHOLD_EXCEED)`,
			Severity: "major", Category: "resource",
			Description: "设备内存使用率超过预警安全上限",
			Advice:      "排查是否存在内存泄漏进程，准备在维护期重启受影响子系统",
			Enabled:     true,
		},
		{
			Vendor: "generic", Family: "COMMON", AlarmName: "TEMP_OVER_THRESHOLD",
			Pattern:  `(?i)(Temperature\s+is\s+higher\s+than\s+threshold|High\s+temperature\s+warning|TEMPERATURE_TOO_HIGH)`,
			Severity: "major", Category: "system",
			Description: "机箱内部温度传感器超标预警",
			Advice:      "检查机房空调与环境制冷温度，检查滤网是否积灰阻断冷风",
			Enabled:     true,
		},
	}
}
