package report

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// JournalLogger 记录结构化执行事件，作为投影与排障的事实源。
type JournalLogger struct {
	mu     sync.Mutex
	file   *os.File
	writer *bufio.Writer
	path   string
	lines  int
}

// NewJournalLogger 创建结构化事件日志。
func NewJournalLogger(filePath string) (*JournalLogger, error) {
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return nil, err
	}

	file, err := os.Create(filePath)
	if err != nil {
		return nil, err
	}

	return &JournalLogger{
		file:   file,
		writer: bufio.NewWriterSize(file, 32*1024),
		path:   filePath,
	}, nil
}

// WriteRecord 以 JSON Lines 形式写入单条结构化事件。
func (l *JournalLogger) WriteRecord(record interface{}) error {
	if l == nil {
		return nil
	}

	data, err := json.Marshal(record)
	if err != nil {
		return err
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if _, err := l.writer.Write(data); err != nil {
		return err
	}
	if _, err := l.writer.WriteString("\n"); err != nil {
		return err
	}
	l.lines++
	return l.writer.Flush()
}

// LineCount 返回事件条数。
func (l *JournalLogger) LineCount() int {
	if l == nil {
		return 0
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.lines
}

// Path 返回日志路径。
func (l *JournalLogger) Path() string {
	if l == nil {
		return ""
	}
	return l.path
}

// Close 关闭日志句柄。
func (l *JournalLogger) Close() error {
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
