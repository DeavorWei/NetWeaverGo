package device_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/NetWeaverGo/core/internal/device"
)

func TestGolden_VendorIdentification(t *testing.T) {
	goldenBase := filepath.Join("..", "..", "testdata", "regression", "vendor_golden")

	// 1. Huawei Golden
	hwFile := filepath.Join(goldenBase, "huawei", "display_version", "input.txt")
	hwBytes, err := os.ReadFile(hwFile)
	if err != nil {
		t.Fatalf("读取华为 golden 失败: %v", err)
	}
	hwId, err := device.Identify("huawei", map[string]string{
		"display version": string(hwBytes),
	})
	if err != nil {
		t.Fatalf("华为 golden 识别失败: %v", err)
	}
	if hwId.Model != "CE12804" && hwId.Model != "CE12800" && hwId.Model != "CE12804S" {
		t.Errorf("华为 golden Model = %q, unexpected", hwId.Model)
	}
	if hwId.Series != "CE12800" {
		t.Errorf("华为 golden Series = %q, want CE12800", hwId.Series)
	}
	if hwId.Version != "V200R005C10SPC607B607" {
		t.Errorf("华为 golden Version = %q, want V200R005C10SPC607B607", hwId.Version)
	}

	// 2. H3C Golden
	h3cFile := filepath.Join(goldenBase, "h3c", "display_version", "input.txt")
	h3cBytes, err := os.ReadFile(h3cFile)
	if err != nil {
		t.Fatalf("读取 H3C golden 失败: %v", err)
	}
	h3cId, err := device.Identify("h3c", map[string]string{
		"display version": string(h3cBytes),
	})
	if err != nil {
		t.Fatalf("H3C golden 识别失败: %v", err)
	}
	if h3cId.Model != "S5560X" && h3cId.Model != "S5560" {
		t.Errorf("H3C golden Model = %q, unexpected", h3cId.Model)
	}
	if h3cId.Version == "" {
		t.Errorf("H3C golden Version 不应为空")
	}

	// 3. Cisco Golden
	ciscoFile := filepath.Join(goldenBase, "cisco", "show_version", "input.txt")
	ciscoBytes, err := os.ReadFile(ciscoFile)
	if err != nil {
		t.Fatalf("读取 Cisco golden 失败: %v", err)
	}
	ciscoId, err := device.Identify("cisco", map[string]string{
		"show version": string(ciscoBytes),
	})
	if err != nil {
		t.Fatalf("Cisco golden 识别失败: %v", err)
	}
	if ciscoId.Version != "17.06.03" {
		t.Errorf("Cisco golden Version = %q, want 17.06.03", ciscoId.Version)
	}
}
