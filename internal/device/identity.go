package device

import (
	"regexp"
	"strings"
)

var reCiscoIOS = regexp.MustCompile(`(?i)\b(Cisco|IOS|IOS-XE|NX-OS)\b`)

// Identity 设备形态认知身份模型
// 独立于拓扑链路，作为纯识别领域的标准化输出模型
type Identity struct {
	Vendor     string            `json:"vendor"`               // 厂商标识 (huawei / h3c / cisco / generic)
	Model      string            `json:"model"`                // 款型 (DEV_TYPE: 如 S5735, CE6866, USG6600)
	DetailType string            `json:"detailType,omitempty"` // 详细款型 (DEV_DETTYPE: 如 S5735-L24P4S-A2)
	Version    string            `json:"version"`              // 软件版本 (DEV_VERSION: 如 V200R019C00SPC500)
	Patch      string            `json:"patch,omitempty"`      // 补丁版本 (PATCH_VERSION: 如 V200R019SPH005)
	VRBD       string            `json:"vrbd,omitempty"`       // V/R/B/D 版本格式 (DEV_VRBD: 如 V200R019B010)
	Series     string            `json:"series"`               // 归一化系列 (SERIES: 如 S5700, CE6800)
	SysName    string            `json:"sysName,omitempty"`    // 主机名/设备名
	Evidence   []string          `json:"evidence,omitempty"`   // 命中的判定证据（支持可解释与审计）
	Raws       map[string]string `json:"-"`                    // 原始输入缓存，供增量多命令合并判定
}

// Identify 依据 display version、display patch-information、display device 等命令回显联合判定
func Identify(vendor string, raws map[string]string) (*Identity, error) {
	v := strings.ToLower(strings.TrimSpace(vendor))

	// 提取核心回显文本
	verText := ""
	patchText := ""
	devText := ""

	// 1. 精确 key 优先
	for k, val := range raws {
		lowerK := strings.ToLower(strings.TrimSpace(k))
		switch lowerK {
		case "version", "display version", "show version":
			if verText == "" {
				verText = val
			}
		case "patch", "patch_info", "patch-information", "display patch-information":
			if patchText == "" {
				patchText = val
			}
		case "device", "display device", "inventory", "show inventory":
			if devText == "" {
				devText = val
			}
		}
	}

	// 2. 模糊 key 兜底（patch 优先于 version）
	for k, val := range raws {
		lowerK := strings.ToLower(strings.TrimSpace(k))
		if patchText == "" && strings.Contains(lowerK, "patch") {
			patchText = val
		} else if verText == "" && strings.Contains(lowerK, "version") {
			verText = val
		} else if devText == "" && (strings.Contains(lowerK, "device") || strings.Contains(lowerK, "inventory")) {
			devText = val
		}
	}

	var result *Identity
	var err error

	switch v {
	case "huawei":
		result, err = identifyHuawei(verText, patchText, devText)
	case "h3c":
		result, err = identifyH3C(verText, patchText, devText)
	case "cisco":
		result, err = identifyCisco(verText, patchText, devText)
	default:
		// 通用或未知厂商：优先使用通用规则探测，若识别出华为特征则自动校正
		if strings.Contains(verText, "Huawei") || strings.Contains(verText, "VRP (R)") || strings.Contains(verText, "VRP(R)") {
			result, err = identifyHuawei(verText, patchText, devText)
		} else if strings.Contains(verText, "H3C") || strings.Contains(verText, "Comware") {
			result, err = identifyH3C(verText, patchText, devText)
		} else if reCiscoIOS.MatchString(verText) {
			result, err = identifyCisco(verText, patchText, devText)
		} else {
			result, err = identifyGeneric(v, verText)
		}
	}

	if result != nil && len(raws) > 0 {
		result.Raws = make(map[string]string, len(raws))
		for k, val := range raws {
			result.Raws[k] = val
		}
	}
	return result, err
}
