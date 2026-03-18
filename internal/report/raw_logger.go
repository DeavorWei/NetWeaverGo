package report

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// RawLogger 记录完整 SSH 字节流。
type RawLogger struct {
	mu     sync.Mutex
	file   *os.File
	writer *bufio.Writer
	path   string
}

// NewRawLogger 创建原始日志。
func NewRawLogger(filePath string) (*RawLogger, error) {
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return nil, err
	}

	file, err := os.Create(filePath)
	if err != nil {
		return nil, err
	}

	return &RawLogger{
		file:   file,
		writer: bufio.NewWriterSize(file, 32*1024),
		path:   filePath,
	}, nil
}

// Write 写入原始字节。
func (l *RawLogger) Write(p []byte) (int, error) {
	if l == nil {
		return len(p), nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	n, err := l.writer.Write(p)
	if err != nil {
		return n, err
	}
	return n, l.writer.Flush()
}

// WriteMarker 写入结构化标记。
func (l *RawLogger) WriteMarker(format string, args ...interface{}) {
	if l == nil {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	_, _ = fmt.Fprintf(l.writer, format, args...)
	_ = l.writer.Flush()
}

// Path 返回日志路径。
func (l *RawLogger) Path() string {
	if l == nil {
		return ""
	}
	return l.path
}

// Close 关闭句柄。
func (l *RawLogger) Close() error {
	if l == nil {
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.writer != nil {
		if err := l.writer.Flush(); err != nil {
			_ = l.file.Close()
			l.writer = nil
			l.file = nil
			return err
		}
		l.writer = nil
	}

	if l.file != nil {
		err := l.file.Close()
		l.file = nil
		return err
	}
	return nil
}
