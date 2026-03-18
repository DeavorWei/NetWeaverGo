// Package discovery 提供网络拓扑发现功能
// 数据库模型已迁移到 internal/models 包
package discovery

import (
	"github.com/NetWeaverGo/core/internal/models"
)

// 重新导出 models 包中的类型，保持向后兼容
type DiscoveryTask = models.DiscoveryTask
type DiscoveryDevice = models.DiscoveryDevice
type RawCommandOutput = models.RawCommandOutput
type StartDiscoveryRequest = models.StartDiscoveryRequest
type TaskStartResponse = models.TaskStartResponse
type DiscoveryTaskView = models.DiscoveryTaskView
type DiscoveryDeviceView = models.DiscoveryDeviceView
type DeviceInfo = models.DeviceInfo
