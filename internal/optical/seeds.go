package optical

import (
	"github.com/NetWeaverGo/core/internal/models"
	"gorm.io/gorm"
)

// DefaultOpticalRules 内置弱光检测规则种子（P1-10：为前端规则管理提供数据基线）
func DefaultOpticalRules() []models.OpticalCheckRule {
	return []models.OpticalCheckRule{
		{
			RuleName:        "通用 10G 光模块收发功率门限",
			Vendor:          "*",
			TransceiverType: "10GE-*",
			Mode:            models.OpticalModeAbsolute,
			MinRxPower:      -18.0,
			MaxRxPower:      0.0,
			MinTxPower:      -8.5,
			MaxTxPower:      3.0,
			Enabled:         true,
			Description:     "适用于 10GE 光模块的通用接收/发射光功率门限（参考 IEEE 802.3ae 与常见 LR/SR 模块）",
		},
		{
			RuleName:        "通用 25G/100G 光模块收发功率门限",
			Vendor:          "*",
			TransceiverType: "100GE-*",
			Mode:            models.OpticalModeAbsolute,
			MinRxPower:      -13.0,
			MaxRxPower:      2.0,
			MinTxPower:      -6.0,
			MaxTxPower:      4.0,
			Enabled:         true,
			Description:     "适用于 25G/100G 高速光模块的通用功率门限",
		},
		{
			RuleName:        "光模块功率偏离预警（百分比模式）",
			Vendor:          "*",
			TransceiverType: "*",
			Mode:            models.OpticalModePercent,
			WarningPercent:  15.0,
			CritPercent:     30.0,
			Enabled:         true,
			Description:     "无绝对门限数据时，按标称功率的相对偏离百分比判定预警/严重（RSSI 弱光检测兜底策略）",
		},
	}
}

// EnsureOpticalCheckSeeds 确保内置弱光检测规则已落库（幂等：非空即跳过）
func EnsureOpticalCheckSeeds(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	var count int64
	if err := db.Model(&models.OpticalCheckRule{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	seeds := DefaultOpticalRules()
	return db.Create(&seeds).Error
}
