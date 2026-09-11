package report

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/NetWeaverGo/core/internal/logger"
)

const maxLineBufferSize = 64 * 1024

// RawLogger 记录完整 SSH 字节流。
type RawLogger struct {
	mu      sync.Mutex
	file    *os.File
	writer  *bufio.Writer
	lineBuf []byte
	path    string
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
		file:    file,
		writer:  bufio.NewWriterSize(file, 32*1024),
		lineBuf: make([]byte, 0, 1024),
		path:    filePath,
	}, nil
}

// Write 写入原始字节（按行缓冲并脱敏处理，防止跨 chunk 导致敏感口令泄露）。
func (l *RawLogger) Write(p []byte) (int, error) {
	if l == nil {
		return len(p), nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	l.lineBuf = append(l.lineBuf, p...)

	// 按 \n 切割所有完整逻辑行进行脱敏写入
	for {
		idx := bytes.IndexByte(l.lineBuf, '\n')
		if idx < 0 {
			break
		}
		line := l.lineBuf[:idx+1]
		sanitized := logger.SanitizeText(string(line))
		if _, err := l.writer.WriteString(sanitized); err != nil {
			return 0, err
		}
		l.lineBuf = l.lineBuf[idx+1:]
	}

	// 内存安全兜底：如果单行无换行超过 64KB，强制脱敏并冲刷
	if len(l.lineBuf) > maxLineBufferSize {
		sanitized := logger.SanitizeText(string(l.lineBuf))
		if _, err := l.writer.WriteString(sanitized); err != nil {
			return 0, err
		}
		l.lineBuf = l.lineBuf[:0]
	}

	return len(p), l.writer.Flush()
}

// WriteMarker 写入结构化标记（经过脱敏处理）。
func (l *RawLogger) WriteMarker(format string, args ...interface{}) {
	if l == nil {
		return
	}

	formatted := fmt.Sprintf(format, args...)
	sanitized := logger.SanitizeText(formatted)

	l.mu.Lock()
	defer l.mu.Unlock()
	_, _ = l.writer.WriteString(sanitized)
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

	// 冲刷残留 lineBuf
	if len(l.lineBuf) > 0 && l.writer != nil {
		sanitized := logger.SanitizeText(string(l.lineBuf))
		_, _ = l.writer.WriteString(sanitized)
		l.lineBuf = nil
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
