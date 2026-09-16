package models

import (
	"time"
)

// OpticalCheckMode 光模块功率检测模式
type OpticalCheckMode string

const (
	OpticalModeAbsolute OpticalCheckMode = "absolute" // 绝对门限 (dBm)
	OpticalModePercent  OpticalCheckMode = "percent"  // 相对偏离百分比
)

// OpticalCheckRule 光功率告警与健康规则表
type OpticalCheckRule struct {
	ID              uint             `gorm:"primaryKey;autoIncrement" json:"id"`
	RuleName        string           `gorm:"size:128;not null" json:"ruleName"`
	Vendor          string           `gorm:"size:64;index" json:"vendor"`            // huawei | cisco | h3c | *
	TransceiverType string           `gorm:"size:64;index" json:"transceiverType"`   // 10GE-LR, 100GE-SR4, etc. 或 *
	Mode            OpticalCheckMode `gorm:"size:32;default:'absolute'" json:"mode"` // absolute | percent
	MinRxPower      float64          `json:"minRxPower"`                             // 接收光功率绝对下限 dBm (如 -18.0)
	MaxRxPower      float64          `json:"maxRxPower"`                             // 接收光功率绝对上限 dBm (如 0.0)
	MinTxPower      float64          `json:"minTxPower"`                             // 发射光功率绝对下限 dBm (如 -8.0)
	MaxTxPower      float64          `json:"maxTxPower"`                             // 发射光功率绝对上限 dBm (如 3.0)
	WarningPercent  float64          `json:"warningPercent"`                         // 偏离预警百分比 (如 15.0%)
	CritPercent     float64          `json:"critPercent"`                            // 偏离严重百分比 (如 30.0%)
	Enabled         bool             `gorm:"default:true" json:"enabled"`
	Description     string           `gorm:"size:255" json:"description"`
	CreatedAt       time.Time        `json:"createdAt"`
	UpdatedAt       time.Time        `json:"updatedAt"`
}

// TableName 指定表名
func (OpticalCheckRule) TableName() string {
	return "optical_check_rules"
}

// DefaultOpticalRule 提供内置兜底光功率规则
func DefaultOpticalRule() OpticalCheckRule {
	return OpticalCheckRule{
		RuleName:        "默认万兆/通用光模块阈值",
		Vendor:          "*",
		TransceiverType: "*",
		Mode:            OpticalModeAbsolute,
		MinRxPower:      -18.0,
		MaxRxPower:      -1.0,
		MinTxPower:      -8.0,
		MaxTxPower:      2.0,
		WarningPercent:  15.0,
		CritPercent:     30.0,
		Enabled:         true,
		Description:     "内置默认光功率检测规则",
	}
}
