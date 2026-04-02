package taskexec

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/NetWeaverGo/core/internal/config"
	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/models"
	"github.com/NetWeaverGo/core/internal/repository"
)

type LaunchNormalizer struct {
	deviceRepo repository.DeviceRepository
}

type LaunchValidator struct {
	repo       Repository
	deviceRepo repository.DeviceRepository
}

type TaskLaunchService struct {
	taskexec   *TaskExecutionService
	normalizer *LaunchNormalizer
	validator  *LaunchValidator
}

type CanonicalLaunchSpec struct {
	TaskGroupID       uint               `json:"taskGroupId"`
	TaskNameSnapshot  string             `json:"taskNameSnapshot"`
	TaskDescription   string             `json:"taskDescription"`
	RunKind           string             `json:"runKind"`
	Mode              string             `json:"mode,omitempty"`
	Concurrency       int                `json:"concurrency"`
	TimeoutSec        int                `json:"timeoutSec"`
	EnableRawLog      bool               `json:"enableRawLog"`
	TopologyVendor    string             `json:"topologyVendor,omitempty"`
	AutoBuildTopology bool               `json:"autoBuildTopology,omitempty"`
	Normal            *CanonicalNormal   `json:"normal,omitempty"`
	Topology          *CanonicalTopology `json:"topology,omitempty"`
}

type CanonicalNormal struct {
	Mode           string                `json:"mode"`
	GroupCommandID string                `json:"commandGroupID,omitempty"`
	GroupCommands  []string              `json:"commands,omitempty"`
	DeviceIDs      []uint                `json:"deviceIDs,omitempty"`
	DeviceIPs      []string              `json:"deviceIPs,omitempty"`
	Items          []CanonicalNormalItem `json:"items,omitempty"`
}

type CanonicalNormalItem struct {
	DeviceIDs      []uint   `json:"deviceIDs"`
	DeviceIPs      []string `json:"deviceIPs"`
	CommandGroupID string   `json:"commandGroupID,omitempty"`
	Commands       []string `json:"commands,omitempty"`
}

type CanonicalTopology struct {
	DeviceIDs      []uint                             `json:"deviceIDs"`
	DeviceIPs      []string                           `json:"deviceIPs"`
	Vendor         string                             `json:"vendor,omitempty"`
	FieldOverrides []models.TopologyTaskFieldOverride `json:"fieldOverrides,omitempty"`
}

func NewTaskLaunchService(service *TaskExecutionService) *TaskLaunchService {
	deviceRepo := repository.NewDeviceRepository()
	return &TaskLaunchService{
		taskexec: service,
		normalizer: &LaunchNormalizer{
			deviceRepo: deviceRepo,
		},
		validator: &LaunchValidator{
			repo:       service.repo,
			deviceRepo: deviceRepo,
		},
	}
}

func (s *TaskLaunchService) StartTaskGroup(ctx context.Context, taskGroupID uint) (string, error) {
	if s == nil || s.taskexec == nil {
		logger.Error("TaskLaunchService", "-", "启动任务组失败: service 未初始化, taskGroupID=%d", taskGroupID)
		return "", fmt.Errorf("task launch service not initialized")
	}

	logger.Debug("TaskLaunchService", "-", "开始启动任务组: taskGroupID=%d", taskGroupID)
	taskGroup, err := config.GetTaskGroup(taskGroupID)
	if err != nil {
		logger.Error("TaskLaunchService", "-", "加载任务组失败: taskGroupID=%d, err=%v", taskGroupID, err)
		return "", err
	}
	logger.Verbose("TaskLaunchService", "-", "任务组加载完成: id=%d, name=%s, mode=%s, taskType=%s, items=%d", taskGroup.ID, taskGroup.Name, taskGroup.Mode, taskGroup.TaskType, len(taskGroup.Items))

	spec, err := s.normalizer.NormalizeTaskGroup(taskGroup)
	if err != nil {
		logger.Error("TaskLaunchService", "-", "任务组归一化失败: taskGroupID=%d, err=%v", taskGroupID, err)
		return "", err
	}

	launchSpecJSON, err := json.Marshal(spec)
	if err != nil {
		logger.Error("TaskLaunchService", "-", "序列化启动规格失败: taskGroupID=%d, err=%v", taskGroupID, err)
		return "", fmt.Errorf("marshal launch spec failed: %w", err)
	}
	logger.Verbose("TaskLaunchService", "-", "启动规格: %s", string(launchSpecJSON))

	if err := s.validator.ValidateLaunchSpec(ctx, spec); err != nil {
		logger.Error("TaskLaunchService", "-", "启动规格校验失败: taskGroupID=%d, err=%v", taskGroupID, err)
		return "", err
	}

	def, err := s.taskexec.CreateTaskDefinitionFromLaunchSpec(spec)
	if err != nil {
		logger.Error("TaskLaunchService", "-", "创建任务定义失败: taskGroupID=%d, err=%v", taskGroupID, err)
		return "", err
	}

	runID, err := s.taskexec.StartTaskWithMetadata(ctx, def, &RunMetadata{
		TaskGroupID:      taskGroupID,
		TaskNameSnapshot: spec.TaskNameSnapshot,
		LaunchSpecJSON:   launchSpecJSON,
	})
	if err != nil {
		logger.Error("TaskLaunchService", "-", "启动任务组失败: taskGroupID=%d, err=%v", taskGroupID, err)
		return "", err
	}
	logger.Info("TaskLaunchService", "-", "任务组启动成功: taskGroupID=%d, runID=%s", taskGroupID, runID)
	return runID, nil
}

func (n *LaunchNormalizer) NormalizeTaskGroup(taskGroup *models.TaskGroup) (*CanonicalLaunchSpec, error) {
	if taskGroup == nil {
		return nil, fmt.Errorf("任务组不能为空")
	}

	spec := &CanonicalLaunchSpec{
		TaskGroupID:       taskGroup.ID,
		TaskNameSnapshot:  strings.TrimSpace(taskGroup.Name),
		TaskDescription:   strings.TrimSpace(taskGroup.Description),
		RunKind:           normalizeRunKind(taskGroup.TaskType),
		Mode:              strings.TrimSpace(taskGroup.Mode),
		Concurrency:       taskGroup.MaxWorkers,
		TimeoutSec:        taskGroup.Timeout,
		EnableRawLog:      taskGroup.EnableRawLog,
		TopologyVendor:    strings.TrimSpace(taskGroup.TopologyVendor),
		AutoBuildTopology: taskGroup.AutoBuildTopology,
	}

	switch spec.RunKind {
	case string(RunKindTopology):
		topology, err := n.normalizeTopology(taskGroup)
		if err != nil {
			return nil, err
		}
		spec.Topology = topology
	default:
		normal, err := n.normalizeNormal(taskGroup)
		if err != nil {
			return nil, err
		}
		spec.Normal = normal
	}

	return spec, nil
}

func (n *LaunchNormalizer) normalizeNormal(taskGroup *models.TaskGroup) (*CanonicalNormal, error) {
	mode := strings.TrimSpace(taskGroup.Mode)
	if mode == "" {
		mode = "group"
	}

	normal := &CanonicalNormal{Mode: mode}
	logger.Debug("TaskLaunchService", "-", "开始归一化普通任务: taskGroupID=%d, mode=%s, items=%d", taskGroup.ID, mode, len(taskGroup.Items))
	switch mode {
	case "group":
		deviceIDs := make([]uint, 0)
		deviceIPs := make([]string, 0)
		var canonicalCommands []string
		var canonicalCommandGroupID string

		for _, item := range taskGroup.Items {
			currentCommands, currentCommandGroupID, err := n.resolveTaskItemCommands(item)
			if err != nil {
				return nil, err
			}
			if len(currentCommands) == 0 {
				return nil, fmt.Errorf("group 模式任务项缺少命令")
			}

			if len(canonicalCommands) == 0 {
				canonicalCommands = append([]string(nil), currentCommands...)
				canonicalCommandGroupID = currentCommandGroupID
			} else if !sameStringSlice(canonicalCommands, currentCommands) || canonicalCommandGroupID != currentCommandGroupID {
				return nil, fmt.Errorf("group 模式存在多个不一致的命令源，拒绝启动")
			}

			deviceIDs = append(deviceIDs, item.DeviceIDs...)
			deviceIPs = append(deviceIPs, n.lookupDeviceIPs(item.DeviceIDs)...)
		}

		normal.GroupCommandID = canonicalCommandGroupID
		normal.GroupCommands = uniqueSortedStrings(canonicalCommands)
		normal.DeviceIDs = uniqueSortedUint(deviceIDs)
		normal.DeviceIPs = uniqueSortedStrings(deviceIPs)
		logger.Verbose("TaskLaunchService", "-", "普通任务归一化完成(group): taskGroupID=%d, commandGroupID=%s, deviceIDs=%d, deviceIPs=%d, commands=%d", taskGroup.ID, normal.GroupCommandID, len(normal.DeviceIDs), len(normal.DeviceIPs), len(normal.GroupCommands))
	case "binding":
		items := make([]CanonicalNormalItem, 0, len(taskGroup.Items))
		for _, item := range taskGroup.Items {
			commands, commandGroupID, err := n.resolveTaskItemCommands(item)
			if err != nil {
				return nil, err
			}
			items = append(items, CanonicalNormalItem{
				DeviceIDs:      uniqueSortedUint(item.DeviceIDs),
				DeviceIPs:      uniqueSortedStrings(n.lookupDeviceIPs(item.DeviceIDs)),
				CommandGroupID: commandGroupID,
				Commands:       uniqueSortedStrings(commands),
			})
		}
		normal.Items = items
		logger.Verbose("TaskLaunchService", "-", "普通任务归一化完成(binding): taskGroupID=%d, items=%d", taskGroup.ID, len(normal.Items))
	default:
		return nil, fmt.Errorf("不支持的任务模式: %s", mode)
	}

	return normal, nil
}

func (n *LaunchNormalizer) normalizeTopology(taskGroup *models.TaskGroup) (*CanonicalTopology, error) {
	deviceIDs := make([]uint, 0)
	for _, item := range taskGroup.Items {
		deviceIDs = append(deviceIDs, item.DeviceIDs...)
	}
	topology := &CanonicalTopology{
		DeviceIDs:      uniqueSortedUint(deviceIDs),
		DeviceIPs:      uniqueSortedStrings(n.lookupDeviceIPs(deviceIDs)),
		Vendor:         strings.TrimSpace(taskGroup.TopologyVendor),
		FieldOverrides: append([]models.TopologyTaskFieldOverride(nil), taskGroup.TopologyFieldOverrides...),
	}
	logger.Verbose("TaskLaunchService", "-", "拓扑任务归一化完成: taskGroupID=%d, deviceIDs=%d, deviceIPs=%d, vendor=%s, overrides=%d", taskGroup.ID, len(topology.DeviceIDs), len(topology.DeviceIPs), topology.Vendor, len(topology.FieldOverrides))
	return topology, nil
}

func (n *LaunchNormalizer) resolveTaskItemCommands(item models.TaskItem) ([]string, string, error) {
	if commands := normalizeCommands(item.Commands); len(commands) > 0 {
		return commands, "", nil
	}

	commandGroupID := strings.TrimSpace(item.CommandGroupID)
	if commandGroupID == "" {
		return nil, "", nil
	}

	group, err := config.GetCommandGroup(parseUintID(commandGroupID))
	if err != nil || group == nil {
		return nil, "", fmt.Errorf("命令组不存在: %s", commandGroupID)
	}
	return normalizeCommands(group.Commands), commandGroupID, nil
}

func (n *LaunchNormalizer) lookupDeviceIPs(deviceIDs []uint) []string {
	result := make([]string, 0, len(deviceIDs))
	for _, deviceID := range uniqueSortedUint(deviceIDs) {
		device, err := n.deviceRepo.FindByID(deviceID)
		if err != nil || device == nil {
			logger.Warn("TaskLaunchService", "-", "设备解析失败: deviceID=%d, err=%v", deviceID, err)
			continue
		}
		ip := strings.TrimSpace(device.IP)
		if ip != "" {
			result = append(result, ip)
			continue
		}
		logger.Warn("TaskLaunchService", "-", "设备缺少管理IP: deviceID=%d", deviceID)
	}
	return result
}

func (v *LaunchValidator) ValidateLaunchSpec(ctx context.Context, spec *CanonicalLaunchSpec) error {
	if spec == nil {
		return fmt.Errorf("启动规格不能为空")
	}
	if strings.TrimSpace(spec.TaskNameSnapshot) == "" {
		return fmt.Errorf("任务名称不能为空")
	}

	switch spec.RunKind {
	case string(RunKindTopology):
		if spec.Topology == nil || len(spec.Topology.DeviceIPs) == 0 {
			return fmt.Errorf("拓扑任务至少需要一台设备")
		}
	default:
		if spec.Normal == nil {
			return fmt.Errorf("普通任务缺少规范化配置")
		}
		if spec.Normal.Mode == "group" {
			if len(spec.Normal.GroupCommands) == 0 {
				return fmt.Errorf("group 模式至少需要一条命令")
			}
			if len(spec.Normal.DeviceIPs) == 0 {
				return fmt.Errorf("group 模式至少需要一台设备")
			}
		} else {
			if len(spec.Normal.Items) == 0 {
				return fmt.Errorf("binding 模式至少需要一个任务项")
			}
			for _, item := range spec.Normal.Items {
				if len(item.DeviceIPs) == 0 {
					return fmt.Errorf("binding 模式任务项至少需要一台设备")
				}
				if len(item.Commands) == 0 {
					return fmt.Errorf("binding 模式任务项至少需要一条命令")
				}
			}
		}
	}

	conflictIPs, err := v.findConflictingActiveRunTargets(ctx, spec)
	if err != nil {
		return err
	}
	if len(conflictIPs) > 0 {
		return fmt.Errorf("目标设备存在活跃运行冲突: %s", strings.Join(conflictIPs, ", "))
	}
	return nil
}

func (v *LaunchValidator) findConflictingActiveRunTargets(ctx context.Context, spec *CanonicalLaunchSpec) ([]string, error) {
	runs, err := v.repo.ListRunningRuns(ctx)
	if err != nil {
		return nil, err
	}
	if len(runs) == 0 {
		return nil, nil
	}

	targetSet := make(map[string]struct{})
	for _, ip := range specTargetIPs(spec) {
		targetSet[ip] = struct{}{}
	}
	if len(targetSet) == 0 {
		return nil, nil
	}

	conflicts := make(map[string]struct{})
	for _, run := range runs {
		units, unitErr := v.repo.GetUnitsByRun(ctx, run.ID)
		if unitErr != nil {
			return nil, unitErr
		}
		for _, unit := range units {
			if unit.TargetType != "device_ip" {
				continue
			}
			if _, ok := targetSet[unit.TargetKey]; ok {
				conflicts[unit.TargetKey] = struct{}{}
			}
		}
	}
	return mapKeysSorted(conflicts), nil
}

func (s *TaskExecutionService) CreateTaskDefinitionFromLaunchSpec(spec *CanonicalLaunchSpec) (*TaskDefinition, error) {
	if spec == nil {
		return nil, fmt.Errorf("launch spec is nil")
	}

	var (
		configJSON []byte
		err        error
	)

	switch spec.RunKind {
	case string(RunKindTopology):
		if spec.Topology == nil {
			return nil, fmt.Errorf("topology launch spec is nil")
		}
		taskVendor := strings.TrimSpace(spec.Topology.Vendor)
		if taskVendor == "" {
			taskVendor = strings.TrimSpace(spec.TopologyVendor)
		}
		resolver := NewTopologyCommandResolver()
		resolution, resolveErr := resolver.Resolve(taskVendor, nil, spec.Topology.FieldOverrides)
		if resolveErr != nil {
			return nil, fmt.Errorf("resolve topology commands failed: %w", resolveErr)
		}
		configJSON, err = json.Marshal(&TopologyTaskConfig{
			DeviceIDs:         append([]uint(nil), spec.Topology.DeviceIDs...),
			DeviceIPs:         append([]string(nil), spec.Topology.DeviceIPs...),
			Vendor:            taskVendor,
			FieldOverrides:    append([]models.TopologyTaskFieldOverride(nil), spec.Topology.FieldOverrides...),
			ResolvedCommands:  append([]ResolvedTopologyCommand(nil), resolution.Commands...),
			MaxWorkers:        spec.Concurrency,
			TimeoutSec:        spec.TimeoutSec,
			AutoBuildTopology: spec.AutoBuildTopology,
			EnableRawLog:      spec.EnableRawLog,
		})
		logger.Debug("TaskLaunchService", "-", "创建拓扑任务定义: taskGroupID=%d, deviceIDs=%d, deviceIPs=%d, vendor=%s, resolvedVendor=%s, overrides=%d", spec.TaskGroupID, len(spec.Topology.DeviceIDs), len(spec.Topology.DeviceIPs), taskVendor, resolution.ResolvedVendor, len(spec.Topology.FieldOverrides))
	default:
		if spec.Normal == nil {
			return nil, fmt.Errorf("normal launch spec is nil")
		}
		mode := strings.TrimSpace(spec.Normal.Mode)
		if mode == "" {
			mode = strings.TrimSpace(spec.Mode)
		}
		if mode == "" {
			mode = "group"
		}
		config := &NormalTaskConfig{
			Mode:           mode,
			DeviceIDs:      append([]uint(nil), spec.Normal.DeviceIDs...),
			DeviceIPs:      append([]string(nil), spec.Normal.DeviceIPs...),
			CommandGroupID: strings.TrimSpace(spec.Normal.GroupCommandID),
			Commands:       append([]string(nil), spec.Normal.GroupCommands...),
			Concurrency:    spec.Concurrency,
			TimeoutSec:     spec.TimeoutSec,
			EnableRawLog:   spec.EnableRawLog,
		}
		if mode == "binding" {
			config.Items = make([]NormalTaskItem, 0, len(spec.Normal.Items))
			for _, item := range spec.Normal.Items {
				config.Items = append(config.Items, NormalTaskItem{
					DeviceIDs:      append([]uint(nil), item.DeviceIDs...),
					DeviceIPs:      append([]string(nil), item.DeviceIPs...),
					CommandGroupID: strings.TrimSpace(item.CommandGroupID),
					Commands:       append([]string(nil), item.Commands...),
				})
			}
		}
		configJSON, err = json.Marshal(config)
		logger.Debug("TaskLaunchService", "-", "创建普通任务定义: taskGroupID=%d, mode=%s, deviceIDs=%d, deviceIPs=%d, items=%d, commandGroupID=%s", spec.TaskGroupID, config.Mode, len(config.DeviceIDs), len(config.DeviceIPs), len(config.Items), config.CommandGroupID)
	}
	if err != nil {
		return nil, fmt.Errorf("marshal compiler config failed: %w", err)
	}

	return &TaskDefinition{
		ID:     newDefinitionID(),
		Name:   spec.TaskNameSnapshot,
		Kind:   spec.RunKind,
		Config: configJSON,
	}, nil
}

func parseUintID(value string) uint {
	var parsed uint
	fmt.Sscanf(strings.TrimSpace(value), "%d", &parsed)
	return parsed
}

func normalizeRunKind(taskType string) string {
	value := strings.ToLower(strings.TrimSpace(taskType))
	if value == string(RunKindTopology) {
		return string(RunKindTopology)
	}
	return string(RunKindNormal)
}

func normalizeCommands(commands []string) []string {
	result := make([]string, 0, len(commands))
	for _, command := range commands {
		trimmed := strings.TrimSpace(command)
		if trimmed == "" {
			continue
		}
		result = append(result, trimmed)
	}
	return result
}

func uniqueSortedUint(values []uint) []uint {
	set := make(map[uint]struct{})
	for _, value := range values {
		if value == 0 {
			continue
		}
		set[value] = struct{}{}
	}
	result := make([]uint, 0, len(set))
	for value := range set {
		result = append(result, value)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}

func uniqueSortedStrings(values []string) []string {
	set := make(map[string]struct{})
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		set[trimmed] = struct{}{}
	}
	result := make([]string, 0, len(set))
	for value := range set {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func sameStringSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for index := range a {
		if strings.TrimSpace(a[index]) != strings.TrimSpace(b[index]) {
			return false
		}
	}
	return true
}

func specTargetIPs(spec *CanonicalLaunchSpec) []string {
	result := make([]string, 0)
	if spec == nil {
		return result
	}
	if spec.Normal != nil {
		if spec.Normal.Mode == "group" {
			result = append(result, spec.Normal.DeviceIPs...)
		} else {
			for _, item := range spec.Normal.Items {
				result = append(result, item.DeviceIPs...)
			}
		}
	}
	if spec.Topology != nil {
		result = append(result, spec.Topology.DeviceIPs...)
	}
	return uniqueSortedStrings(result)
}

func mapKeysSorted(values map[string]struct{}) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
