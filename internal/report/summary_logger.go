package report

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// SummaryLogger 记录执行进度、关键事件与用户决策。
type SummaryLogger struct {
	mu     sync.Mutex
	file   *os.File
	writer *bufio.Writer
	path   string
	lines  int
}

// NewSummaryLogger 创建简略日志。
func NewSummaryLogger(filePath string) (*SummaryLogger, error) {
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return nil, err
	}

	file, err := os.Create(filePath)
	if err != nil {
		return nil, err
	}

	return &SummaryLogger{
		file:   file,
		writer: bufio.NewWriterSize(file, 32*1024),
		path:   filePath,
	}, nil
}

// WriteLine 写入单行简略日志。
func (l *SummaryLogger) WriteLine(message string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if _, err := l.writer.WriteString(fmt.Sprintf("[%s] %s\n", time.Now().Format("15:04:05"), message)); err != nil {
		return err
	}
	l.lines++
	return l.writer.Flush()
}

// LineCount 返回简略日志行数。
func (l *SummaryLogger) LineCount() int {
	if l == nil {
		return 0
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.lines
}

// Path 返回日志路径。
func (l *SummaryLogger) Path() string {
	if l == nil {
		return ""
	}
	return l.path
}

// Close 关闭句柄。
func (l *SummaryLogger) Close() error {
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
