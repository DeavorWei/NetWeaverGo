package taskexec

import (
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/NetWeaverGo/core/internal/logger"
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
	basePath  string
	store     *report.ExecutionLogStore
	enableRaw bool
}

// NewDefaultLoggerFactory 创建默认日志工厂
func NewDefaultLoggerFactory(basePath string, store *report.ExecutionLogStore, enableRaw bool) *DefaultLoggerFactory {
	return &DefaultLoggerFactory{
		basePath:  basePath,
		store:     store,
		enableRaw: enableRaw,
	}
}

// CreateLogger 创建日志记录器
func (f *DefaultLoggerFactory) CreateLogger(scope LogScope) *report.DeviceLogSession {
	if f == nil || f.store == nil {
		return nil
	}

	// 使用ExecutionLogStore确保设备日志会话
	if scope.UnitKey == "" {
		scope.UnitKey = "default"
	}

	session, err := f.store.EnsureDevice(scope.UnitKey, f.enableRaw)
	if err != nil {
		logger.Error("TaskExecLog", scope.UnitKey, "创建设备日志会话失败: %v", err)
		return nil
	}

	logger.Verbose("TaskExecLog", scope.UnitKey, "日志会话已就绪: run=%s, stage=%s, unit=%s, raw=%t", scope.RunID, scope.StageID, scope.UnitID, f.enableRaw)
	return session
}

// RuntimeLogger Runtime日志接口 - 供StageExecutor使用
type RuntimeLogger interface {
	// 获取或创建日志会话
	Session(scope LogScope) *report.DeviceLogSession
	// 写结构化事件日志
	WriteJournal(scope LogScope, record interface{})
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
	mu       sync.Mutex
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
	l.mu.Lock()
	defer l.mu.Unlock()

	key := l.getSessionKey(scope)
	if session, ok := l.sessions[key]; ok {
		return session
	}

	session := l.factory.CreateLogger(scope)
	l.sessions[key] = session
	return session
}

// Session 获取或创建日志会话
func (l *DefaultRuntimeLogger) Session(scope LogScope) *report.DeviceLogSession {
	return l.getOrCreateSession(scope)
}

// WriteJournal 写结构化事件日志
func (l *DefaultRuntimeLogger) WriteJournal(scope LogScope, record interface{}) {
	session := l.getOrCreateSession(scope)
	if session != nil {
		_ = session.WriteJournalRecord(record)
	}
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
	l.mu.Lock()
	defer l.mu.Unlock()

	key := l.getSessionKey(scope)
	delete(l.sessions, key)
	return nil
}

// CloseAll 关闭所有日志
func (l *DefaultRuntimeLogger) CloseAll() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.sessions = make(map[string]*report.DeviceLogSession)
}

// noopRuntimeLogger 在日志链路不可用时提供空实现，避免执行器判空分支四散。
type noopRuntimeLogger struct{}

func (l *noopRuntimeLogger) Session(scope LogScope) *report.DeviceLogSession {
	return nil
}

func (l *noopRuntimeLogger) WriteJournal(scope LogScope, record interface{}) {}

func (l *noopRuntimeLogger) WriteSummary(scope LogScope, message string) {}

func (l *noopRuntimeLogger) WriteDetail(scope LogScope, message string) {}

func (l *noopRuntimeLogger) WriteRaw(scope LogScope, data []byte) {}

func (l *noopRuntimeLogger) Close(scope LogScope) error { return nil }

func newNoopRuntimeLogger() RuntimeLogger {
	return &noopRuntimeLogger{}
}
