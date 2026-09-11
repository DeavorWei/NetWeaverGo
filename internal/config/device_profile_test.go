package config

import (
	"strings"
	"testing"
)

// TestPTYConfig 测试 PTY 配置
func TestPTYConfig(t *testing.T) {
	cfg := DefaultPTYConfig()

	if cfg.TermType != "vt100" {
		t.Errorf("TermType = %q, want %q", cfg.TermType, "vt100")
	}
	if cfg.Width != 256 {
		t.Errorf("Width = %d, want %d", cfg.Width, 256)
	}
	if cfg.Height != 200 {
		t.Errorf("Height = %d, want %d", cfg.Height, 200)
	}
	if cfg.EchoMode != 0 {
		t.Errorf("EchoMode = %d, want %d", cfg.EchoMode, 0)
	}
}

// TestPromptConfig 测试提示符配置
func TestPromptConfig(t *testing.T) {
	cfg := DefaultPromptConfig()

	if len(cfg.Suffixes) != 3 {
		t.Errorf("Suffixes count = %d, want %d", len(cfg.Suffixes), 3)
	}

	expectedSuffixes := []string{">", "#", "]"}
	for i, suffix := range expectedSuffixes {
		if i < len(cfg.Suffixes) && cfg.Suffixes[i] != suffix {
			t.Errorf("Suffixes[%d] = %q, want %q", i, cfg.Suffixes[i], suffix)
		}
	}
}

// TestPagerConfig 测试分页配置
func TestPagerConfig(t *testing.T) {
	cfg := DefaultPagerConfig()

	if len(cfg.Patterns) == 0 {
		t.Error("Patterns should not be empty")
	}

	if len(cfg.ContinueBytes) != 1 || cfg.ContinueBytes[0] != ' ' {
		t.Errorf("ContinueBytes = %v, want [' ']", cfg.ContinueBytes)
	}
}

// TestInitConfig 测试初始化配置
func TestInitConfig(t *testing.T) {
	cfg := DefaultInitConfig()

	if cfg.PromptTimeoutSec != 30 {
		t.Errorf("PromptTimeoutSec = %d, want %d", cfg.PromptTimeoutSec, 30)
	}
}

// TestGetDeviceProfile 测试获取设备画像
func TestGetDeviceProfile(t *testing.T) {
	// 测试获取华为画像
	huawei := GetDeviceProfile("huawei")
	if huawei == nil {
		t.Fatal("huawei profile should not be nil")
	}
	if huawei.Vendor != "huawei" {
		t.Errorf("Vendor = %q, want %q", huawei.Vendor, "huawei")
	}
	if huawei.Name != "华为" {
		t.Errorf("Name = %q, want %q", huawei.Name, "华为")
	}

	// 测试获取 H3C 画像
	h3c := GetDeviceProfile("h3c")
	if h3c == nil {
		t.Fatal("h3c profile should not be nil")
	}
	if h3c.Vendor != "h3c" {
		t.Errorf("Vendor = %q, want %q", h3c.Vendor, "h3c")
	}

	// 测试获取 Cisco 画像
	cisco := GetDeviceProfile("cisco")
	if cisco == nil {
		t.Fatal("cisco profile should not be nil")
	}
	if cisco.Vendor != "cisco" {
		t.Errorf("Vendor = %q, want %q", cisco.Vendor, "cisco")
	}

	// 测试获取 Linux 画像
	linux := GetDeviceProfile("linux")
	if linux == nil {
		t.Fatal("linux profile should not be nil")
	}
	if linux.Vendor != "linux" {
		t.Errorf("Vendor = %q, want %q", linux.Vendor, "linux")
	}

	// 测试获取 Generic 通用画像
	generic := GetDeviceProfile("generic")
	if generic == nil {
		t.Fatal("generic profile should not be nil")
	}
	if generic.Vendor != "generic" {
		t.Errorf("Vendor = %q, want %q", generic.Vendor, "generic")
	}

	// 测试未知厂商返回全局保守兜底画像 (default)
	unknown := GetDeviceProfile("unknown")
	if unknown == nil {
		t.Fatal("unknown profile should return default conservative profile")
	}
	if unknown.Vendor != "default" {
		t.Errorf("Unknown vendor should return default, got %q", unknown.Vendor)
	}
	if unknown.TopologyEnabled {
		t.Errorf("Default conservative profile should have TopologyEnabled = false")
	}
}

// TestGetDeviceProfileByVendor 测试获取设备画像（带存在性检查）
func TestGetDeviceProfileByVendor(t *testing.T) {
	profile, ok := GetDeviceProfileByVendor("huawei")
	if !ok {
		t.Error("huawei profile should exist")
	}
	if profile == nil {
		t.Fatal("profile should not be nil")
	}

	_, ok = GetDeviceProfileByVendor("nonexistent")
	if ok {
		t.Error("nonexistent profile should not exist")
	}
}

// TestGetAllDeviceProfiles 测试获取所有设备画像
func TestGetAllDeviceProfiles(t *testing.T) {
	profiles := GetAllDeviceProfiles()

	if len(profiles) < 3 {
		t.Errorf("Profiles count = %d, want at least %d", len(profiles), 3)
	}

	vendors := make(map[string]bool)
	for _, p := range profiles {
		vendors[p.Vendor] = true
	}

	expectedVendors := []string{"huawei", "h3c", "cisco"}
	for _, v := range expectedVendors {
		if !vendors[v] {
			t.Errorf("Missing vendor: %s", v)
		}
	}
}

// TestHuaweiProfileInitCommands 测试华为初始化命令
func TestHuaweiProfileInitCommands(t *testing.T) {
	profile := GetDeviceProfile("huawei")

	if len(profile.Init.DisablePagerCommands) == 0 {
		t.Error("Huawei should have DisablePagerCommands")
	}

	found := false
	for _, cmd := range profile.Init.DisablePagerCommands {
		if cmd == "screen-length 0 temporary" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Huawei should have 'screen-length 0 temporary' command")
	}
}

// TestH3CProfileInitCommands 测试 H3C 初始化命令
func TestH3CProfileInitCommands(t *testing.T) {
	profile := GetDeviceProfile("h3c")

	if len(profile.Init.DisablePagerCommands) == 0 {
		t.Error("H3C should have DisablePagerCommands")
	}

	found := false
	for _, cmd := range profile.Init.DisablePagerCommands {
		if cmd == "screen-length disable" {
			found = true
			break
		}
	}
	if !found {
		t.Error("H3C should have 'screen-length disable' command")
	}
}

// TestCiscoProfileInitCommands 测试 Cisco 初始化命令
func TestCiscoProfileInitCommands(t *testing.T) {
	profile := GetDeviceProfile("cisco")

	if len(profile.Init.DisablePagerCommands) == 0 {
		t.Error("Cisco should have DisablePagerCommands")
	}

	found := false
	for _, cmd := range profile.Init.DisablePagerCommands {
		if cmd == "terminal length 0" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Cisco should have 'terminal length 0' command")
	}
}

// TestProfilePTYConfig 测试画像 PTY 配置
func TestProfilePTYConfig(t *testing.T) {
	profile := GetDeviceProfile("huawei")

	if profile.PTY.Width != 256 {
		t.Errorf("PTY.Width = %d, want %d", profile.PTY.Width, 256)
	}
	if profile.PTY.Height != 200 {
		t.Errorf("PTY.Height = %d, want %d", profile.PTY.Height, 200)
	}
	if profile.PTY.TermType != "vt100" {
		t.Errorf("PTY.TermType = %q, want %q", profile.PTY.TermType, "vt100")
	}
}

// TestProfilePromptPatterns 测试画像提示符正则模式
func TestProfilePromptPatterns(t *testing.T) {
	profile := GetDeviceProfile("huawei")

	if len(profile.Prompt.Patterns) == 0 {
		t.Error("Huawei should have PromptPatterns")
	}
}

// TestProfileCommands 测试画像命令列表
func TestProfileCommands(t *testing.T) {
	profile := GetDeviceProfile("huawei")

	if len(profile.Commands) == 0 {
		t.Error("Huawei should have commands")
	}

	// 检查是否有 version 命令
	found := false
	for _, cmd := range profile.Commands {
		if cmd.CommandKey == "version" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Huawei should have 'version' command")
	}
}

// TestResolveProfile_FourLevelFallback 测试四级回退解析
func TestResolveProfile_FourLevelFallback(t *testing.T) {
	restore := ResetProfilesForTest()
	t.Cleanup(restore)

	// 动态注册细分画像用于测试 Level 1 和 Level 2
	exactProfile := &DeviceProfile{
		Vendor:          "huawei",
		Name:            "华为 CE16800 定制画像",
		TopologyEnabled: true,
		Selector: &ProfileSelector{
			ModelPattern: `^CE168\d{2}$`,
		},
	}
	RegisterDeviceProfile(exactProfile)

	seriesProfile := &DeviceProfile{
		Vendor:          "huawei",
		Name:            "华为 S5700 系列画像",
		TopologyEnabled: true,
		Selector: &ProfileSelector{
			Series: "S5700",
		},
	}
	RegisterDeviceProfile(seriesProfile)

	// 1. Level 1: 精确款型匹配
	p1, match1 := ResolveProfile("huawei", "CE16804", "V200R019")
	if p1 != exactProfile {
		t.Errorf("Level 1 匹配错误: 期望 exactProfile, 实际 %s", p1.Name)
	}
	if match1 != "exact:CE16804" {
		t.Errorf("Level 1 matchPath 错误: %s", match1)
	}

	// 2. Level 2: 系列归一化匹配 (S5735-L 归一化为 S5700)
	p2, match2 := ResolveProfile("huawei", "S5735-L24P4S-A2", "V200R019")
	if p2 != seriesProfile {
		t.Errorf("Level 2 匹配错误: 期望 seriesProfile, 实际 %s", p2.Name)
	}
	if match2 != "series:S5700" {
		t.Errorf("Level 2 matchPath 错误: %s", match2)
	}

	// 3. Level 3: 厂商默认主画像 (AR6280 无细分画像，回退厂商主画像)
	p3, match3 := ResolveProfile("huawei", "AR6280", "V300R019")
	if p3 == nil || p3.Name != "华为" {
		t.Errorf("Level 3 匹配错误: 期望华为默认主画像, 实际 %v", p3)
	}
	if match3 != "vendor:huawei" {
		t.Errorf("Level 3 matchPath 错误: %s", match3)
	}

	// 4. Level 4: 全局保守兜底画像 (未知厂商)
	p4, match4 := ResolveProfile("ruijie_unknown", "RG-S5750", "V1.0")
	if p4 == nil || p4.Vendor != "default" {
		t.Errorf("Level 4 匹配错误: 期望 default 兜底画像, 实际 %v", p4)
	}
	if match4 != "global:default" {
		t.Errorf("Level 4 matchPath 错误: %s", match4)
	}
}

// TestTopologyEnabled_Isolation 测试网络拓扑与主机/通用画像隔离
func TestTopologyEnabled_Isolation(t *testing.T) {
	huawei := GetDeviceProfile("huawei")
	if !huawei.TopologyEnabled {
		t.Error("Huawei TopologyEnabled should be true")
	}

	h3c := GetDeviceProfile("h3c")
	if !h3c.TopologyEnabled {
		t.Error("H3C TopologyEnabled should be true")
	}

	cisco := GetDeviceProfile("cisco")
	if !cisco.TopologyEnabled {
		t.Error("Cisco TopologyEnabled should be true")
	}

	linux := GetDeviceProfile("linux")
	if linux.TopologyEnabled {
		t.Error("Linux TopologyEnabled must be false (host profile)")
	}

	generic := GetDeviceProfile("generic")
	if generic.TopologyEnabled {
		t.Error("Generic TopologyEnabled must be false")
	}

	def := GetDeviceProfile("default")
	if def.TopologyEnabled {
		t.Error("Default fallback TopologyEnabled must be false")
	}
}

// TestResolveCommands_AppliesWhen 测试命令条件过滤
func TestResolveCommands_AppliesWhen(t *testing.T) {
	profile := &DeviceProfile{
		Vendor: "test_vendor",
		Commands: []CommandSpec{
			{Command: "display version", CommandKey: "version"},
			{
				Command:    "display ce-specific-info",
				CommandKey: "ce_info",
				AppliesWhen: &CommandCondition{
					ModelPattern: `^CE\d+`,
				},
			},
			{
				Command:    "display v2-only-info",
				CommandKey: "v2_info",
				AppliesWhen: &CommandCondition{
					VersionPattern: `^V200`,
				},
			},
		},
	}

	// 1. CE 款型 + V200 版本 -> 3条全出
	cmds1 := profile.ResolveCommands("CE6866", "V200R019")
	if len(cmds1) != 3 {
		t.Errorf("期望 3 条命令, 实际 %d", len(cmds1))
	}

	// 2. S 系列 + V200 版本 -> 2条 (version, v2_info)
	cmds2 := profile.ResolveCommands("S5735", "V200R019")
	if len(cmds2) != 2 {
		t.Errorf("期望 2 条命令, 实际 %d", len(cmds2))
	}

	// 3. S 系列 + V300 版本 -> 仅 1条 (version)
	cmds3 := profile.ResolveCommands("S5735", "V300R019")
	if len(cmds3) != 1 {
		t.Errorf("期望 1 条命令, 实际 %d", len(cmds3))
	}
}

// TestEmbeddedProfiles_Loaded 验证内嵌 JSON 细分画像已成功注册并可匹配
func TestEmbeddedProfiles_Loaded(t *testing.T) {
	// 1. 验证 CE 细分画像
	ceProfile, matchPath := ResolveProfile("huawei", "CE6866", "V200R005")
	if ceProfile == nil {
		t.Fatal("未命中 CE 画像")
	}
	if !strings.HasPrefix(matchPath, "exact:") && !strings.HasPrefix(matchPath, "series:") {
		t.Errorf("CE 画像应命中 exact 或 series，实际 matchPath: %s", matchPath)
	}

	// 2. 验证 S 系列细分画像
	sProfile, sMatch := ResolveProfile("huawei", "S5735", "V200R019")
	if sProfile == nil {
		t.Fatal("未命中 S 系列画像")
	}
	if !strings.HasPrefix(sMatch, "exact:") && !strings.HasPrefix(sMatch, "series:") {
		t.Errorf("S 系列画像应命中 exact 或 series，实际 matchPath: %s", sMatch)
	}
}


