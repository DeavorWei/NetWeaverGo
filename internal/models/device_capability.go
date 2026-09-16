package models

import (
	"gorm.io/gorm"
)

// DeviceCapability 设备能力准入记录
type DeviceCapability struct {
	ID            uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	ModelPattern  string `gorm:"index;size:128;not null" json:"modelPattern"` // 款型正则/名称
	Versions      string `gorm:"type:text" json:"versions"`                   // JSON 版本列表
	CapabilityKey string `gorm:"index;size:128;not null" json:"capabilityKey"` // 能力标识 (如 "inspection", "bizcompare", "topology")
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
		{ModelPattern: "^(?:S|CE|AR|NE|USG|CX|ME)", Versions: `["*"]`, CapabilityKey: "inspection", Enabled: true},
		{ModelPattern: "^(?:S|CE|AR|NE)", Versions: `["*"]`, CapabilityKey: "topology", Enabled: true},
		{ModelPattern: "^(?:CE|NE|S)", Versions: `["*"]`, CapabilityKey: "bizcompare", Enabled: true},
		{ModelPattern: ".*", Versions: `["*"]`, CapabilityKey: "batchping", Enabled: true},
		{ModelPattern: ".*", Versions: `["*"]`, CapabilityKey: "configbuild", Enabled: true},
	}
	return db.Create(&seeds).Error
}
