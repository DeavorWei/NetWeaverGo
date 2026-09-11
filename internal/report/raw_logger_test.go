package report

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRawLogger_CrossChunkSanitization(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test_raw.log")

	logger, err := NewRawLogger(logPath)
	if err != nil {
		t.Fatalf("创建 RawLogger 失败: %v", err)
	}

	// 敏感口令跨两个 chunk 到达:
	// chunk 1: "user admin password cipher adm"
	// chunk 2: "in123\n"
	chunk1 := []byte("user admin password cipher adm")
	chunk2 := []byte("in123\n")

	if _, err := logger.Write(chunk1); err != nil {
		t.Fatalf("写入 chunk1 失败: %v", err)
	}
	if _, err := logger.Write(chunk2); err != nil {
		t.Fatalf("写入 chunk2 失败: %v", err)
	}

	// 写入一个尾部无换行的数据，通过 Close() 冲刷
	tailChunk := []byte("enable secret sec" + "ret999")
	if _, err := logger.Write(tailChunk); err != nil {
		t.Fatalf("写入 tailChunk 失败: %v", err)
	}

	if err := logger.Close(); err != nil {
		t.Fatalf("关闭 RawLogger 失败: %v", err)
	}

	contentBytes, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("读取测试日志文件失败: %v", err)
	}
	content := string(contentBytes)

	// 验证敏感明文口令未泄露
	if strings.Contains(content, "admin123") {
		t.Errorf("跨 chunk 脱敏失败，文件中发现明文口令 'admin123'，内容: %q", content)
	}
	if strings.Contains(content, "secret999") {
		t.Errorf("Close 冲刷脱敏失败，文件中发现明文口令 'secret999'，内容: %q", content)
	}

	// 验证包含脱敏标记
	if !strings.Contains(content, "****") {
		t.Errorf("文件中未发现脱敏掩码 '****'，内容: %q", content)
	}
}
