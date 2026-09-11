package models

import (
	"time"
)

// DeviceProfileRecord 设备画像覆盖记录模型（对应 DB 覆盖表 device_profiles，落实规划方案 §6.2/§9）
type DeviceProfileRecord struct {
	ID           uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	Vendor       string    `gorm:"index;size:64;not null" json:"vendor"`
	Series       string    `gorm:"index;size:64" json:"series"`
	ModelsJSON   string    `gorm:"column:models_json;type:text" json:"modelsJson"`     // 适用款型列表 JSON 字符串
	VersionsJSON string    `gorm:"column:versions_json;type:text" json:"versionsJson"` // 适用版本列表 JSON 字符串
	ProfileJSON  string    `gorm:"column:profile_json;type:text;not null" json:"profileJson"` // 序列化的 DeviceProfile JSON
	Description  string    `gorm:"size:256" json:"description"`
	Enabled      bool      `gorm:"default:true" json:"enabled"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// TableName 指定表名
func (DeviceProfileRecord) TableName() string {
	return "device_profiles"
}
