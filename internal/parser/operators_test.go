package parser

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOperators_ToUpperAndToLower(t *testing.T) {
	row := map[string]string{
		"status": "up",
		"name":   "GE1/0/1",
	}

	opUpper := &ToUpperOperator{Fields: []string{"status"}}
	ctx := &OperatorContext{Row: row}
	_ = opUpper.Execute(ctx)
	assert.Equal(t, "UP", row["status"])

	opLower := &ToLowerOperator{Fields: []string{"name"}}
	_ = opLower.Execute(ctx)
	assert.Equal(t, "ge1/0/1", row["name"])
}

func TestOperators_MatchAndSet(t *testing.T) {
	row := map[string]string{"foo": "bar"}
	op := &MatchAndSetOperator{Names: []string{"type"}, Value: "dynamic"}
	ctx := &OperatorContext{Row: row}
	_ = op.Execute(ctx)
	assert.Equal(t, "dynamic", row["type"])
}

func TestOperators_ValueMapping(t *testing.T) {
	row := map[string]string{"state": "1"}
	op := &ValueMappingOperator{
		Names: []string{"state"},
		Mapping: map[string]string{
			"1": "UP",
			"2": "DOWN",
		},
	}
	ctx := &OperatorContext{Row: row}
	_ = op.Execute(ctx)
	assert.Equal(t, "UP", row["state"])
}

func TestOperators_StrExtract(t *testing.T) {
	row := map[string]string{"slot": "Slot_3/Port_1"}
	re := regexp.MustCompile(`Slot_(\d+)`)
	op := &StrExtractOperator{
		NameField: "slot",
		Regex:     re,
		GroupID:   1,
	}
	ctx := &OperatorContext{Row: row}
	_ = op.Execute(ctx)
	assert.Equal(t, "3", row["slot"])
}

func TestOperators_DefaultValue(t *testing.T) {
	row := map[string]string{"vendor": ""}
	op := &DefaultValueOperator{
		Defaults: map[string]string{"vendor": "HUAWEI", "role": "access"},
	}
	ctx := &OperatorContext{Row: row}
	_ = op.Execute(ctx)
	assert.Equal(t, "HUAWEI", row["vendor"])
	assert.Equal(t, "access", row["role"])
}

func TestOperators_Filter(t *testing.T) {
	row1 := map[string]string{"status": "UP"}
	op := &FilterOperator{FieldName: "status", FilterType: "EQUALS", Value: "UP"}
	ctx1 := &OperatorContext{Row: row1}
	_ = op.Execute(ctx1)
	assert.False(t, ctx1.Dropped)

	row2 := map[string]string{"status": "DOWN"}
	ctx2 := &OperatorContext{Row: row2}
	_ = op.Execute(ctx2)
	assert.True(t, ctx2.Dropped)
}

func TestOperators_SplitAndMerge(t *testing.T) {
	row := map[string]string{"full": "10.1.1.1/24"}
	opSplit := &SplitFieldOperator{
		SrcField:  "full",
		Delimiter: "/",
		DstFields: []string{"ip", "mask"},
	}
	ctx := &OperatorContext{Row: row}
	_ = opSplit.Execute(ctx)
	assert.Equal(t, "10.1.1.1", row["ip"])
	assert.Equal(t, "24", row["mask"])

	opMerge := &MergeFieldOperator{
		SrcFields: []string{"ip", "mask"},
		Delimiter: "#",
		DstField:  "merged",
	}
	_ = opMerge.Execute(ctx)
	assert.Equal(t, "10.1.1.1#24", row["merged"])
}

func TestOperators_ReplaceAll(t *testing.T) {
	row := map[string]string{"text": "foo--bar--baz"}
	re := regexp.MustCompile(`--`)
	op := &ReplaceAllOperator{
		Names:       []string{"text"},
		Regex:       re,
		Replacement: "/",
	}
	ctx := &OperatorContext{Row: row}
	_ = op.Execute(ctx)
	assert.Equal(t, "foo/bar/baz", row["text"])
}

func TestOperators_AssignAndRename(t *testing.T) {
	row := map[string]string{"vlan": "10"}
	opAssign := &AssignOperator{
		AssignMap: map[string]string{"vlanif": "Vlanif#{vlan}"},
	}
	ctx := &OperatorContext{Row: row}
	_ = opAssign.Execute(ctx)
	assert.Equal(t, "Vlanif10", row["vlanif"])

	opRename := &RenameFieldOperator{
		FieldMap: map[string]string{"vlanif": "ifName"},
	}
	_ = opRename.Execute(ctx)
	assert.Equal(t, "Vlanif10", row["ifName"])
	_, hasOld := row["vlanif"]
	assert.False(t, hasOld)
}

func TestOperators_StrConcat(t *testing.T) {
	row := map[string]string{"id": "100"}
	op := &StrConcatOperator{
		SrcField:  "id",
		PrefixStr: "Eth-Trunk",
		SuffixStr: ".1",
		DstField:  "subIf",
	}
	ctx := &OperatorContext{Row: row}
	_ = op.Execute(ctx)
	assert.Equal(t, "Eth-Trunk100.1", row["subIf"])
}
