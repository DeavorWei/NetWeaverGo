package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// GlobalSettings 全局运行参数
type GlobalSettings struct {
	MaxWorkers     int    `yaml:"max_workers"`     // 并发数 (当前硬编码为 32)
	ConnectTimeout string `yaml:"connect_timeout"` // SSH/SFTP 连接超时 (如 "10s")
	CommandTimeout string `yaml:"command_timeout"` // 单条命令默认超时 (如 "30s")
	OutputDir      string `yaml:"output_dir"`      // 回显输出与配置备份的根目录
	LogDir         string `yaml:"log_dir"`         // 系统运行日志存放目录
	ErrorMode      string `yaml:"error_mode"`      // "pause" | "skip" | "abort"
}

// rootConfig 用于解析 YAML 格式的根结构
type rootConfig struct {
	Settings GlobalSettings `yaml:"settings"`
}

const settingsFile = "settings.yaml"

// DefaultSettings 返回默认配置
func DefaultSettings() GlobalSettings {
	return GlobalSettings{
		MaxWorkers:     32,
		ConnectTimeout: "10s",
		CommandTimeout: "30s",
		OutputDir:      "output",
		LogDir:         "logs",
		ErrorMode:      "pause",
	}
}

// LoadSettings 读取并解析 settings.yaml，如果不存在则创建默认模板
func LoadSettings() (*GlobalSettings, bool, error) {
	root := rootConfig{Settings: DefaultSettings()}
	isNew := false

	if _, err := os.Stat(settingsFile); os.IsNotExist(err) {
		generateSettingsTemplate()
		isNew = true
	} else {
		content, err := os.ReadFile(settingsFile)
		if err != nil {
			return nil, isNew, fmt.Errorf("读取配置文件 %s 失败: %v", settingsFile, err)
		}
		if err := yaml.Unmarshal(content, &root); err != nil {
			return nil, isNew, fmt.Errorf("解析配置文件 %s 失败: %v", settingsFile, err)
		}
	}

	// 在解析完成后，将取到的值应用回去
	return &root.Settings, isNew, nil
}

// SaveSettings 保存全局设置到 settings.yaml 文件
func SaveSettings(settings GlobalSettings) error {
	cwd, _ := os.Getwd()
	path := filepath.Join(cwd, settingsFile)

	root := rootConfig{Settings: settings}
	content, err := yaml.Marshal(root)
	if err != nil {
		return fmt.Errorf("序列化配置失败: %v", err)
	}

	// 添加文件头注释
	header := []byte(`# NetWeaverGo 全局运行参数配置

`)
	fullContent := append(header, content...)

	err = os.WriteFile(path, fullContent, 0666)
	if err != nil {
		return fmt.Errorf("写入配置文件失败: %v", err)
	}

	return nil
}

// generateSettingsTemplate 生成默认的全局运行参数模板文件
func generateSettingsTemplate() {
	cwd, _ := os.Getwd()
	path := filepath.Join(cwd, settingsFile)

	content := []byte(`# NetWeaverGo 全局运行参数配置

settings:
  # 最大并发协程数量
  max_workers: 32
  
  # SSH/SFTP 连接阶段总体超时时间
  connect_timeout: "10s"
  
  # 单条命令执行的默认超时时间
  command_timeout: "30s"
  
  # 回显输出与配置备份的存放根目录
  output_dir: "output"
  
  # 系统运行日志存放目录
  log_dir: "logs"
  
  # 发生错误时的默认防线动作: 
  # pause (挂起询问), skip (仅提示并跳过后续同异常动作), abort (终止该设备的执行并退出)
  error_mode: "pause"
`)

	err := os.WriteFile(path, content, 0666)
	if err != nil {
		fmt.Printf("无法创建配置模板文件: %v\n", err)
	}
}
