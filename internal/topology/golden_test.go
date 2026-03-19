package topology

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/NetWeaverGo/core/internal/models"
)

type edgeGolden struct {
	ADeviceID string `json:"aDeviceId"`
	AIf       string `json:"aIf"`
	BDeviceID string `json:"bDeviceId"`
	BIf       string `json:"bIf"`
	EdgeType  string `json:"edgeType"`
	Status    string `json:"status"`
}

func TestTopologyGoldenDualSwitchConfirmed(t *testing.T) {
	builder, db := newTopologyTestBuilder(t)
	taskID := "task_topology_golden"

	devices := []models.DiscoveryDevice{
		{TaskID: taskID, DeviceIP: "10.9.0.1", MgmtIP: "10.9.0.1", Hostname: "GOLDEN-SW1", NormalizedName: "GOLDEN-SW1", Status: "success"},
		{TaskID: taskID, DeviceIP: "10.9.0.2", MgmtIP: "10.9.0.2", Hostname: "GOLDEN-SW2", NormalizedName: "GOLDEN-SW2", Status: "success"},
	}
	if err := db.Create(&devices).Error; err != nil {
		t.Fatalf("create devices failed: %v", err)
	}

	neighbors := []models.TopologyLLDPNeighbor{
		{TaskID: taskID, DeviceIP: "10.9.0.1", LocalInterface: "GE1/0/1", NeighborName: "GOLDEN-SW2", NeighborPort: "GE1/0/24", NeighborIP: "10.9.0.2"},
		{TaskID: taskID, DeviceIP: "10.9.0.2", LocalInterface: "GE1/0/24", NeighborName: "GOLDEN-SW1", NeighborPort: "GE1/0/1", NeighborIP: "10.9.0.1"},
	}
	if err := db.Create(&neighbors).Error; err != nil {
		t.Fatalf("create neighbors failed: %v", err)
	}

	if _, err := builder.Build(taskID); err != nil {
		t.Fatalf("build failed: %v", err)
	}

	var edges []models.TopologyEdge
	if err := db.Where("task_id = ?", taskID).Find(&edges).Error; err != nil {
		t.Fatalf("load edges failed: %v", err)
	}

	got := make([]edgeGolden, 0, len(edges))
	for _, edge := range edges {
		if edge.EdgeType != "physical" || edge.Status != "confirmed" {
			continue
		}
		got = append(got, canonicalEdge(edge))
	}
	sortEdges(got)

	var want []edgeGolden
	expectedPath := filepath.Join("..", "..", "testdata", "huawei", "topology", "dual_switch_confirmed_edges.json")
	if err := loadTopologyJSON(expectedPath, &want); err != nil {
		t.Fatalf("load expected topology failed: %v", err)
	}
	sortEdges(want)

	if len(got) != len(want) {
		t.Fatalf("topology golden size mismatch: want=%d got=%d, got=%+v", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("topology golden mismatch at %d: want=%+v got=%+v", i, want[i], got[i])
		}
	}
}

func canonicalEdge(edge models.TopologyEdge) edgeGolden {
	aDev, aIf := edge.ADeviceID, edge.AIf
	bDev, bIf := edge.BDeviceID, edge.BIf
	if aDev > bDev || (aDev == bDev && aIf > bIf) {
		aDev, bDev = bDev, aDev
		aIf, bIf = bIf, aIf
	}
	return edgeGolden{
		ADeviceID: aDev,
		AIf:       aIf,
		BDeviceID: bDev,
		BIf:       bIf,
		EdgeType:  edge.EdgeType,
		Status:    edge.Status,
	}
}

func sortEdges(edges []edgeGolden) {
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].ADeviceID != edges[j].ADeviceID {
			return edges[i].ADeviceID < edges[j].ADeviceID
		}
		if edges[i].AIf != edges[j].AIf {
			return edges[i].AIf < edges[j].AIf
		}
		if edges[i].BDeviceID != edges[j].BDeviceID {
			return edges[i].BDeviceID < edges[j].BDeviceID
		}
		if edges[i].BIf != edges[j].BIf {
			return edges[i].BIf < edges[j].BIf
		}
		if edges[i].EdgeType != edges[j].EdgeType {
			return edges[i].EdgeType < edges[j].EdgeType
		}
		return edges[i].Status < edges[j].Status
	})
}

func loadTopologyJSON(path string, target interface{}) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}
