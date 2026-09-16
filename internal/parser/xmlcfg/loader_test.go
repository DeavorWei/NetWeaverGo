package xmlcfg

import (
	"encoding/json"
	"encoding/xml"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestXmlConfig_LoadAndScan(t *testing.T) {
	xmlDir := filepath.Join("..", "templates", "parsecfg", "xmlconfig")
	if _, err := os.Stat(xmlDir); os.IsNotExist(err) {
		t.Skipf("目录不存在: %s", xmlDir)
	}

	res, err := LoadAndScanDir(xmlDir)
	require.NoError(t, err)

	t.Logf("扫描统计:")
	t.Logf("  总 XML 规则文件数: %d", res.TotalFiles)
	t.Logf("  成功反序列化数: %d (成功率: %.2f%%)", res.LoadedFiles, float64(res.LoadedFiles)*100/float64(res.TotalFiles))
	t.Logf("  总提取正则数: %d", res.TotalPatterns)
	t.Logf("  编译成功正则数: %d (通过率: %.2f%%)", res.CompiledPatterns, float64(res.CompiledPatterns)*100/float64(res.TotalPatterns))
	t.Logf("  自动语法适配改写数: %d", res.RewrittenCount)
	t.Logf("  编译失败正则数: %d", len(res.BrokenList))

	// 验收标准 1: XML 反序列化率 100%
	assert.Equal(t, res.TotalFiles, res.LoadedFiles, "XML 反序列化成功率必须达到 100%")

	// 验收标准 2: 正则编译成功率 ≥ 90%
	compileRate := float64(res.CompiledPatterns) * 100 / float64(res.TotalPatterns)
	assert.GreaterOrEqual(t, compileRate, 90.0, "RE2 正则编译通过率必须 ≥ 90%")

	// 输出不兼容清单到 parsecfg_broken.json 供启动告警与审计使用
	if len(res.BrokenList) > 0 {
		outBrokenPath := filepath.Join("..", "templates", "parsecfg", "parsecfg_broken.json")
		b, _ := json.MarshalIndent(res.BrokenList, "", "  ")
		_ = os.WriteFile(outBrokenPath, b, 0644)
		t.Logf("已生成 RE2 不兼容正则清单: %s (共 %d 项)", outBrokenPath, len(res.BrokenList))
	}
}

func TestParseItem_Load(t *testing.T) {
	parseItemDir := filepath.Join("..", "templates", "parsecfg", "parseitem")
	if _, err := os.Stat(parseItemDir); os.IsNotExist(err) {
		t.Skipf("parseitem 目录不存在: %s", parseItemDir)
	}

	entries, err := os.ReadDir(parseItemDir)
	require.NoError(t, err)

	count := 0
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".xml" {
			data, readErr := os.ReadFile(filepath.Join(parseItemDir, e.Name()))
			require.NoError(t, readErr)

			var item CommonParseItem
			unmarshalErr := xml.Unmarshal(data, &item)
			require.NoErrorf(t, unmarshalErr, "解析 %s 失败", e.Name())
			assert.NotEmpty(t, item.VendorCommons, "%s 必须包含 VendorCommon 节点", e.Name())
			count++
		}
	}

	assert.Equal(t, 6, count, "应当成功解析 6 份 parseitem 规则")
}
