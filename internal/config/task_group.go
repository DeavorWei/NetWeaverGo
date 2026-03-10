package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/google/uuid"
)

// 有效状态值常量
var validStatuses = map[string]bool{
	"pending":   true,
	"running":   true,
	"completed": true,
	"failed":    true,
}

// TaskItem 单个任务项（一组命令绑定一组设备）
type TaskItem struct {
	CommandGroupID string   `json:"commandGroupId"` // 命令组ID（模式A使用）
	Commands       []string `json:"commands"`       // 直接命令列表（模式B使用）
	DeviceIPs      []string `json:"deviceIPs"`      // 绑定的设备IP列表
}

// TaskGroup 任务组
type TaskGroup struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Mode        string     `json:"mode"`  // "group" 模式A | "binding" 模式B
	Items       []TaskItem `json:"items"` // 任务项列表
	Tags        []string   `json:"tags"`
	Status      string     `json:"status"` // "pending" | "running" | "completed" | "failed"
	CreatedAt   string     `json:"createdAt"`
	UpdatedAt   string     `json:"updatedAt"`
}

// TaskGroupsFile 任务组存储文件结构
type TaskGroupsFile struct {
	Groups []TaskGroup `json:"groups"`
}

// 任务组存储相关常量
const (
	taskGroupsFile = "tasks.json"
)

// 全局任务组管理器
var (
	tasksMu     sync.RWMutex
	tasksCached *TaskGroupsFile
)

// getTaskGroupsPath 获取任务组文件路径
func getTaskGroupsPath() string {
	cwd, _ := os.Getwd()
	dir := filepath.Join(cwd, commandGroupsDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		logger.Warn("Config", "-", "创建任务组目录失败: %v", err)
	}
	return filepath.Join(dir, taskGroupsFile)
}

// loadTaskGroupsFromFile 从文件加载任务组
func loadTaskGroupsFromFile() (*TaskGroupsFile, error) {
	path := getTaskGroupsPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &TaskGroupsFile{Groups: []TaskGroup{}}, nil
		}
		return nil, fmt.Errorf("读取任务组文件失败: %v", err)
	}
	var file TaskGroupsFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("解析任务组文件失败: %v", err)
	}
	if file.Groups == nil {
		file.Groups = []TaskGroup{}
	}
	return &file, nil
}

// saveTaskGroupsToFile 保存任务组到文件
func saveTaskGroupsToFile(data *TaskGroupsFile) error {
	path := getTaskGroupsPath()
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化任务组失败: %v", err)
	}
	if err := os.WriteFile(path, jsonData, 0644); err != nil {
		return fmt.Errorf("写入任务组文件失败: %v", err)
	}
	tasksCached = data
	return nil
}

// ========== 任务组管理 API ==========

// ListTaskGroups 获取所有任务组列表
func ListTaskGroups() ([]TaskGroup, error) {
	tasksMu.RLock()
	defer tasksMu.RUnlock()

	if tasksCached != nil {
		// 返回深拷贝副本，防止外部修改影响缓存
		return deepCopyTaskGroups(tasksCached.Groups)
	}

	file, err := loadTaskGroupsFromFile()
	if err != nil {
		return nil, err
	}
	tasksCached = file
	// 返回深拷贝副本
	return deepCopyTaskGroups(file.Groups)
}

// GetTaskGroup 根据 ID 获取单个任务组
func GetTaskGroup(id string) (*TaskGroup, error) {
	tasksMu.RLock()
	defer tasksMu.RUnlock()

	// 优先使用缓存
	if tasksCached != nil {
		for _, g := range tasksCached.Groups {
			if g.ID == id {
				// 返回深拷贝副本，防止外部修改
				return deepCopyTaskGroup(g)
			}
		}
		return nil, fmt.Errorf("任务组不存在: %s", id)
	}

	// 缓存为空时才读取文件
	file, err := loadTaskGroupsFromFile()
	if err != nil {
		return nil, err
	}
	for _, g := range file.Groups {
		if g.ID == id {
			return deepCopyTaskGroup(g)
		}
	}
	return nil, fmt.Errorf("任务组不存在: %s", id)
}

// CreateTaskGroup 创建新任务组
func CreateTaskGroup(group TaskGroup) (*TaskGroup, error) {
	tasksMu.Lock()
	defer tasksMu.Unlock()

	file, err := loadTaskGroupsFromFile()
	if err != nil {
		return nil, err
	}

	// 校验
	if err := validateTaskGroup(group); err != nil {
		return nil, err
	}

	// 生成ID和时间戳
	group.ID = uuid.New().String()
	group.CreatedAt = nowFormatted()
	group.UpdatedAt = nowFormatted()
	if group.Status == "" {
		group.Status = "pending"
	}
	if group.Tags == nil {
		group.Tags = []string{}
	}

	file.Groups = append(file.Groups, group)
	if err := saveTaskGroupsToFile(file); err != nil {
		return nil, err
	}

	logger.Info("Config", "-", "创建任务组: %s (%s)", group.Name, group.ID)
	return &group, nil
}

// UpdateTaskGroup 更新任务组
func UpdateTaskGroup(id string, group TaskGroup) (*TaskGroup, error) {
	tasksMu.Lock()
	defer tasksMu.Unlock()

	file, err := loadTaskGroupsFromFile()
	if err != nil {
		return nil, err
	}

	found := false
	for i, g := range file.Groups {
		if g.ID == id {
			group.ID = id
			group.CreatedAt = g.CreatedAt
			group.UpdatedAt = nowFormatted()
			if group.Tags == nil {
				group.Tags = []string{}
			}
			file.Groups[i] = group
			found = true
			break
		}
	}

	if !found {
		return nil, fmt.Errorf("任务组不存在: %s", id)
	}

	if err := saveTaskGroupsToFile(file); err != nil {
		return nil, err
	}

	logger.Info("Config", "-", "更新任务组: %s (%s)", group.Name, id)
	return &group, nil
}

// DeleteTaskGroup 删除任务组
func DeleteTaskGroup(id string) error {
	tasksMu.Lock()
	defer tasksMu.Unlock()

	file, err := loadTaskGroupsFromFile()
	if err != nil {
		return err
	}

	found := false
	for i, g := range file.Groups {
		if g.ID == id {
			file.Groups = append(file.Groups[:i], file.Groups[i+1:]...)
			found = true
			logger.Info("Config", "-", "删除任务组: %s (%s)", g.Name, id)
			break
		}
	}

	if !found {
		return fmt.Errorf("任务组不存在: %s", id)
	}

	return saveTaskGroupsToFile(file)
}

// UpdateTaskGroupStatus 更新任务组状态
func UpdateTaskGroupStatus(id string, status string) error {
	// 校验状态值有效性
	if !validStatuses[status] {
		return fmt.Errorf("无效的状态值: %s (应为 pending/running/completed/failed)", status)
	}

	tasksMu.Lock()
	defer tasksMu.Unlock()

	file, err := loadTaskGroupsFromFile()
	if err != nil {
		return err
	}

	for i, g := range file.Groups {
		if g.ID == id {
			file.Groups[i].Status = status
			file.Groups[i].UpdatedAt = nowFormatted()
			return saveTaskGroupsToFile(file)
		}
	}

	return fmt.Errorf("任务组不存在: %s", id)
}

// validateTaskGroup 校验任务组
func validateTaskGroup(group TaskGroup) error {
	if group.Name == "" {
		return fmt.Errorf("任务组名称不能为空")
	}
	if group.Mode != "group" && group.Mode != "binding" {
		return fmt.Errorf("无效的任务模式: %s (应为 group 或 binding)", group.Mode)
	}
	if len(group.Items) == 0 {
		return fmt.Errorf("任务项不能为空")
	}
	for i, item := range group.Items {
		if len(item.DeviceIPs) == 0 {
			return fmt.Errorf("第 %d 个任务项的设备列表不能为空", i+1)
		}
		if group.Mode == "group" && item.CommandGroupID == "" {
			return fmt.Errorf("模式A下命令组ID不能为空")
		}
		if group.Mode == "binding" && len(item.Commands) == 0 {
			return fmt.Errorf("第 %d 个任务项的命令列表不能为空", i+1)
		}
	}
	return nil
}

// deepCopyTaskGroup 深拷贝单个任务组
func deepCopyTaskGroup(g TaskGroup) (*TaskGroup, error) {
	data, err := json.Marshal(g)
	if err != nil {
		return nil, fmt.Errorf("序列化任务组失败: %v", err)
	}
	var copy TaskGroup
	if err := json.Unmarshal(data, &copy); err != nil {
		return nil, fmt.Errorf("反序列化任务组失败: %v", err)
	}
	return &copy, nil
}

// deepCopyTaskGroups 深拷贝任务组列表
func deepCopyTaskGroups(groups []TaskGroup) ([]TaskGroup, error) {
	if groups == nil {
		return nil, nil
	}
	data, err := json.Marshal(groups)
	if err != nil {
		return nil, fmt.Errorf("序列化任务组列表失败: %v", err)
	}
	var copy []TaskGroup
	if err := json.Unmarshal(data, &copy); err != nil {
		return nil, fmt.Errorf("反序列化任务组列表失败: %v", err)
	}
	return copy, nil
}
