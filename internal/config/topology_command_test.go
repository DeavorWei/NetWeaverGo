package config

import (
	"testing"

	"github.com/NetWeaverGo/core/internal/models"
)

// P1-7：Scene 必须随规则落库，且同字段多场景不得被去重吞掉
func TestNormalizeTopologyVendorFieldCommands_PreservesScene(t *testing.T) {
	items := []models.TopologyVendorFieldCommand{
		{FieldKey: "lldp_neighbor", Scene: "enterprise", Command: "display lldp neighbor brief", Enabled: true},
		{FieldKey: "lldp_neighbor", Scene: "dc", Command: "display lldp neighbor", Enabled: true},
		{FieldKey: "lldp_neighbor", Scene: "enterprise", Command: "duplicated", Enabled: true},
		{FieldKey: "", Scene: "enterprise", Command: "ignored"},
	}
	out := normalizeTopologyVendorFieldCommands("huawei", items)
	if len(out) != 2 {
		t.Fatalf("同字段多场景应保留 2 条（去重按 fieldKey+scene），当前 %d: %+v", len(out), out)
	}
	for _, item := range out {
		if item.Scene == "" {
			t.Fatalf("Scene 不得丢失: %+v", item)
		}
		if item.Vendor != "huawei" {
			t.Fatalf("Vendor 应归一为 huawei: %+v", item)
		}
	}
	if out[0].Scene == out[1].Scene {
		t.Fatalf("两条记录场景应不同: %+v", out)
	}
}
