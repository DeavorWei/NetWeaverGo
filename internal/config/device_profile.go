package config

import (
	"embed"
	"encoding/json"
	"regexp"
	"strings"

	"github.com/NetWeaverGo/core/internal/device"
	"github.com/NetWeaverGo/core/internal/logger"
)

//go:embed profiles/*.json
var embeddedProfilesFS embed.FS

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
	Suffixes        []string `json:"suffixes"`        // 提示符后缀：>, #, ]
	Patterns        []string `json:"patterns"`        // 正则模式（可选）
	ConfirmPatterns []string `json:"confirmPatterns"` // 交互确认正则模式（如 [Y/N]）
	ConfirmPolicy   string   `json:"confirmPolicy"`   // 确认策略: auto_yes / auto_no / ask_user / off
}

// DefaultPromptConfig 返回默认提示符配置
func DefaultPromptConfig() PromptConfig {
	return PromptConfig{
		Suffixes:        []string{">", "#", "]"},
		Patterns:        []string{},
		ConfirmPatterns: []string{},
		ConfirmPolicy:   "ask_user", // 默认触发工程师挂起确认，保证安全
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

// CommandCondition 命令执行条件过滤规范
type CommandCondition struct {
	ModelPattern   string `json:"modelPattern,omitempty"`   // 匹配款型的正则表达式
	VersionPattern string `json:"versionPattern,omitempty"` // 匹配版本的正则表达式

	modelRegex   *regexp.Regexp
	versionRegex *regexp.Regexp
}

// CompileRegex 预编译过滤条件正则
func (c *CommandCondition) CompileRegex() {
	if c == nil {
		return
	}
	if c.ModelPattern != "" && c.modelRegex == nil {
		c.modelRegex, _ = regexp.Compile(c.ModelPattern)
	}
	if c.VersionPattern != "" && c.versionRegex == nil {
		c.versionRegex, _ = regexp.Compile(c.VersionPattern)
	}
}

// Match 检查 model 和 version 是否满足条件
func (c *CommandCondition) Match(model, version string) bool {
	if c == nil {
		return true
	}
	if c.ModelPattern != "" {
		if c.modelRegex != nil {
			if !c.modelRegex.MatchString(model) {
				return false
			}
		} else {
			if m, _ := regexp.MatchString(c.ModelPattern, model); !m {
				return false
			}
		}
	}
	if c.VersionPattern != "" {
		if c.versionRegex != nil {
			if !c.versionRegex.MatchString(version) {
				return false
			}
		} else {
			if m, _ := regexp.MatchString(c.VersionPattern, version); !m {
				return false
			}
		}
	}
	return true
}

// CommandSpec 命令规格定义
type CommandSpec struct {
	Command     string            `json:"command"`               // 实际执行的命令
	CommandKey  string            `json:"commandKey"`            // 唯一标识：version, lldp_neighbor, interface 等
	TimeoutSec  int               `json:"timeoutSec"`            // 超时秒数
	AppliesWhen *CommandCondition `json:"appliesWhen,omitempty"` // 条件过滤规则
}

// ProfileSelector 细分款型/系列画像匹配器
type ProfileSelector struct {
	ModelPattern   string `json:"modelPattern,omitempty"`   // 正则匹配 Model
	VersionPattern string `json:"versionPattern,omitempty"` // 正则匹配 Version
	Series         string `json:"series,omitempty"`         // 精确匹配归一化 Series（如 S5700）

	modelRegex   *regexp.Regexp
	versionRegex *regexp.Regexp
}

// CompileRegex 预编译选择器正则
func (s *ProfileSelector) CompileRegex() {
	if s == nil {
		return
	}
	if s.ModelPattern != "" && s.modelRegex == nil {
		s.modelRegex, _ = regexp.Compile(s.ModelPattern)
	}
	if s.VersionPattern != "" && s.versionRegex == nil {
		s.versionRegex, _ = regexp.Compile(s.VersionPattern)
	}
}

// Match 检查 model 和 version 是否满足细分画像条件
func (s *ProfileSelector) Match(model, version string) bool {
	if s == nil {
		return false
	}
	matched := true
	hasCondition := false
	if s.ModelPattern != "" {
		hasCondition = true
		if s.modelRegex != nil {
			if !s.modelRegex.MatchString(model) {
				matched = false
			}
		} else {
			if m, _ := regexp.MatchString(s.ModelPattern, model); !m {
				matched = false
			}
		}
	}
	if s.VersionPattern != "" {
		hasCondition = true
		if s.versionRegex != nil {
			if !s.versionRegex.MatchString(version) {
				matched = false
			}
		} else {
			if m, _ := regexp.MatchString(s.VersionPattern, version); !m {
				matched = false
			}
		}
	}
	return matched && hasCondition
}

// DeviceProfile 设备画像 - 统一的厂商/款型配置
type DeviceProfile struct {
	Vendor          string           `json:"vendor"`             // 厂商标识
	Name            string           `json:"name"`               // 厂商名称
	TopologyEnabled bool             `json:"topologyEnabled"`    // 是否可用于网络拓扑采集（主机/通用设为 false）
	Selector        *ProfileSelector `json:"selector,omitempty"` // 细分款型/系列匹配器（主画像为 nil）
	PTY             PTYConfig        `json:"pty"`                // PTY 配置
	Prompt          PromptConfig     `json:"prompt"`             // 提示符配置
	Pager           PagerConfig      `json:"pager"`              // 分页配置
	Init            InitConfig       `json:"init"`               // 初始化配置
	Commands        []CommandSpec    `json:"commands"`           // 命令列表
}

// ResolveCommands 根据设备 Model 和 Version 动态过滤生效命令
func (p *DeviceProfile) ResolveCommands(model, version string) []CommandSpec {
	if len(p.Commands) == 0 {
		return nil
	}
	result := make([]CommandSpec, 0, len(p.Commands))
	for _, cmd := range p.Commands {
		if cmd.AppliesWhen == nil || cmd.AppliesWhen.Match(model, version) {
			result = append(result, cmd)
		}
	}
	return result
}

// deviceProfileRegistry 设备画像注册表，支持多档画像与全局兜底
type deviceProfileRegistry struct {
	profiles       map[string][]*DeviceProfile // vendor -> 多画像列表
	defaultProfile *DeviceProfile              // global:default 全局保守兜底画像
}

// 全局画像注册表
var globalRegistry = &deviceProfileRegistry{
	profiles: make(map[string][]*DeviceProfile),
}

func (r *deviceProfileRegistry) register(p *DeviceProfile) {
	if p == nil {
		return
	}
	if p.Selector != nil {
		p.Selector.CompileRegex()
	}
	for i := range p.Commands {
		if p.Commands[i].AppliesWhen != nil {
			p.Commands[i].AppliesWhen.CompileRegex()
		}
	}
	v := strings.ToLower(strings.TrimSpace(p.Vendor))
	r.profiles[v] = append([]*DeviceProfile{p}, r.profiles[v]...)
}

func init() {
	// 初始化厂商与通用画像
	registerVendorProfiles()
}

// registerVendorProfiles 注册厂商画像
func registerVendorProfiles() {
	// 1. Huawei 画像（网络设备，TopologyEnabled = true）
	globalRegistry.register(&DeviceProfile{
		Vendor:          "huawei",
		Name:            "华为",
		TopologyEnabled: true,
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
			{Command: "display patch-information", CommandKey: "patch_info", TimeoutSec: 20},
			{Command: "display current-configuration | include sysname", CommandKey: "sysname", TimeoutSec: 20},
			{Command: "display lldp neighbor", CommandKey: "lldp_neighbor", TimeoutSec: 60},
			{Command: "display interface brief", CommandKey: "interface_brief", TimeoutSec: 30},
			{Command: "display interface", CommandKey: "interface_detail", TimeoutSec: 60},
			{Command: "display eth-trunk", CommandKey: "eth_trunk", TimeoutSec: 30},
			{Command: "display arp", CommandKey: "arp_all", TimeoutSec: 60},
			{Command: "display mac-address", CommandKey: "mac_address", TimeoutSec: 60},
		},
	})

	// 2. H3C 画像（网络设备，TopologyEnabled = true）
	globalRegistry.register(&DeviceProfile{
		Vendor:          "h3c",
		Name:            "华三",
		TopologyEnabled: true,
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
			{Command: "display link-aggregation verbose", CommandKey: "eth_trunk", TimeoutSec: 30},
			{Command: "display arp all", CommandKey: "arp_all", TimeoutSec: 60},
			{Command: "display mac-address", CommandKey: "mac_address", TimeoutSec: 60},
		},
	})

	// 3. Cisco 画像（网络设备，TopologyEnabled = true）
	globalRegistry.register(&DeviceProfile{
		Vendor:          "cisco",
		Name:            "思科",
		TopologyEnabled: true,
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
			{Command: "show etherchannel summary", CommandKey: "eth_trunk", TimeoutSec: 30},
			{Command: "show ip arp", CommandKey: "arp_all", TimeoutSec: 60},
			{Command: "show mac address-table", CommandKey: "mac_address", TimeoutSec: 60},
		},
	})

	// 4. Linux 画像（主机设备，TopologyEnabled = false）
	globalRegistry.register(&DeviceProfile{
		Vendor:          "linux",
		Name:            "Linux主机",
		TopologyEnabled: false,
		PTY: PTYConfig{
			TermType: "xterm",
			Width:    256,
			Height:   200,
			EchoMode: 0,
			ISpeed:   14400,
			OSpeed:   14400,
		},
		Prompt: PromptConfig{
			Suffixes: []string{"$", "#"},
			Patterns: []string{`[\w\-]+@[\w\-]+:[^#$]*[#$]`, `[#$]`},
		},
		Pager: PagerConfig{
			Patterns: []string{
				"--More--",
				"(END)",
			},
			ContinueBytes: []byte{' '},
		},
		Init: InitConfig{
			DisablePagerCommands: []string{"export PAGER=cat"},
			ExtraCommands:        []string{},
			PromptTimeoutSec:     15,
		},
		Commands: []CommandSpec{
			{Command: "uname -a", CommandKey: "version", TimeoutSec: 15},
			{Command: "cat /etc/os-release", CommandKey: "os_release", TimeoutSec: 15},
			{Command: "hostname", CommandKey: "sysname", TimeoutSec: 10},
			{Command: "ip addr", CommandKey: "interface_brief", TimeoutSec: 20},
			{Command: "ip route", CommandKey: "route_table", TimeoutSec: 20},
		},
	})

	// 5. Generic 通用画像（TopologyEnabled = false）
	globalRegistry.register(&DeviceProfile{
		Vendor:          "generic",
		Name:            "通用设备",
		TopologyEnabled: false,
		PTY: PTYConfig{
			TermType: "vt100",
			Width:    256,
			Height:   200,
			EchoMode: 0,
			ISpeed:   14400,
			OSpeed:   14400,
		},
		Prompt: PromptConfig{
			Suffixes: []string{">", "#", "$", "]"},
			Patterns: []string{`[A-Za-z0-9_\-\.()\[\]<>]+\s*[>#$\]]`},
		},
		Pager: PagerConfig{
			Patterns: []string{
				"---- More ----",
				"--More--",
				"---- More",
				"More:",
			},
			ContinueBytes: []byte{' '},
		},
		Init: InitConfig{
			DisablePagerCommands: []string{},
			ExtraCommands:        []string{},
			PromptTimeoutSec:     30,
		},
		Commands: []CommandSpec{},
	})

	// 6. global:default 全局保守默认画像（未注册厂商统一安全兜底，TopologyEnabled = false）
	defaultProfile := &DeviceProfile{
		Vendor:          "default",
		Name:            "默认兜底画像",
		TopologyEnabled: false,
		PTY:             DefaultPTYConfig(),
		Prompt: PromptConfig{
			Suffixes:        []string{">", "#", "$", "]"},
			Patterns:        []string{`[A-Za-z0-9_\-\.()\[\]<>]+\s*[>#$\]]`},
			ConfirmPatterns: []string{},
			ConfirmPolicy:   "ask_user",
		},
		Pager:    DefaultPagerConfig(),
		Init:     DefaultInitConfig(),
		Commands: []CommandSpec{},
	}
	globalRegistry.defaultProfile = defaultProfile
	globalRegistry.register(defaultProfile)

	// 5. 加载内嵌细分画像 (Level 1/2 profiles/*.json)
	loadEmbeddedProfiles()
}

// loadEmbeddedProfiles 加载 embedded profiles/*.json 细分画像
func loadEmbeddedProfiles() {
	entries, err := embeddedProfilesFS.ReadDir("profiles")
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := embeddedProfilesFS.ReadFile("profiles/" + entry.Name())
		if err != nil {
			logger.Warn("DeviceProfile", "", "读取内嵌画像文件失败: %s, %v", entry.Name(), err)
			continue
		}
		var p DeviceProfile
		if err := json.Unmarshal(data, &p); err != nil {
			logger.Warn("DeviceProfile", "", "解析内嵌画像 JSON 失败: %s, %v", entry.Name(), err)
			continue
		}
		globalRegistry.register(&p)
		logger.Verbose("DeviceProfile", "", "成功载入内嵌细分画像: %s (vendor=%s, name=%s)", entry.Name(), p.Vendor, p.Name)
	}
}

// ResetProfilesForTest 仅用于单测备份并重置画像注册表，返回恢复函数
func ResetProfilesForTest() func() {
	snapshot := make(map[string][]*DeviceProfile, len(globalRegistry.profiles))
	for k, list := range globalRegistry.profiles {
		cp := make([]*DeviceProfile, len(list))
		copy(cp, list)
		snapshot[k] = cp
	}
	defaultProfile := globalRegistry.defaultProfile

	return func() {
		globalRegistry.profiles = snapshot
		globalRegistry.defaultProfile = defaultProfile
	}
}

// ResolveProfile 按照四级回退解析设备画像：
// 1. 精确款型/版本匹配 (exact:<model>)
// 2. 系列匹配 (series:<series>)
// 3. 厂商默认画像 (vendor:<vendor>)
// 4. 全局保守兜底画像 (global:default)
func ResolveProfile(vendor, model, version string) (*DeviceProfile, string) {
	v := strings.ToLower(strings.TrimSpace(vendor))
	list := globalRegistry.profiles[v]

	// 1. 精确匹配 (Selector 中的 ModelPattern 或 VersionPattern)
	if (model != "" || version != "") && len(list) > 0 {
		for _, p := range list {
			if p.Selector == nil {
				continue
			}
			if p.Selector.Match(model, version) {
				return p, "exact:" + model
			}
		}
	}

	// 2. 系列匹配 (Series)
	if model != "" && len(list) > 0 {
		series := device.ConvertSeries(model)
		if series != "" {
			for _, p := range list {
				if p.Selector != nil && strings.EqualFold(p.Selector.Series, series) {
					return p, "series:" + series
				}
			}
		}
	}

	// 3. 厂商默认画像 (Selector == nil 的主画像)
	if len(list) > 0 {
		for _, p := range list {
			if p.Selector == nil {
				return p, "vendor:" + v
			}
		}
		return list[0], "vendor:" + v
	}

	// 4. 全局保守兜底画像 (global:default)
	logger.Warn("DeviceProfile", "", "未注册厂商或无可用画像: vendor=%s, model=%s, 回退全局保守画像", vendor, model)
	if globalRegistry.defaultProfile != nil {
		return globalRegistry.defaultProfile, "global:default"
	}
	if genList := globalRegistry.profiles["generic"]; len(genList) > 0 {
		return genList[0], "global:default"
	}
	return nil, "none"
}

// GetDeviceProfile 根据厂商获取设备主画像；未知厂商时报警并回退全局保守画像
func GetDeviceProfile(vendor string) *DeviceProfile {
	v := strings.ToLower(strings.TrimSpace(vendor))
	if list, ok := globalRegistry.profiles[v]; ok && len(list) > 0 {
		for _, p := range list {
			if p.Selector == nil {
				return p
			}
		}
		return list[0]
	}
	logger.Warn("DeviceProfile", "", "未注册厂商画像: %s, 回退全局保守画像", vendor)
	if globalRegistry.defaultProfile != nil {
		return globalRegistry.defaultProfile
	}
	if genList := globalRegistry.profiles["generic"]; len(genList) > 0 {
		return genList[0]
	}
	return nil
}

// GetDeviceProfileByVendor 根据厂商获取设备画像，返回是否存在
func GetDeviceProfileByVendor(vendor string) (*DeviceProfile, bool) {
	v := strings.ToLower(strings.TrimSpace(vendor))
	if list, ok := globalRegistry.profiles[v]; ok && len(list) > 0 {
		for _, p := range list {
			if p.Selector == nil {
				return p, true
			}
		}
		return list[0], true
	}
	return nil, false
}

// GetAllDeviceProfiles 获取所有主画像列表
func GetAllDeviceProfiles() []*DeviceProfile {
	profiles := make([]*DeviceProfile, 0, len(globalRegistry.profiles))
	for _, list := range globalRegistry.profiles {
		for _, p := range list {
			if p.Selector == nil {
				profiles = append(profiles, p)
				break
			}
		}
	}
	return profiles
}

// RegisterDeviceProfile 注册或追加设备画像（可用于测试或扩展）
func RegisterDeviceProfile(profile *DeviceProfile) {
	if profile != nil {
		globalRegistry.register(profile)
	}
}
