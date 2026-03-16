package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	appLogPath   = filepath.Join("logs", "app.log")
	appLogFile   *os.File
	appLogMu     sync.Mutex
	ConsoleMuted bool
)

// InitGlobalLogger 初始化全局应用日志文件（首次启动调用）
func InitGlobalLogger(logPath string) error {
	return openGlobalLogFile(logPath, true)
}

// ReconfigureGlobalLogger 重新配置日志文件路径（切换 storageRoot 时调用）
func ReconfigureGlobalLogger(logPath string) error {
	return openGlobalLogFile(logPath, false)
}

func openGlobalLogFile(logPath string, truncate bool) error {
	if logPath == "" {
		logPath = appLogPath
	}
	dir := filepath.Dir(logPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("无法创建日志目录: %w", err)
	}

	flags := os.O_CREATE | os.O_WRONLY | os.O_APPEND
	if truncate {
		flags = os.O_CREATE | os.O_WRONLY | os.O_TRUNC
	}

	file, err := os.OpenFile(logPath, flags, 0666)
	if err != nil {
		return fmt.Errorf("无法打开全局日志文件: %w", err)
	}

	appLogMu.Lock()
	defer appLogMu.Unlock()
	if appLogFile != nil {
		_ = appLogFile.Close()
	}
	appLogFile = file
	appLogPath = logPath
	return nil
}

// GetGlobalLogPath 返回当前全局日志文件路径
func GetGlobalLogPath() string {
	appLogMu.Lock()
	defer appLogMu.Unlock()
	return appLogPath
}

// writeGlobalLog 写入全局日志，同时输出到终端
func writeGlobalLog(level string, module string, ip string, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	timestamp := time.Now().Format("2006/01/02 15:04:05")
	if module == "" {
		module = "-"
	}
	if ip == "" {
		ip = "-"
	}
	logLine := fmt.Sprintf("[%s] [%s] [%s] [%s] %s\n", timestamp, level, module, ip, msg)

	// 终端输出
	if !ConsoleMuted {
		fmt.Print(logLine)
	}

	// 文件落盘
	appLogMu.Lock()
	defer appLogMu.Unlock()
	if appLogFile != nil {
		appLogFile.WriteString(logLine)
	}
}

// Info 输出信息级别日志
func Info(module string, ip string, format string, args ...interface{}) {
	writeGlobalLog("Info", module, ip, format, args...)
}

// Warn 输出警告级别日志
func Warn(module string, ip string, format string, args ...interface{}) {
	writeGlobalLog("Warn", module, ip, format, args...)
}

// Error 输出错误级别日志
func Error(module string, ip string, format string, args ...interface{}) {
	writeGlobalLog("Error", module, ip, format, args...)
}

var (
	EnableDebug    bool // 普通调试日志开关
	EnableDebugAll bool // 全量底细调试日志开关
)

// Debug 输出调试级别日志
func Debug(module string, ip string, format string, args ...interface{}) {
	if !EnableDebug {
		return
	}
	writeGlobalLog("Debug", module, ip, format, args...)
}

// DebugAll 输出全量调试级别日志（仅在 EnableDebugAll 为 true 时生效）
func DebugAll(module string, ip string, format string, args ...interface{}) {
	if !EnableDebugAll {
		return
	}
	// 为了在日志里区分，这里将前缀设为 Debug-All
	writeGlobalLog("Debug-All", module, ip, format, args...)
}
