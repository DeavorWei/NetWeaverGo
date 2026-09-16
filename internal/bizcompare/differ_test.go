package bizcompare

import (
	"testing"
)

func TestSceneManager_ListByDomain(t *testing.T) {
	mgr := GetGlobalSceneManager()

	sScenes := mgr.ListByDomain("S")
	if len(sScenes) == 0 {
		t.Errorf("Domain S 场景列表不应为空")
	}

	neScenes := mgr.ListByDomain("NE-SR")
	if len(neScenes) == 0 {
		t.Errorf("Domain NE-SR 场景列表不应为空")
	}

	ceScenes := mgr.ListByDomain("CE")
	if len(ceScenes) == 0 {
		t.Errorf("Domain CE 场景列表不应为空")
	}
}

func TestDiffer_CompareSnapshots(t *testing.T) {
	before := NewDeviceSnapshot("run-before", "10.0.0.1", "NE-SR", "ne_core_router", "before")
	before.AddItem("GE0/0/1.status", "interface", "UP")
	before.AddItem("GE0/0/2.status", "interface", "UP")
	before.AddItem("bgp.peer.10.1.1.2", "route", "Established")
	before.AddItem("bgp.peer.10.1.1.3", "route", "Established")
	before.AddItem("arp.count", "arp", "120")

	after := NewDeviceSnapshot("run-after", "10.0.0.1", "NE-SR", "ne_core_router", "after")
	// GE0/0/1 保持 UP，无变化
	after.AddItem("GE0/0/1.status", "interface", "UP")
	// GE0/0/2 变为 DOWN (重大故障)
	after.AddItem("GE0/0/2.status", "interface", "DOWN")
	// bgp 10.1.1.2 变为 Active (BGP中断)
	after.AddItem("bgp.peer.10.1.1.2", "route", "Active")
	// bgp 10.1.1.3 丢失 (被删除)
	// 新增一条路由对等体
	after.AddItem("bgp.peer.10.1.1.4", "route", "Established")
	// ARP 条目漂移 (120 -> 135)
	after.AddItem("arp.count", "arp", "135")

	differ := NewDiffer()
	diffs := differ.CompareSnapshots(1, before, after)

	if len(diffs) != 5 {
		t.Fatalf("预期 5 项差异，实际 %d 项", len(diffs))
	}

	foundDown := false
	foundBgpCut := false
	foundDeleted := false
	foundAdded := false
	foundDrift := false

	for _, d := range diffs {
		if d.ItemKey == "GE0/0/2.status" {
			foundDown = true
			if d.ImpactLevel != "critical" {
				t.Errorf("接口 UP->DOWN 应为 critical，实际为 %s", d.ImpactLevel)
			}
		}
		if d.ItemKey == "bgp.peer.10.1.1.2" {
			foundBgpCut = true
			if d.ImpactLevel != "critical" {
				t.Errorf("BGP中断应为 critical，实际为 %s", d.ImpactLevel)
			}
		}
		if d.ItemKey == "bgp.peer.10.1.1.3" && d.DiffType == "deleted" {
			foundDeleted = true
		}
		if d.ItemKey == "bgp.peer.10.1.1.4" && d.DiffType == "added" {
			foundAdded = true
		}
		if d.ItemKey == "arp.count" && d.DiffType == "drift" {
			foundDrift = true
		}
	}

	if !foundDown || !foundBgpCut || !foundDeleted || !foundAdded || !foundDrift {
		t.Errorf("未全部命中预期的各类差异: down=%v, bgpCut=%v, deleted=%v, added=%v, drift=%v",
			foundDown, foundBgpCut, foundDeleted, foundAdded, foundDrift)
	}

	summary := FormatDiffSummary(diffs)
	if summary == "" {
		t.Errorf("汇总信息不应为空")
	}
}

func TestSnapshotStore_SaveAndGet(t *testing.T) {
	store := GetGlobalSnapshotStore()

	snap := NewDeviceSnapshot("test-run-1", "192.168.10.1", "S", "s_campus_switch", "before")
	snap.AddItem("vlan.10", "vlan", "active")

	if err := store.SaveSnapshot(snap); err != nil {
		t.Fatalf("SaveSnapshot 失败: %v", err)
	}

	got, err := store.GetSnapshot("test-run-1", "192.168.10.1")
	if err != nil {
		t.Fatalf("GetSnapshot 失败: %v", err)
	}
	if got.DeviceIP != "192.168.10.1" {
		t.Errorf("DeviceIP = %s, want 192.168.10.1", got.DeviceIP)
	}
	if got.Items["vlan.10"].Value != "active" {
		t.Errorf("vlan.10 value = %s, want active", got.Items["vlan.10"].Value)
	}
}
