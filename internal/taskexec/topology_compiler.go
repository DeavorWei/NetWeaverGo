package taskexec

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// TopologyTaskCompiler 拓扑任务编译器
type TopologyTaskCompiler struct {
	options *CompileOptions
}

// NewTopologyTaskCompiler 创建拓扑任务编译器
func NewTopologyTaskCompiler(options *CompileOptions) *TopologyTaskCompiler {
	if options == nil {
		options = DefaultCompileOptions()
	}
	return &TopologyTaskCompiler{options: options}
}

// Compile 编译拓扑任务定义
func (c *TopologyTaskCompiler) Compile(ctx context.Context, def *TaskDefinition) (*ExecutionPlan, error) {
	var config TopologyTaskConfig
	if err := json.Unmarshal(def.Config, &config); err != nil {
		return nil, fmt.Errorf("解析拓扑任务配置失败: %w", err)
	}

	if len(config.DeviceIPs) == 0 && len(config.GroupNames) == 0 {
		return nil, fmt.Errorf("拓扑任务需要至少一个设备或设备组")
	}

	// 构建多阶段计划: collect -> parse -> topology_build
	stages := make([]StagePlan, 0, 3)

	// Stage 1: 设备采集
	collectStage := c.buildCollectStage(&config)
	stages = append(stages, collectStage)

	// Stage 2: 解析 (针对采集成功的设备)
	parseStage := c.buildParseStage(&config)
	stages = append(stages, parseStage)

	// Stage 3: 拓扑构建
	buildStage := c.buildTopologyBuildStage(&config)
	stages = append(stages, buildStage)

	return &ExecutionPlan{
		RunKind: string(RunKindTopology),
		Name:    def.Name,
		Stages:  stages,
	}, nil
}

// Supports 返回是否支持
func (c *TopologyTaskCompiler) Supports(kind string) bool {
	return kind == string(RunKindTopology)
}

// buildCollectStage 构建设备采集阶段
func (c *TopologyTaskCompiler) buildCollectStage(config *TopologyTaskConfig) StagePlan {
	// 每台设备一个Unit
	units := make([]UnitPlan, 0, len(config.DeviceIPs))

	for _, deviceIP := range config.DeviceIPs {
		// 根据厂商确定采集命令
		steps := c.buildCollectSteps(config.Vendor)

		units = append(units, UnitPlan{
			ID:      fmt.Sprintf("collect-%s", deviceIP),
			Kind:    string(UnitKindDevice),
			Target:  TargetRef{Type: "device_ip", Key: deviceIP},
			Timeout: time.Duration(config.TimeoutSec) * time.Second,
			Steps:   steps,
		})
	}

	concurrency := config.MaxWorkers
	if concurrency <= 0 {
		concurrency = c.options.DefaultDiscoveryWorkers
	}

	return StagePlan{
		ID:          uuid.New().String()[:8],
		Kind:        string(StageKindDeviceCollect),
		Name:        "设备信息采集",
		Order:       1,
		Concurrency: concurrency,
		Units:       units,
	}
}

// buildParseStage 构建解析阶段
func (c *TopologyTaskCompiler) buildParseStage(config *TopologyTaskConfig) StagePlan {
	// 每台设备一个Unit，输入为采集阶段的输出
	units := make([]UnitPlan, 0, len(config.DeviceIPs))

	for _, deviceIP := range config.DeviceIPs {
		units = append(units, UnitPlan{
			ID:     fmt.Sprintf("parse-%s", deviceIP),
			Kind:   string(UnitKindDevice),
			Target: TargetRef{Type: "device_ip", Key: deviceIP},
			Steps: []StepPlan{
				{
					ID:   fmt.Sprintf("parse-step-%s", deviceIP),
					Kind: "parse",
					Name: "解析设备信息",
				},
			},
		})
	}

	return StagePlan{
		ID:          uuid.New().String()[:8],
		Kind:        string(StageKindParse),
		Name:        "信息解析",
		Order:       2,
		Concurrency: len(units), // 解析可以全并发
		Units:       units,
	}
}

// buildTopologyBuildStage 构建拓扑构建阶段
func (c *TopologyTaskCompiler) buildTopologyBuildStage(config *TopologyTaskConfig) StagePlan {
	// 整个任务只有一个Unit
	return StagePlan{
		ID:          uuid.New().String()[:8],
		Kind:        string(StageKindTopologyBuild),
		Name:        "拓扑构建",
		Order:       3,
		Concurrency: 1, // 构建只能单线程
		Units: []UnitPlan{
			{
				ID:     "build-1",
				Kind:   string(UnitKindDataset),
				Target: TargetRef{Type: "task_run", Key: "all_devices"},
				Steps: []StepPlan{
					{
						ID:   "build-step-1",
						Kind: "build",
						Name: "构建拓扑图",
					},
				},
			},
		},
	}
}

// buildCollectSteps 构建设备采集步骤
func (c *TopologyTaskCompiler) buildCollectSteps(vendor string) []StepPlan {
	// 根据厂商返回标准采集命令集
	// 实际实现需要从config.GetDeviceProfile获取
	commandKeys := []string{
		"version",
		"sysname",
		"esn",
		"device_info",
		"interface_brief",
		"interface_detail",
		"lldp_neighbor",
		"mac_address",
		"arp_all",
		"eth_trunk",
	}

	steps := make([]StepPlan, 0, len(commandKeys))
	for i, key := range commandKeys {
		steps = append(steps, StepPlan{
			ID:         fmt.Sprintf("collect-step-%d", i),
			Kind:       "command",
			Name:       key,
			CommandKey: key,
		})
	}

	return steps
}
