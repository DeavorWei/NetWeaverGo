package ceas

import (
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

// DefaultBOMWatchlistSeeds 华为 6 大产品家族代表性高危与重点关注 BOM 预警种子（共 36 条）
var DefaultBOMWatchlistSeeds = []models.BOMWatchlistItem{
	// 1. CE 数据中心交换机
	{
		Item:        "02311UHE",
		Category:    "ce_items",
		Model:       "CE6850-48S4Q-EI",
		Description: "主板内置电容老化批次隐患预警 (Notice-2023-01)",
		Severity:    "critical",
		Enabled:     true,
		BatchNo:     "PCN-CE-2023-001",
	},
	{
		Item:        "02312DWR",
		Category:    "ce_items",
		Model:       "CE6866-48S8CQ-EI",
		Description: "PHY 芯片时钟驱动偶发失锁风险预警",
		Severity:    "danger",
		Enabled:     true,
		BatchNo:     "PCN-CE-2024-008",
	},
	{
		Item:        "02311VHB",
		Category:    "ce_items",
		Model:       "CE-MPU-D",
		Description: "主控板带外管理芯片闪存读写异常",
		Severity:    "warning",
		Enabled:     true,
		BatchNo:     "PCN-CE-2023-045",
	},
	{
		Item:        "02312NVE",
		Category:    "ce_items",
		Model:       "PAC-600WA-F",
		Description: "600W 交流电源模块风扇偶发转速异常",
		Severity:    "warning",
		Enabled:     true,
		BatchNo:     "PCN-PWR-2023-019",
	},
	{
		Item:        "02311HGB",
		Category:    "ce_items",
		Model:       "CE12804",
		Description: "主控板跨板总线偶发同步丢包隐患",
		Severity:    "critical",
		Enabled:     true,
		BatchNo:     "PCN-CE-2022-019",
	},
	{
		Item:        "02312CBA",
		Category:    "ce_items",
		Model:       "CE8850-64CQ-EI",
		Description: "100GE QSFP28 端口链路抖动风险",
		Severity:    "danger",
		Enabled:     true,
		BatchNo:     "PCN-CE-2023-021",
	},
	{
		Item:        "02311XEE",
		Category:    "ce_items",
		Model:       "CE-SFU04G",
		Description: "交换网板高速SerDes信号劣化预警",
		Severity:    "critical",
		Enabled:     true,
		BatchNo:     "PCN-CE-2023-030",
	},
	{
		Item:        "02312WSD",
		Category:    "ce_items",
		Model:       "FAN-40B-F",
		Description: "风扇模块轴承异常磨损导致转速下降预警",
		Severity:    "warning",
		Enabled:     true,
		BatchNo:     "PCN-FAN-2024-003",
	},

	// 2. S 园区交换机
	{
		Item:        "02351XGA",
		Category:    "s_items",
		Model:       "S5720-36C-EI-AC",
		Description: "Combo 口光电切换偶发不同步问题预警",
		Severity:    "danger",
		Enabled:     true,
		BatchNo:     "PCN-S-2022-011",
	},
	{
		Item:        "02352BBR",
		Category:    "s_items",
		Model:       "S5735-L24P4S-A1",
		Description: "PoE 供电芯片特定负荷保护误触发风险",
		Severity:    "critical",
		Enabled:     true,
		BatchNo:     "PCN-S-2023-033",
	},
	{
		Item:        "02353HJJ",
		Category:    "s_items",
		Model:       "ES5D21X08S00",
		Description: "8端口万兆光接口子卡特定高温告警",
		Severity:    "warning",
		Enabled:     true,
		BatchNo:     "PCN-S-2024-005",
	},
	{
		Item:        "02350VBA",
		Category:    "s_items",
		Model:       "S6720-30C-EI-24S-AC",
		Description: "万兆SFP+端口信号抖动预警",
		Severity:    "danger",
		Enabled:     true,
		BatchNo:     "PCN-S-2022-088",
	},
	{
		Item:        "02354AAB",
		Category:    "s_items",
		Model:       "S12700E-4",
		Description: "主控交换单元总线通信偶发超时",
		Severity:    "critical",
		Enabled:     true,
		BatchNo:     "PCN-S-2023-018",
	},
	{
		Item:        "02352FFD",
		Category:    "s_items",
		Model:       "S5731-H24T4XC",
		Description: "2.5G/5G 多速率端口握手异常排查",
		Severity:    "danger",
		Enabled:     true,
		BatchNo:     "PCN-S-2023-044",
	},
	{
		Item:        "02351MNK",
		Category:    "s_items",
		Model:       "ES5D21G16S00",
		Description: "16端口千兆光接口卡光功率读取偏低",
		Severity:    "warning",
		Enabled:     true,
		BatchNo:     "PCN-S-2022-061",
	},
	{
		Item:        "02353TTW",
		Category:    "s_items",
		Model:       "W0PSA1150",
		Description: "1150W PoE 电源模块特定输入欠压恢复延迟",
		Severity:    "warning",
		Enabled:     true,
		BatchNo:     "PCN-PWR-2024-006",
	},

	// 3. AR 企业路由器
	{
		Item:        "02311KLT",
		Category:    "ar_items",
		Model:       "AR6280",
		Description: "SRU主控单板背板总线时钟同步异常",
		Severity:    "critical",
		Enabled:     true,
		BatchNo:     "PCN-AR-2023-012",
	},
	{
		Item:        "02312HQQ",
		Category:    "ar_items",
		Model:       "1E1T1-M",
		Description: "E1/T1 语音子卡阻抗匹配跳线隐患",
		Severity:    "warning",
		Enabled:     true,
		BatchNo:     "PCN-AR-2022-044",
	},
	{
		Item:        "02310MMA",
		Category:    "ar_items",
		Model:       "AR6140E-9G-2AC",
		Description: "内置Flash在突发掉电下文件系统损坏风险",
		Severity:    "danger",
		Enabled:     true,
		BatchNo:     "PCN-AR-2024-002",
	},
	{
		Item:        "02311BBY",
		Category:    "ar_items",
		Model:       "SRU40",
		Description: "转发处理引擎在突发大流量下偶发死锁",
		Severity:    "danger",
		Enabled:     true,
		BatchNo:     "PCN-AR-2023-028",
	},
	{
		Item:        "02312KPP",
		Category:    "ar_items",
		Model:       "4GE-WAN",
		Description: "4端口GE广域接口卡PHY温度告警阈值偏移",
		Severity:    "warning",
		Enabled:     true,
		BatchNo:     "PCN-AR-2023-009",
	},
	{
		Item:        "02310XZZ",
		Category:    "ar_items",
		Model:       "AR3260",
		Description: "双主控切换时静态路由状态丢失隐患",
		Severity:    "critical",
		Enabled:     true,
		BatchNo:     "PCN-AR-2022-033",
	},

	// 4. NE 核心路由器
	{
		Item:        "03031AAA",
		Category:    "ne_items",
		Model:       "NE40E-X8",
		Description: "SFU 交换网板高速差分走线信号衰减预警",
		Severity:    "critical",
		Enabled:     true,
		BatchNo:     "PCN-NE-2023-009",
	},
	{
		Item:        "03032BBB",
		Category:    "ne_items",
		Model:       "LPUF-240",
		Description: "业务处理板光模块I2C总线死锁隐患",
		Severity:    "danger",
		Enabled:     true,
		BatchNo:     "PCN-NE-2024-015",
	},
	{
		Item:        "03033CCC",
		Category:    "ne_items",
		Model:       "CR52-MPU",
		Description: "主控板内存ECC偶发双位翻转排查清单",
		Severity:    "warning",
		Enabled:     true,
		BatchNo:     "PCN-NE-2023-087",
	},
	{
		Item:        "03032CR5",
		Category:    "ne_items",
		Model:       "CR52-P10-FP",
		Description: "10端口10GE灵活插卡物理接口频繁震荡",
		Severity:    "danger",
		Enabled:     true,
		BatchNo:     "PCN-NE-2024-003",
	},
	{
		Item:        "03031DXF",
		Category:    "ne_items",
		Model:       "NetEngine 8000 X8",
		Description: "业务板NP处理器特定微码指令执行异常",
		Severity:    "critical",
		Enabled:     true,
		BatchNo:     "PCN-NE-2024-021",
	},
	{
		Item:        "03032OFC",
		Category:    "ne_items",
		Model:       "OFC-10G",
		Description: "光接口适配子卡特定激光器寿命衰减",
		Severity:    "warning",
		Enabled:     true,
		BatchNo:     "PCN-NE-2023-049",
	},

	// 5. WLAN 无线控制器与 AP
	{
		Item:        "02353WLAN",
		Category:    "wlan_items",
		Model:       "AirEngine9700-M",
		Description: "无线控制器CAPWAP隧道心跳处理线程死锁隐患",
		Severity:    "critical",
		Enabled:     true,
		BatchNo:     "PCN-WLAN-2023-004",
	},
	{
		Item:        "02353WMB",
		Category:    "wlan_items",
		Model:       "AC-MAIN-BD",
		Description: "AC主控板硬件加密芯片偶发重置",
		Severity:    "danger",
		Enabled:     true,
		BatchNo:     "PCN-WLAN-2023-017",
	},
	{
		Item:        "02354AP8",
		Category:    "wlan_items",
		Model:       "AirEngine8760-X1-PRO",
		Description: "射频前端PA功率放大器过热降速预警",
		Severity:    "danger",
		Enabled:     true,
		BatchNo:     "PCN-WLAN-2024-011",
	},
	{
		Item:        "02353AP5",
		Category:    "wlan_items",
		Model:       "AirEngine5760-51",
		Description: "蓝牙/IoT射频天线信号干扰筛查",
		Severity:    "warning",
		Enabled:     true,
		BatchNo:     "PCN-WLAN-2023-039",
	},

	// 6. FW 防火墙与安全网关
	{
		Item:        "02354FW1",
		Category:    "fw_items",
		Model:       "USG6680E",
		Description: "SPU业务处理单元IPS引擎特定正则匹配内存泄漏",
		Severity:    "critical",
		Enabled:     true,
		BatchNo:     "PCN-SEC-2023-007",
	},
	{
		Item:        "02354FW2",
		Category:    "fw_items",
		Model:       "USG6530E",
		Description: "SSL硬件加速芯片在极端并发下握手失败",
		Severity:    "danger",
		Enabled:     true,
		BatchNo:     "PCN-SEC-2024-001",
	},
	{
		Item:        "02354SPU",
		Category:    "fw_items",
		Model:       "SPU-100G",
		Description: "深度报文检测协处理器时钟偶发偏移",
		Severity:    "critical",
		Enabled:     true,
		BatchNo:     "PCN-SEC-2023-025",
	},
	{
		Item:        "02354BYP",
		Category:    "fw_items",
		Model:       "WSIC-2Bypass",
		Description: "光Bypass子卡继电器触点微震动排查",
		Severity:    "warning",
		Enabled:     true,
		BatchNo:     "PCN-SEC-2022-016",
	},
}

// EnsureBOMWatchlistSeeds 初始化 BOM 观察清单种子数据（幂等保证）
func EnsureBOMWatchlistSeeds(db *gorm.DB) error {
	var err error
	seedOnce.Do(func() {
		now := time.Now()
		seeds := make([]models.BOMWatchlistItem, len(DefaultBOMWatchlistSeeds))
		for i, s := range DefaultBOMWatchlistSeeds {
			seeds[i] = s
			seeds[i].CreatedAt = now
			seeds[i].UpdatedAt = now
		}

		// 使用 Clauses OnConflict DoNothing 保证幂等且保留用户对既有项的修改
		if txErr := db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "item"}},
			DoNothing: true,
		}).Create(&seeds).Error; txErr != nil {
			logger.Warn("CEAS", "-", "初始化 BOM 观察清单种子失败: %v", txErr)
			err = txErr
			return
		}
		logger.Info("CEAS", "-", "成功初始化内置 BOM 观察清单种子，共 %d 条", len(seeds))
	})
	return err
}
