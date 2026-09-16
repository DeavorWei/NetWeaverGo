package taskexec

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBizCompareTaskCompiler_Supports(t *testing.T) {
	compiler := NewBizCompareTaskCompiler(nil)
	assert.True(t, compiler.Supports(string(RunKindBizCompare)))
	assert.False(t, compiler.Supports(string(RunKindNormal)))
	assert.False(t, compiler.Supports(string(RunKindTopology)))
	assert.False(t, compiler.Supports(string(RunKindBackup)))
	assert.False(t, compiler.Supports(string(RunKindCEAS)))
	assert.False(t, compiler.Supports(string(RunKindInspection)))
}

func TestBizCompareTaskCompiler_Compile(t *testing.T) {
	compiler := NewBizCompareTaskCompiler(nil)

	cfg := BizCompareTaskConfig{
		DeviceIPs:   []string{"192.168.1.1", "192.168.1.2"},
		Domain:      "NE-SR",
		SceneID:     "ne_core_router",
		Phase:       "before",
		TimeoutSec:  45,
		Concurrency: 5,
	}
	cfgBytes, _ := json.Marshal(cfg)

	def := &TaskDefinition{
		ID:     "task-bizcmp-1",
		Name:   "NE路由器割接前业务采集",
		Kind:   string(RunKindBizCompare),
		Config: cfgBytes,
	}

	plan, err := compiler.Compile(context.Background(), def)
	assert.NoError(t, err)
	assert.NotNil(t, plan)
	assert.Equal(t, string(RunKindBizCompare), plan.RunKind)
	assert.Equal(t, 1, len(plan.Stages))

	stage := plan.Stages[0]
	assert.Equal(t, string(StageKindBizCompareCollect), stage.Kind)
	assert.Equal(t, 2, len(stage.Units))

	unit1 := stage.Units[0]
	assert.Equal(t, "192.168.1.1", unit1.Target.Key)
	assert.True(t, len(unit1.Steps) > 0)
	assert.Equal(t, "interface_brief", unit1.Steps[0].CommandKey)
}
