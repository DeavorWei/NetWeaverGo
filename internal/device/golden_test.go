package device

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// updateGolden 仅在 *_test.go 中注册，避免调试用 flag 污染生产二进制（方案 C1）。
var updateGolden = flag.Bool("update", false, "update golden files")

func shouldUpdateGolden() bool {
	return *updateGolden || os.Getenv("UPDATE_GOLDEN") == "1"
}

// collectDeviceSamples 扫描 testdata/device/versions 下的样本：
// - <vendor>_<model>.txt            → display version 回显
// - <vendor>_<model>.patch.txt      → 可选的 display patch-information 回显
func collectDeviceSamples(t *testing.T) map[string]*Identity {
	t.Helper()

	dir := filepath.Join("..", "..", "testdata", "device", "versions")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("读取设备样本目录失败: %v", err)
	}

	result := make(map[string]*Identity)
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".txt") || strings.HasSuffix(name, ".patch.txt") {
			continue
		}
		base := strings.TrimSuffix(name, ".txt")
		vendor := strings.SplitN(base, "_", 2)[0]

		verData, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("读取样本 %s 失败: %v", name, err)
		}
		raws := map[string]string{"version": string(verData)}
		if patchData, patchErr := os.ReadFile(filepath.Join(dir, base+".patch.txt")); patchErr == nil {
			raws["patch"] = string(patchData)
		}

		id, idErr := Identify(vendor, raws)
		if idErr != nil {
			t.Fatalf("样本 %s 识别失败: %v", base, idErr)
		}
		result[base] = NormalizeIdentityForGolden(id)
	}
	return result
}

// TestIdentify_Golden 对 testdata/device/versions 下全部样本做身份 golden 基准比对
func TestIdentify_Golden(t *testing.T) {
	got := collectDeviceSamples(t)
	if len(got) == 0 {
		t.Fatal("未发现任何设备样本，请检查 testdata/device/versions 目录")
	}

	path := filepath.Join("..", "..", "testdata", "device", "identities.json")
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

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取 golden 失败（可先执行 UPDATE_GOLDEN=1 go test ./internal/device/）: %v", err)
	}
	var want map[string]*Identity
	if err := json.Unmarshal(data, &want); err != nil {
		t.Fatalf("解析 golden 失败: %v", err)
	}

	gotJSON, _ := json.Marshal(got)
	wantJSON, _ := json.Marshal(want)
	if string(gotJSON) != string(wantJSON) {
		t.Fatalf("设备身份 golden 不匹配\nwant=%s\ngot=%s", wantJSON, gotJSON)
	}
}

// TestIdentify_Golden_SeriesNormalized 验证系列归一化的关键契约（与 golden 解耦的显式断言）
func TestIdentify_Golden_SeriesNormalized(t *testing.T) {
	cases := []struct {
		sample string
		series string
	}{
		{"huawei_s5735", "S5700"},
		{"huawei_ce6866", "CE6800"},
		{"huawei_ce16800", "CE16800"},
		{"huawei_ar6280", "AR6000"},
	}
	got := collectDeviceSamples(t)
	for _, c := range cases {
		id, ok := got[c.sample]
		if !ok {
			t.Fatalf("缺少样本 %s", c.sample)
		}
		if id.Series != c.series {
			t.Errorf("样本 %s 的 Series = %q, want %q", c.sample, id.Series, c.series)
		}
	}
}

// TestIdentify_Golden_SampleCoverage 样本规模与关键款型提取的显式契约（与 golden 文件解耦）
func TestIdentify_Golden_SampleCoverage(t *testing.T) {
	got := collectDeviceSamples(t)

	// 1. 样本规模：方案要求 20+ 款型
	if len(got) < 20 {
		t.Fatalf("设备样本应达到 20+ 款型，当前仅 %d", len(got))
	}

	// 2. Cisco 硬件款型可提取（样本必须包含 "cisco <model> (<arch>) processor" 行）
	c9300, ok := got["cisco_c9300"]
	if !ok {
		t.Fatal("缺少 cisco_c9300 样本")
	}
	if c9300.Model == "" {
		t.Error("cisco_c9300 应能提取硬件款型，请确认样本包含 cisco X (Y) processor 行")
	}

	// 3. 未注册厂商绝不误套内置（华为）画像
	for _, name := range []string{"ruijie_s5750", "zte_zxr10"} {
		id, ok := got[name]
		if !ok {
			t.Fatalf("缺少未注册厂商样本 %s", name)
		}
		if id.Series != "" {
			t.Errorf("%s 不应命中任何内置系列画像，实际 series=%q", name, id.Series)
		}
	}
}
