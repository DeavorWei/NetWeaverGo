package config

// PTYConfig PTY 终端配置
type PTYConfig struct {
	TermType string `json:"termType"` // 终端类型：vt100, xterm 等
	Width    int    `json:"width"`    // 终端宽度
	Height   int    `json:"height"`   // 终端高度
	EchoMode int    `json:"echoMode"` // 回显模式
	ISpeed   int    `json:"iSpeed"`   // 输入速率
	OSpeed   int    `json:"oSpeed"`   // 输出速率
}

// DefaultPTYConfig 返回默认 PTY 配置
func DefaultPTYConfig() PTYConfig {
	return PTYConfig{
		TermType: "vt100",
		Width:    256,
		Height:   200,
		EchoMode: 0,
		ISpeed:   14400,
		OSpeed:   14400,
	}
}

// PromptConfig 提示符配置
type PromptConfig struct {
	Suffixes []string `json:"suffixes"` // 提示符后缀：>, #, ]
	Patterns []string `json:"patterns"` // 正则模式（可选）
}

// DefaultPromptConfig 返回默认提示符配置
func DefaultPromptConfig() PromptConfig {
	return PromptConfig{
		Suffixes: []string{">", "#", "]"},
		Patterns: []string{},
	}
}

// PagerConfig 分页配置
type PagerConfig struct {
	Patterns      []string `json:"patterns"`      // 分页提示符模式
	ContinueBytes []byte   `json:"continueBytes"` // 续页发送的字节（默认空格）
}

// DefaultPagerConfig 返回默认分页配置
func DefaultPagerConfig() PagerConfig {
	return PagerConfig{
		Patterns: []string{
			"---- More ----",
			"--More--",
			"---- More",
			"---- More System ----",
			"More:",
		},
		ContinueBytes: []byte{' '},
	}
}

// InitConfig 初始化配置
type InitConfig struct {
	DisablePagerCommands []string `json:"disablePagerCommands"` // 禁用分页的命令列表
	ExtraCommands        []string `json:"extraCommands"`        // 额外初始化命令
	PromptTimeoutSec     int      `json:"promptTimeoutSec"`     // 等待提示符超时（秒）
}

// DefaultInitConfig 返回默认初始化配置
func DefaultInitConfig() InitConfig {
	return InitConfig{
		DisablePagerCommands: []string{},
		ExtraCommands:        []string{},
		PromptTimeoutSec:     30,
	}
}

// CommandSpec 命令规格定义
type CommandSpec struct {
	Command    string `json:"command"`    // 实际执行的命令
	CommandKey string `json:"commandKey"` // 唯一标识：version, lldp_neighbor, interface 等
	TimeoutSec int    `json:"timeoutSec"` // 超时秒数
}

// DeviceProfile 设备画像 - 统一的厂商配置
type DeviceProfile struct {
	Vendor   string        `json:"vendor"`   // 厂商标识
	Name     string        `json:"name"`     // 厂商名称
	PTY      PTYConfig     `json:"pty"`      // PTY 配置
	Prompt   PromptConfig  `json:"prompt"`   // 提示符配置
	Pager    PagerConfig   `json:"pager"`    // 分页配置
	Init     InitConfig    `json:"init"`     // 初始化配置
	Commands []CommandSpec `json:"commands"` // 命令列表
}

// ProfileProvider 画像提供者接口
type ProfileProvider interface {
	// GetByVendor 根据厂商标识获取设备画像
	GetByVendor(vendor string) (*DeviceProfile, bool)
	// DetectFallback 当厂商未知时，根据提示符和 banner 探测厂商
	DetectFallback(prompt string, banner string) string
}

// deviceProfileRegistry 设备画像注册表
type deviceProfileRegistry struct {
	profiles map[string]*DeviceProfile
}

// 全局画像注册表
var globalRegistry = &deviceProfileRegistry{
	profiles: make(map[string]*DeviceProfile),
}

func init() {
	// 初始化厂商画像
	registerVendorProfiles()
}

// registerVendorProfiles 注册厂商画像
func registerVendorProfiles() {
	// Huawei 画像
	globalRegistry.profiles["huawei"] = &DeviceProfile{
		Vendor: "huawei",
		Name:   "华为",
		PTY: PTYConfig{
			TermType: "vt100",
			Width:    256,
			Height:   200,
			EchoMode: 0,
			ISpeed:   14400,
			OSpeed:   14400,
		},
		Prompt: PromptConfig{
			Suffixes: []string{">", "#", "]"},
			Patterns: []string{`<[^>]+>[#>\]]`, `\[[^\]]+\][#>\]]`},
		},
		Pager: PagerConfig{
			Patterns: []string{
				"---- More ----",
				"--More--",
				"---- More",
			},
			ContinueBytes: []byte{' '},
		},
		Init: InitConfig{
			DisablePagerCommands: []string{"screen-length 0 temporary"},
			ExtraCommands:        []string{},
			PromptTimeoutSec:     30,
		},
		Commands: []CommandSpec{
			{Command: "display version", CommandKey: "version", TimeoutSec: 30},
			{Command: "display current-configuration | include sysname", CommandKey: "sysname", TimeoutSec: 20},
			{Command: "display device esn", CommandKey: "esn", TimeoutSec: 20},
			{Command: "display lldp neighbor", CommandKey: "lldp_neighbor", TimeoutSec: 60},
			{Command: "display interface brief", CommandKey: "interface_brief", TimeoutSec: 30},
			{Command: "display interface", CommandKey: "interface_detail", TimeoutSec: 60},
			{Command: "display mac-address", CommandKey: "mac_address", TimeoutSec: 60},
			{Command: "display eth-trunk", CommandKey: "eth_trunk", TimeoutSec: 30},
			{Command: "display arp all", CommandKey: "arp_all", TimeoutSec: 60},
			{Command: "display device", CommandKey: "device_info", TimeoutSec: 30},
		},
	}

	// H3C 画像
	globalRegistry.profiles["h3c"] = &DeviceProfile{
		Vendor: "h3c",
		Name:   "华三",
		PTY: PTYConfig{
			TermType: "vt100",
			Width:    256,
			Height:   200,
			EchoMode: 0,
			ISpeed:   14400,
			OSpeed:   14400,
		},
		Prompt: PromptConfig{
			Suffixes: []string{">", "#", "]"},
			Patterns: []string{`<[^>]+>[#>\]]`, `\[[^\]]+\][#>\]]`},
		},
		Pager: PagerConfig{
			Patterns: []string{
				"---- More ----",
				"--More--",
				"---- More",
			},
			ContinueBytes: []byte{' '},
		},
		Init: InitConfig{
			DisablePagerCommands: []string{"screen-length disable"},
			ExtraCommands:        []string{},
			PromptTimeoutSec:     30,
		},
		Commands: []CommandSpec{
			{Command: "display version", CommandKey: "version", TimeoutSec: 30},
			{Command: "display lldp neighbor-information verbose", CommandKey: "lldp_neighbor", TimeoutSec: 60},
			{Command: "display interface brief", CommandKey: "interface_brief", TimeoutSec: 30},
			{Command: "display mac-address", CommandKey: "mac_address", TimeoutSec: 60},
			{Command: "display link-aggregation verbose", CommandKey: "eth_trunk", TimeoutSec: 30},
			{Command: "display arp all", CommandKey: "arp_all", TimeoutSec: 60},
		},
	}

	// Cisco 画像
	globalRegistry.profiles["cisco"] = &DeviceProfile{
		Vendor: "cisco",
		Name:   "思科",
		PTY: PTYConfig{
			TermType: "vt100",
			Width:    256,
			Height:   200,
			EchoMode: 0,
			ISpeed:   14400,
			OSpeed:   14400,
		},
		Prompt: PromptConfig{
			Suffixes: []string{">", "#"},
			Patterns: []string{`[A-Za-z][A-Za-z0-9_-]*[#>]`},
		},
		Pager: PagerConfig{
			Patterns: []string{
				"--More--",
				"---- More ----",
			},
			ContinueBytes: []byte{' '},
		},
		Init: InitConfig{
			DisablePagerCommands: []string{"terminal length 0", "terminal width 0"},
			ExtraCommands:        []string{},
			PromptTimeoutSec:     30,
		},
		Commands: []CommandSpec{
			{Command: "show version", CommandKey: "version", TimeoutSec: 30},
			{Command: "show lldp neighbors detail", CommandKey: "lldp_neighbor", TimeoutSec: 60},
			{Command: "show interface status", CommandKey: "interface_brief", TimeoutSec: 30},
			{Command: "show mac address-table", CommandKey: "mac_address", TimeoutSec: 60},
			{Command: "show etherchannel summary", CommandKey: "eth_trunk", TimeoutSec: 30},
			{Command: "show ip arp", CommandKey: "arp_all", TimeoutSec: 60},
		},
	}
}

// GetDeviceProfile 根据厂商获取设备画像
func GetDeviceProfile(vendor string) *DeviceProfile {
	if profile, ok := globalRegistry.profiles[vendor]; ok {
		return profile
	}
	// 返回默认画像（华为）
	return globalRegistry.profiles["huawei"]
}

// GetDeviceProfileByVendor 根据厂商获取设备画像，返回是否存在
func GetDeviceProfileByVendor(vendor string) (*DeviceProfile, bool) {
	profile, ok := globalRegistry.profiles[vendor]
	return profile, ok
}

// GetAllDeviceProfiles 获取所有设备画像
func GetAllDeviceProfiles() []*DeviceProfile {
	profiles := make([]*DeviceProfile, 0, len(globalRegistry.profiles))
	for _, profile := range globalRegistry.profiles {
		profiles = append(profiles, profile)
	}
	return profiles
}

// DetectVendorFromOutput 根据输出特征探测厂商
// 这是一个轻量级探测，只作为 Vendor 缺失时的 fallback
func DetectVendorFromOutput(prompt string, banner string) string {
	// 根据提示符特征探测
	promptLower := prompt
	bannerLower := banner

	// Huawei 特征
	if containsAny(promptLower, []string{"<", "Huawei", "HUAWEI"}) ||
		containsAny(bannerLower, []string{"Huawei", "HUAWEI", "VRP", "Versatile Routing Platform"}) {
		return "huawei"
	}

	// H3C 特征
	if containsAny(promptLower, []string{"H3C", "h3c"}) ||
		containsAny(bannerLower, []string{"H3C", "H3C Comware", "Comware Software"}) {
		return "h3c"
	}

	// Cisco 特征
	if containsAny(promptLower, []string{"Cisco", "cisco"}) ||
		containsAny(bannerLower, []string{"Cisco", "CISCO", "Cisco IOS"}) {
		return "cisco"
	}

	// 无法识别，返回默认
	return "huawei"
}

// containsAny 检查字符串是否包含任意子串
func containsAny(s string, substrs []string) bool {
	for _, substr := range substrs {
		if len(s) >= len(substr) {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
		}
	}
	return false
}

// GlobalProfileProvider 全局画像提供者实现
type GlobalProfileProvider struct{}

// GetByVendor 实现 ProfileProvider 接口
func (p *GlobalProfileProvider) GetByVendor(vendor string) (*DeviceProfile, bool) {
	return GetDeviceProfileByVendor(vendor)
}

// DetectFallback 实现 ProfileProvider 接口
func (p *GlobalProfileProvider) DetectFallback(prompt string, banner string) string {
	return DetectVendorFromOutput(prompt, banner)
}

// NewProfileProvider 创建画像提供者
func NewProfileProvider() ProfileProvider {
	return &GlobalProfileProvider{}
}
