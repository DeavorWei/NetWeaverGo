package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/NetWeaverGo/core/internal/logger"
)

const (
	defaultStorageRootName = "netWeaverGoData"
	storageBootstrapFile   = "storage_root.json"
	sqliteFileName         = "netweaver.db"
)

type storageRootBootstrap struct {
	StorageRoot string `json:"storageRoot"`
}

// PathManager 统一管理应用所有运行时路径
type PathManager struct {
	mu sync.RWMutex

	WorkDir            string
	DefaultStorageRoot string
	StorageRoot        string

	DBDir               string
	DBPath              string
	AppLogDir           string
	AppLogPath          string
	ExecutionReportDir  string
	ExecutionLiveLogDir string
	BackupConfigDir     string
	SSHDir              string
	SSHKnownHostsPath   string

	// 拓扑发现相关路径
	TopologyRawDir    string // 原始 CLI 输出目录
	TopologyExportDir string // 导出图谱目录
	PlanImportDir     string // 规划文件导入目录

	bootstrapPath string
}

// 全局路径管理器
var (
	pathManager     *PathManager
	pathManagerOnce sync.Once
)

// GetPathManager 获取全局路径管理器（单例）
func GetPathManager() *PathManager {
	pathManagerOnce.Do(func() {
		pathManager = newPathManager()
	})
	return pathManager
}

// NormalizeStorageRoot 将用户输入路径标准化为绝对路径；空字符串时回退默认根目录
func NormalizeStorageRoot(candidate string) string {
	pm := GetPathManager()
	pm.mu.RLock()
	workDir := pm.WorkDir
	pm.mu.RUnlock()
	return normalizeStorageRootCandidate(workDir, candidate)
}

// ValidateStorageRootWritable 校验目录可写
func ValidateStorageRootWritable(candidate string) error {
	normalized := NormalizeStorageRoot(candidate)
	if err := os.MkdirAll(normalized, 0755); err != nil {
		return fmt.Errorf("创建数据根目录失败: %w", err)
	}

	fp, err := os.CreateTemp(normalized, ".write-check-*")
	if err != nil {
		return fmt.Errorf("数据根目录不可写: %w", err)
	}
	path := fp.Name()
	_ = fp.Close()
	_ = os.Remove(path)
	return nil
}

// newPathManager 创建路径管理器
func newPathManager() *PathManager {
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}

	defaultRoot := filepath.Join(cwd, defaultStorageRootName)
	pm := &PathManager{
		WorkDir:            cwd,
		DefaultStorageRoot: defaultRoot,
		StorageRoot:        defaultRoot,
		bootstrapPath:      filepath.Join(defaultRoot, storageBootstrapFile),
	}

	if storageRoot, loadErr := pm.loadBootstrapStorageRoot(); loadErr == nil && strings.TrimSpace(storageRoot) != "" {
		pm.StorageRoot = normalizeStorageRootCandidate(cwd, storageRoot)
	} else if loadErr != nil && !os.IsNotExist(loadErr) {
		logger.Warn("Config", "-", "读取 storage root bootstrap 失败，回退默认目录: %v", loadErr)
	}

	pm.rebuildDerivedPathsLocked()
	return pm
}

func normalizeStorageRootCandidate(workDir, candidate string) string {
	trimmed := strings.TrimSpace(candidate)
	if trimmed == "" {
		trimmed = filepath.Join(workDir, defaultStorageRootName)
	}
	if !filepath.IsAbs(trimmed) {
		trimmed = filepath.Join(workDir, trimmed)
	}
	return filepath.Clean(trimmed)
}

func (pm *PathManager) rebuildDerivedPathsLocked() {
	pm.DBDir = filepath.Join(pm.StorageRoot, "db")
	pm.DBPath = filepath.Join(pm.DBDir, sqliteFileName)
	pm.AppLogDir = filepath.Join(pm.StorageRoot, "logs", "app")
	pm.AppLogPath = filepath.Join(pm.AppLogDir, "app.log")
	pm.ExecutionReportDir = filepath.Join(pm.StorageRoot, "execution", "reports")
	pm.ExecutionLiveLogDir = filepath.Join(pm.StorageRoot, "execution", "live-logs")
	pm.BackupConfigDir = filepath.Join(pm.StorageRoot, "backup", "config")
	pm.SSHDir = filepath.Join(pm.StorageRoot, "ssh")
	pm.SSHKnownHostsPath = filepath.Join(pm.SSHDir, "known_hosts")

	// 拓扑发现相关路径
	pm.TopologyRawDir = filepath.Join(pm.StorageRoot, "topology", "raw")
	pm.TopologyExportDir = filepath.Join(pm.StorageRoot, "topology", "export")
	pm.PlanImportDir = filepath.Join(pm.StorageRoot, "topology", "plans")
}

func (pm *PathManager) ensureDirectoriesLocked() error {
	dirs := []string{
		pm.DBDir,
		pm.AppLogDir,
		pm.ExecutionReportDir,
		pm.ExecutionLiveLogDir,
		pm.BackupConfigDir,
		pm.SSHDir,
		filepath.Dir(pm.bootstrapPath),
		// 拓扑发现相关目录
		pm.TopologyRawDir,
		pm.TopologyExportDir,
		pm.PlanImportDir,
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	return nil
}

func (pm *PathManager) loadBootstrapStorageRoot() (string, error) {
	data, err := os.ReadFile(pm.bootstrapPath)
	if err != nil {
		return "", err
	}
	var bootstrap storageRootBootstrap
	if err := json.Unmarshal(data, &bootstrap); err != nil {
		return "", err
	}
	return bootstrap.StorageRoot, nil
}

func (pm *PathManager) persistBootstrapLocked() error {
	if err := os.MkdirAll(filepath.Dir(pm.bootstrapPath), 0755); err != nil {
		return err
	}
	payload := storageRootBootstrap{StorageRoot: pm.StorageRoot}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(pm.bootstrapPath, data, 0644)
}

// EnsureDirectories 确保所有必要目录存在
func (pm *PathManager) EnsureDirectories() error {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	return pm.ensureDirectoriesLocked()
}

// UpdateStorageRoot 更新数据根目录，并持久化 bootstrap
func (pm *PathManager) UpdateStorageRoot(candidate string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.StorageRoot = normalizeStorageRootCandidate(pm.WorkDir, candidate)
	pm.rebuildDerivedPathsLocked()

	if err := pm.ensureDirectoriesLocked(); err != nil {
		return err
	}
	if err := pm.persistBootstrapLocked(); err != nil {
		return err
	}
	return nil
}

func (pm *PathManager) GetStorageRoot() string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.StorageRoot
}

func (pm *PathManager) GetDBPath() string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.DBPath
}

func (pm *PathManager) GetAppLogPath() string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.AppLogPath
}

func (pm *PathManager) GetExecutionReportDir() string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.ExecutionReportDir
}

func (pm *PathManager) GetExecutionLiveLogDir() string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.ExecutionLiveLogDir
}

func (pm *PathManager) GetBackupDir() string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.BackupConfigDir
}

func (pm *PathManager) GetSSHKnownHostsPath() string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.SSHKnownHostsPath
}

func (pm *PathManager) GetBackupFilePath(subDir, fileName string) string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	if strings.TrimSpace(subDir) == "" {
		return filepath.Join(pm.BackupConfigDir, fileName)
	}
	return filepath.Join(pm.BackupConfigDir, subDir, fileName)
}

func resolvePathPattern(pattern, deviceIP string, t time.Time) string {
	res := pattern
	
	cleanIP := strings.ReplaceAll(deviceIP, ":", "-")
	
	res = strings.ReplaceAll(res, "%H", cleanIP)
	res = strings.ReplaceAll(res, "%Y", fmt.Sprintf("%04d", t.Year()))
	res = strings.ReplaceAll(res, "%M", fmt.Sprintf("%02d", t.Month()))
	res = strings.ReplaceAll(res, "%D", fmt.Sprintf("%02d", t.Day()))
	res = strings.ReplaceAll(res, "%h", fmt.Sprintf("%02d", t.Hour()))
	res = strings.ReplaceAll(res, "%m", fmt.Sprintf("%02d", t.Minute()))
	res = strings.ReplaceAll(res, "%s", fmt.Sprintf("%02d", t.Second()))
	
	return filepath.Clean(res)
}

// isPathWithinRoot 判断 path 是否在 root 目录下（含 root 本身）。
// 仅对 root 调用 EvalSymlinks 解析符号链接（root 应已存在），
// path 是新创建的路径，不存在符号链接，仅做 filepath.Clean。
// 在 Windows 上做大小写不敏感比较。
func isPathWithinRoot(path, root string) bool {
	// 仅解析 root 的符号链接（root 应已存在）
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		// EvalSymlinks 失败（root 不存在等），回退到原始路径
		realRoot = root
	}

	// path 是新创建的路径，不存在符号链接，直接 Clean
	cleanPath := filepath.Clean(path)
	cleanRoot := filepath.Clean(realRoot)

	if cleanPath == cleanRoot {
		return true
	}

	prefix := cleanRoot + string(filepath.Separator)
	// Windows 文件系统不区分大小写，使用 EqualFold 做前缀比较
	if filepath.IsAbs(cleanRoot) && len(cleanPath) >= len(prefix) {
		candidate := cleanPath[:len(prefix)]
		if strings.EqualFold(candidate, prefix) {
			return true
		}
	}
	// 非 Windows 或路径长度不足，回退到精确匹配
	return strings.HasPrefix(cleanPath, prefix)
}

func buildBackupLocalPath(saveRoot, dirPattern, filePattern, deviceIP string, timestamp time.Time) string {
	dirName := resolvePathPattern(dirPattern, deviceIP, timestamp)
	fileName := resolvePathPattern(filePattern, deviceIP, timestamp)

	// saveRoot 为空时拒绝生成路径，避免写入不可预期的位置
	if strings.TrimSpace(saveRoot) == "" {
		logger.Warn("Config", "-", "buildBackupLocalPath: saveRoot 为空，使用 fallback 路径")
		return filepath.Join("escape_prevented", fileName)
	}

	fullPath := filepath.Join(saveRoot, dirName, fileName)

	if !isPathWithinRoot(fullPath, saveRoot) {
		cleanRoot := filepath.Clean(saveRoot)
		logger.Warn("Config", "-", "路径逃逸防护触发: fullPath=%s, saveRoot=%s, 重定向到 escape_prevented", fullPath, saveRoot)
		return filepath.Join(cleanRoot, "escape_prevented", fileName)
	}

	return filepath.Clean(fullPath)
}

func (pm *PathManager) GetBackupConfigFilePath(saveRoot, dirPattern, filePattern, deviceIP string, timestamp time.Time) string {
	root := saveRoot
	if strings.TrimSpace(root) == "" {
		pm.mu.RLock()
		root = pm.BackupConfigDir
		pm.mu.RUnlock()
	}
	return buildBackupLocalPath(root, dirPattern, filePattern, deviceIP, timestamp)
}

// GetTopologyRawDir 获取拓扑原始输出目录
func (pm *PathManager) GetTopologyRawDir() string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.TopologyRawDir
}

// GetTopologyExportDir 获取拓扑导出目录
func (pm *PathManager) GetTopologyExportDir() string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.TopologyExportDir
}

// GetPlanImportDir 获取规划文件导入目录
func (pm *PathManager) GetPlanImportDir() string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.PlanImportDir
}

// GetDiscoveryRawFilePath 获取发现任务原始审计输出文件路径
// 格式: <TopologyRawDir>/<taskID>/<deviceIP>/<commandKey>.txt
// 用于保存原始字节流，供审计和排障使用
func (pm *PathManager) GetDiscoveryRawFilePath(taskID, deviceIP, commandKey string) string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return filepath.Join(pm.TopologyRawDir, taskID, deviceIP, commandKey+".txt")
}

// GetDiscoveryNormalizedFilePath 获取发现任务规范化输出文件路径
// 格式: <TopologyRawDir>/../normalized/<taskID>/<deviceIP>/<commandKey>.txt
// 用于保存规范化后的输出，供 parser 读取
func (pm *PathManager) GetDiscoveryNormalizedFilePath(taskID, deviceIP, commandKey string) string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	normalizedDir := filepath.Join(pm.StorageRoot, "topology", "normalized")
	return filepath.Join(normalizedDir, taskID, deviceIP, commandKey+".txt")
}

// GetAllPaths 获取全部路径（调试用）
func (pm *PathManager) GetAllPaths() map[string]string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return map[string]string{
		"workDir":             pm.WorkDir,
		"defaultStorageRoot":  pm.DefaultStorageRoot,
		"storageRoot":         pm.StorageRoot,
		"dbPath":              pm.DBPath,
		"appLogPath":          pm.AppLogPath,
		"executionReportDir":  pm.ExecutionReportDir,
		"executionLiveLogDir": pm.ExecutionLiveLogDir,
		"backupConfigDir":     pm.BackupConfigDir,
		"sshKnownHostsPath":   pm.SSHKnownHostsPath,
	}
}
