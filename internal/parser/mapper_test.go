package parser

import "testing"



func TestHuaweiMapperToLLDP(t *testing.T) {
	mapper := NewHuaweiMapper()
	rows := []map[string]string{
		{"local_if": "GigabitEthernet1/0/1"},
		{"neighbor_name": "core-sw-1"},
		{"neighbor_port": "GigabitEthernet1/0/24"},
		{"chassis_id": "00e0-fc00-0001"},
		{"mgmt_ip": "10.0.0.1"},
	}

	got, err := mapper.ToLLDP(rows)
	if err != nil {
		t.Fatalf("ToLLDP failed: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 neighbor, got %d", len(got))
	}
	if got[0].LocalInterface != "GE1/0/1" {
		t.Fatalf("unexpected local interface: %s", got[0].LocalInterface)
	}
	if got[0].NeighborPort != "GE1/0/24" {
		t.Fatalf("unexpected neighbor port: %s", got[0].NeighborPort)
	}
	if got[0].NeighborName != "CORE-SW-1" {
		t.Fatalf("unexpected neighbor name: %s", got[0].NeighborName)
	}
}

func TestHuaweiMapperToAggregate(t *testing.T) {
	mapper := NewHuaweiMapper()
	rows := []map[string]string{
		{"trunk_id": "10"},
		{"if_type": "GigabitEthernet", "if_num": "1/0/1"},
		{"if_type": "GigabitEthernet", "if_num": "1/0/2"},
	}

	got, err := mapper.ToAggregate(rows)
	if err != nil {
		t.Fatalf("ToAggregate failed: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 aggregate, got %d", len(got))
	}
	if got[0].AggregateName != "Trunk10" {
		t.Fatalf("unexpected aggregate name: %s", got[0].AggregateName)
	}
	if len(got[0].MemberPorts) != 2 {
		t.Fatalf("expected 2 members, got %d", len(got[0].MemberPorts))
	}
	if got[0].MemberPorts[0] != "GE1/0/1" || got[0].MemberPorts[1] != "GE1/0/2" {
		t.Fatalf("unexpected member ports: %+v", got[0].MemberPorts)
	}
}
