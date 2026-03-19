package discovery

// CommandSpec 命令规格定义
type CommandSpec struct {
	Command    string `json:"command"`    // 实际执行的命令
	CommandKey string `json:"commandKey"` // 唯一标识：version, lldp_neighbor, interface 等
	TimeoutSec int    `json:"timeoutSec"` // 超时秒数
}

// VendorCommandProfile 厂商命令配置
type VendorCommandProfile struct {
	Vendor   string        `json:"vendor"`   // 厂商标识
	Name     string        `json:"name"`     // 厂商名称
	Commands []CommandSpec `json:"commands"` // 命令列表
}

// 预定义的厂商命令配置
var vendorProfiles = map[string]*VendorCommandProfile{
	"huawei": {
		Vendor: "huawei",
		Name:   "华为",
		Commands: []CommandSpec{
			{Command: "display version", CommandKey: "version", TimeoutSec: 30},
			{Command: "display current-configuration | include sysname", CommandKey: "sysname", TimeoutSec: 20},
			{Command: "display esn", CommandKey: "esn", TimeoutSec: 20},
			{Command: "display lldp neighbor verbose", CommandKey: "lldp_neighbor", TimeoutSec: 60},
			{Command: "display interface brief", CommandKey: "interface_brief", TimeoutSec: 30},
			{Command: "display interface", CommandKey: "interface_detail", TimeoutSec: 60},
			{Command: "display mac-address", CommandKey: "mac_address", TimeoutSec: 60},
			{Command: "display eth-trunk", CommandKey: "eth_trunk", TimeoutSec: 30},
			{Command: "display eth-trunk verbose", CommandKey: "eth_trunk_verbose", TimeoutSec: 45},
			{Command: "display arp all", CommandKey: "arp_all", TimeoutSec: 60},
			{Command: "display device", CommandKey: "device_info", TimeoutSec: 30},
		},
	},
	"h3c": {
		Vendor: "h3c",
		Name:   "华三",
		Commands: []CommandSpec{
			{Command: "display version", CommandKey: "version", TimeoutSec: 30},
			{Command: "display lldp neighbor-information verbose", CommandKey: "lldp_neighbor", TimeoutSec: 60},
			{Command: "display interface brief", CommandKey: "interface_brief", TimeoutSec: 30},
			{Command: "display mac-address", CommandKey: "mac_address", TimeoutSec: 60},
			{Command: "display link-aggregation verbose", CommandKey: "eth_trunk", TimeoutSec: 30},
			{Command: "display arp all", CommandKey: "arp_all", TimeoutSec: 60},
		},
	},
	"cisco": {
		Vendor: "cisco",
		Name:   "思科",
		Commands: []CommandSpec{
			{Command: "show version", CommandKey: "version", TimeoutSec: 30},
			{Command: "show lldp neighbors detail", CommandKey: "lldp_neighbor", TimeoutSec: 60},
			{Command: "show interface status", CommandKey: "interface_brief", TimeoutSec: 30},
			{Command: "show mac address-table", CommandKey: "mac_address", TimeoutSec: 60},
			{Command: "show etherchannel summary", CommandKey: "eth_trunk", TimeoutSec: 30},
			{Command: "show ip arp", CommandKey: "arp_all", TimeoutSec: 60},
		},
	},
}

// GetVendorProfile 获取厂商命令配置
func GetVendorProfile(vendor string) *VendorCommandProfile {
	if profile, ok := vendorProfiles[vendor]; ok {
		return profile
	}
	// 默认返回华为配置
	return vendorProfiles["huawei"]
}

// GetAllVendorProfiles 获取所有厂商配置
func GetAllVendorProfiles() []*VendorCommandProfile {
	profiles := make([]*VendorCommandProfile, 0, len(vendorProfiles))
	for _, profile := range vendorProfiles {
		profiles = append(profiles, profile)
	}
	return profiles
}

// GetCommandKeys 获取命令键列表
func GetCommandKeys(vendor string) []string {
	profile := GetVendorProfile(vendor)
	keys := make([]string, 0, len(profile.Commands))
	for _, cmd := range profile.Commands {
		keys = append(keys, cmd.CommandKey)
	}
	return keys
}

// GetCommandByKey 根据命令键获取命令规格
func GetCommandByKey(vendor, commandKey string) *CommandSpec {
	profile := GetVendorProfile(vendor)
	for i := range profile.Commands {
		if profile.Commands[i].CommandKey == commandKey {
			return &profile.Commands[i]
		}
	}
	return nil
}

// DefaultVendor 默认厂商
const DefaultVendor = "huawei"

// SupportedVendors 支持的厂商列表
var SupportedVendors = []string{"huawei", "h3c", "cisco"}

// IsVendorSupported 检查厂商是否支持
func IsVendorSupported(vendor string) bool {
	for _, v := range SupportedVendors {
		if v == vendor {
			return true
		}
	}
	return false
}
