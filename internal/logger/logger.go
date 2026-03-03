package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	globalLogDir    = "logs"
	globalOutputDir = "output"
	appLogFile      *os.File
	appLogMu        sync.Mutex
	initOnce        sync.Once
)

// InitGlobalLogger 初始化全局应用日志和输出目录
func InitGlobalLogger() error {
	var initErr error
	initOnce.Do(func() {
		// 创建日志目录
		if err := os.MkdirAll(globalLogDir, 0755); err != nil {
			initErr = fmt.Errorf("无法创建日志目录: %w", err)
			return
		}
		// 创建输出目录
		if err := os.MkdirAll(globalOutputDir, 0755); err != nil {
			initErr = fmt.Errorf("无法创建输出目录: %w", err)
			return
		}

		// 创建或打开全局应用日志文件
		logPath := filepath.Join(globalLogDir, "app.log")
		appLogFile, initErr = os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if initErr != nil {
			initErr = fmt.Errorf("无法打开全局日志文件: %w", initErr)
			return
		}
	})
	return initErr
}

// writeGlobalLog 写入全局日志，同时输出到终端
func writeGlobalLog(level, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	timestamp := time.Now().Format("2006/01/02 15:04:05")
	logLine := fmt.Sprintf("[%s] [%s] %s\n", timestamp, level, msg)

	// 终端输出
	fmt.Print(logLine)

	// 文件落盘
	appLogMu.Lock()
	defer appLogMu.Unlock()
	if appLogFile != nil {
		appLogFile.WriteString(logLine)
	}
}

// Info 输出信息级别日志
func Info(format string, args ...interface{}) {
	writeGlobalLog("INFO", format, args...)
}

// Warn 输出警告级别日志
func Warn(format string, args ...interface{}) {
	writeGlobalLog("WARN", format, args...)
}

// Error 输出错误级别日志
func Error(format string, args ...interface{}) {
	writeGlobalLog("ERROR", format, args...)
}

// Debug 输出调试级别日志
func Debug(format string, args ...interface{}) {
	writeGlobalLog("DEBUG", format, args...)
}

// DeviceOutput 处理单台设备的回显流落盘
type DeviceOutput struct {
	IP       string
	File     *os.File
	mu       sync.Mutex
	Disabled bool
}

// NewDeviceOutput 创建或追加打开设备对应的独立输出文件
func NewDeviceOutput(ip string) (*DeviceOutput, error) {
	// 即使因为某些原因 InitGlobalLogger 没被调用，也尝试创建目录作为后备
	os.MkdirAll(globalOutputDir, 0755)

	fileName := fmt.Sprintf("%s_%s.log", ip, time.Now().Format("20060102_150405"))
	filePath := filepath.Join(globalOutputDir, fileName)

	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return nil, fmt.Errorf("无法打开输出文件 %s: %w", filePath, err)
	}

	return &DeviceOutput{
		IP:   ip,
		File: file,
	}, nil
}

// Write 实现 io.Writer，提供给底层流双写使用
func (l *DeviceOutput) Write(p []byte) (n int, err error) {
	if l.Disabled || l.File == nil {
		return len(p), nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.File.Write(p)
}

// Close 安全关闭输出文件描述符
func (l *DeviceOutput) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.File != nil {
		err := l.File.Close()
		l.File = nil
		return err
	}
	return nil
}
