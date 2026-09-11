package parser

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTreeEngine_DeviceEcho(t *testing.T) {
	deviceEcho := `
Slot 1:
  Board Type : LPU
  Status     : Normal
  GE1/0/1    up    1000M
  GE1/0/2    down  1000M
Slot 2:
  Board Type : MPU
  Status     : Normal
  MEth0/0/1  up    100M
Slot 3:
  Board Type : SFU
  Status     : Abnormal
`

	// 配置规则树
	rules := []TreeRule{
		{
			ParseItem:  "slotId",
			ParentItem: "",
			IsList:     true,
			SplitRegex: `(?m)^Slot\s+(\d+):`,
			GroupIndex: 1,
			IsOutput:   true,
			Order:      1,
		},
		{
			ParseItem:    "boardType",
			ParentItem:   "slotId",
			IsList:       false,
			ParseRegex:   `Board Type\s*:\s*(\S+)`,
			GroupIndex:   1,
			IsOutput:     true,
			DefaultValue: "N/A",
			Order:        2,
		},
		{
			ParseItem:    "status",
			ParentItem:   "slotId",
			IsList:       false,
			ParseRegex:   `Status\s*:\s*(\S+)`,
			GroupIndex:   1,
			IsOutput:     true,
			DefaultValue: "Unknown",
			Order:        3,
		},
		{
			ParseItem:  "portName",
			ParentItem: "slotId",
			IsList:     true,
			ParseRegex: `(?m)^\s+((?:GE|MEth)\S+)\s+`,
			GroupIndex: 1,
			IsOutput:   true,
			Order:      4,
		},
	}

	compiledRules, rootRules, err := CompileTreeRules(rules)
	require.NoError(t, err)
	require.Len(t, rootRules, 1)
	require.Len(t, compiledRules, 4)

	tpl := &CompiledTemplate{
		RegexTemplate: RegexTemplate{
			CommandKey: "display device",
			Engine:     EngineTree,
			TreeConfig: &TreeTemplate{
				Rules: rules,
			},
		},
		CompiledTreeRules: compiledRules,
		TreeRootRules:     rootRules,
	}

	engine := NewTreeEngine()

	// 1. 结构化树输出
	treeNodes, err := engine.ParseTree(tpl, deviceEcho)
	require.NoError(t, err)
	assert.Len(t, treeNodes, 3) // 3 个 Slot
	assert.Equal(t, "1", treeNodes[0].Attrs["slotId"])
	assert.Equal(t, "2", treeNodes[1].Attrs["slotId"])
	assert.Equal(t, "3", treeNodes[2].Attrs["slotId"])

	// 2. 拍平展开（笛卡尔展开为多行表格）
	rows, err := engine.ParseWithTemplate(tpl, deviceEcho)
	require.NoError(t, err)

	// Slot 1 有 2 个口 -> 2 行
	// Slot 2 有 1 个口 -> 1 行
	// Slot 3 有 0 个口 -> 1 行（继承属性，portName 为空或未指定）
	// 总共 4 行
	require.Len(t, rows, 4)

	// 行 0：Slot 1 GE1/0/1
	assert.Equal(t, "1", rows[0]["slotId"])
	assert.Equal(t, "LPU", rows[0]["boardType"])
	assert.Equal(t, "Normal", rows[0]["status"])
	assert.Equal(t, "GE1/0/1", rows[0]["portName"])

	// 行 1：Slot 1 GE1/0/2
	assert.Equal(t, "1", rows[1]["slotId"])
	assert.Equal(t, "LPU", rows[1]["boardType"])
	assert.Equal(t, "Normal", rows[1]["status"])
	assert.Equal(t, "GE1/0/2", rows[1]["portName"])

	// 行 2：Slot 2 MEth0/0/1
	assert.Equal(t, "2", rows[2]["slotId"])
	assert.Equal(t, "MPU", rows[2]["boardType"])
	assert.Equal(t, "Normal", rows[2]["status"])
	assert.Equal(t, "MEth0/0/1", rows[2]["portName"])

	// 行 3：Slot 3 无端口，严格验证 M1：未命中且无默认值时补空串，确保列不缺失
	assert.Equal(t, "3", rows[3]["slotId"])
	assert.Equal(t, "SFU", rows[3]["boardType"])
	assert.Equal(t, "Abnormal", rows[3]["status"])
	assert.Equal(t, "", rows[3]["portName"])
}

func TestTreeEngine_MaxOutputLevel(t *testing.T) {
	deviceEcho := `
Slot 1:
  Board Type : LPU
  GE1/0/1    up    1000M
  GE1/0/2    down  1000M
`
	rules := []TreeRule{
		{
			ParseItem:  "slotId",
			ParentItem: "",
			IsList:     true,
			SplitRegex: `(?m)^Slot\s+(\d+):`,
			GroupIndex: 1,
			IsOutput:   true,
			Order:      1,
		},
		{
			ParseItem:  "portName",
			ParentItem: "slotId",
			IsList:     true,
			ParseRegex: `(?m)^\s+((?:GE|MEth)\S+)\s+`,
			GroupIndex: 1,
			IsOutput:   true,
			Order:      2,
		},
	}

	compiledRules, rootRules, err := CompileTreeRules(rules)
	require.NoError(t, err)

	// maxOutputLevel = 1：只拍平到 slotId 层，不展开子端口
	tpl := &CompiledTemplate{
		RegexTemplate: RegexTemplate{
			Engine: EngineTree,
			TreeConfig: &TreeTemplate{
				Rules:          rules,
				MaxOutputLevel: 1,
			},
		},
		CompiledTreeRules: compiledRules,
		TreeRootRules:     rootRules,
	}

	engine := NewTreeEngine()
	rows, err := engine.ParseWithTemplate(tpl, deviceEcho)
	require.NoError(t, err)
	// 预期仅生成 1 行（Slot 1），不展开多行
	assert.Len(t, rows, 1)
	assert.Equal(t, "1", rows[0]["slotId"])
	assert.Equal(t, "", rows[0]["portName"]) // 子属性未展开补空串
}

func TestTreeEngine_ScenarioB_Multimatch(t *testing.T) {
	text := `
Port: GE1/0/1 Status: Up Speed: 1000M
Port: GE1/0/2 Status: Down Speed: 100M
`
	rules := []TreeRule{
		{
			ParseItem:  "portRow",
			ParentItem: "",
			IsList:     true,
			ParseRegex: `(?m)^Port:\s+(\S+)\s+Status:\s+(\S+)\s+Speed:\s+(\S+)`,
			IsOutput:   false,
			Order:      1,
		},
		{
			ParseItem:  "port",
			ParentItem: "portRow",
			IsList:     false,
			ParseRegex: `(?m)Port:\s+(\S+)`,
			GroupIndex: 1,
			IsOutput:   true,
			Order:      2,
		},
		{
			ParseItem:  "status",
			ParentItem: "portRow",
			IsList:     false,
			ParseRegex: `(?m)Status:\s+(\S+)`,
			GroupIndex: 1,
			IsOutput:   true,
			Order:      3,
		},
	}

	compiledRules, rootRules, err := CompileTreeRules(rules)
	require.NoError(t, err)

	tpl := &CompiledTemplate{
		RegexTemplate: RegexTemplate{
			Engine:     EngineTree,
			TreeConfig: &TreeTemplate{Rules: rules},
		},
		CompiledTreeRules: compiledRules,
		TreeRootRules:     rootRules,
	}

	engine := NewTreeEngine()
	rows, err := engine.ParseWithTemplate(tpl, text)
	require.NoError(t, err)
	assert.Len(t, rows, 2)
	assert.Equal(t, "GE1/0/1", rows[0]["port"])
	assert.Equal(t, "Up", rows[0]["status"])
	assert.Equal(t, "GE1/0/2", rows[1]["port"])
	assert.Equal(t, "Down", rows[1]["status"])
}

func TestTreeEngine_InvalidRegex(t *testing.T) {
	// 测试非法正则报错拦截
	rules := []TreeRule{
		{
			ParseItem:  "badRegex",
			ParentItem: "",
			ParseRegex: `[invalid(regex`,
		},
	}
	_, _, err := CompileTreeRules(rules)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidTreeRule)
}

func TestTreeEngine_ConcurrentParsing(t *testing.T) {
	deviceEcho := `
Slot 1:
  Board Type : LPU
  GE1/0/1    up    1000M
`
	rules := []TreeRule{
		{
			ParseItem:  "slotId",
			ParentItem: "",
			IsList:     true,
			SplitRegex: `(?m)^Slot\s+(\d+):`,
			GroupIndex: 1,
			IsOutput:   true,
			Order:      1,
		},
		{
			ParseItem:  "portName",
			ParentItem: "slotId",
			IsList:     true,
			ParseRegex: `(?m)^\s+((?:GE|MEth)\S+)\s+`,
			GroupIndex: 1,
			IsOutput:   true,
			Order:      2,
		},
	}

	compiledRules, rootRules, err := CompileTreeRules(rules)
	require.NoError(t, err)

	tpl := &CompiledTemplate{
		RegexTemplate: RegexTemplate{
			Engine:     EngineTree,
			TreeConfig: &TreeTemplate{Rules: rules},
		},
		CompiledTreeRules: compiledRules,
		TreeRootRules:     rootRules,
	}

	engine := NewTreeEngine()

	// 并发执行 50 个 goroutine，验证 IsPath 固化后无数据竞争
	done := make(chan bool)
	for i := 0; i < 50; i++ {
		go func() {
			rows, err := engine.ParseWithTemplate(tpl, deviceEcho)
			assert.NoError(t, err)
			assert.Len(t, rows, 1)
			done <- true
		}()
	}

	for i := 0; i < 50; i++ {
		<-done
	}
}

func TestTreeEngine_CyclicDependency(t *testing.T) {
	// A -> B -> A 环路
	rules := []TreeRule{
		{
			ParseItem:  "A",
			ParentItem: "B",
			IsOutput:   true,
		},
		{
			ParseItem:  "B",
			ParentItem: "A",
			IsOutput:   true,
		},
	}

	_, _, err := CompileTreeRules(rules)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrCyclicTreeRule)
}

func TestTreeEngine_MissingParent(t *testing.T) {
	rules := []TreeRule{
		{
			ParseItem:  "Child",
			ParentItem: "NonExistentParent",
			IsOutput:   true,
		},
	}

	_, _, err := CompileTreeRules(rules)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidTreeRule)
}
