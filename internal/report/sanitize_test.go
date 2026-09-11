package report

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRawLogger_Sanitize(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "raw_log_test_*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	logPath := filepath.Join(tempDir, "raw.log")
	rawLogger, err := NewRawLogger(logPath)
	if err != nil {
		t.Fatalf("创建 RawLogger 失败: %v", err)
	}

	// 写入带明文或密文口令的设备回显
	sensitiveStream := "user admin\n password simple Huawei@123\n password cipher $c$3$xyz123==\n quit\n"
	_, err = rawLogger.Write([]byte(sensitiveStream))
	if err != nil {
		t.Fatalf("写入原始日志失败: %v", err)
	}
	_ = rawLogger.Close()

	// 读取落盘文件验证脱敏
	contentBytes, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("读取日志文件失败: %v", err)
	}
	content := string(contentBytes)

	if strings.Contains(content, "Huawei@123") {
		t.Errorf("原始日志未能脱敏明文口令: %s", content)
	}
	if strings.Contains(content, "$c$3$xyz123==") {
		t.Errorf("原始日志未能脱敏 cipher 密文口令: %s", content)
	}
	if !strings.Contains(content, "****") {
		t.Errorf("脱敏后的文本中应包含 **** 遮掩符: %s", content)
	}
}

func TestJournalLogger_Sanitize(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "journal_log_test_*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	logPath := filepath.Join(tempDir, "journal.jsonl")
	jLogger, err := NewJournalLogger(logPath)
	if err != nil {
		t.Fatalf("创建 JournalLogger 失败: %v", err)
	}

	record := map[string]interface{}{
		"event":    "device_auth",
		"username": "admin",
		"password": "SuperSecretPassword999",
	}

	if err := jLogger.WriteRecord(record); err != nil {
		t.Fatalf("写入 Journal 失败: %v", err)
	}
	_ = jLogger.Close()

	contentBytes, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("读取 Journal 文件失败: %v", err)
	}
	content := string(contentBytes)

	if strings.Contains(content, "SuperSecretPassword999") {
		t.Errorf("Journal 日志中暴露了未脱敏的口令: %s", content)
	}
	if !strings.Contains(content, "****") {
		t.Errorf("Journal 日志中应包含 **** 遮掩符: %s", content)
	}
}
