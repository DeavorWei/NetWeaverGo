package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// fallbackCase 画像四级回退用例（数据来自 testdata/device/fallback_cases.json）
type fallbackCase struct {
	Vendor          string `json:"vendor"`
	Model           string `json:"model"`
	Version         string `json:"version"`
	ExpectMatchPath string `json:"expectMatchPath"`
	ExpectVendor    string `json:"expectVendor"`
	Note            string `json:"note"`
}

// TestResolveProfile_FourLevelFallbackCases 依据样本用例验证四级回退与"未注册厂商不误套华为画像"缺陷修复
func TestResolveProfile_FourLevelFallbackCases(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "device", "fallback_cases.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取回退用例失败: %v", err)
	}

	var cases []fallbackCase
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatalf("解析回退用例失败: %v", err)
	}
	if len(cases) == 0 {
		t.Fatal("回退用例为空")
	}

	for _, c := range cases {
		t.Run(c.Vendor+"_"+c.Model, func(t *testing.T) {
			profile, matchPath := ResolveProfile(c.Vendor, c.Model, c.Version)
			if profile == nil {
				t.Fatalf("未解析到任何画像（用例: %s）", c.Note)
			}
			if matchPath != c.ExpectMatchPath {
				t.Errorf("matchPath = %q, want %q（用例: %s）", matchPath, c.ExpectMatchPath, c.Note)
			}
			if profile.Vendor != c.ExpectVendor {
				t.Errorf("profile.Vendor = %q, want %q（用例: %s）", profile.Vendor, c.ExpectVendor, c.Note)
			}
		})
	}
}

// TestResolveProfile_UnregisteredVendorNeverFallsBackToHuawei 关键缺陷回归：
// 未注册厂商必须回退全局 default，绝不能误用华为画像。
func TestResolveProfile_UnregisteredVendorNeverFallsBackToHuawei(t *testing.T) {
	for _, vendor := range []string{"ruijie", "zte", "maipu", "vendor-not-exist"} {
		profile, matchPath := ResolveProfile(vendor, "SOME-MODEL", "1.0")
		if profile == nil {
			t.Fatalf("厂商 %s 应回退到全局兜底画像，实际为 nil", vendor)
		}
		if profile.Vendor == "huawei" {
			t.Fatalf("厂商 %s 不得误套华为画像", vendor)
		}
		if matchPath != "global:default" {
			t.Errorf("厂商 %s 的 matchPath = %q, want global:default", vendor, matchPath)
		}
	}
}
