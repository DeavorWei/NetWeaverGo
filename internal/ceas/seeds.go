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

// DefaultBOMWatchlistSeeds 华为 4 大产品线代表性 BOM 白名单预警种子
var DefaultBOMWatchlistSeeds = []models.BOMWatchlistItem{
	// CE 数据中心交换机
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

	// S 园区交换机
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

	// AR 企业路由器
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

	// NE 核心路由器
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
