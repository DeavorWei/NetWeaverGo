package report

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDetailLogger_WriteNormalizedText(t *testing.T) {
	// 创建临时目录
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "detail.log")

	logger, err := NewDetailLogger(logPath)
	if err != nil {
		t.Fatalf("NewDetailLogger failed: %v", err)
	}
	defer logger.Close()

	// 测试写入规范化文本
	text := "line1\nline2\nline3"
	if err := logger.WriteNormalizedText(text); err != nil {
		t.Fatalf("WriteNormalizedText failed: %v", err)
	}

	// 验证行数
	if count := logger.LineCount(); count != 3 {
		t.Errorf("LineCount = %d, want 3", count)
	}

	// 读取文件验证内容
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	expected := "line1\nline2\nline3\n"
	if string(content) != expected {
		t.Errorf("Content = %q, want %q", string(content), expected)
	}
}

func TestDetailLogger_WriteNormalizedLines(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "detail.log")

	logger, err := NewDetailLogger(logPath)
	if err != nil {
		t.Fatalf("NewDetailLogger failed: %v", err)
	}
	defer logger.Close()

	lines := []string{"first line", "second line", "third line"}
	if err := logger.WriteNormalizedLines(lines); err != nil {
		t.Fatalf("WriteNormalizedLines failed: %v", err)
	}

	if count := logger.LineCount(); count != 3 {
		t.Errorf("LineCount = %d, want 3", count)
	}

	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	expected := "first line\nsecond line\nthird line\n"
	if string(content) != expected {
		t.Errorf("Content = %q, want %q", string(content), expected)
	}
}

func TestDetailLogger_WriteCommand(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "detail.log")

	logger, err := NewDetailLogger(logPath)
	if err != nil {
		t.Fatalf("NewDetailLogger failed: %v", err)
	}
	defer logger.Close()

	if err := logger.WriteCommand("display version"); err != nil {
		t.Fatalf("WriteCommand failed: %v", err)
	}

	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	// 验证包含时间戳和命令
	contentStr := string(content)
	if !strings.Contains(contentStr, ">>> display version") {
		t.Errorf("Content should contain '>>> display version', got %q", contentStr)
	}
	if !strings.Contains(contentStr, "[") {
		t.Errorf("Content should contain timestamp, got %q", contentStr)
	}
}

func TestDetailLogger_WriteChunk_BackwardCompatible(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "detail.log")

	logger, err := NewDetailLogger(logPath)
	if err != nil {
		t.Fatalf("NewDetailLogger failed: %v", err)
	}
	defer logger.Close()

	// 测试废弃的 WriteChunk 方法仍然可用
	chunk := "line1\r\nline2\rline3\x00line4"
	if err := logger.WriteChunk(chunk); err != nil {
		t.Fatalf("WriteChunk failed: %v", err)
	}

	if err := logger.FlushPending(); err != nil {
		t.Fatalf("FlushPending failed: %v", err)
	}

	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	// 验证换行标准化和空字符删除
	contentStr := string(content)
	if strings.Contains(contentStr, "\r") {
		t.Errorf("Content should not contain CR, got %q", contentStr)
	}
	if strings.Contains(contentStr, "\x00") {
		t.Errorf("Content should not contain null bytes, got %q", contentStr)
	}
}

func TestDetailLogger_NoTerminalRepair(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "detail.log")

	logger, err := NewDetailLogger(logPath)
	if err != nil {
		t.Fatalf("NewDetailLogger failed: %v", err)
	}
	defer logger.Close()

	// 测试 ANSI 转义序列不会被删除（职责已移至 Replayer）
	textWithANSI := "\x1b[32mgreen text\x1b[0m"
	if err := logger.WriteNormalizedText(textWithANSI); err != nil {
		t.Fatalf("WriteNormalizedText failed: %v", err)
	}

	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	// ANSI 序列应该保留（因为 detail_logger 不再负责终端修复）
	contentStr := string(content)
	if !strings.Contains(contentStr, "\x1b[32m") {
		t.Errorf("ANSI escape should be preserved, got %q", contentStr)
	}
}

func TestDetailLogger_EmptyInput(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "detail.log")

	logger, err := NewDetailLogger(logPath)
	if err != nil {
		t.Fatalf("NewDetailLogger failed: %v", err)
	}
	defer logger.Close()

	// 空输入应该被忽略
	if err := logger.WriteNormalizedText(""); err != nil {
		t.Fatalf("WriteNormalizedText('') failed: %v", err)
	}
	if err := logger.WriteNormalizedLines(nil); err != nil {
		t.Fatalf("WriteNormalizedLines(nil) failed: %v", err)
	}

	if count := logger.LineCount(); count != 0 {
		t.Errorf("LineCount = %d, want 0", count)
	}
}

func TestDetailLogger_Sanitization(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "detail.log")

	logger, err := NewDetailLogger(logPath)
	if err != nil {
		t.Fatalf("NewDetailLogger failed: %v", err)
	}
	defer logger.Close()

	// 测试敏感信息脱敏（使用会被脱敏的格式）
	// 规则: cipher_value - (?i)(cipher\s+)(\S+) -> cipher ****
	text := "local-user admin cipher mysecret123"
	if err := logger.WriteNormalizedText(text); err != nil {
		t.Fatalf("WriteNormalizedText failed: %v", err)
	}

	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	// 验证脱敏处理（logger.Sanitize 应该处理 cipher 后的密码）
	contentStr := string(content)
	if strings.Contains(contentStr, "mysecret123") {
		t.Errorf("Cipher password should be sanitized, got %q", contentStr)
	}
	if !strings.Contains(contentStr, "cipher ****") {
		t.Errorf("Expected sanitized output 'cipher ****', got %q", contentStr)
	}
}

func TestDetailLogger_Path(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "subdir", "detail.log")

	logger, err := NewDetailLogger(logPath)
	if err != nil {
		t.Fatalf("NewDetailLogger failed: %v", err)
	}
	defer logger.Close()

	if logger.Path() != logPath {
		t.Errorf("Path() = %q, want %q", logger.Path(), logPath)
	}
}

func TestDetailLogger_NilSafety(t *testing.T) {
	var logger *DetailLogger

	// nil 安全性检查
	if logger.LineCount() != 0 {
		t.Error("nil LineCount should return 0")
	}
	if logger.Path() != "" {
		t.Error("nil Path should return empty string")
	}
	if err := logger.Close(); err != nil {
		t.Errorf("nil Close should return nil, got %v", err)
	}
}

func TestDetailLogger_MultipleWrites(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "detail.log")

	logger, err := NewDetailLogger(logPath)
	if err != nil {
		t.Fatalf("NewDetailLogger failed: %v", err)
	}
	defer logger.Close()

	// 多次写入
	if err := logger.WriteNormalizedText("first"); err != nil {
		t.Fatalf("First write failed: %v", err)
	}
	if err := logger.WriteNormalizedText("second"); err != nil {
		t.Fatalf("Second write failed: %v", err)
	}
	if err := logger.WriteCommand("cmd"); err != nil {
		t.Fatalf("WriteCommand failed: %v", err)
	}
	if err := logger.WriteNormalizedLines([]string{"line1", "line2"}); err != nil {
		t.Fatalf("WriteNormalizedLines failed: %v", err)
	}

	if count := logger.LineCount(); count != 5 { // first, second, cmd, line1, line2
		t.Errorf("LineCount = %d, want 5", count)
	}
}
