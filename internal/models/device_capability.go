package models

import (
	"gorm.io/gorm"
)

// 能力键常量（P3-4：统一口径，避免种子与准入校验各自使用字面量而漂移）
const (
	CapabilityInspection  = "inspection"  // 巡检编排
	CapabilityTopology    = "topology"    // 拓扑采集
	CapabilityBizCompare  = "bizcompare"  // 业务比对（严格款型矩阵准入）
	CapabilityBatchPing   = "batchping"   // 批量 Ping
	CapabilityConfigBuild = "configbuild" // 配置生成
	CapabilityOptical     = "optical"     // 弱光专项（软件形态不支持）
	CapabilityHardware    = "hardware"    // 硬件专项（软件形态不支持）
)

// DeviceCapability 设备能力准入记录
type DeviceCapability struct {
	ID            uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	ModelPattern  string `gorm:"index;size:128;not null" json:"modelPattern"`  // 款型正则/名称
	Versions      string `gorm:"type:text" json:"versions"`                    // JSON 版本列表
	CapabilityKey string `gorm:"index;size:128;not null" json:"capabilityKey"` // 能力标识（见 Capability* 常量）
	Enabled       bool   `gorm:"default:true" json:"enabled"`
}

// TableName 表名
func (DeviceCapability) TableName() string {
	return "device_capabilities"
}

// EnsureDeviceCapabilitySeeds 初始化设备能力初始种子
func EnsureDeviceCapabilitySeeds(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	var count int64
	if err := db.Model(&DeviceCapability{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	seeds := []DeviceCapability{
		{ModelPattern: "^(?:S|CE|AR|NE|USG|CX|ME)", Versions: `["*"]`, CapabilityKey: CapabilityInspection, Enabled: true},
		{ModelPattern: "^(?:S|CE|AR|NE)", Versions: `["*"]`, CapabilityKey: CapabilityTopology, Enabled: true},
		{ModelPattern: "^(?:CE|NE|S)", Versions: `["*"]`, CapabilityKey: CapabilityBizCompare, Enabled: true},
		{ModelPattern: ".*", Versions: `["*"]`, CapabilityKey: CapabilityBatchPing, Enabled: true},
		{ModelPattern: ".*", Versions: `["*"]`, CapabilityKey: CapabilityConfigBuild, Enabled: true},
	}
	return db.Create(&seeds).Error
}
