package parser

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

// TestHuaweiGoldenParsedFacts 使用正则解析器的 golden 测试
func TestHuaweiGoldenParsedFacts(t *testing.T) {
	// 初始化新解析器管理器
	manager := NewParserManager()
	if err := manager.Bootstrap(); err != nil {
		t.Fatalf("解析器管理器启动失败: %v", err)
	}

	// 获取华为解析器
	parser, err := manager.GetParser("huawei")
	if err != nil {
		t.Fatalf("获取华为解析器失败: %v", err)
	}

	mapper := NewHuaweiMapper()

	lldpRawPath := filepath.Join("..", "..", "testdata", "huawei", "raw", "lldp_neighbor_verbose.txt")
	lldpRaw, err := os.ReadFile(lldpRawPath)
	if err != nil {
		t.Fatalf("read lldp raw failed: %v", err)
	}

	lldpRows, err := parser.Parse("lldp_neighbor", string(lldpRaw))
	if err != nil {
		t.Fatalf("parse lldp raw failed: %v", err)
	}
	gotLLDP, err := mapper.ToLLDP(lldpRows)
	if err != nil {
		t.Fatalf("map lldp failed: %v", err)
	}

	var wantLLDP []LLDPFact
	lldpExpectedPath := filepath.Join("..", "..", "testdata", "huawei", "parsed", "lldp_facts.json")
	if err := loadJSON(lldpExpectedPath, &wantLLDP); err != nil {
		t.Fatalf("load expected lldp failed: %v", err)
	}
	if !reflect.DeepEqual(gotLLDP, wantLLDP) {
		t.Fatalf("lldp golden mismatch\nwant=%+v\ngot=%+v", wantLLDP, gotLLDP)
	}

	aggRawPath := filepath.Join("..", "..", "testdata", "huawei", "raw", "eth_trunk.txt")
	aggRaw, err := os.ReadFile(aggRawPath)
	if err != nil {
		t.Fatalf("read aggregate raw failed: %v", err)
	}

	aggRows, err := parser.Parse("eth_trunk", string(aggRaw))
	if err != nil {
		t.Fatalf("parse aggregate raw failed: %v", err)
	}
	gotAgg, err := mapper.ToAggregate(aggRows)
	if err != nil {
		t.Fatalf("map aggregate failed: %v", err)
	}

	var wantAgg []AggregateFact
	aggExpectedPath := filepath.Join("..", "..", "testdata", "huawei", "parsed", "aggregate_facts.json")
	if err := loadJSON(aggExpectedPath, &wantAgg); err != nil {
		t.Fatalf("load expected aggregate failed: %v", err)
	}
	sortAggregateFacts(gotAgg)
	sortAggregateFacts(wantAgg)
	if !reflect.DeepEqual(gotAgg, wantAgg) {
		t.Fatalf("aggregate golden mismatch\nwant=%+v\ngot=%+v", wantAgg, gotAgg)
	}
}

// TestHuaweiGoldenTreeParser 使用规则树引擎解析华为 display device 回显并验证 Golden 期望
func TestHuaweiGoldenTreeParser(t *testing.T) {
	rawPath := filepath.Join("..", "..", "testdata", "parser", "tree", "display_device.txt")
	rawBytes, err := os.ReadFile(rawPath)
	if err != nil {
		t.Fatalf("读取测试回显失败: %v", err)
	}

	expectedPath := filepath.Join("..", "..", "testdata", "parser", "tree", "display_device_expected.json")
	var expected []map[string]string
	if err := loadJSON(expectedPath, &expected); err != nil {
		t.Fatalf("读取期望结果失败: %v", err)
	}

	// 规则定义：先按槽位号拆分（Slot 1、Slot 2），再匹配每行子卡
	rules := []TreeRule{
		{
			ParseItem:  "slot",
			ParentItem: "",
			IsList:     true,
			SplitRegex: `(?m)^(\d+)\s+`,
			GroupIndex: 1,
			IsOutput:   true,
			Order:      1,
		},
		{
			ParseItem:  "subCard",
			ParentItem: "slot",
			IsList:     true,
			ParseRegex: `(?m)^(?:\d+|\s+)\s+(\S+)\s+(\S+)\s+\S+\s+\S+\s+\S+\s+(\S+)\s+(\S+)`,
			IsOutput:   false,
			Order:      2,
		},
		{
			ParseItem:  "sub",
			ParentItem: "subCard",
			IsList:     false,
			ParseRegex: `(?m)^(?:\d+|\s+)\s+(\S+)`,
			GroupIndex: 1,
			IsOutput:   true,
			Order:      3,
		},
		{
			ParseItem:  "type",
			ParentItem: "subCard",
			IsList:     false,
			ParseRegex: `(?m)^(?:\d+|\s+)\s+\S+\s+(\S+)`,
			GroupIndex: 1,
			IsOutput:   true,
			Order:      4,
		},
		{
			ParseItem:  "status",
			ParentItem: "subCard",
			IsList:     false,
			ParseRegex: `(?m)^(?:\d+|\s+)\s+\S+\s+\S+\s+\S+\s+\S+\s+\S+\s+(\S+)`,
			GroupIndex: 1,
			IsOutput:   true,
			Order:      5,
		},
		{
			ParseItem:  "role",
			ParentItem: "subCard",
			IsList:     false,
			ParseRegex: `(?m)^(?:\d+|\s+)\s+\S+\s+\S+\s+\S+\s+\S+\s+\S+\s+\S+\s+(\S+)`,
			GroupIndex: 1,
			IsOutput:   true,
			Order:      6,
		},
	}

	compiledRules, rootRules, err := CompileTreeRules(rules)
	if err != nil {
		t.Fatalf("编译规则树失败: %v", err)
	}

	tpl := &CompiledTemplate{
		RegexTemplate: RegexTemplate{
			CommandKey: "display_device",
			Engine:     EngineTree,
			TreeConfig: &TreeTemplate{Rules: rules},
		},
		CompiledTreeRules: compiledRules,
		TreeRootRules:     rootRules,
	}

	engine := NewTreeEngine()
	rows, err := engine.ParseWithTemplate(tpl, string(rawBytes))
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}

	if len(rows) != len(expected) {
		t.Fatalf("行数不匹配: got %d, want %d\ngot: %+v", len(rows), len(expected), rows)
	}

	for i := range expected {
		for k, wantVal := range expected[i] {
			if gotVal := rows[i][k]; gotVal != wantVal {
				t.Fatalf("行 %d 字段 %s 不匹配: got %s, want %s", i, k, gotVal, wantVal)
			}
		}
	}
}

// TestHuaweiGoldenTreeParser_TableStyle 表格式回显 Golden 测试（display interface brief，验证场景 B 列表多匹配）
func TestHuaweiGoldenTreeParser_TableStyle(t *testing.T) {
	rawPath := filepath.Join("..", "..", "testdata", "parser", "tree", "display_interface_brief.txt")
	rawBytes, err := os.ReadFile(rawPath)
	if err != nil {
		t.Fatalf("读取测试回显失败: %v", err)
	}

	expectedPath := filepath.Join("..", "..", "testdata", "parser", "tree", "display_interface_brief_expected.json")
	var expected []map[string]string
	if err := loadJSON(expectedPath, &expected); err != nil {
		t.Fatalf("读取期望结果失败: %v", err)
	}

	// 规则定义：场景 B（列表节点通过 ParseRegex 逐行匹配，无 splitRegex）
	rules := []TreeRule{
		{
			ParseItem:  "portRow",
			ParentItem: "",
			IsList:     true,
			ParseRegex: `(?m)^([A-Za-z0-9/]+)\s+(\*?[a-z]+)\s+([a-z]+)\s+\S+\s+\S+\s+(\d+)\s+(\d+)`,
			IsOutput:   false,
			Order:      1,
		},
		{
			ParseItem:  "interface",
			ParentItem: "portRow",
			IsList:     false,
			ParseRegex: `(?m)^([A-Za-z0-9/]+)`,
			GroupIndex: 1,
			IsOutput:   true,
			Order:      2,
		},
		{
			ParseItem:  "phy",
			ParentItem: "portRow",
			IsList:     false,
			ParseRegex: `(?m)^[A-Za-z0-9/]+\s+(\*?[a-z]+)`,
			GroupIndex: 1,
			IsOutput:   true,
			Order:      3,
		},
		{
			ParseItem:  "protocol",
			ParentItem: "portRow",
			IsList:     false,
			ParseRegex: `(?m)^[A-Za-z0-9/]+\s+\*?[a-z]+\s+([a-z]+)`,
			GroupIndex: 1,
			IsOutput:   true,
			Order:      4,
		},
		{
			ParseItem:  "inErrors",
			ParentItem: "portRow",
			IsList:     false,
			ParseRegex: `(?m)^[A-Za-z0-9/]+\s+\*?[a-z]+\s+[a-z]+\s+\S+\s+\S+\s+(\d+)`,
			GroupIndex: 1,
			IsOutput:   true,
			Order:      5,
		},
		{
			ParseItem:  "outErrors",
			ParentItem: "portRow",
			IsList:     false,
			ParseRegex: `(?m)^[A-Za-z0-9/]+\s+\*?[a-z]+\s+[a-z]+\s+\S+\s+\S+\s+\d+\s+(\d+)`,
			GroupIndex: 1,
			IsOutput:   true,
			Order:      6,
		},
	}

	compiledRules, rootRules, err := CompileTreeRules(rules)
	if err != nil {
		t.Fatalf("编译表格式规则树失败: %v", err)
	}

	tpl := &CompiledTemplate{
		RegexTemplate: RegexTemplate{
			CommandKey: "display_interface_brief",
			Engine:     EngineTree,
			TreeConfig: &TreeTemplate{Rules: rules},
		},
		CompiledTreeRules: compiledRules,
		TreeRootRules:     rootRules,
	}

	engine := NewTreeEngine()
	rows, err := engine.ParseWithTemplate(tpl, string(rawBytes))
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}

	if len(rows) != len(expected) {
		t.Fatalf("行数不匹配: got %d, want %d\ngot: %+v", len(rows), len(expected), rows)
	}

	for i := range expected {
		for k, wantVal := range expected[i] {
			if gotVal := rows[i][k]; gotVal != wantVal {
				t.Fatalf("行 %d 字段 %s 不匹配: got %s, want %s", i, k, gotVal, wantVal)
			}
		}
	}
}

// TestHuaweiGoldenTreeParser_MultiLevelNested 三层多级嵌套 Golden 测试（display vlan）
func TestHuaweiGoldenTreeParser_MultiLevelNested(t *testing.T) {
	rawPath := filepath.Join("..", "..", "testdata", "parser", "tree", "display_vlan.txt")
	rawBytes, err := os.ReadFile(rawPath)
	if err != nil {
		t.Fatalf("读取测试回显失败: %v", err)
	}

	expectedPath := filepath.Join("..", "..", "testdata", "parser", "tree", "display_vlan_expected.json")
	var expected []map[string]string
	if err := loadJSON(expectedPath, &expected); err != nil {
		t.Fatalf("读取期望结果失败: %v", err)
	}

	// 规则定义：三层嵌套
	// 根规则：VLAN 分块（SplitRegex: `(?m)^vlan\s+(\d+)`）
	// 子规则 1：description（单值标量，默认值 N/A，验证 M1）
	// 子规则 2：port 列表（多匹配列表：`(?m)^\s+port\s+(\S+)`）
	// 子规则 3：linkType（挂在 port 下的标量：`(?m)^\s+port link-type\s+(\S+)`）
	rules := []TreeRule{
		{
			ParseItem:  "vlanBlock",
			ParentItem: "",
			IsList:     true,
			SplitRegex: `(?m)^vlan\s+(\d+)`,
			GroupIndex: 1,
			IsOutput:   false,
			Order:      1,
		},
		{
			ParseItem:  "vlanId",
			ParentItem: "vlanBlock",
			IsList:     false,
			ParseRegex: `(?m)^vlan\s+(\d+)`,
			GroupIndex: 1,
			IsOutput:   true,
			Order:      2,
		},
		{
			ParseItem:    "description",
			ParentItem:   "vlanBlock",
			IsList:       false,
			ParseRegex:   `(?m)^\s+description\s+(\S+)`,
			GroupIndex:   1,
			IsOutput:     true,
			DefaultValue: "N/A",
			Order:        3,
		},
		{
			ParseItem:  "portName",
			ParentItem: "vlanBlock",
			IsList:     true,
			SplitRegex: `(?m)^\s+port\s+(GigabitEthernet\S+)`,
			GroupIndex: 1,
			IsOutput:   true,
			Order:      4,
		},
		{
			ParseItem:  "linkType",
			ParentItem: "portName",
			IsList:     false,
			ParseRegex: `(?m)^\s+port link-type\s+(\S+)`,
			GroupIndex: 1,
			IsOutput:   true,
			Order:      5,
		},
	}

	compiledRules, rootRules, err := CompileTreeRules(rules)
	if err != nil {
		t.Fatalf("编译多级嵌套规则树失败: %v", err)
	}

	tpl := &CompiledTemplate{
		RegexTemplate: RegexTemplate{
			CommandKey: "display_vlan",
			Engine:     EngineTree,
			TreeConfig: &TreeTemplate{Rules: rules},
		},
		CompiledTreeRules: compiledRules,
		TreeRootRules:     rootRules,
	}

	engine := NewTreeEngine()
	rows, err := engine.ParseWithTemplate(tpl, string(rawBytes))
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}

	if len(rows) != len(expected) {
		t.Fatalf("行数不匹配: got %d, want %d\ngot: %+v", len(rows), len(expected), rows)
	}

	for i := range expected {
		for k, wantVal := range expected[i] {
			if gotVal := rows[i][k]; gotVal != wantVal {
				t.Fatalf("行 %d 字段 %s 不匹配: got %s, want %s", i, k, gotVal, wantVal)
			}
		}
	}
}

func sortAggregateFacts(items []AggregateFact) {
	sort.Slice(items, func(i, j int) bool {
		return items[i].AggregateName < items[j].AggregateName
	})
}

func loadJSON(path string, target interface{}) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}
