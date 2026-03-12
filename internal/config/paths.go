package config

import (
	"os"
	"path/filepath"
	"sync"
)

// PathManager 统一管理应用的所有文件路径
type PathManager struct {
	mu sync.RWMutex

	// 基础目录
	WorkDir   string // 工作目录
	DataDir   string // 数据目录
	OutputDir string // 输出目录
	LogDir    string // 日志目录

	// 数据库文件
	DBPath string

	// 配置文件
	SettingsFile string // settings.yaml

	// 遗留文件（兼容旧版本）
	InventoryFile string // inventory.csv
	ConfigFile    string // config.txt
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

// newPathManager 创建路径管理器
func newPathManager() *PathManager {
	cwd, _ := os.Getwd()

	pm := &PathManager{
		WorkDir:   cwd,
		DataDir:   filepath.Join(cwd, "data"),
		OutputDir: filepath.Join(cwd, "output"),
		LogDir:    filepath.Join(cwd, "logs"),

		// 遗留文件路径
		InventoryFile: filepath.Join(cwd, "inventory.csv"),
		ConfigFile:    filepath.Join(cwd, "config.txt"),
	}

	// 数据库路径
	pm.DBPath = filepath.Join(pm.DataDir, "netweaver.db")

	// 配置文件路径
	pm.SettingsFile = filepath.Join(pm.DataDir, "settings.yaml")

	return pm
}

// EnsureDirectories 确保所有必要的目录存在
func (pm *PathManager) EnsureDirectories() error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	dirs := []string{
		pm.DataDir,
		pm.OutputDir,
		pm.LogDir,
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	return nil
}

// GetReportPath 获取报告文件路径
func (pm *PathManager) GetReportPath(filename string) string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return filepath.Join(pm.OutputDir, filename)
}

// GetBackupPath 获取备份文件路径
func (pm *PathManager) GetBackupPath(subdir string) string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return filepath.Join(pm.OutputDir, "confBakup", subdir)
}

// GetLogPath 获取日志文件路径
func (pm *PathManager) GetLogPath(filename string) string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return filepath.Join(pm.LogDir, filename)
}

// SetOutputDir 设置输出目录（允许运行时修改）
func (pm *PathManager) SetOutputDir(dir string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.OutputDir = dir
}

// SetLogDir 设置日志目录
func (pm *PathManager) SetLogDir(dir string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.LogDir = dir
}

// GetAllPaths 获取所有路径信息（用于调试）
func (pm *PathManager) GetAllPaths() map[string]string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	return map[string]string{
		"workDir":       pm.WorkDir,
		"dataDir":       pm.DataDir,
		"outputDir":     pm.OutputDir,
		"logDir":        pm.LogDir,
		"dbPath":        pm.DBPath,
		"settingsFile":  pm.SettingsFile,
		"inventoryFile": pm.InventoryFile,
		"configFile":    pm.ConfigFile,
	}
}
