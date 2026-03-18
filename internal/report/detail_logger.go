package report

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

var (
	ansiEscapeRegex  = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)
	paginationRegex  = regexp.MustCompile(`(?i)-+\s*more(?:\s*\([^)]*\))?\s*-+`)
	promptTokenRegex = regexp.MustCompile(`^<[^<>\s]+>`)
)

// DetailLogger 记录清洗后的命令回显。
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

// WriteChunk 写入原始输出块，并做清洗。
func (l *DetailLogger) WriteChunk(chunk string) error {
	cleaned := cleanDetailChunk(chunk)
	if cleaned == "" {
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	l.pending += cleaned
	lines := strings.Split(l.pending, "\n")
	l.pending = lines[len(lines)-1]

	for _, line := range lines[:len(lines)-1] {
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
	normalized := normalizePromptArtifacts(message)
	normalized = strings.TrimSpace(normalized)
	if normalized == "" {
		return nil
	}

	// 输出行不再添加时间戳，保持原始内容
	if _, err := l.writer.WriteString(normalized + "\n"); err != nil {
		return err
	}
	l.lineCount++
	return nil
}

func cleanDetailChunk(chunk string) string {
	cleaned := ansiEscapeRegex.ReplaceAllString(chunk, "")
	cleaned = strings.ReplaceAll(cleaned, "\r\n", "\n")
	cleaned = strings.ReplaceAll(cleaned, "\r", "\n")
	cleaned = strings.ReplaceAll(cleaned, "\x00", "")
	cleaned = paginationRegex.ReplaceAllString(cleaned, "")
	cleaned = strings.ReplaceAll(cleaned, "\b", "")
	return collapseBlankLines(cleaned)
}

func collapseBlankLines(content string) string {
	if content == "" {
		return ""
	}

	lines := strings.Split(content, "\n")
	filtered := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		filtered = append(filtered, line)
	}

	result := strings.Join(filtered, "\n")
	if strings.HasSuffix(content, "\n") && result != "" {
		result += "\n"
	}
	return result
}

func normalizePromptArtifacts(line string) string {
	normalized := strings.TrimSpace(line)
	if normalized == "" {
		return ""
	}

	// 部分设备在报错或翻页后会输出连续重复提示符（如 <SW2><SW2>...）。
	// 这里仅做展示层去重，不改变执行逻辑。
	prompt := promptTokenRegex.FindString(normalized)
	if prompt == "" {
		return normalized
	}

	rest := strings.TrimPrefix(normalized, prompt)
	for {
		trimmed := strings.TrimLeft(rest, " \t")
		if strings.HasPrefix(trimmed, prompt) {
			rest = strings.TrimPrefix(trimmed, prompt)
			continue
		}
		break
	}

	normalized = prompt + rest
	return normalized
}
