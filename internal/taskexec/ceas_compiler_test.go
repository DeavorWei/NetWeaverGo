package taskexec

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/NetWeaverGo/core/internal/ceas"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCEASTaskCompiler_Supports(t *testing.T) {
	compiler := NewCEASTaskCompiler(nil)
	assert.True(t, compiler.Supports(string(RunKindCEAS)))
	assert.False(t, compiler.Supports(string(RunKindNormal)))
	assert.False(t, compiler.Supports(string(RunKindTopology)))
	assert.False(t, compiler.Supports(string(RunKindBackup)))
}

func TestCEASTaskCompiler_Compile_Success(t *testing.T) {
	compiler := NewCEASTaskCompiler(nil)

	cfg := ceas.CEASTaskConfig{
		DeviceIPs:   []string{"192.168.1.1", "192.168.1.2", "192.168.1.1", " "},
		Concurrency: 5,
		TimeoutSec:  45,
	}
	cfgBytes, err := json.Marshal(cfg)
	require.NoError(t, err)

	def := &TaskDefinition{
		ID:     "test-def-1",
		Name:   "CEAS采集测试",
		Kind:   string(RunKindCEAS),
		Config: cfgBytes,
	}

	plan, err := compiler.Compile(context.Background(), def)
	require.NoError(t, err)
	require.NotNil(t, plan)
	assert.Equal(t, string(RunKindCEAS), plan.RunKind)
	assert.Equal(t, "CEAS采集测试", plan.Name)
	require.Len(t, plan.Stages, 1)

	stage := plan.Stages[0]
	assert.Equal(t, string(StageKindCEASCollect), stage.Kind)
	assert.Equal(t, 5, stage.Concurrency)
	require.Len(t, stage.Units, 2) // 重复与空字符串已被去重过滤

	assert.Equal(t, "192.168.1.1", stage.Units[0].Target.Key)
	assert.Equal(t, 45*time.Second, stage.Units[0].Timeout)
	require.Len(t, stage.Units[0].Steps, 1)
	assert.Equal(t, "ceas_collect", stage.Units[0].Steps[0].Kind)

	assert.Equal(t, "192.168.1.2", stage.Units[1].Target.Key)
}

func TestCEASTaskCompiler_Compile_Errors(t *testing.T) {
	compiler := NewCEASTaskCompiler(nil)

	// 1. 无效 JSON
	defInvalidJSON := &TaskDefinition{
		Name:   "bad-json",
		Config: []byte("{invalid"),
	}
	_, err := compiler.Compile(context.Background(), defInvalidJSON)
	assert.Error(t, err)

	// 2. 空设备列表
	cfgEmpty := ceas.CEASTaskConfig{
		DeviceIPs: []string{" ", ""},
	}
	cfgBytes, _ := json.Marshal(cfgEmpty)
	defEmpty := &TaskDefinition{
		Name:   "empty-devices",
		Config: cfgBytes,
	}
	_, err = compiler.Compile(context.Background(), defEmpty)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "至少一台设备")
}

func TestCEASExecutor_Kind(t *testing.T) {
	exec := NewCEASExecutor(nil, nil)
	assert.Equal(t, string(StageKindCEASCollect), exec.Kind())
}
