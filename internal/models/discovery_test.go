package models

import (
	"testing"
)

func TestRawCommandOutput_GetParseFilePath(t *testing.T) {
	output := &RawCommandOutput{
		FilePath:    "/data/topology/normalized/task1/device1/version.txt",
		RawFilePath: "/data/topology/raw/task1/device1/version.txt",
	}

	// GetParseFilePath 应该返回 FilePath（规范化输出路径）
	parsePath := output.GetParseFilePath()
	if parsePath != output.FilePath {
		t.Errorf("GetParseFilePath() = %q, want %q", parsePath, output.FilePath)
	}
}

func TestRawCommandOutput_GetAuditFilePath(t *testing.T) {
	output := &RawCommandOutput{
		FilePath:    "/data/topology/normalized/task1/device1/version.txt",
		RawFilePath: "/data/topology/raw/task1/device1/version.txt",
	}

	// GetAuditFilePath 应该返回 RawFilePath（原始审计输出路径）
	auditPath := output.GetAuditFilePath()
	if auditPath != output.RawFilePath {
		t.Errorf("GetAuditFilePath() = %q, want %q", auditPath, output.RawFilePath)
	}
}

func TestRawCommandOutput_EmptyPaths(t *testing.T) {
	output := &RawCommandOutput{}

	// 空路径应该返回空字符串
	if parsePath := output.GetParseFilePath(); parsePath != "" {
		t.Errorf("GetParseFilePath() = %q, want empty", parsePath)
	}
	if auditPath := output.GetAuditFilePath(); auditPath != "" {
		t.Errorf("GetAuditFilePath() = %q, want empty", auditPath)
	}
}

func TestRawCommandOutput_NewFields(t *testing.T) {
	output := &RawCommandOutput{
		TaskID:         "task-001",
		DeviceIP:       "192.168.1.1",
		CommandKey:     "version",
		Command:        "display version",
		FilePath:       "/data/normalized/task-001/192.168.1.1/version.txt",
		RawFilePath:    "/data/raw/task-001/192.168.1.1/version.txt",
		Status:         "success",
		RawSize:        1024,
		NormalizedSize: 900,
		LineCount:      50,
		PagerCount:     2,
		EchoConsumed:   true,
		PromptMatched:  true,
	}

	// 验证所有新字段
	if output.RawFilePath == "" {
		t.Error("RawFilePath should not be empty")
	}
	if output.RawSize != 1024 {
		t.Errorf("RawSize = %d, want 1024", output.RawSize)
	}
	if output.NormalizedSize != 900 {
		t.Errorf("NormalizedSize = %d, want 900", output.NormalizedSize)
	}
	if output.LineCount != 50 {
		t.Errorf("LineCount = %d, want 50", output.LineCount)
	}
	if output.PagerCount != 2 {
		t.Errorf("PagerCount = %d, want 2", output.PagerCount)
	}
	if !output.EchoConsumed {
		t.Error("EchoConsumed should be true")
	}
	if !output.PromptMatched {
		t.Error("PromptMatched should be true")
	}
}
