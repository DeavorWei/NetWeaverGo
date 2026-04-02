package taskexec

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListTopologyCollectionPlansReturnsPlanArtifacts(t *testing.T) {
	db := setupTestDB(t)
	service := NewTaskExecutionService(db, nil)
	runID := "run-plan-1"
	now := time.Now()

	planPath := filepath.Join(t.TempDir(), "topology_collection_plan.json")
	planDoc := TopologyCollectionPlanArtifact{
		RunID:          runID,
		StageID:        "stage-1",
		UnitID:         "unit-1",
		DeviceIP:       "10.0.0.1",
		ResolvedVendor: "huawei",
		VendorSource:   TopologyVendorSourceTask,
		Commands: []TopologyCollectionPlanCommand{
			{FieldKey: "version", Enabled: true, Command: "display version", CommandSource: TopologyCommandSourceTaskOverride},
		},
		CollectedFields: []string{"version"},
		GeneratedAt:     now,
	}
	payload, err := json.Marshal(planDoc)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(planPath, payload, 0o644))

	require.NoError(t, db.Create(&TaskArtifact{
		ID:           "artifact-plan-1",
		TaskRunID:    runID,
		ArtifactType: string(ArtifactTypeTopologyCollectionPlan),
		ArtifactKey:  "10.0.0.1:topology_collection_plan",
		FilePath:     planPath,
		CreatedAt:    now,
	}).Error)
	require.NoError(t, db.Create(&TaskArtifact{
		ID:           "artifact-raw-1",
		TaskRunID:    runID,
		ArtifactType: string(ArtifactTypeRawOutput),
		ArtifactKey:  "10.0.0.1:version",
		FilePath:     filepath.Join(t.TempDir(), "raw.txt"),
		CreatedAt:    now,
	}).Error)

	plans, err := service.ListTopologyCollectionPlans(runID)
	require.NoError(t, err)
	require.Len(t, plans, 1)
	assert.Equal(t, "10.0.0.1", plans[0].DeviceIP)
	assert.Equal(t, "huawei", plans[0].ResolvedVendor)
	assert.Equal(t, "10.0.0.1:topology_collection_plan", plans[0].ArtifactKey)
	assert.Equal(t, planPath, plans[0].FilePath)
	require.Len(t, plans[0].Commands, 1)
	assert.True(t, plans[0].Commands[0].Enabled)
}

func TestListTopologyCollectionPlansSkipsInvalidArtifacts(t *testing.T) {
	db := setupTestDB(t)
	service := NewTaskExecutionService(db, nil)
	runID := "run-plan-2"
	now := time.Now()

	invalidPath := filepath.Join(t.TempDir(), "invalid_plan.json")
	require.NoError(t, os.WriteFile(invalidPath, []byte("not-json"), 0o644))

	require.NoError(t, db.Create(&TaskArtifact{
		ID:           "artifact-plan-invalid",
		TaskRunID:    runID,
		ArtifactType: string(ArtifactTypeTopologyCollectionPlan),
		ArtifactKey:  "10.0.0.2:topology_collection_plan",
		FilePath:     invalidPath,
		CreatedAt:    now,
	}).Error)
	require.NoError(t, db.Create(&TaskArtifact{
		ID:           "artifact-plan-missing",
		TaskRunID:    runID,
		ArtifactType: string(ArtifactTypeTopologyCollectionPlan),
		ArtifactKey:  "10.0.0.3:topology_collection_plan",
		FilePath:     filepath.Join(t.TempDir(), "missing.json"),
		CreatedAt:    now,
	}).Error)

	plans, err := service.ListTopologyCollectionPlans(runID)
	require.NoError(t, err)
	assert.Empty(t, plans)
}
