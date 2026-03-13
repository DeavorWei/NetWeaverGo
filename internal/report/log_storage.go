package report

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/NetWeaverGo/core/internal/logger"
)

// LogStorage 日志存储管理器（磁盘 + 内存索引）
type LogStorage struct {
	mu sync.RWMutex

	// 内存索引：只存储日志的行号和偏移量
	index map[string]*LogIndex

	// 日志文件句柄
	logFiles map[string]*os.File

	// 写入缓冲区
	writers map[string]*bufio.Writer

	// 临时文件目录
	storageDir string
}

// LogIndex 日志索引信息
type LogIndex struct {
	IP          string
	TotalCount  int     // 总日志条数
	LineOffsets []int64 // 每行在文件中的偏移量
	FilePath    string
	LastAccess  time.Time // 最后访问时间（用于清理）
}

// NewLogStorage 创建日志存储管理器
func NewLogStorage() (*LogStorage, error) {
	cwd, _ := os.Getwd()
	storageDir := filepath.Join(cwd, "data", "logs")

	if err := os.MkdirAll(storageDir, 0755); err != nil {
		return nil, fmt.Errorf("创建日志目录失败: %v", err)
	}

	return &LogStorage{
		index:      make(map[string]*LogIndex),
		logFiles:   make(map[string]*os.File),
		writers:    make(map[string]*bufio.Writer),
		storageDir: storageDir,
	}, nil
}

// InitDevice 初始化设备日志文件
func (ls *LogStorage) InitDevice(ip string) error {
	ls.mu.Lock()
	defer ls.mu.Unlock()

	if _, exists := ls.index[ip]; exists {
		return nil
	}

	// 创建日志文件
	filePath := filepath.Join(ls.storageDir, fmt.Sprintf("%s_%d.log",
		sanitizeIP(ip), time.Now().Unix()))

	file, err := os.Create(filePath)
	if err != nil {
		return err
	}

	// 创建缓冲写入器
	writer := bufio.NewWriterSize(file, 32*1024) // 32KB 缓冲区

	ls.logFiles[ip] = file
	ls.writers[ip] = writer
	ls.index[ip] = &LogIndex{
		IP:          ip,
		TotalCount:  0,
		LineOffsets: []int64{0}, // 第一行从0开始
		FilePath:    filePath,
		LastAccess:  time.Now(),
	}

	return nil
}

// AppendLog 追加日志
func (ls *LogStorage) AppendLog(ip string, message string) error {
	ls.mu.Lock()
	defer ls.mu.Unlock()

	idx, exists := ls.index[ip]
	if !exists {
		return fmt.Errorf("设备 %s 未初始化", ip)
	}

	writer, ok := ls.writers[ip]
	if !ok {
		return fmt.Errorf("设备 %s 写入器未找到", ip)
	}

	// 记录当前偏移量
	currentOffset, _ := ls.logFiles[ip].Seek(0, 1)
	idx.LineOffsets = append(idx.LineOffsets, currentOffset)

	// 写入日志
	line := fmt.Sprintf("[%s] %s\n", time.Now().Format("15:04:05"), message)
	if _, err := writer.WriteString(line); err != nil {
		return err
	}

	idx.TotalCount++
	idx.LastAccess = time.Now()

	// 定期刷新（每100条）
	if idx.TotalCount%100 == 0 {
		return writer.Flush()
	}

	return nil
}

// GetLogs 获取日志（支持分页）
func (ls *LogStorage) GetLogs(ip string, offset int, limit int) ([]string, error) {
	ls.mu.RLock()
	defer ls.mu.RUnlock()

	idx, exists := ls.index[ip]
	if !exists {
		return []string{}, nil
	}

	idx.LastAccess = time.Now()

	// 计算读取范围
	startLine := offset
	if startLine < 0 {
		// 负数表示从末尾计算（如 -100 表示最后100条）
		startLine = idx.TotalCount + offset
		if startLine < 0 {
			startLine = 0
		}
	}

	endLine := startLine + limit
	if endLine > idx.TotalCount {
		endLine = idx.TotalCount
	}

	if startLine >= endLine {
		return []string{}, nil
	}

	// 打开文件读取
	file, err := os.Open(idx.FilePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var result []string
	scanner := bufio.NewScanner(file)

	currentLine := 0
	for scanner.Scan() {
		if currentLine >= startLine && currentLine < endLine {
			result = append(result, scanner.Text())
		}
		if currentLine >= endLine {
			break
		}
		currentLine++
	}

	return result, scanner.Err()
}

// GetLastLogs 获取最后N条日志
func (ls *LogStorage) GetLastLogs(ip string, n int) ([]string, error) {
	return ls.GetLogs(ip, -n, n)
}

// GetLogCount 获取日志总数
func (ls *LogStorage) GetLogCount(ip string) int {
	ls.mu.RLock()
	defer ls.mu.RUnlock()

	if idx, exists := ls.index[ip]; exists {
		return idx.TotalCount
	}
	return 0
}

// Close 关闭并清理
func (ls *LogStorage) Close() {
	ls.mu.Lock()
	defer ls.mu.Unlock()

	// 刷新所有缓冲区
	for ip, writer := range ls.writers {
		if err := writer.Flush(); err != nil {
			logger.Error("LogStorage", ip, "刷新日志失败: %v", err)
		}
	}

	// 关闭文件
	for ip, file := range ls.logFiles {
		if err := file.Close(); err != nil {
			logger.Error("LogStorage", ip, "关闭日志文件失败: %v", err)
		}
	}

	ls.writers = nil
	ls.logFiles = nil
}

// CleanupOldFiles 清理过期文件（可定期调用）
func (ls *LogStorage) CleanupOldFiles(maxAge time.Duration) {
	ls.mu.Lock()
	defer ls.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)

	for ip, idx := range ls.index {
		if idx.LastAccess.Before(cutoff) {
			// 关闭文件句柄
			if file, ok := ls.logFiles[ip]; ok {
				file.Close()
				delete(ls.logFiles, ip)
			}
			if writer, ok := ls.writers[ip]; ok {
				writer.Flush()
				delete(ls.writers, ip)
			}

			// 可选：删除文件或归档
			// os.Remove(idx.FilePath)

			delete(ls.index, ip)
		}
	}
}

func sanitizeIP(ip string) string {
	// 将 IP 中的特殊字符替换为安全字符
	result := ""
	for _, c := range ip {
		if c >= '0' && c <= '9' {
			result += string(c)
		} else {
			result += "_"
		}
	}
	return result
}
