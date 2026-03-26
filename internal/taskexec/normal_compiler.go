package taskexec

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/NetWeaverGo/core/internal/config"
)

// NormalTaskCompiler 普通任务编译器
type NormalTaskCompiler struct {
	options *CompileOptions
}

// NewNormalTaskCompiler 创建普通任务编译器
func NewNormalTaskCompiler(options *CompileOptions) *NormalTaskCompiler {
	if options == nil {
		options = DefaultCompileOptions()
	}
	return &NormalTaskCompiler{options: options}
}

// Compile 编译普通任务定义
func (c *NormalTaskCompiler) Compile(ctx context.Context, def *TaskDefinition) (*ExecutionPlan, error) {
	var config NormalTaskConfig
	if err := json.Unmarshal(def.Config, &config); err != nil {
		return nil, fmt.Errorf("解析普通任务配置失败: %w", err)
	}

	switch config.Mode {
	case "group":
		return c.compileModeA(def, &config)
	case "binding":
		return c.compileModeB(def, &config)
	default:
		return nil, fmt.Errorf("未知的普通任务模式: %s", config.Mode)
	}
}

// Supports 返回是否支持
func (c *NormalTaskCompiler) Supports(kind string) bool {
	return kind == string(RunKindNormal)
}

// compileModeA 编译模式A：一组命令发送给所有设备
func (c *NormalTaskCompiler) compileModeA(def *TaskDefinition, config *NormalTaskConfig) (*ExecutionPlan, error) {
	deviceIPs := normalizeDeviceIPs(config.DeviceIPs)
	if len(deviceIPs) == 0 {
		return nil, fmt.Errorf("Mode A需要至少一台设备")
	}

	commands := c.resolveCommands(config)
	if len(commands) == 0 {
		return nil, fmt.Errorf("Mode A需要至少一条命令")
	}

	// 创建一个Stage，包含所有设备作为Units
	units := make([]UnitPlan, 0, len(deviceIPs))
	for i, deviceIP := range deviceIPs {
		steps := make([]StepPlan, 0, len(commands))
		for i, cmd := range commands {
			steps = append(steps, StepPlan{
				ID:         fmt.Sprintf("step-%d", i),
				Kind:       "command",
				Name:       fmt.Sprintf("命令%d", i+1),
				Command:    cmd,
				CommandKey: fmt.Sprintf("cmd_%d", i),
			})
		}

		units = append(units, UnitPlan{
			ID:      fmt.Sprintf("unit-%d", i),
			Kind:    string(UnitKindDevice),
			Target:  TargetRef{Type: "device_ip", Key: deviceIP},
			Timeout: time.Duration(config.TimeoutSec) * time.Second,
			Steps:   steps,
		})
	}

	concurrency := config.Concurrency
	if concurrency <= 0 {
		concurrency = c.options.DefaultConcurrency
	}

	stage := StagePlan{
		ID:          newStageID(),
		Kind:        string(StageKindDeviceCommand),
		Name:        "命令执行",
		Order:       1,
		Concurrency: concurrency,
		Units:       units,
	}

	return &ExecutionPlan{
		RunKind: string(RunKindNormal),
		Name:    def.Name,
		Stages:  []StagePlan{stage},
	}, nil
}

// compileModeB 编译模式B：每台设备执行各自的独立命令
func (c *NormalTaskCompiler) compileModeB(def *TaskDefinition, config *NormalTaskConfig) (*ExecutionPlan, error) {
	if len(config.Items) == 0 {
		return nil, fmt.Errorf("Mode B需要至少一个任务项")
	}

	// 所有任务项合并到一个Stage，每个任务项展开为多个Unit
	units := make([]UnitPlan, 0)

	for itemIdx, item := range config.Items {
		commands := c.resolveItemCommands(&item)
		if len(commands) == 0 {
			continue
		}

		deviceIPs := normalizeDeviceIPs(item.DeviceIPs)
		for _, deviceIP := range deviceIPs {
			steps := make([]StepPlan, 0, len(commands))
			for cmdIdx, cmd := range commands {
				steps = append(steps, StepPlan{
					ID:         fmt.Sprintf("step-%d-%d", itemIdx, cmdIdx),
					Kind:       "command",
					Name:       fmt.Sprintf("任务%d-命令%d", itemIdx+1, cmdIdx+1),
					Command:    cmd,
					CommandKey: fmt.Sprintf("item%d_cmd%d", itemIdx, cmdIdx),
				})
			}

			units = append(units, UnitPlan{
				ID:      fmt.Sprintf("unit-%d-%s", itemIdx, deviceIP),
				Kind:    string(UnitKindDevice),
				Target:  TargetRef{Type: "device_ip", Key: deviceIP},
				Timeout: time.Duration(config.TimeoutSec) * time.Second,
				Steps:   steps,
			})
		}
	}

	if len(units) == 0 {
		return nil, fmt.Errorf("Mode B没有可执行的单元")
	}

	concurrency := config.Concurrency
	if concurrency <= 0 {
		concurrency = c.options.DefaultConcurrency
	}

	stage := StagePlan{
		ID:          newStageID(),
		Kind:        string(StageKindDeviceCommand),
		Name:        "命令执行",
		Order:       1,
		Concurrency: concurrency,
		Units:       units,
	}

	return &ExecutionPlan{
		RunKind: string(RunKindNormal),
		Name:    def.Name,
		Stages:  []StagePlan{stage},
	}, nil
}

// resolveCommands 解析命令列表 (Mode A)
func (c *NormalTaskCompiler) resolveCommands(taskConfig *NormalTaskConfig) []string {
	// 如果直接指定了命令，优先使用
	if len(taskConfig.Commands) > 0 {
		return filterEmptyCommands(taskConfig.Commands)
	}

	// 否则从命令组获取
	if taskConfig.CommandGroupID == "" {
		return nil
	}
	commandGroupID, err := strconv.ParseUint(strings.TrimSpace(taskConfig.CommandGroupID), 10, 64)
	if err != nil {
		return nil
	}
	group, err := config.GetCommandGroup(uint(commandGroupID))
	if err != nil || group == nil {
		return nil
	}
	return filterEmptyCommands(group.Commands)
}

func normalizeDeviceIPs(deviceIPs []string) []string {
	result := make([]string, 0, len(deviceIPs))
	for _, ip := range deviceIPs {
		value := strings.TrimSpace(ip)
		if value == "" {
			continue
		}
		result = append(result, value)
	}
	return result
}

// resolveItemCommands 解析任务项命令列表 (Mode B)
func (c *NormalTaskCompiler) resolveItemCommands(item *NormalTaskItem) []string {
	if len(item.Commands) > 0 {
		return filterEmptyCommands(item.Commands)
	}
	if item.CommandGroupID == "" {
		return nil
	}
	commandGroupID, err := strconv.ParseUint(strings.TrimSpace(item.CommandGroupID), 10, 64)
	if err != nil {
		return nil
	}
	group, err := config.GetCommandGroup(uint(commandGroupID))
	if err != nil || group == nil {
		return nil
	}
	return filterEmptyCommands(group.Commands)
}

// filterEmptyCommands 过滤空命令
func filterEmptyCommands(commands []string) []string {
	result := make([]string, 0, len(commands))
	for _, cmd := range commands {
		if cmd != "" {
			result = append(result, cmd)
		}
	}
	return result
}
