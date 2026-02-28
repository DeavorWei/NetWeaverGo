package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var globalLogDir = "logs"
var initOnce sync.Once

// DeviceLogger 处理单台设备的流式日志落盘
type DeviceLogger struct {
	IP       string
	File     *os.File
	mu       sync.Mutex
	Disabled bool
}

// NewDeviceLogger 创建或追加打开设备对应的独立日志文件
func NewDeviceLogger(ip string) (*DeviceLogger, error) {
	var mkdirErr error
	initOnce.Do(func() {
		mkdirErr = os.MkdirAll(globalLogDir, 0755)
	})
	if mkdirErr != nil {
		return nil, fmt.Errorf("无法创建日志目录: %w", mkdirErr)
	}

	fileName := fmt.Sprintf("%s_%s.log", ip, time.Now().Format("20060102_150405"))
	filePath := filepath.Join(globalLogDir, fileName)

	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return nil, fmt.Errorf("无法打开日志文件 %s: %w", filePath, err)
	}

	return &DeviceLogger{
		IP:   ip,
		File: file,
	}, nil
}

// Write 实现 io.Writer，提供给底层流双写使用
func (l *DeviceLogger) Write(p []byte) (n int, err error) {
	if l.Disabled || l.File == nil {
		return len(p), nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.File.Write(p)
}

// Log 往文件中写入系统级别的审计提示（带时标）
func (l *DeviceLogger) Log(format string, args ...interface{}) {
	if l.Disabled || l.File == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	msg := fmt.Sprintf("[%s] %s\n", time.Now().Format("2006/01/02 15:04:05"), fmt.Sprintf(format, args...))
	l.File.WriteString(msg)
}

// Close 安全关闭日志描述符
func (l *DeviceLogger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.File != nil {
		err := l.File.Close()
		l.File = nil
		return err
	}
	return nil
}
