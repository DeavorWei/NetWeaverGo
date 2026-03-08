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
	"github.com/google/uuid"
)

// CommandGroup 命令组定义
type CommandGroup struct {
	ID          string   `json:"id"`          // 唯一标识（UUID）
	Name        string   `json:"name"`        // 命令组名称
	Description string   `json:"description"` // 描述信息
	Commands    []string `json:"commands"`    // 命令列表
	CreatedAt   string   `json:"createdAt"`   // 创建时间
	UpdatedAt   string   `json:"updatedAt"`   // 更新时间
	Tags        []string `json:"tags"`        // 标签（用于分类）
}

// CommandGroupsFile 命令组存储文件结构
type CommandGroupsFile struct {
	Groups []CommandGroup `json:"groups"`
}

// 命令组存储相关常量
const (
	commandGroupsDir  = "commands"
	commandGroupsFile = "groups.json"
)

// 时间格式
const TimeFormat = "2006-01-02T15:04:05"

// 全局命令组管理器
var (
	groupsMu     sync.RWMutex
	groupsCached *CommandGroupsFile
)

// getCommandGroupsPath 获取命令组文件路径
func getCommandGroupsPath() string {
	cwd, _ := os.Getwd()
	// 确保目录存在
	dirPath := filepath.Join(cwd, commandGroupsDir)
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		os.MkdirAll(dirPath, 0755)
	}
	return filepath.Join(dirPath, commandGroupsFile)
}

// loadCommandGroupsFromFile 从文件加载命令组
func loadCommandGroupsFromFile() (*CommandGroupsFile, error) {
	path := getCommandGroupsPath()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// 文件不存在，返回空结构
			return &CommandGroupsFile{Groups: []CommandGroup{}}, nil
		}
		return nil, fmt.Errorf("读取命令组文件失败: %v", err)
	}

	var fileData CommandGroupsFile
	if err := json.Unmarshal(data, &fileData); err != nil {
		return nil, fmt.Errorf("解析命令组文件失败: %v", err)
	}

	return &fileData, nil
}

// saveCommandGroupsToFile 保存命令组到文件
func saveCommandGroupsToFile(data *CommandGroupsFile) error {
	path := getCommandGroupsPath()

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化命令组失败: %v", err)
	}

	if err := os.WriteFile(path, jsonData, 0666); err != nil {
		return fmt.Errorf("写入命令组文件失败: %v", err)
	}

	logger.Info("Config", "-", "成功保存 %d 个命令组到 %s", len(data.Groups), path)
	return nil
}

// nowFormatted 获取当前时间格式化字符串
func nowFormatted() string {
	return time.Now().Format(TimeFormat)
}

// generateID 生成唯一标识
func generateID() string {
	return uuid.New().String()
}

// ========== 命令组管理 API ==========

// ListCommandGroups 获取所有命令组列表
func ListCommandGroups() ([]CommandGroup, error) {
	groupsMu.RLock()
	defer groupsMu.RUnlock()

	data, err := loadCommandGroupsFromFile()
	if err != nil {
		return nil, err
	}

	groupsCached = data
	return data.Groups, nil
}

// GetCommandGroup 根据 ID 获取单个命令组
func GetCommandGroup(id string) (*CommandGroup, error) {
	groupsMu.RLock()
	defer groupsMu.RUnlock()

	data, err := loadCommandGroupsFromFile()
	if err != nil {
		return nil, err
	}

	for _, group := range data.Groups {
		if group.ID == id {
			return &group, nil
		}
	}

	return nil, fmt.Errorf("未找到命令组: %s", id)
}

// CreateCommandGroup 创建新命令组
func CreateCommandGroup(group CommandGroup) (*CommandGroup, error) {
	groupsMu.Lock()
	defer groupsMu.Unlock()

	data, err := loadCommandGroupsFromFile()
	if err != nil {
		return nil, err
	}

	// 生成 ID 和时间戳
	group.ID = generateID()
	group.CreatedAt = nowFormatted()
	group.UpdatedAt = group.CreatedAt

	// 校验
	if err := validateCommandGroup(group); err != nil {
		return nil, err
	}

	// 检查名称是否重复
	for _, g := range data.Groups {
		if g.Name == group.Name {
			return nil, fmt.Errorf("命令组名称已存在: %s", group.Name)
		}
	}

	data.Groups = append(data.Groups, group)

	if err := saveCommandGroupsToFile(data); err != nil {
		return nil, err
	}

	logger.Info("Config", "-", "成功创建命令组: %s", group.Name)
	return &group, nil
}

// UpdateCommandGroup 更新命令组
func UpdateCommandGroup(id string, group CommandGroup) (*CommandGroup, error) {
	groupsMu.Lock()
	defer groupsMu.Unlock()

	data, err := loadCommandGroupsFromFile()
	if err != nil {
		return nil, err
	}

	// 查找要更新的命令组
	index := -1
	for i, g := range data.Groups {
		if g.ID == id {
			index = i
			break
		}
	}

	if index == -1 {
		return nil, fmt.Errorf("未找到命令组: %s", id)
	}

	// 保留原有的 ID 和创建时间
	group.ID = id
	group.CreatedAt = data.Groups[index].CreatedAt
	group.UpdatedAt = nowFormatted()

	// 校验
	if err := validateCommandGroup(group); err != nil {
		return nil, err
	}

	// 检查名称是否与其他命令组重复
	for i, g := range data.Groups {
		if i != index && g.Name == group.Name {
			return nil, fmt.Errorf("命令组名称已存在: %s", group.Name)
		}
	}

	data.Groups[index] = group

	if err := saveCommandGroupsToFile(data); err != nil {
		return nil, err
	}

	logger.Info("Config", "-", "成功更新命令组: %s", group.Name)
	return &group, nil
}

// DeleteCommandGroup 删除命令组
func DeleteCommandGroup(id string) error {
	groupsMu.Lock()
	defer groupsMu.Unlock()

	data, err := loadCommandGroupsFromFile()
	if err != nil {
		return err
	}

	// 查找要删除的命令组
	index := -1
	for i, g := range data.Groups {
		if g.ID == id {
			index = i
			break
		}
	}

	if index == -1 {
		return fmt.Errorf("未找到命令组: %s", id)
	}

	// 删除命令组
	deletedName := data.Groups[index].Name
	data.Groups = append(data.Groups[:index], data.Groups[index+1:]...)

	if err := saveCommandGroupsToFile(data); err != nil {
		return err
	}

	logger.Info("Config", "-", "成功删除命令组: %s", deletedName)
	return nil
}

// DuplicateCommandGroup 复制命令组
func DuplicateCommandGroup(id string) (*CommandGroup, error) {
	groupsMu.Lock()
	defer groupsMu.Unlock()

	data, err := loadCommandGroupsFromFile()
	if err != nil {
		return nil, err
	}

	// 查找要复制的命令组
	var source *CommandGroup
	for i := range data.Groups {
		if data.Groups[i].ID == id {
			source = &data.Groups[i]
			break
		}
	}

	if source == nil {
		return nil, fmt.Errorf("未找到命令组: %s", id)
	}

	// 创建副本
	newGroup := CommandGroup{
		ID:          generateID(),
		Name:        source.Name + " - 副本",
		Description: source.Description,
		Commands:    append([]string{}, source.Commands...),
		CreatedAt:   nowFormatted(),
		UpdatedAt:   nowFormatted(),
		Tags:        append([]string{}, source.Tags...),
	}

	// 确保名称唯一
	baseName := newGroup.Name
	counter := 1
	for {
		nameExists := false
		for _, g := range data.Groups {
			if g.Name == newGroup.Name {
				nameExists = true
				break
			}
		}
		if !nameExists {
			break
		}
		counter++
		newGroup.Name = fmt.Sprintf("%s (%d)", baseName, counter)
	}

	data.Groups = append(data.Groups, newGroup)

	if err := saveCommandGroupsToFile(data); err != nil {
		return nil, err
	}

	logger.Info("Config", "-", "成功复制命令组: %s -> %s", source.Name, newGroup.Name)
	return &newGroup, nil
}

// ImportCommandGroup 从 JSON 文件导入命令组
func ImportCommandGroup(filePath string) (*CommandGroup, error) {
	groupsMu.Lock()
	defer groupsMu.Unlock()

	// 读取文件
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("读取导入文件失败: %v", err)
	}

	var group CommandGroup
	if err := json.Unmarshal(data, &group); err != nil {
		return nil, fmt.Errorf("解析导入文件失败: %v", err)
	}

	// 生成新的 ID 和时间
	group.ID = generateID()
	group.CreatedAt = nowFormatted()
	group.UpdatedAt = nowFormatted()

	// 校验
	if err := validateCommandGroup(group); err != nil {
		return nil, err
	}

	// 加载现有数据
	existingData, err := loadCommandGroupsFromFile()
	if err != nil {
		return nil, err
	}

	// 确保名称唯一
	baseName := group.Name
	counter := 1
	for {
		nameExists := false
		for _, g := range existingData.Groups {
			if g.Name == group.Name {
				nameExists = true
				break
			}
		}
		if !nameExists {
			break
		}
		counter++
		group.Name = fmt.Sprintf("%s (%d)", baseName, counter)
	}

	existingData.Groups = append(existingData.Groups, group)

	if err := saveCommandGroupsToFile(existingData); err != nil {
		return nil, err
	}

	logger.Info("Config", "-", "成功导入命令组: %s", group.Name)
	return &group, nil
}

// ExportCommandGroup 导出命令组到 JSON 文件
func ExportCommandGroup(id string, filePath string) error {
	groupsMu.RLock()
	defer groupsMu.RUnlock()

	data, err := loadCommandGroupsFromFile()
	if err != nil {
		return err
	}

	// 查找命令组
	var group *CommandGroup
	for i := range data.Groups {
		if data.Groups[i].ID == id {
			group = &data.Groups[i]
			break
		}
	}

	if group == nil {
		return fmt.Errorf("未找到命令组: %s", id)
	}

	// 序列化
	jsonData, err := json.MarshalIndent(group, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化命令组失败: %v", err)
	}

	// 确保目录存在
	dir := filepath.Dir(filePath)
	if dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("创建目录失败: %v", err)
		}
	}

	if err := os.WriteFile(filePath, jsonData, 0666); err != nil {
		return fmt.Errorf("写入导出文件失败: %v", err)
	}

	logger.Info("Config", "-", "成功导出命令组到: %s", filePath)
	return nil
}

// validateCommandGroup 校验命令组
func validateCommandGroup(group CommandGroup) error {
	if strings.TrimSpace(group.Name) == "" {
		return fmt.Errorf("命令组名称不能为空")
	}

	if len(group.Commands) == 0 {
		return fmt.Errorf("命令列表不能为空")
	}

	// 过滤空命令
	var validCommands []string
	for _, cmd := range group.Commands {
		trimmed := strings.TrimSpace(cmd)
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			validCommands = append(validCommands, trimmed)
		}
	}

	if len(validCommands) == 0 {
		return fmt.Errorf("命令列表不能为空")
	}

	group.Commands = validCommands
	return nil
}

// MigrateLegacyCommands 迁移旧版命令文件到命令组
func MigrateLegacyCommands() error {
	groupsMu.Lock()
	defer groupsMu.Unlock()

	// 检查 groups.json 是否已存在
	path := getCommandGroupsPath()
	if _, err := os.Stat(path); err == nil {
		// 文件已存在，无需迁移
		return nil
	}

	// 检查 config.txt 是否存在
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		// config.txt 不存在，无需迁移
		return nil
	}

	// 读取旧命令
	commands, err := readCommands()
	if err != nil {
		// 读取失败或文件为空
		return nil
	}

	// 创建默认命令组
	defaultGroup := CommandGroup{
		ID:          generateID(),
		Name:        "默认命令组",
		Description: "从 config.txt 自动迁移的默认命令组",
		Commands:    commands,
		CreatedAt:   nowFormatted(),
		UpdatedAt:   nowFormatted(),
		Tags:        []string{"系统默认"},
	}

	data := &CommandGroupsFile{
		Groups: []CommandGroup{defaultGroup},
	}

	if err := saveCommandGroupsToFile(data); err != nil {
		return err
	}

	logger.Info("Config", "-", "成功迁移 config.txt 到默认命令组")
	return nil
}

// GetCommandGroupCommands 获取指定命令组的命令列表
func GetCommandGroupCommands(id string) ([]string, error) {
	group, err := GetCommandGroup(id)
	if err != nil {
		return nil, err
	}
	return group.Commands, nil
}
