package taskexec

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/NetWeaverGo/core/internal/report"
)

// LogScope 日志作用域
type LogScope struct {
	RunID   string
	StageID string
	UnitID  string
	UnitKey string // 设备IP或其他标识
}

// LogScopeBuilder 日志作用域构建器
type LogScopeBuilder struct {
	basePath string
}

// NewLogScopeBuilder 创建日志作用域构建器
func NewLogScopeBuilder(basePath string) *LogScopeBuilder {
	return &LogScopeBuilder{basePath: basePath}
}

// BuildRunScope 构建Run级日志作用域
func (b *LogScopeBuilder) BuildRunScope(runID string) LogScope {
	return LogScope{
		RunID: runID,
	}
}

// BuildStageScope 构建Stage级日志作用域
func (b *LogScopeBuilder) BuildStageScope(runID, stageID, stageName string) LogScope {
	return LogScope{
		RunID:   runID,
		StageID: stageID,
		UnitKey: stageName,
	}
}

// BuildUnitScope 构建Unit级日志作用域
func (b *LogScopeBuilder) BuildUnitScope(runID, stageID, unitID, unitKey string) LogScope {
	return LogScope{
		RunID:   runID,
		StageID: stageID,
		UnitID:  unitID,
		UnitKey: unitKey,
	}
}

// GetLogPath 获取日志路径
func (s LogScope) GetLogPath(basePath string) string {
	now := time.Now()
	timestamp := now.Format("20060102_150405")

	if s.UnitID != "" {
		// Unit级日志
		return filepath.Join(
			basePath,
			"execution",
			"live-logs",
			fmt.Sprintf("%s_%s_%s", timestamp, s.RunID[:8], s.UnitKey),
		)
	}

	if s.StageID != "" {
		// Stage级日志
		return filepath.Join(
			basePath,
			"execution",
			"live-logs",
			fmt.Sprintf("%s_%s_%s_stage", timestamp, s.RunID[:8], s.UnitKey),
		)
	}

	// Run级日志
	return filepath.Join(
		basePath,
		"execution",
		"live-logs",
		fmt.Sprintf("%s_%s_run", timestamp, s.RunID[:8]),
	)
}

// LoggerFactory 日志工厂接口
type LoggerFactory interface {
	CreateLogger(scope LogScope) *report.DeviceLogSession
}

// DefaultLoggerFactory 默认日志工厂
type DefaultLoggerFactory struct {
	basePath string
	store    *report.ExecutionLogStore
}

// NewDefaultLoggerFactory 创建默认日志工厂
func NewDefaultLoggerFactory(basePath string, store *report.ExecutionLogStore) *DefaultLoggerFactory {
	return &DefaultLoggerFactory{
		basePath: basePath,
		store:    store,
	}
}

// CreateLogger 创建日志记录器
func (f *DefaultLoggerFactory) CreateLogger(scope LogScope) *report.DeviceLogSession {
	// 使用ExecutionLogStore确保设备日志会话
	if scope.UnitKey == "" {
		scope.UnitKey = "default"
	}

	session, err := f.store.EnsureDevice(scope.UnitKey, true)
	if err != nil {
		return nil
	}

	return session
}

// RuntimeLogger Runtime日志接口 - 供StageExecutor使用
type RuntimeLogger interface {
	// 写summary日志
	WriteSummary(scope LogScope, message string)
	// 写detail日志
	WriteDetail(scope LogScope, message string)
	// 写raw日志
	WriteRaw(scope LogScope, data []byte)
	// 关闭日志
	Close(scope LogScope) error
}

// DefaultRuntimeLogger 默认Runtime日志实现
type DefaultRuntimeLogger struct {
	factory  LoggerFactory
	sessions map[string]*report.DeviceLogSession
}

// NewDefaultRuntimeLogger 创建默认Runtime日志
func NewDefaultRuntimeLogger(factory LoggerFactory) *DefaultRuntimeLogger {
	return &DefaultRuntimeLogger{
		factory:  factory,
		sessions: make(map[string]*report.DeviceLogSession),
	}
}

// getSessionKey 获取session key
func (l *DefaultRuntimeLogger) getSessionKey(scope LogScope) string {
	if scope.UnitID != "" {
		return fmt.Sprintf("%s:%s:%s", scope.RunID, scope.StageID, scope.UnitID)
	}
	if scope.StageID != "" {
		return fmt.Sprintf("%s:%s", scope.RunID, scope.StageID)
	}
	return scope.RunID
}

// getOrCreateSession 获取或创建session
func (l *DefaultRuntimeLogger) getOrCreateSession(scope LogScope) *report.DeviceLogSession {
	key := l.getSessionKey(scope)
	if session, ok := l.sessions[key]; ok {
		return session
	}

	session := l.factory.CreateLogger(scope)
	l.sessions[key] = session
	return session
}

// WriteSummary 写summary日志
func (l *DefaultRuntimeLogger) WriteSummary(scope LogScope, message string) {
	session := l.getOrCreateSession(scope)
	if session != nil {
		_ = session.WriteSummary(message)
	}
}

// WriteDetail 写detail日志
func (l *DefaultRuntimeLogger) WriteDetail(scope LogScope, message string) {
	session := l.getOrCreateSession(scope)
	if session != nil {
		_ = session.WriteDetailChunk(message)
	}
}

// WriteRaw 写raw日志
func (l *DefaultRuntimeLogger) WriteRaw(scope LogScope, data []byte) {
	session := l.getOrCreateSession(scope)
	if session != nil && session.Raw != nil {
		_, _ = session.Raw.Write(data)
	}
}

// Close 关闭日志
func (l *DefaultRuntimeLogger) Close(scope LogScope) error {
	key := l.getSessionKey(scope)
	delete(l.sessions, key)
	return nil
}

// CloseAll 关闭所有日志
func (l *DefaultRuntimeLogger) CloseAll() {
	l.sessions = make(map[string]*report.DeviceLogSession)
}
