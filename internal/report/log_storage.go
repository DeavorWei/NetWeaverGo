package report

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/NetWeaverGo/core/internal/config"
	"github.com/NetWeaverGo/core/internal/logger"
)

// RawTranscriptSink 为 SSH 原始流提供统一的写入接口。
type RawTranscriptSink interface {
	Write([]byte) (int, error)
	WriteMarker(format string, args ...interface{})
}

// DeviceLogPaths 记录单台设备的三类日志路径。
type DeviceLogPaths struct {
	SummaryPath string
	DetailPath  string
	RawPath     string
	JournalPath string
}

// DeviceLogSession 表示单台设备的一次执行日志会话。
type DeviceLogSession struct {
	IP      string
	Summary *SummaryLogger
	Detail  *DetailLogger
	Raw     *RawLogger
	Journal *JournalLogger
}

// WriteSummary 追加简略日志。
func (s *DeviceLogSession) WriteSummary(message string) error {
	if s == nil || s.Summary == nil {
		return nil
	}
	return s.Summary.WriteLine(message)
}

// WriteDetailCommand 记录发送命令。
func (s *DeviceLogSession) WriteDetailCommand(command string) error {
	if s == nil || s.Detail == nil {
		return nil
	}
	return s.Detail.WriteCommand(command)
}

// WriteDetailChunk 记录规范化后的 SSH 输出文本。
func (s *DeviceLogSession) WriteDetailChunk(chunk string) error {
	if s == nil || s.Detail == nil {
		return nil
	}
	return s.Detail.WriteNormalizedText(chunk)
}

// FlushDetail 刷新详细日志尾部缓冲。
func (s *DeviceLogSession) FlushDetail() error {
	if s == nil || s.Detail == nil {
		return nil
	}
	return s.Detail.FlushPending()
}

// RawSink 返回原始日志 sink。
func (s *DeviceLogSession) RawSink() RawTranscriptSink {
	if s == nil {
		return nil
	}
	return s.Raw
}

// WriteJournalRecord 追加结构化事件。
func (s *DeviceLogSession) WriteJournalRecord(record interface{}) error {
	if s == nil || s.Journal == nil {
		return nil
	}
	return s.Journal.WriteRecord(record)
}

// ExecutionLogStore 统一管理单次执行中的所有设备日志。
type ExecutionLogStore struct {
	mu         sync.RWMutex
	storageDir string
	taskName   string
	startTime  time.Time
	sessions   map[string]*DeviceLogSession
}

// NewExecutionLogStore 创建日志存储管理器。
func NewExecutionLogStore(taskName string, startTime time.Time) (*ExecutionLogStore, error) {
	storageDir := config.GetPathManager().GetExecutionLiveLogDir()
	if err := os.MkdirAll(storageDir, 0755); err != nil {
		return nil, fmt.Errorf("创建日志目录失败: %v", err)
	}
	if startTime.IsZero() {
		startTime = time.Now()
	}

	return &ExecutionLogStore{
		storageDir: storageDir,
		taskName:   strings.TrimSpace(taskName),
		startTime:  startTime,
		sessions:   make(map[string]*DeviceLogSession),
	}, nil
}

// SetTaskName 设置任务名称，用于后续创建的文件名。
func (ls *ExecutionLogStore) SetTaskName(taskName string) {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	ls.taskName = strings.TrimSpace(taskName)
}

// EnsureDevice 初始化单台设备的日志会话。
func (ls *ExecutionLogStore) EnsureDevice(ip string, enableRaw bool) (*DeviceLogSession, error) {
	ls.mu.Lock()
	defer ls.mu.Unlock()

	if session, exists := ls.sessions[ip]; exists {
		if session.Journal == nil {
			journalLogger, err := NewJournalLogger(ls.buildFilePath(ip, "journal"))
			if err != nil {
				return nil, err
			}
			session.Journal = journalLogger
		}
		if enableRaw && session.Raw == nil {
			rawLogger, err := NewRawLogger(ls.buildFilePath(ip, "raw"))
			if err != nil {
				return nil, err
			}
			session.Raw = rawLogger
		}
		return session, nil
	}

	summaryLogger, err := NewSummaryLogger(ls.buildFilePath(ip, "summary"))
	if err != nil {
		return nil, err
	}

	detailLogger, err := NewDetailLogger(ls.buildFilePath(ip, "detail"))
	if err != nil {
		_ = summaryLogger.Close()
		return nil, err
	}

	session := &DeviceLogSession{
		IP:      ip,
		Summary: summaryLogger,
		Detail:  detailLogger,
	}

	journalLogger, err := NewJournalLogger(ls.buildFilePath(ip, "journal"))
	if err != nil {
		_ = detailLogger.Close()
		_ = summaryLogger.Close()
		return nil, err
	}
	session.Journal = journalLogger

	if enableRaw {
		rawLogger, rawErr := NewRawLogger(ls.buildFilePath(ip, "raw"))
		if rawErr != nil {
			_ = journalLogger.Close()
			_ = detailLogger.Close()
			_ = summaryLogger.Close()
			return nil, rawErr
		}
		session.Raw = rawLogger
	}

	ls.sessions[ip] = session
	return session, nil
}

// GetDeviceSession 获取设备日志会话。
func (ls *ExecutionLogStore) GetDeviceSession(ip string) *DeviceLogSession {
	ls.mu.RLock()
	defer ls.mu.RUnlock()
	return ls.sessions[ip]
}

// GetDetailLogCount 获取详细日志总行数。
func (ls *ExecutionLogStore) GetDetailLogCount(ip string) int {
	ls.mu.RLock()
	session := ls.sessions[ip]
	ls.mu.RUnlock()
	if session == nil || session.Detail == nil {
		return 0
	}
	return session.Detail.LineCount()
}

// GetSummaryLogCount 获取简略日志总行数。
func (ls *ExecutionLogStore) GetSummaryLogCount(ip string) int {
	ls.mu.RLock()
	session := ls.sessions[ip]
	ls.mu.RUnlock()
	if session == nil || session.Summary == nil {
		return 0
	}
	return session.Summary.LineCount()
}

// GetDetailLastLogs 获取详细日志最后 N 条。
func (ls *ExecutionLogStore) GetDetailLastLogs(ip string, n int) ([]string, error) {
	ls.mu.RLock()
	session := ls.sessions[ip]
	ls.mu.RUnlock()
	if session == nil || session.Detail == nil {
		return []string{}, nil
	}
	return readLastLogLines(session.Detail.Path(), n)
}

// GetSummaryLastLogs 获取简略日志最后 N 条。
func (ls *ExecutionLogStore) GetSummaryLastLogs(ip string, n int) ([]string, error) {
	ls.mu.RLock()
	session := ls.sessions[ip]
	ls.mu.RUnlock()
	if session == nil || session.Summary == nil {
		return []string{}, nil
	}
	return readLastLogLines(session.Summary.Path(), n)
}

// GetDeviceLogPaths 获取设备日志路径集合。
func (ls *ExecutionLogStore) GetDeviceLogPaths(ip string) DeviceLogPaths {
	ls.mu.RLock()
	session := ls.sessions[ip]
	ls.mu.RUnlock()
	if session == nil {
		return DeviceLogPaths{}
	}

	paths := DeviceLogPaths{}
	if session.Summary != nil {
		paths.SummaryPath = session.Summary.Path()
	}
	if session.Detail != nil {
		paths.DetailPath = session.Detail.Path()
	}
	if session.Raw != nil {
		paths.RawPath = session.Raw.Path()
	}
	if session.Journal != nil {
		paths.JournalPath = session.Journal.Path()
	}
	return paths
}

// Close 关闭全部日志句柄。
func (ls *ExecutionLogStore) Close() {
	ls.mu.Lock()
	defer ls.mu.Unlock()

	for ip, session := range ls.sessions {
		if session.Detail != nil {
			if err := session.Detail.Close(); err != nil {
				logger.Error("LogStore", ip, "关闭详细日志失败: %v", err)
			}
		}
		if session.Summary != nil {
			if err := session.Summary.Close(); err != nil {
				logger.Error("LogStore", ip, "关闭简略日志失败: %v", err)
			}
		}
		if session.Raw != nil {
			if err := session.Raw.Close(); err != nil {
				logger.Error("LogStore", ip, "关闭原始日志失败: %v", err)
			}
		}
		if session.Journal != nil {
			if err := session.Journal.Close(); err != nil {
				logger.Error("LogStore", ip, "关闭结构化事件日志失败: %v", err)
			}
		}
	}

	ls.sessions = make(map[string]*DeviceLogSession)
}

func (ls *ExecutionLogStore) buildFilePath(ip string, suffix string) string {
	taskName := sanitizeLogName(ls.taskName)
	if taskName == "" {
		taskName = "task"
	}
	stem := fmt.Sprintf("%s_%s_%s", ls.startTime.Format("20060102_150405"), taskName, sanitizeLogName(ip))
	return filepath.Join(ls.storageDir, fmt.Sprintf("%s_%s.log", stem, suffix))
}

func sanitizeLogName(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}

	var builder strings.Builder
	for _, r := range trimmed {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			builder.WriteRune(r)
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		case r == '-' || r == '_' || r == '.':
			builder.WriteRune(r)
		default:
			builder.WriteRune('_')
		}
	}

	return strings.Trim(builder.String(), "_")
}

func readLastLogLines(filePath string, n int) ([]string, error) {
	if strings.TrimSpace(filePath) == "" || n <= 0 {
		return []string{}, nil
	}

	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	lines := make([]string, 0, n)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
		if len(lines) > n {
			lines = lines[1:]
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return lines, nil
}
