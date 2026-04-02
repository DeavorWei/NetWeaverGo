package config

import (
	"fmt"
	"testing"

	"github.com/NetWeaverGo/core/internal/models"
)

// TestNormalizeTaskGroup_DefaultTaskType 验证任务模板默认类型规范化
func TestNormalizeTaskGroup_DefaultTaskType(t *testing.T) {
	group := &models.TaskGroup{}
	normalizeTaskGroup(group)
	if group.TaskType != "normal" {
		t.Fatalf("TaskType = %q, want %q", group.TaskType, "normal")
	}
}

// TestNormalizeTaskGroup_KeepTaskType 验证已有任务类型不会被覆盖
func TestNormalizeTaskGroup_KeepTaskType(t *testing.T) {
	group := &models.TaskGroup{TaskType: "topology"}
	normalizeTaskGroup(group)
	if group.TaskType != "topology" {
		t.Fatalf("TaskType = %q, want %q", group.TaskType, "topology")
	}
}

func TestValidateTaskGroupTopology_InvalidField(t *testing.T) {
	group := &models.TaskGroup{
		Name:     "topology-invalid-field",
		TaskType: "topology",
		TopologyFieldOverrides: []models.TopologyTaskFieldOverride{
			{FieldKey: "unknown_field", Command: "display foo", TimeoutSec: 30},
		},
	}

	err := validateTaskGroup(group)
	if err == nil {
		t.Fatal("expected error for invalid topology field key, got nil")
	}
}

func TestValidateTaskGroupTopology_EnableWithoutCommand(t *testing.T) {
	enabled := true
	group := &models.TaskGroup{
		Name:     "topology-enable-empty-command",
		TaskType: "topology",
		TopologyFieldOverrides: []models.TopologyTaskFieldOverride{
			{FieldKey: "lldp_neighbor", Enabled: &enabled, TimeoutSec: 30},
		},
	}

	err := validateTaskGroup(group)
	if err == nil {
		t.Fatal("expected error for enabled topology field with empty command, got nil")
	}
}

func TestValidateTaskGroupTopology_DisableRequiredField(t *testing.T) {
	disabled := false
	group := &models.TaskGroup{
		Name:     "topology-disable-required",
		TaskType: "topology",
		TopologyFieldOverrides: []models.TopologyTaskFieldOverride{
			{FieldKey: "version", Enabled: &disabled},
		},
	}

	err := validateTaskGroup(group)
	if err == nil {
		t.Fatal("expected error when disabling required topology field, got nil")
	}
}

func TestValidateTaskGroupTopology_AllFieldsDisabled(t *testing.T) {
	disabled := false
	overrides := make([]models.TopologyTaskFieldOverride, 0)
	for _, spec := range topologyFieldCatalogForValidation() {
		overrides = append(overrides, models.TopologyTaskFieldOverride{
			FieldKey: spec.FieldKey,
			Enabled:  &disabled,
		})
	}
	group := &models.TaskGroup{
		Name:                   "topology-all-disabled",
		TaskType:               "topology",
		TopologyFieldOverrides: overrides,
	}

	err := validateTaskGroup(group)
	if err == nil {
		t.Fatal("expected error when all topology fields are disabled, got nil")
	}
}

func TestValidateTaskGroupTopology_ValidOverrides(t *testing.T) {
	enabled := true
	group := &models.TaskGroup{
		Name:     "topology-valid-overrides",
		TaskType: "topology",
		TopologyFieldOverrides: []models.TopologyTaskFieldOverride{
			{FieldKey: "lldp_neighbor", Command: "display lldp neighbor verbose", TimeoutSec: 60, Enabled: &enabled},
			{FieldKey: "eth_trunk", Enabled: &enabled, Command: "display eth-trunk", TimeoutSec: 30},
		},
	}

	err := validateTaskGroup(group)
	if err != nil {
		t.Fatalf("expected valid topology overrides, got error: %v", err)
	}
}

func TestValidateTaskGroupTopology_DuplicateField(t *testing.T) {
	enabled := true
	group := &models.TaskGroup{
		Name:     "topology-duplicate-field",
		TaskType: "topology",
		TopologyFieldOverrides: []models.TopologyTaskFieldOverride{
			{FieldKey: "lldp_neighbor", Command: "display lldp", TimeoutSec: 30, Enabled: &enabled},
			{FieldKey: "lldp_neighbor", Command: "display lldp verbose", TimeoutSec: 60, Enabled: &enabled},
		},
	}

	err := validateTaskGroup(group)
	if err == nil {
		t.Fatal("expected error for duplicate topology field override, got nil")
	}
}

func TestValidateTaskGroupTopology_NegativeTimeout(t *testing.T) {
	group := &models.TaskGroup{
		Name:     "topology-negative-timeout",
		TaskType: "topology",
		TopologyFieldOverrides: []models.TopologyTaskFieldOverride{
			{FieldKey: "lldp_neighbor", Command: "display lldp neighbor", TimeoutSec: -1},
		},
	}

	err := validateTaskGroup(group)
	if err == nil {
		t.Fatal("expected error for negative timeout, got nil")
	}
}

func TestValidateTaskGroupTopology_AllRequiredFieldsStillEnabled(t *testing.T) {
	enabled := true
	overrides := make([]models.TopologyTaskFieldOverride, 0)
	for _, spec := range topologyFieldCatalogForValidation() {
		if spec.Required {
			overrides = append(overrides, models.TopologyTaskFieldOverride{
				FieldKey:   spec.FieldKey,
				Enabled:    &enabled,
				Command:    fmt.Sprintf("display %s", spec.FieldKey),
				TimeoutSec: 30,
			})
		}
	}
	group := &models.TaskGroup{
		Name:                   "topology-required-enabled",
		TaskType:               "topology",
		TopologyFieldOverrides: overrides,
	}

	err := validateTaskGroup(group)
	if err != nil {
		t.Fatalf("expected required fields enabled case to pass, got: %v", err)
	}
}
