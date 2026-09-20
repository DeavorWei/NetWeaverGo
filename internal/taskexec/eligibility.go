package taskexec

import (
	"fmt"
	"strings"

	"github.com/NetWeaverGo/core/internal/device"
	"github.com/NetWeaverGo/core/internal/models"
)

// CheckDeviceEligibility 校验设备是否具备执行某项能力的准入资格
// 当设备不受支持时返回 isEligible=false 与可读原因，便于编排编译器将设备标记为 unsupported 而非硬性失败
func CheckDeviceEligibility(d *models.DeviceAsset, capabilityKey string) (bool, string) {
	if d == nil {
		return false, "设备对象为空"
	}

	model := strings.TrimSpace(d.Model)
	if model == "" {
		// 未知款型默认允许尝试
		return true, ""
	}

	// 1. 业务比对 (bizcompare) 准入：严格基于 support_devices.json 矩阵
	if strings.EqualFold(capabilityKey, models.CapabilityBizCompare) {
		reg := device.GetDefaultProfileRegistry()
		return reg.IsDeviceSupported(model, d.Version)
	}

	// 2. 形态准入：软件形态设备跳过硬件/弱光类专项检测
	if strings.EqualFold(d.FormFactor, "software") {
		if strings.EqualFold(capabilityKey, models.CapabilityOptical) ||
			strings.EqualFold(capabilityKey, models.CapabilityHardware) {
			return false, fmt.Sprintf("软件形态设备 %s 不支持 %s 专项采集", model, capabilityKey)
		}
	}

	// 3. 其余能力键（inspection/topology/batchping/configbuild）当前口径为"默认允许"，
	//    统一由 models.Capability* 常量定义，避免字面量漂移（P3-4）。
	return true, ""
}
