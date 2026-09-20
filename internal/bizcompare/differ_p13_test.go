package bizcompare

import (
	"errors"
	"testing"
)

// P1-3：设备级采集失败时必须跳过比对，不得产出假差异
func TestDiffer_SkipFailedSnapshot(t *testing.T) {
	d := NewDiffer()

	before := NewDeviceSnapshot("r1", "10.0.0.1", "S", "s1", "before")
	before.AddItem("cmd.a", "c", "v1")

	after := NewDeviceSnapshot("r2", "10.0.0.1", "S", "s1", "after")
	after.AddItem("cmd.a", "c", "v2")
	after.MarkFailed(errors.New("connect timeout"))

	if diffs := d.CompareSnapshots(1, before, after); len(diffs) != 0 {
		t.Fatalf("失败快照不应产出差异，实际 %d 条: %+v", len(diffs), diffs)
	}

	after.Status = SnapshotStatusOK
	after.Error = ""
	diffs := d.CompareSnapshots(1, before, after)
	if len(diffs) != 1 || diffs[0].DiffType != "modified" {
		t.Fatalf("恢复正常后应产出 1 条 modified 差异，实际: %+v", diffs)
	}
}

// P1-3：数值漂移阈值可配置
func TestDiffer_DriftThreshold(t *testing.T) {
	mk := func(v string) *DeviceSnapshot {
		s := NewDeviceSnapshot("r", "10.0.0.1", "S", "s1", "before")
		s.AddItem("cmd.value", "c", v)
		return s
	}
	d := NewDiffer()
	d.DriftThreshold = 10

	diffs := d.CompareSnapshots(1, mk("100"), mk("105"))
	if len(diffs) != 1 || diffs[0].DiffType != "modified" {
		t.Fatalf("低于漂移阈值应判定 modified: %+v", diffs)
	}

	diffs = d.CompareSnapshots(1, mk("100"), mk("120"))
	if len(diffs) != 1 || diffs[0].DiffType != "drift" {
		t.Fatalf("超过漂移阈值应判定 drift: %+v", diffs)
	}
}
