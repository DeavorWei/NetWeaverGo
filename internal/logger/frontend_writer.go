package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// FrontendLogEntry 前端日志条目
type FrontendLogEntry struct {
	Level     string  `json:"level"`     // 日志级别: error, warn, info, debug
	Message   string  `json:"message"`   // 日志消息
	Timestamp string  `json:"timestamp"` // ISO 8601 时间戳 (前端生成)
	Module    string  `json:"module"`    // 来源模块 (可选)
	Stack     *string `json:"stack"`     // 堆栈信息 (可选，错误时提供)
	URL       *string `json:"url"`       // 当前页面 URL (可选)
	UserAgent *string `json:"userAgent"` // 用户代理 (可选)
}

// FrontendLogWriterConfig 前端日志写入器配置
type FrontendLogWriterConfig struct {
	BufferSize     int           // 缓冲队列大小，默认 1000
	FlushInterval  time.Duration // 刷新间隔，默认 500ms
	MaxFileSize    int64         // 单文件最大大小，默认 10MB
	MaxBackupFiles int           // 保留历史文件数，默认 5
}

// DefaultFrontendLogWriterConfig 默认配置
var DefaultFrontendLogWriterConfig = FrontendLogWriterConfig{
	BufferSize:     1000,
	FlushInterval:  500 * time.Millisecond,
	MaxFileSize:    10 * 1024 * 1024, // 10MB
	MaxBackupFiles: 5,
}

// FrontendLogWriter 前端日志写入器
type FrontendLogWriter struct {
	mu            sync.Mutex
	file          *os.File
	filePath      string
	currentSize   int64
	buffer        chan string
	bufferSize    int
	flushInterval time.Duration
	maxFileSize   int64
	maxBackups    int
	stopCh        chan struct{}
	doneCh        chan struct{}
	flushCh       chan struct{}
}

// 全局前端日志写入器实例
var globalFrontendWriter *FrontendLogWriter
var frontendWriterMu sync.Mutex

// InitFrontendLogWriter 初始化前端日志写入器
func InitFrontendLogWriter(logPath string, config FrontendLogWriterConfig) error {
	if logPath == "" {
		return fmt.Errorf("日志路径不能为空")
	}

	// 应用默认值
	if config.BufferSize <= 0 {
		config.BufferSize = DefaultFrontendLogWriterConfig.BufferSize
	}
	if config.FlushInterval <= 0 {
		config.FlushInterval = DefaultFrontendLogWriterConfig.FlushInterval
	}
	if config.MaxFileSize <= 0 {
		config.MaxFileSize = DefaultFrontendLogWriterConfig.MaxFileSize
	}
	if config.MaxBackupFiles <= 0 {
		config.MaxBackupFiles = DefaultFrontendLogWriterConfig.MaxBackupFiles
	}

	// 确保目录存在
	dir := filepath.Dir(logPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("无法创建前端日志目录: %w", err)
	}

	// 打开文件 (追加模式)
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return fmt.Errorf("无法打开前端日志文件: %w", err)
	}

	// 获取当前文件大小
	stat, err := file.Stat()
	if err != nil {
		file.Close()
		return fmt.Errorf("无法获取前端日志文件信息: %w", err)
	}

	frontendWriterMu.Lock()
	defer frontendWriterMu.Unlock()

	// 关闭旧实例
	if globalFrontendWriter != nil {
		globalFrontendWriter.Close()
	}

	writer := &FrontendLogWriter{
		file:          file,
		filePath:      logPath,
		currentSize:   stat.Size(),
		buffer:        make(chan string, config.BufferSize),
		bufferSize:    config.BufferSize,
		flushInterval: config.FlushInterval,
		maxFileSize:   config.MaxFileSize,
		maxBackups:    config.MaxBackupFiles,
		stopCh:        make(chan struct{}),
		doneCh:        make(chan struct{}),
		flushCh:       make(chan struct{}, 1),
	}

	// 启动后台刷新协程
	go writer.flushLoop()

	globalFrontendWriter = writer
	return nil
}

// Write 写入日志条目 (非阻塞)
func (w *FrontendLogWriter) Write(entry FrontendLogEntry) {
	// 脱敏处理
	entry.Message = globalSanitizer.Sanitize(entry.Message)
	if entry.Stack != nil && *entry.Stack != "" {
		sanitized := globalSanitizer.Sanitize(*entry.Stack)
		entry.Stack = &sanitized
	}
	if entry.URL != nil && *entry.URL != "" {
		sanitized := globalSanitizer.Sanitize(*entry.URL)
		entry.URL = &sanitized
	}

	// 格式化日志行
	logLine := w.formatLogLine(entry)

	// 非阻塞写入缓冲区
	select {
	case w.buffer <- logLine:
		// 成功写入缓冲区
	default:
		// 缓冲区满，丢弃日志 (避免阻塞)
		fmt.Printf("[FrontendLog] 缓冲区已满，丢弃日志: %s\n", truncateString(logLine, 100))
	}
}

// WriteImmediate 立即写入日志条目（用于 error 级别）
func (w *FrontendLogWriter) WriteImmediate(entry FrontendLogEntry) {
	// 脱敏处理
	entry.Message = globalSanitizer.Sanitize(entry.Message)
	if entry.Stack != nil && *entry.Stack != "" {
		sanitized := globalSanitizer.Sanitize(*entry.Stack)
		entry.Stack = &sanitized
	}
	if entry.URL != nil && *entry.URL != "" {
		sanitized := globalSanitizer.Sanitize(*entry.URL)
		entry.URL = &sanitized
	}

	// 格式化日志行
	logLine := w.formatLogLine(entry)

	// 直接写入文件
	w.writeToFile([]string{logLine})
}

// WriteBatch 批量写入日志条目
func (w *FrontendLogWriter) WriteBatch(entries []FrontendLogEntry) {
	for _, entry := range entries {
		w.Write(entry)
	}
}

// formatLogLine 格式化日志行
func (w *FrontendLogWriter) formatLogLine(entry FrontendLogEntry) string {
	timestamp := entry.Timestamp
	if timestamp == "" {
		timestamp = time.Now().Format("2006/01/02 15:04:05")
	}

	module := entry.Module
	if module == "" {
		module = "Frontend"
	}

	// 标准化日志级别（首字母大写）
	level := normalizeLevel(entry.Level)

	logLine := fmt.Sprintf("[%s] [%s] [%s] %s", timestamp, level, module, entry.Message)

	if entry.Stack != nil && *entry.Stack != "" {
		logLine += "\n  Stack: " + *entry.Stack
	}

	return logLine + "\n"
}

// normalizeLevel 标准化日志级别
func normalizeLevel(level string) string {
	switch level {
	case "error":
		return "Error"
	case "warn":
		return "Warn"
	case "info":
		return "Info"
	case "debug":
		return "Debug"
	default:
		return level
	}
}

// truncateString 截断字符串
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// flushLoop 后台刷新循环
func (w *FrontendLogWriter) flushLoop() {
	ticker := time.NewTicker(w.flushInterval)
	defer ticker.Stop()
	defer close(w.doneCh)

	var pending []string

	for {
		select {
		case logLine := <-w.buffer:
			pending = append(pending, logLine)
			// 达到批量写入阈值时立即刷新
			if len(pending) >= 10 {
				w.writeToFile(pending)
				pending = nil
			}
		case <-ticker.C:
			if len(pending) > 0 {
				w.writeToFile(pending)
				pending = nil
			}
		case <-w.flushCh:
			// 强制刷新信号
			if len(pending) > 0 {
				w.writeToFile(pending)
				pending = nil
			}
		case <-w.stopCh:
			// 退出前刷新剩余日志
			if len(pending) > 0 {
				w.writeToFile(pending)
			}
			// 清空缓冲区剩余
			for len(w.buffer) > 0 {
				logLine := <-w.buffer
				w.writeToFile([]string{logLine})
			}
			return
		}
	}
}

// writeToFile 写入文件
func (w *FrontendLogWriter) writeToFile(lines []string) {
	if len(lines) == 0 {
		return
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	if w.file == nil {
		return
	}

	for _, line := range lines {
		n, err := w.file.WriteString(line)
		if err != nil {
			fmt.Printf("[FrontendLog] 写入文件失败: %v\n", err)
			return
		}
		w.currentSize += int64(n)

		// 检查是否需要轮转
		if w.currentSize >= w.maxFileSize {
			w.rotateFileLocked()
		}
	}
}

// rotateFileLocked 日志轮转（调用前必须持有锁）
func (w *FrontendLogWriter) rotateFileLocked() {
	if w.file == nil {
		return
	}

	// 关闭当前文件
	w.file.Close()
	w.file = nil

	// 删除最旧的备份文件
	oldestBackup := fmt.Sprintf("%s.%d", w.filePath, w.maxBackups)
	os.Remove(oldestBackup)

	// 重命名现有备份文件
	for i := w.maxBackups - 1; i >= 1; i-- {
		oldPath := fmt.Sprintf("%s.%d", w.filePath, i)
		newPath := fmt.Sprintf("%s.%d", w.filePath, i+1)
		os.Rename(oldPath, newPath)
	}

	// 重命名当前文件为 .1
	backupPath := fmt.Sprintf("%s.1", w.filePath)
	os.Rename(w.filePath, backupPath)

	// 创建新文件
	file, err := os.OpenFile(w.filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		fmt.Printf("[FrontendLog] 无法创建新日志文件: %v\n", err)
		return
	}

	w.file = file
	w.currentSize = 0
}

// Flush 强制刷新缓冲区
func (w *FrontendLogWriter) Flush() {
	select {
	case w.flushCh <- struct{}{}:
		// 发送刷新信号
	default:
		// 已有待处理的刷新信号
	}
}

// Close 关闭写入器
func (w *FrontendLogWriter) Close() {
	close(w.stopCh)
	<-w.doneCh // 等待刷新协程退出

	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file != nil {
		w.file.Close()
		w.file = nil
	}
}

// GetFrontendLogWriter 获取全局前端日志写入器
func GetFrontendLogWriter() *FrontendLogWriter {
	frontendWriterMu.Lock()
	defer frontendWriterMu.Unlock()
	return globalFrontendWriter
}

// CloseFrontendLogWriter 关闭全局前端日志写入器
func CloseFrontendLogWriter() {
	frontendWriterMu.Lock()
	defer frontendWriterMu.Unlock()

	if globalFrontendWriter != nil {
		globalFrontendWriter.Close()
		globalFrontendWriter = nil
	}
}
