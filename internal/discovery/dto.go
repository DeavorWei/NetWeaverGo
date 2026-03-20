// Package discovery 提供网络拓扑发现功能
package discovery

import "github.com/NetWeaverGo/core/internal/models"

// DeviceAssetRow 设备资产查询结果行
// 用于从 device_assets 表查询设备信息并转换为 DeviceInfo
type DeviceAssetRow struct {
	ID          uint   `gorm:"column:id"`
	IP          string `gorm:"column:ip"`
	Port        int    `gorm:"column:port"`
	Username    string `gorm:"column:username"`
	Password    string `gorm:"column:password"`
	Vendor      string `gorm:"column:vendor"`
	DisplayName string `gorm:"column:display_name"`
	Role        string `gorm:"column:role"`
	Site        string `gorm:"column:site"`
	Group       string `gorm:"column:group_name"`
}

// ToDeviceInfo 将 DeviceAssetRow 转换为 DeviceInfo
func (r *DeviceAssetRow) ToDeviceInfo() models.DeviceInfo {
	return models.DeviceInfo{
		ID:          r.ID,
		IP:          r.IP,
		Port:        r.Port,
		Username:    r.Username,
		Password:    r.Password,
		Vendor:      r.Vendor,
		DisplayName: r.DisplayName,
		Role:        r.Role,
		Site:        r.Site,
	}
}
