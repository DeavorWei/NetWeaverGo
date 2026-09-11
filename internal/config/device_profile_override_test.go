package config

import (
	"testing"

	"github.com/NetWeaverGo/core/internal/models"
)

func TestBuildGlobAlternation(t *testing.T) {
	cases := []struct {
		name string
		in   []string
		want string
	}{
		{"无通配按前缀", []string{"S5700", "S5735"}, `^(?:S5700.*|S5735.*)$`},
		{"尾部通配", []string{"S57*"}, `^(?:S57.*)$`},
		{"版本通配", []string{"V200R019*"}, `^(?:V200R019.*)$`},
		{"全通配忽略", []string{"*"}, ``},
		{"空列表", nil, ``},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := buildGlobAlternation(c.in); got != c.want {
				t.Fatalf("buildGlobAlternation(%v) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestBuildProfileFromRecord_SelectorAndVendor(t *testing.T) {
	rec := &models.DeviceProfileRecord{
		ID:           1,
		Vendor:       "ruijie",
		ModelsJSON:   `["RG-S57*","RG-S29"]`,
		VersionsJSON: `["RGOS 11*"]`,
		ProfileJSON:  `{"name":"锐捷自定义","topologyEnabled":true,"commands":[{"command":"show version","commandKey":"version","timeoutSec":20}]}`,
	}

	p := buildProfileFromRecord(rec)
	if p == nil {
		t.Fatal("buildProfileFromRecord 返回 nil")
	}
	if p.Vendor != "ruijie" {
		t.Fatalf("Vendor = %q, want ruijie", p.Vendor)
	}
	if p.Name != "锐捷自定义" {
		t.Fatalf("Name = %q, want 锐捷自定义", p.Name)
	}
	if !p.TopologyEnabled {
		t.Fatal("TopologyEnabled 期望 true")
	}
	if len(p.Commands) != 1 || p.Commands[0].Command != "show version" {
		t.Fatalf("命令列表未按 ProfileJSON 还原: %+v", p.Commands)
	}
	if p.Selector == nil {
		t.Fatal("Selector 未组装")
	}
	if !p.Selector.Match("RG-S5750", "RGOS 11.4") {
		t.Fatalf("Selector 未命中 RG-S5750/RGOS 11.4, model=%s version=%s", p.Selector.ModelPattern, p.Selector.VersionPattern)
	}
	if p.Selector.Match("RG-S2910", "RGOS 12.0") {
		t.Fatal("Selector 不应命中版本不匹配的 RG-S2910/RGOS 12.0")
	}
}

func TestLoadDeviceProfileOverrides_NilDB(t *testing.T) {
	if err := LoadDeviceProfileOverrides(nil); err != nil {
		t.Fatalf("nil DB 应安全返回 nil, got %v", err)
	}
}
