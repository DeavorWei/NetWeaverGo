package taskexec

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/NetWeaverGo/core/internal/ceas"
	"github.com/NetWeaverGo/core/internal/logger"
)

// CEASTaskCompiler CEAS硬件清单任务编译器
type CEASTaskCompiler struct {
	options *CompileOptions
}

// NewCEASTaskCompiler 创建CEAS任务编译器
func NewCEASTaskCompiler(options *CompileOptions) *CEASTaskCompiler {
	if options == nil {
		options = DefaultCompileOptions()
	}
	return &CEASTaskCompiler{options: options}
}

// Supports 返回是否支持指定任务类型
func (c *CEASTaskCompiler) Supports(kind string) bool {
	return kind == string(RunKindCEAS)
}

// Compile 编译CEAS任务定义为执行计划
func (c *CEASTaskCompiler) Compile(ctx context.Context, def *TaskDefinition) (*ExecutionPlan, error) {
	var config ceas.CEASTaskConfig
	if err := json.Unmarshal(def.Config, &config); err != nil {
		logger.Error("TaskCompiler", "-", "解析CEAS任务配置失败: task=%s, err=%v", def.Name, err)
		return nil, fmt.Errorf("解析CEAS任务配置失败: %w", err)
	}

	deviceIPs := c.normalizeDeviceIPs(config.DeviceIPs)
	if len(deviceIPs) == 0 {
		return nil, fmt.Errorf("CEAS任务需要至少一台设备")
	}

	timeoutSec := config.TimeoutSec
	if timeoutSec <= 0 {
		timeoutSec = 60
	}

	units := make([]UnitPlan, 0, len(deviceIPs))
	for i, deviceIP := range deviceIPs {
		steps := []StepPlan{
			{
				ID:         "step-0",
				Kind:       "ceas_collect",
				Name:       "采集电子标签与ESN",
				CommandKey: "ceas_collect",
			},
		}

		units = append(units, UnitPlan{
			ID:      fmt.Sprintf("unit-%d", i),
			Kind:    string(UnitKindDevice),
			Target:  TargetRef{Type: "device_ip", Key: deviceIP},
			Timeout: time.Duration(timeoutSec) * time.Second,
			Steps:   steps,
		})
	}

	concurrency := config.Concurrency
	if concurrency <= 0 {
		concurrency = c.options.DefaultConcurrency
	}
	if concurrency <= 0 {
		concurrency = 10
	}

	stage := StagePlan{
		ID:          newStageID(),
		Kind:        string(StageKindCEASCollect),
		Name:        "CEAS硬件采集",
		Order:       1,
		Concurrency: concurrency,
		Units:       units,
	}

	return &ExecutionPlan{
		RunKind: string(RunKindCEAS),
		Name:    def.Name,
		Stages:  []StagePlan{stage},
	}, nil
}

func (c *CEASTaskCompiler) normalizeDeviceIPs(deviceIPs []string) []string {
	seen := make(map[string]struct{}, len(deviceIPs))
	result := make([]string, 0, len(deviceIPs))
	for _, ip := range deviceIPs {
		trimmed := strings.TrimSpace(ip)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}
