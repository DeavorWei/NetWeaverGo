package report

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/NetWeaverGo/core/internal/logger"
)

// DetailLogger 记录规范化后的命令回显。
//
// 职责：
// 1. 写命令头
// 2. 写 normalized lines / text
// 3. 换行标准化
// 4. 脱敏
//
// 不得承担：
// 1. ANSI 删除 - 由 terminal.Replayer 处理
// 2. 分页删除 - 由 terminal.Replayer 处理
// 3. 退格删除 - 由 terminal.Replayer 处理
// 4. 终端语义修复 - 由 terminal.Replayer 处理
type DetailLogger struct {
	mu        sync.Mutex
	file      *os.File
	writer    *bufio.Writer
	path      string
	pending   string
	lineCount int
}

// NewDetailLogger 创建详细日志。
func NewDetailLogger(filePath string) (*DetailLogger, error) {
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return nil, err
	}

	file, err := os.Create(filePath)
	if err != nil {
		return nil, err
	}

	return &DetailLogger{
		file:   file,
		writer: bufio.NewWriterSize(file, 32*1024),
		path:   filePath,
	}, nil
}

// WriteCommand 写入发送命令记录（带时间戳）。
func (l *DetailLogger) WriteCommand(cmd string) error {
	normalized := strings.TrimSpace(cmd)
	if normalized == "" {
		normalized = "<enter>"
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	timestampedLine := fmt.Sprintf("[%s] >>> %s", time.Now().Format("15:04:05"), normalized)
	if err := l.writeLineLocked(timestampedLine); err != nil {
		return err
	}
	return l.writer.Flush()
}

// WriteNormalizedText 写入规范化后的文本。
// 这是 detail.log 的主要写入方法，接收已经由 terminal.Replayer 处理过的规范化文本。
func (l *DetailLogger) WriteNormalizedText(text string) error {
	if text == "" {
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// 直接按行写入，不做任何终端修复
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		if err := l.writeLineLocked(line); err != nil {
			return err
		}
	}

	return l.writer.Flush()
}

// WriteNormalizedLines 写入规范化后的行列表。
func (l *DetailLogger) WriteNormalizedLines(lines []string) error {
	if len(lines) == 0 {
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	for _, line := range lines {
		if err := l.writeLineLocked(line); err != nil {
			return err
		}
	}

	return l.writer.Flush()
}

// FlushPending 刷新未完成的尾部内容。
func (l *DetailLogger) FlushPending() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if strings.TrimSpace(l.pending) == "" {
		l.pending = ""
		return l.writer.Flush()
	}

	pending := l.pending
	l.pending = ""
	if err := l.writeLineLocked(pending); err != nil {
		return err
	}

	return l.writer.Flush()
}

// LineCount 返回详细日志行数。
func (l *DetailLogger) LineCount() int {
	if l == nil {
		return 0
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	return l.lineCount
}

// Path 返回日志路径。
func (l *DetailLogger) Path() string {
	if l == nil {
		return ""
	}
	return l.path
}

// Close 关闭句柄。
func (l *DetailLogger) Close() error {
	if l == nil {
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if strings.TrimSpace(l.pending) != "" {
		if err := l.writeLineLocked(l.pending); err != nil {
			_ = l.file.Close()
			l.writer = nil
			l.file = nil
			l.pending = ""
			return err
		}
		l.pending = ""
	}

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

func (l *DetailLogger) writeLine(message string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if err := l.writeLineLocked(message); err != nil {
		return err
	}
	return l.writer.Flush()
}

func (l *DetailLogger) writeLineLocked(message string) error {
	normalized := strings.TrimSpace(message)
	if normalized == "" {
		return nil
	}

	// 统一脱敏处理：设备回显中的敏感信息脱敏
	normalized = logger.Sanitize(normalized)

	// 输出行不再添加时间戳，保持原始内容
	if _, err := l.writer.WriteString(normalized + "\n"); err != nil {
		return err
	}
	l.lineCount++
	return nil
}
