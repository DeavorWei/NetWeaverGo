package forge

import (
	"reflect"
	"testing"
)

// ==================== ExpandSyntaxSugar 测试 ====================

func TestExpandSyntaxSugar_BasicRange(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{"1-5", "1-5", []string{"1", "2", "3", "4", "5"}},
		{"10-13", "10-13", []string{"10", "11", "12", "13"}},
		{"tilde separator", "1~3", []string{"1", "2", "3"}},
		{"prefix: vlan10-13", "vlan10-13", []string{"vlan10", "vlan11", "vlan12", "vlan13"}},
		{"IP suffix: 192.168.1.1-3", "192.168.1.1-3", []string{"192.168.1.1", "192.168.1.2", "192.168.1.3"}},
		{"single value", "42", []string{"42"}},
		{"leading zeros", "01-03", []string{"01", "02", "03"}},
		{"leading zeros with prefix", "port01-03", []string{"port01", "port02", "port03"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ExpandSyntaxSugar(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("got %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestExpandSyntaxSugar_ReverseRange(t *testing.T) {
	result, err := ExpandSyntaxSugar("5-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := []string{"5", "4", "3", "2", "1"}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("got %v, want %v", result, expected)
	}
}

func TestExpandSyntaxSugar_ZeroLengthRange(t *testing.T) {
	result, err := ExpandSyntaxSugar("5-5")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := []string{"5"}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("got %v, want %v", result, expected)
	}
}

func TestExpandSyntaxSugar_TooLargeRange(t *testing.T) {
	// 超过 1000 的范围应返回原值
	result, err := ExpandSyntaxSugar("1-2000")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 || result[0] != "1-2000" {
		t.Errorf("expected original value for too-large range, got %v", result)
	}
}

func TestExpandSyntaxSugar_NonRange(t *testing.T) {
	result, err := ExpandSyntaxSugar("hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 || result[0] != "hello" {
		t.Errorf("expected original value for non-range input, got %v", result)
	}
}

// ==================== DetectArithmeticSequence 测试 ====================

func TestDetectArithmeticSequence_Valid(t *testing.T) {
	tests := []struct {
		name       string
		values     []string
		wantArith  bool
		wantDiff   int
	}{
		{"simple 2,4,6", []string{"2", "4", "6"}, true, 2},
		{"simple 1,2,3", []string{"1", "2", "3"}, true, 1},
		{"prefixed GE1,GE3", []string{"GE1", "GE3"}, true, 2},
		{"two elements", []string{"10", "20"}, true, 10},
		{"single element", []string{"42"}, false, 0},
		{"non-arithmetic", []string{"1", "3", "8"}, false, 0},
		{"mixed prefix", []string{"GE0/0/1", "FE0/0/2"}, false, 0},
		{"leading zeros", []string{"01", "02", "03"}, true, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seq := DetectArithmeticSequence(tt.values)
			if seq.IsArithmetic != tt.wantArith {
				t.Errorf("IsArithmetic = %v, want %v", seq.IsArithmetic, tt.wantArith)
			}
			if tt.wantArith && seq.CommonDiff != tt.wantDiff {
				t.Errorf("CommonDiff = %d, want %d", seq.CommonDiff, tt.wantDiff)
			}
		})
	}
}

// ==================== InferArithmeticSequence 测试 ====================

func TestInferArithmeticSequence_ExtendTo5(t *testing.T) {
	// 输入 2,4，目标 5 个，应推断为 2,4,6,8,10
	result, err := InferArithmeticSequence([]string{"2", "4"}, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := []string{"2", "4", "6", "8", "10"}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("got %v, want %v", result, expected)
	}
}

func TestInferArithmeticSequence_AlreadySufficient(t *testing.T) {
	// 输入已有 5 个值，目标 3，应原样返回
	input := []string{"1", "2", "3", "4", "5"}
	result, err := InferArithmeticSequence(input, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(result, input) {
		t.Errorf("expected unchanged result, got %v", result)
	}
}

func TestInferArithmeticSequence_NonArithmetic(t *testing.T) {
	// 非等差数列，应原样返回
	input := []string{"a", "b", "c"}
	result, err := InferArithmeticSequence(input, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(result, input) {
		t.Errorf("expected unchanged result for non-arithmetic, got %v", result)
	}
}

func TestInferArithmeticSequence_WithLeadingZeros(t *testing.T) {
	// 01,02 推断到 5 个
	result, err := InferArithmeticSequence([]string{"01", "02"}, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := []string{"01", "02", "03", "04", "05"}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("got %v, want %v", result, expected)
	}
}

func TestInferArithmeticSequence_MaxLen1000(t *testing.T) {
	// 目标长度超过 1000 应被截断
	result, err := InferArithmeticSequence([]string{"1", "2"}, 2000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) > 1000 {
		t.Errorf("result length %d exceeds max 1000", len(result))
	}
}

// ==================== ParseVariableValues 测试 ====================

func TestParseVariableValues_CommaAndNewline(t *testing.T) {
	result, err := ParseVariableValues("a,b\nc, d")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := []string{"a", "b", "c", "d"}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("got %v, want %v", result, expected)
	}
}

func TestParseVariableValues_WithSyntaxSugar(t *testing.T) {
	result, err := ParseVariableValues("1-3, hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := []string{"1", "2", "3", "hello"}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("got %v, want %v", result, expected)
	}
}

func TestParseVariableValues_EmptyParts(t *testing.T) {
	result, err := ParseVariableValues("a,,\nb,  ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := []string{"a", "b"}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("got %v, want %v", result, expected)
	}
}

// ==================== SortVariablesByLength 测试 ====================

func TestSortVariablesByLength_LongestFirst(t *testing.T) {
	vars := []Variable{
		{Name: "[A]", Values: []string{"a"}},
		{Name: "[ABC]", Values: []string{"abc"}},
		{Name: "[AB]", Values: []string{"ab"}},
	}
	sorted := SortVariablesByLength(vars)

	if sorted[0].Name != "[ABC]" {
		t.Errorf("expected [ABC] first, got %s", sorted[0].Name)
	}
	if sorted[1].Name != "[AB]" {
		t.Errorf("expected [AB] second, got %s", sorted[1].Name)
	}
	if sorted[2].Name != "[A]" {
		t.Errorf("expected [A] third, got %s", sorted[2].Name)
	}
}

func TestSortVariablesByLength_SameLength(t *testing.T) {
	vars := []Variable{
		{Name: "[A]", Values: []string{"a"}},
		{Name: "[B]", Values: []string{"b"}},
	}
	sorted := SortVariablesByLength(vars)
	// 同长度的变量顺序不重要，但不应 panic
	if len(sorted) != 2 {
		t.Errorf("expected 2 variables, got %d", len(sorted))
	}
}

func TestSortVariablesByLength_DoesNotMutateOriginal(t *testing.T) {
	vars := []Variable{
		{Name: "[B]", Values: []string{"b"}},
		{Name: "[ABC]", Values: []string{"abc"}},
		{Name: "[A]", Values: []string{"a"}},
	}
	originalFirst := vars[0].Name
	_ = SortVariablesByLength(vars)
	if vars[0].Name != originalFirst {
		t.Error("SortVariablesByLength should not mutate the original slice")
	}
}

// ==================== ParseVariables 测试 ====================

func TestParseVariables_ExpansionAndInference(t *testing.T) {
	vars := []Variable{
		{Name: "[A]", ValueString: "1-3"},
		{Name: "[B]", ValueString: "10"},
	}
	result, err := ParseVariables(vars)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// [A] 展开为 3 个值
	if len(result[0].Values) != 3 {
		t.Errorf("[A] expected 3 values, got %d", len(result[0].Values))
	}
	// [B] 只有 1 个值，不足以触发等差推断（需要 >= 2 个值），不应被补全
	if len(result[1].Values) != 1 {
		t.Errorf("[B] expected 1 value (no inference with single value), got %d", len(result[1].Values))
	}
}

func TestParseVariables_InferenceWithTwoValues(t *testing.T) {
	vars := []Variable{
		{Name: "[A]", ValueString: "1-5"},
		{Name: "[B]", ValueString: "10,20"},
	}
	result, err := ParseVariables(vars)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// [B] 有 2 个值且构成等差，目标长度 5，应补全到 5
	if len(result[1].Values) != 5 {
		t.Errorf("[B] expected 5 values after inference, got %d: %v", len(result[1].Values), result[1].Values)
	}
}
