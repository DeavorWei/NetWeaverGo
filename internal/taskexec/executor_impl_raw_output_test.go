package taskexec

import (
	"testing"

	"github.com/NetWeaverGo/core/internal/executor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateTaskRawOutputPersistsFieldEnabledAndSources(t *testing.T) {
	db := setupTestDB(t)
	exec := &DeviceCollectExecutor{db: db}

	result := &executor.CommandResult{
		CommandKey:      "lldp_neighbor",
		Command:         "display lldp neighbor",
		RawText:         "raw-output",
		NormalizedLines: []string{"line-1", "line-2"},
		Success:         true,
		NormalizedText:  "line-1\nline-2",
		PaginationCount: 0,
		PromptMatched:   true,
		ErrorMessage:    "",
		NormalizedSize:  0,
		RawSize:         0,
		Index:           0,
		Duration:        0,
		EchoConsumed:    false,
		DeviceIP:        "10.10.10.10",
	}
	resolved := &ResolvedTopologyCommand{
		FieldKey:       "lldp_neighbor",
		Enabled:        true,
		CommandSource:  TopologyCommandSourceTaskOverride,
		ResolvedVendor: "huawei",
		VendorSource:   TopologyVendorSourceTask,
	}

	err := exec.createTaskRawOutput("run-test-1", "10.10.10.10", result, "raw.txt", "normalized.txt", resolved)
	require.NoError(t, err)

	var output TaskRawOutput
	err = db.Where("task_run_id = ? AND device_ip = ? AND command_key = ?", "run-test-1", "10.10.10.10", "lldp_neighbor").First(&output).Error
	require.NoError(t, err)

	assert.True(t, output.FieldEnabled)
	assert.Equal(t, TopologyCommandSourceTaskOverride, output.CommandSource)
	assert.Equal(t, "huawei", output.ResolvedVendor)
	assert.Equal(t, TopologyVendorSourceTask, output.VendorSource)
}

func TestCreateTaskRawOutputKeepsFieldEnabledFalseWithoutResolved(t *testing.T) {
	db := setupTestDB(t)
	exec := &DeviceCollectExecutor{db: db}

	result := &executor.CommandResult{
		CommandKey:      "version",
		Command:         "display version",
		RawText:         "raw-version",
		NormalizedLines: []string{"version-line"},
		Success:         true,
		DeviceIP:        "10.10.10.11",
	}

	err := exec.createTaskRawOutput("run-test-2", "10.10.10.11", result, "raw-v.txt", "norm-v.txt", nil)
	require.NoError(t, err)

	var output TaskRawOutput
	err = db.Where("task_run_id = ? AND device_ip = ? AND command_key = ?", "run-test-2", "10.10.10.11", "version").First(&output).Error
	require.NoError(t, err)

	assert.False(t, output.FieldEnabled)
	assert.Equal(t, "", output.CommandSource)
	assert.Equal(t, "", output.ResolvedVendor)
	assert.Equal(t, "", output.VendorSource)
}
