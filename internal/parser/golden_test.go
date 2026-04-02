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
