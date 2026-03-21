package terminal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestVendorGoldenSamples 测试厂商特定的 golden 样本
// 验证各厂商设备输出的终端语义处理是否正确
func TestVendorGoldenSamples(t *testing.T) {
	vendorGoldenDir := "../../testdata/regression/vendor_golden"

	// 检查目录是否存在
	if _, err := os.Stat(vendorGoldenDir); os.IsNotExist(err) {
		t.Skip("厂商 golden 测试目录不存在")
	}

	// 定义厂商列表
	vendors := []string{"huawei", "h3c", "cisco"}

	for _, vendor := range vendors {
		vendorDir := filepath.Join(vendorGoldenDir, vendor)

		// 检查厂商目录是否存在
		if _, err := os.Stat(vendorDir); os.IsNotExist(err) {
			continue
		}

		// 遍历厂商下的所有测试案例
		entries, err := os.ReadDir(vendorDir)
		if err != nil {
			t.Fatalf("读取厂商目录失败 %s: %v", vendor, err)
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}

			caseName := entry.Name()
			caseDir := filepath.Join(vendorDir, caseName)

			t.Run(vendor+"/"+caseName, func(t *testing.T) {
				inputPath := filepath.Join(caseDir, "input.txt")
				expectedPath := filepath.Join(caseDir, "expected.txt")

				// 读取输入
				inputData, err := os.ReadFile(inputPath)
				if err != nil {
					t.Fatalf("读取输入文件失败: %v", err)
				}

				// 读取期望输出
				expectedData, err := os.ReadFile(expectedPath)
				if err != nil {
					t.Fatalf("读取期望文件失败: %v", err)
				}

				// 使用 Replayer 处理输入
				replayer := NewReplayer(80)
				replayer.Process(string(inputData))
				result := replayer.Lines()

				// 比较结果 - 统一行尾格式为 LF
				expected := strings.TrimSpace(strings.ReplaceAll(string(expectedData), "\r\n", "\n"))
				actual := strings.TrimSpace(strings.Join(result, "\n"))

				if actual != expected {
					// 提供更详细的调试信息
					t.Errorf("输出不匹配\n期望长度: %d, 实际长度: %d", len(expected), len(actual))

					// 逐行比较找出差异
					expectedLines := strings.Split(expected, "\n")
					actualLines := strings.Split(actual, "\n")

					maxLen := len(expectedLines)
					if len(actualLines) > maxLen {
						maxLen = len(actualLines)
					}

					for i := 0; i < maxLen; i++ {
						var expLine, actLine string
						if i < len(expectedLines) {
							expLine = expectedLines[i]
						}
						if i < len(actualLines) {
							actLine = actualLines[i]
						}

						if expLine != actLine {
							t.Errorf("行 %d 不匹配:\n期望: %q (len=%d)\n实际: %q (len=%d)", i+1, expLine, len(expLine), actLine, len(actLine))
						}
					}
				}
			})
		}
	}
}
