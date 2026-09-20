package taskexec

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/models"
	"github.com/NetWeaverGo/core/internal/repository"
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
	steps := c.buildCollectSteps(config)

	devRepo := repository.NewDeviceRepository()
	devList, _ := devRepo.FindByIPs(config.DeviceIPs)
	devMap := make(map[string]*models.DeviceAsset, len(devList))
	for idx := range devList {
		devMap[devList[idx].IP] = &devList[idx]
	}

	for _, deviceIP := range config.DeviceIPs {
		dev := devMap[deviceIP]
		eligible, reason := true, ""
		if dev != nil {
			eligible, reason = CheckDeviceEligibility(dev, "topology")
		}
		if !eligible {
			units = append(units, UnitPlan{
				ID:            fmt.Sprintf("collect-%s", deviceIP),
				Kind:          string(UnitKindDevice),
				Target:        TargetRef{Type: "device_ip", Key: deviceIP},
				Timeout:       time.Duration(config.TimeoutSec) * time.Second,
				InitialStatus: string(UnitStatusUnsupported),
				ErrorMessage:  reason,
				Steps:         nil,
			})
			continue
		}

		unitSteps := make([]StepPlan, len(steps))
		copy(unitSteps, steps)
		units = append(units, UnitPlan{
			ID:      fmt.Sprintf("collect-%s", deviceIP),
			Kind:    string(UnitKindDevice),
			Target:  TargetRef{Type: "device_ip", Key: deviceIP},
			Timeout: time.Duration(config.TimeoutSec) * time.Second,
			Steps:   unitSteps,
		})
	}

	concurrency := config.MaxWorkers
	if concurrency <= 0 {
		concurrency = c.options.DefaultDiscoveryWorkers
	}

	return StagePlan{
		ID:          newStageID(),
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

	resolvedVendor, vendorSource := topologyParseMetadata(config)
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
					Params: map[string]string{
						"resolvedVendor": resolvedVendor,
						"vendorSource":   vendorSource,
					},
				},
			},
		})
	}

	return StagePlan{
		ID:          newStageID(),
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
		ID:          newStageID(),
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
func (c *TopologyTaskCompiler) buildCollectSteps(config *TopologyTaskConfig) []StepPlan {
	overridesJSON, _ := json.Marshal(config.FieldOverrides)
	sources := normalizeDiscoverySources(config.DiscoverySources)

	// P0-9：发现源风险提示 —— 仅依赖 ARP/MAC 推断链路存在误连风险
	riskNotice := ""
	if len(sources) > 0 && !sources["lldp"] && !sources["cdp"] {
		riskNotice = "当前发现源未包含 LLDP/CDP，链路将主要由 ARP/MAC 推断，存在误连风险；建议至少启用一种邻居协议发现源"
		logger.Warn("TopologyCompiler", "-", "%s", riskNotice)
	}

	steps := make([]StepPlan, 0, len(config.ResolvedCommands))
	for i, cmd := range config.ResolvedCommands {
		if !cmd.Enabled {
			continue
		}
		if len(sources) > 0 && !discoverySourceAllowed(cmd.FieldKey, sources) {
			continue
		}
		params := map[string]string{
			"displayName":          cmd.DisplayName,
			"commandSource":        cmd.CommandSource,
			"resolvedVendor":       cmd.ResolvedVendor,
			"vendorSource":         cmd.VendorSource,
			"parserBinding":        cmd.ParserBinding,
			"description":          cmd.Description,
			"timeoutSec":           fmt.Sprintf("%d", cmd.TimeoutSec),
			"taskVendor":           config.Vendor,
			"fieldOverrides":       string(overridesJSON),
			"previewCommand":       cmd.Command,
			"previewCommandSource": cmd.CommandSource,
			"discoveryRiskNotice":  riskNotice,
		}
		steps = append(steps, StepPlan{
			ID:         fmt.Sprintf("collect-step-%d", i),
			Kind:       "command",
			Name:       cmd.DisplayName,
			CommandKey: cmd.FieldKey,
			Params:     params,
		})
	}
	return steps
}

// normalizeDiscoverySources 归一化发现源配置（lldp/cdp/arp/mac），空表示不限制
func normalizeDiscoverySources(sources []string) map[string]bool {
	if len(sources) == 0 {
		return nil
	}
	result := make(map[string]bool, len(sources))
	for _, s := range sources {
		switch strings.ToLower(strings.TrimSpace(s)) {
		case "lldp", "cdp", "arp", "mac":
			result[strings.ToLower(strings.TrimSpace(s))] = true
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

// discoverySourceAllowed 判断字段是否在发现源白名单内（非发现类字段不受限制）
func discoverySourceAllowed(fieldKey string, sources map[string]bool) bool {
	switch strings.ToLower(strings.TrimSpace(fieldKey)) {
	case "lldp_neighbor", "lldp_neighbor_verbose":
		return sources["lldp"]
	case "cdp_neighbor":
		return sources["cdp"]
	case "arp_all":
		return sources["arp"]
	case "mac_address":
		return sources["mac"]
	}
	return true
}

func topologyParseMetadata(config *TopologyTaskConfig) (string, string) {
	for _, cmd := range config.ResolvedCommands {
		if cmd.ResolvedVendor != "" {
			return cmd.ResolvedVendor, cmd.VendorSource
		}
	}
	return config.Vendor, "task_config"
}
