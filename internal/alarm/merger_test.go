package alarm

import (
	"testing"
	"time"

	"github.com/NetWeaverGo/core/internal/models"
)

func TestRuleRegistry_BuiltinAndMatch(t *testing.T) {
	registry := GetDefaultRegistry()
	if registry == nil {
		t.Fatalf("DefaultRegistry should not be nil")
	}

	// 1. 测试 CE 系列规则匹配
	ceEcho := "Error: Power module 1 is faulty on slot 0"
	hits := registry.MatchLine("CE", ceEcho)
	if len(hits) == 0 {
		t.Errorf("expected CE power fail rule match, got none")
	} else if hits[0].AlarmName != "CE_POWER_FAIL" {
		t.Errorf("expected CE_POWER_FAIL, got %s", hits[0].AlarmName)
	}

	// 2. 测试 Router 系列规则匹配
	routerEcho := "%%Sep 16 10:00:00 2026 ROUTER %%01OSPF/2/NBR_CHANGE: Neighbor 10.1.1.1 status changed to Down"
	routerHits := registry.MatchLine("Router", routerEcho)
	if len(routerHits) == 0 {
		t.Errorf("expected Router OSPF down rule match, got none")
	} else if routerHits[0].AlarmName != "ROUTER_OSPF_NBR_DOWN" {
		t.Errorf("expected ROUTER_OSPF_NBR_DOWN, got %s", routerHits[0].AlarmName)
	}

	// 3. 测试 USG 防火墙规则匹配
	usgEcho := "HRP heartbeat link is down, dual-system switchover triggered"
	usgHits := registry.MatchLine("USG", usgEcho)
	if len(usgHits) == 0 {
		t.Errorf("expected USG HRP heartbeat lost rule match, got none")
	} else if usgHits[0].AlarmName != "USG_HRP_HEARTBEAT_LOST" {
		t.Errorf("expected USG_HRP_HEARTBEAT_LOST, got %s", usgHits[0].AlarmName)
	}

	// 4. 测试通用底座规则跨族继承 (COMMON)
	tempEcho := "High temperature warning: CPU temp 92C"
	sHits := registry.MatchLine("S", tempEcho)
	if len(sHits) == 0 {
		t.Errorf("expected S switch to match COMMON temperature alarm, got none")
	} else if sHits[0].AlarmName != "TEMP_OVER_THRESHOLD" {
		t.Errorf("expected TEMP_OVER_THRESHOLD, got %s", sHits[0].AlarmName)
	}
}

func TestAlarmMerger_OpticalAndBGP(t *testing.T) {
	merger := NewAlarmMerger()
	now := time.Now()

	records := []models.AlarmRecord{
		{
			ID:         1,
			RunID:      "run-001",
			DeviceIP:   "192.168.1.1",
			Family:     "CE",
			AlarmName:  "CE_OPTICAL_LOS",
			Severity:   "major",
			Category:   "interface",
			Summary:    "Optical module 100GE1/0/1 RX power too low",
			OccurredAt: now,
		},
		{
			ID:         2,
			RunID:      "run-001",
			DeviceIP:   "192.168.1.1",
			Family:     "CE",
			AlarmName:  "CE_BGP_PEER_DOWN",
			Severity:   "critical",
			Category:   "routing",
			Summary:    "BGP peer 10.10.1.2 state changed to DOWN",
			OccurredAt: now.Add(2 * time.Second),
		},
	}

	phenomena := merger.Merge(records)
	if len(phenomena) != 1 {
		t.Fatalf("expected 1 merged phenomenon, got %d", len(phenomena))
	}

	p := phenomena[0]
	if p.Severity != "critical" {
		t.Errorf("expected severity critical, got %s", p.Severity)
	}
	if p.Category != "routing_infrastructure" {
		t.Errorf("expected category routing_infrastructure, got %s", p.Category)
	}
	if p.RecordCount != 2 {
		t.Errorf("expected 2 records aggregated, got %d", p.RecordCount)
	}
	if len(p.RecordIDs) != 2 || p.RecordIDs[0] != 1 || p.RecordIDs[1] != 2 {
		t.Errorf("expected record IDs [1, 2], got %v", p.RecordIDs)
	}
}

func TestAlarmMerger_DualPowerFail(t *testing.T) {
	merger := NewAlarmMerger()
	now := time.Now()

	records := []models.AlarmRecord{
		{
			ID:         10,
			RunID:      "run-002",
			DeviceIP:   "10.0.0.1",
			Family:     "CE",
			AlarmName:  "CE_POWER_FAIL",
			Severity:   "major",
			Category:   "power",
			Summary:    "Power module 1 is faulty",
			OccurredAt: now,
		},
		{
			ID:         11,
			RunID:      "run-002",
			DeviceIP:   "10.0.0.1",
			Family:     "CE",
			AlarmName:  "CE_POWER_ABSENT",
			Severity:   "major",
			Category:   "power",
			Summary:    "Power module 2 uninstalled",
			OccurredAt: now.Add(10 * time.Second),
		},
	}

	phenomena := merger.Merge(records)
	if len(phenomena) != 1 {
		t.Fatalf("expected 1 merged phenomenon, got %d", len(phenomena))
	}

	p := phenomena[0]
	// 双电源故障级别应升级至 critical
	if p.Severity != "critical" {
		t.Errorf("expected upgraded severity critical, got %s", p.Severity)
	}
	if p.RecordCount != 2 {
		t.Errorf("expected 2 records merged, got %d", p.RecordCount)
	}
}

func TestAlarmMerger_FanAndTemperature(t *testing.T) {
	merger := NewAlarmMerger()
	now := time.Now()

	records := []models.AlarmRecord{
		{
			ID:         20,
			RunID:      "run-003",
			DeviceIP:   "172.16.0.1",
			Family:     "Router",
			AlarmName:  "FAN_FAIL",
			Severity:   "major",
			Category:   "fan",
			Summary:    "Fan speed abnormal",
			OccurredAt: now,
		},
		{
			ID:         21,
			RunID:      "run-003",
			DeviceIP:   "172.16.0.1",
			Family:     "Router",
			AlarmName:  "TEMP_OVER_THRESHOLD",
			Severity:   "major",
			Category:   "system",
			Summary:    "High temperature warning",
			OccurredAt: now.Add(5 * time.Second),
		},
	}

	phenomena := merger.Merge(records)
	if len(phenomena) != 1 {
		t.Fatalf("expected 1 merged phenomenon, got %d", len(phenomena))
	}

	p := phenomena[0]
	if p.Severity != "critical" {
		t.Errorf("expected severity critical, got %s", p.Severity)
	}
	if p.Category != "hardware_environment" {
		t.Errorf("expected category hardware_environment, got %s", p.Category)
	}
}

func TestAlarmMerger_TimeWindowBucketing(t *testing.T) {
	merger := NewAlarmMerger(MergerConfig{TimeWindow: 10 * time.Minute})
	baseTime := time.Date(2026, 9, 20, 10, 0, 0, 0, time.UTC)

	records := []models.AlarmRecord{
		{
			ID:         1,
			RunID:      "run-tw-01",
			DeviceIP:   "10.0.0.1",
			Family:     "CE",
			AlarmName:  "CE_OPTICAL_LOS",
			Severity:   "major",
			Category:   "interface",
			Summary:    "Morning optical issue",
			OccurredAt: baseTime,
		},
		{
			ID:         2,
			RunID:      "run-tw-01",
			DeviceIP:   "10.0.0.1",
			Family:     "CE",
			AlarmName:  "CE_BGP_PEER_DOWN",
			Severity:   "critical",
			Category:   "routing",
			Summary:    "Afternoon bgp peer down (2 hours later)",
			OccurredAt: baseTime.Add(2 * time.Hour), // 超出 10 分钟窗口
		},
	}

	phenomena := merger.Merge(records)
	// 由于时间超出 10 分钟窗口，分属不同时间桶，不应被聚合为单一衍生故障，而是 2 个独立现象
	if len(phenomena) != 2 {
		t.Fatalf("expected 2 separate phenomena due to time window, got %d", len(phenomena))
	}
}
