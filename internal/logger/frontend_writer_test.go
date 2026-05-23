package logger

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFrontendLogWriter_TruncateAndAppend(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "frontend_writer_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	logPath := filepath.Join(tempDir, "frontend.log")

	// 1. Initial write with truncate = true
	config := DefaultFrontendLogWriterConfig
	config.FlushInterval = 50 * time.Millisecond
	err = InitFrontendLogWriter(logPath, config, true)
	if err != nil {
		t.Fatalf("failed to init log writer: %v", err)
	}

	writer := GetFrontendLogWriter()
	if writer == nil {
		t.Fatalf("writer is nil")
	}

	entry1 := FrontendLogEntry{
		Level:     "info",
		Message:   "First message",
		Timestamp: "2026/05/23 12:00:00",
		Module:    "Test",
	}
	writer.WriteImmediate(entry1)
	CloseFrontendLogWriter()

	// Verify file content
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}
	expected1 := "[2026/05/23 12:00:00] [Info] [Test] First message\n"
	if string(content) != expected1 {
		t.Errorf("expected %q, got %q", expected1, string(content))
	}

	// 2. Re-init with truncate = true (should clear the file)
	err = InitFrontendLogWriter(logPath, config, true)
	if err != nil {
		t.Fatalf("failed to init log writer with truncate: %v", err)
	}
	writer = GetFrontendLogWriter()
	entry2 := FrontendLogEntry{
		Level:     "info",
		Message:   "Second message",
		Timestamp: "2026/05/23 12:00:01",
		Module:    "Test",
	}
	writer.WriteImmediate(entry2)
	CloseFrontendLogWriter()

	// Verify file content (should only have the second message)
	content, err = os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}
	expected2 := "[2026/05/23 12:00:01] [Info] [Test] Second message\n"
	if string(content) != expected2 {
		t.Errorf("expected %q, got %q", expected2, string(content))
	}

	// 3. Re-init with truncate = false (should append)
	err = InitFrontendLogWriter(logPath, config, false)
	if err != nil {
		t.Fatalf("failed to init log writer without truncate: %v", err)
	}
	writer = GetFrontendLogWriter()
	entry3 := FrontendLogEntry{
		Level:     "info",
		Message:   "Third message",
		Timestamp: "2026/05/23 12:00:02",
		Module:    "Test",
	}
	writer.WriteImmediate(entry3)
	CloseFrontendLogWriter()

	// Verify file content (should have second and third messages)
	content, err = os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}
	expected3 := expected2 + "[2026/05/23 12:00:02] [Info] [Test] Third message\n"
	if string(content) != expected3 {
		t.Errorf("expected %q, got %q", expected3, string(content))
	}
}
