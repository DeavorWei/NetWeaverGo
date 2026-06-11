package forge

import (
	"strings"
	"testing"
)

// ==================== Build 基础功能测试 ====================

func TestBuild_BasicSingleVariable(t *testing.T) {
	builder := NewConfigBuilder()
	result, err := builder.Build(&BuildRequest{
		Template:  "interface [A]\n  description test",
		Variables: []VarInput{{Name: "[A]", ValueString: "GigabitEthernet0/0/1"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Total != 1 {
		t.Fatalf("expected 1 block, got %d", result.Total)
	}
	expected := "interface GigabitEthernet0/0/1\n  description test"
	if result.Blocks[0] != expected {
		t.Errorf("block[0] mismatch:\n  got:  %q\n  want: %q", result.Blocks[0], expected)
	}
}

func TestBuild_MultipleValues(t *testing.T) {
	builder := NewConfigBuilder()
	result, err := builder.Build(&BuildRequest{
		Template:  "interface [A]\n  shutdown",
		Variables: []VarInput{{Name: "[A]", ValueString: "Gi0/0/1, Gi0/0/2, Gi0/0/3"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Total != 3 {
		t.Fatalf("expected 3 blocks, got %d", result.Total)
	}
	if !strings.Contains(result.Blocks[0], "Gi0/0/1") {
		t.Errorf("block[0] should contain Gi0/0/1, got: %s", result.Blocks[0])
	}
	if !strings.Contains(result.Blocks[2], "Gi0/0/3") {
		t.Errorf("block[2] should contain Gi0/0/3, got: %s", result.Blocks[2])
	}
}

func TestBuild_MultipleVariables(t *testing.T) {
	builder := NewConfigBuilder()
	result, err := builder.Build(&BuildRequest{
		Template: "interface [A]\n  description [B]\n  vlan [C]",
		Variables: []VarInput{
			{Name: "[A]", ValueString: "Gi0/0/1, Gi0/0/2"},
			{Name: "[B]", ValueString: "Link-1, Link-2"},
			{Name: "[C]", ValueString: "100, 200"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Total != 2 {
		t.Fatalf("expected 2 blocks, got %d", result.Total)
	}
	if !strings.Contains(result.Blocks[0], "Gi0/0/1") || !strings.Contains(result.Blocks[0], "Link-1") || !strings.Contains(result.Blocks[0], "100") {
		t.Errorf("block[0] mismatch: %s", result.Blocks[0])
	}
	if !strings.Contains(result.Blocks[1], "Gi0/0/2") || !strings.Contains(result.Blocks[1], "Link-2") || !strings.Contains(result.Blocks[1], "200") {
		t.Errorf("block[1] mismatch: %s", result.Blocks[1])
	}
}

// ==================== 边界场景测试 ====================

func TestBuild_EmptyTemplate(t *testing.T) {
	builder := NewConfigBuilder()
	result, err := builder.Build(&BuildRequest{
		Template:  "   ",
		Variables: []VarInput{{Name: "[A]", ValueString: "val1"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Total != 0 {
		t.Errorf("expected 0 blocks for empty template, got %d", result.Total)
	}
}

func TestBuild_NoVariables(t *testing.T) {
	builder := NewConfigBuilder()
	result, err := builder.Build(&BuildRequest{
		Template:  "some static config",
		Variables: []VarInput{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Total != 0 {
		t.Errorf("expected 0 blocks for no variables, got %d", result.Total)
	}
}

func TestBuild_AllVariablesEmpty(t *testing.T) {
	builder := NewConfigBuilder()
	result, err := builder.Build(&BuildRequest{
		Template:  "interface [A]",
		Variables: []VarInput{{Name: "[A]", ValueString: ""}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Total != 0 {
		t.Errorf("expected 0 blocks for empty variable values, got %d", result.Total)
	}
}

// ==================== 变量值不足循环补齐测试 ====================

func TestBuild_ValueLoopFill(t *testing.T) {
	builder := NewConfigBuilder()
	// [A] 有3个值, [B] 只有1个值, [B] 应循环补齐
	result, err := builder.Build(&BuildRequest{
		Template:  "[A] -> [B]",
		Variables: []VarInput{
			{Name: "[A]", ValueString: "a1, a2, a3"},
			{Name: "[B]", ValueString: "b1"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Total != 3 {
		t.Fatalf("expected 3 blocks, got %d", result.Total)
	}
	for i, block := range result.Blocks {
		if !strings.Contains(block, "b1") {
			t.Errorf("block[%d] should contain 'b1' (loop fill), got: %s", i, block)
		}
	}
}

// ==================== 变量名冲突防护测试 ====================

func TestBuild_VariableNameConflict_A_AB(t *testing.T) {
	builder := NewConfigBuilder()
	// 验证 [A] 不会误匹配 [AB] 中的 A
	result, err := builder.Build(&BuildRequest{
		Template:  "set [A] and [AB]",
		Variables: []VarInput{
			{Name: "[A]", ValueString: "alpha"},
			{Name: "[AB]", ValueString: "beta"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Total != 1 {
		t.Fatalf("expected 1 block, got %d", result.Total)
	}
	block := result.Blocks[0]
	if !strings.Contains(block, "alpha") || !strings.Contains(block, "beta") {
		t.Errorf("variable conflict detected! block: %s (expected 'alpha' and 'beta')", block)
	}
	// 确保 [AB] 没有被 [A] 的替换破坏
	if strings.Contains(block, "[") {
		t.Errorf("unreplaced variable found in block: %s", block)
	}
}

func TestBuild_VariableNameConflict_ShortLong(t *testing.T) {
	builder := NewConfigBuilder()
	// 更极端的冲突测试: [X] 和 [XYZ]
	result, err := builder.Build(&BuildRequest{
		Template:  "val1=[X] val2=[XYZ]",
		Variables: []VarInput{
			{Name: "[X]", ValueString: "short"},
			{Name: "[XYZ]", ValueString: "longvalue"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	block := result.Blocks[0]
	if block != "val1=short val2=longvalue" {
		t.Errorf("unexpected result: %q", block)
	}
}

// ==================== 语法糖集成测试 ====================

func TestBuild_SyntaxSugarExpansion(t *testing.T) {
	builder := NewConfigBuilder()
	result, err := builder.Build(&BuildRequest{
		Template:  "interface [A]\n  shutdown",
		Variables: []VarInput{{Name: "[A]", ValueString: "1-3"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Total != 3 {
		t.Fatalf("expected 3 blocks from syntax sugar 1-3, got %d", result.Total)
	}
}

func TestBuild_NewlineSeparated(t *testing.T) {
	builder := NewConfigBuilder()
	result, err := builder.Build(&BuildRequest{
		Template:  "hostname [A]",
		Variables: []VarInput{{Name: "[A]", ValueString: "sw1\nsw2\nsw3"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Total != 3 {
		t.Fatalf("expected 3 blocks, got %d", result.Total)
	}
}

// ==================== ExpandValues 测试 ====================

func TestExpandValues_Basic(t *testing.T) {
	builder := NewConfigBuilder()
	result, err := builder.ExpandValues(&ExpandRequest{
		ValueString: "1-5",
		MaxLen:      0,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.HasExpanded {
		t.Error("expected HasExpanded=true for '1-5'")
	}
	if len(result.Values) != 5 {
		t.Errorf("expected 5 values, got %d", len(result.Values))
	}
}

func TestExpandValues_ArithmeticInference(t *testing.T) {
	builder := NewConfigBuilder()
	// 输入 2,4，目标长度 5，应推断为 2,4,6,8,10
	result, err := builder.ExpandValues(&ExpandRequest{
		ValueString: "2,4",
		MaxLen:      5,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.HasInferred {
		t.Error("expected HasInferred=true for arithmetic sequence 2,4 with maxLen=5")
	}
	if len(result.Values) != 5 {
		t.Fatalf("expected 5 values, got %d: %v", len(result.Values), result.Values)
	}
	if result.Values[4] != "10" {
		t.Errorf("expected last value '10', got %q", result.Values[4])
	}
}

func TestExpandValues_NoExpansion(t *testing.T) {
	builder := NewConfigBuilder()
	result, err := builder.ExpandValues(&ExpandRequest{
		ValueString: "hello, world",
		MaxLen:      0,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.HasExpanded {
		t.Error("expected HasExpanded=false for plain text")
	}
	if len(result.Values) != 2 {
		t.Errorf("expected 2 values, got %d", len(result.Values))
	}
}

// ==================== PreviewBlock 测试 ====================

func TestPreviewBlock_Basic(t *testing.T) {
	builder := NewConfigBuilder()
	result, err := builder.PreviewBlock(&PreviewRequest{
		Template:     "interface [A]\n  description [B]",
		VariableName: "[A]",
		Values:       []string{"Gi0/0/1", "Gi0/0/2"},
		Index:        0,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Block, "Gi0/0/1") {
		t.Errorf("preview should contain Gi0/0/1, got: %s", result.Block)
	}
	// [B] should remain unreplaced since we only replaced [A]
	if !strings.Contains(result.Block, "[B]") {
		t.Errorf("[B] should remain unreplaced, got: %s", result.Block)
	}
}

func TestPreviewBlock_IndexOverflow(t *testing.T) {
	builder := NewConfigBuilder()
	result, err := builder.PreviewBlock(&PreviewRequest{
		Template:     "host [A]",
		VariableName: "[A]",
		Values:       []string{"sw1", "sw2"},
		Index:        99,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 应取最后一个值
	if !strings.Contains(result.Block, "sw2") {
		t.Errorf("should fall back to last value 'sw2', got: %s", result.Block)
	}
}
