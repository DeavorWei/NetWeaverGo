package ceas

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

// updateGolden 仅在 *_test.go 中注册，避免调试用 flag 污染生产二进制（方案 C1）。
var updateGolden = flag.Bool("update", false, "update golden files")

// shouldUpdateGolden 双通道刷新判定：
// - 单包调试：go test ./internal/ceas/ -run TestParseELabel_Golden -args -update
// - 全仓/CI：UPDATE_GOLDEN=1 go test ./...
func shouldUpdateGolden() bool {
	return *updateGolden || os.Getenv("UPDATE_GOLDEN") == "1"
}

func goldenPath(family string) string {
	return filepath.Join("..", "..", "testdata", "ceas", family+"_elabel_expected.json")
}

// TestParseELabel_Golden 对 6 大产品族的 display elabel 样本做 golden 基准比对
func TestParseELabel_Golden(t *testing.T) {
	families := []string{"ce", "sw", "ar", "wlan", "fw", "route"}

	for _, family := range families {
		t.Run(family, func(t *testing.T) {
			raw := readSample(t, family+"_elabel.txt")
			got := NormalizeTreeForGolden(ParseELabel(raw))
			path := goldenPath(family)

			if shouldUpdateGolden() {
				data, err := json.MarshalIndent(got, "", "  ")
				if err != nil {
					t.Fatalf("序列化 golden 失败: %v", err)
				}
				if err := os.WriteFile(path, append(data, '\n'), 0644); err != nil {
					t.Fatalf("写入 golden 失败: %v", err)
				}
				return
			}

			var want HardwareTree
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("读取 golden 失败（可先执行 UPDATE_GOLDEN=1 go test ./internal/ceas/）: %v", err)
			}
			if err := json.Unmarshal(data, &want); err != nil {
				t.Fatalf("解析 golden 失败: %v", err)
			}

			gotJSON, _ := json.Marshal(got)
			wantJSON, _ := json.Marshal(&want)
			if string(gotJSON) != string(wantJSON) {
				t.Fatalf("%s 产品族 golden 不匹配\nwant=%s\ngot=%s", family, wantJSON, gotJSON)
			}
		})
	}
}

// TestNormalizeTreeForGolden_Stable 验证规范化结果在多次运行间稳定（无 map 乱序抖动）
func TestNormalizeTreeForGolden_Stable(t *testing.T) {
	raw := readSample(t, "route_elabel.txt")

	first, _ := json.Marshal(NormalizeTreeForGolden(ParseELabel(raw)))
	for i := 0; i < 20; i++ {
		next, _ := json.Marshal(NormalizeTreeForGolden(ParseELabel(raw)))
		if string(next) != string(first) {
			t.Fatalf("第 %d 次规范化结果与首次不一致，存在排序抖动", i+1)
		}
	}
}
