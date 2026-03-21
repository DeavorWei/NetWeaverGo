package config

import (
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

	// 测试未知厂商返回默认
	unknown := GetDeviceProfile("unknown")
	if unknown == nil {
		t.Fatal("unknown profile should return default (huawei)")
	}
	if unknown.Vendor != "huawei" {
		t.Errorf("Unknown vendor should return huawei, got %q", unknown.Vendor)
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

// TestDetectVendorFromOutput 测试厂商探测
func TestDetectVendorFromOutput(t *testing.T) {
	tests := []struct {
		name     string
		prompt   string
		banner   string
		expected string
	}{
		{
			name:     "Huawei prompt",
			prompt:   "<SW1>",
			banner:   "",
			expected: "huawei",
		},
		{
			name:     "Huawei banner",
			prompt:   "",
			banner:   "Huawei Versatile Routing Platform",
			expected: "huawei",
		},
		{
			name:     "H3C prompt",
			prompt:   "H3C-SW1#",
			banner:   "",
			expected: "h3c",
		},
		{
			name:     "H3C banner",
			prompt:   "",
			banner:   "H3C Comware Software",
			expected: "h3c",
		},
		{
			name:     "Cisco prompt",
			prompt:   "Cisco-SW1#",
			banner:   "",
			expected: "cisco",
		},
		{
			name:     "Cisco banner",
			prompt:   "",
			banner:   "Cisco IOS Software",
			expected: "cisco",
		},
		{
			name:     "Unknown",
			prompt:   "switch>",
			banner:   "",
			expected: "huawei", // 默认返回华为
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectVendorFromOutput(tt.prompt, tt.banner)
			if result != tt.expected {
				t.Errorf("DetectVendorFromOutput(%q, %q) = %q, want %q",
					tt.prompt, tt.banner, result, tt.expected)
			}
		})
	}
}

// TestProfileProvider 测试画像提供者接口
func TestProfileProvider(t *testing.T) {
	provider := NewProfileProvider()

	profile, ok := provider.GetByVendor("huawei")
	if !ok {
		t.Error("GetByVendor(huawei) should return true")
	}
	if profile == nil {
		t.Fatal("profile should not be nil")
	}

	_, ok = provider.GetByVendor("nonexistent")
	if ok {
		t.Error("GetByVendor(nonexistent) should return false")
	}

	// 测试 fallback 探测
	vendor := provider.DetectFallback("<SW1>", "")
	if vendor != "huawei" {
		t.Errorf("DetectFallback = %q, want %q", vendor, "huawei")
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
