package taskexec

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/NetWeaverGo/core/internal/bizcompare"
	"github.com/NetWeaverGo/core/internal/logger"
)

// BizCompareTaskConfig 业务比对任务配置
type BizCompareTaskConfig struct {
	DeviceIPs   []string `json:"deviceIps"`
	Domain      string   `json:"domain"`  // S | NE-SR | CE
	SceneID     string   `json:"sceneId"` // 场景ID
	Phase       string   `json:"phase"`   // before | after
	TimeoutSec  int      `json:"timeoutSec"`
	Concurrency int      `json:"concurrency"`
}

// BizCompareTaskCompiler 业务比对任务编译器 (第6个Compiler)
type BizCompareTaskCompiler struct {
	options *CompileOptions
}

// NewBizCompareTaskCompiler 创建业务比对任务编译器
func NewBizCompareTaskCompiler(options *CompileOptions) *BizCompareTaskCompiler {
	if options == nil {
		options = DefaultCompileOptions()
	}
	return &BizCompareTaskCompiler{options: options}
}

// Supports 返回是否支持该类型的任务
func (c *BizCompareTaskCompiler) Supports(kind string) bool {
	return kind == string(RunKindBizCompare)
}

// Compile 将业务比对任务定义编译为执行计划
func (c *BizCompareTaskCompiler) Compile(ctx context.Context, def *TaskDefinition) (*ExecutionPlan, error) {
	var config BizCompareTaskConfig
	if err := json.Unmarshal(def.Config, &config); err != nil {
		logger.Error("TaskCompiler", "-", "解析业务比对任务配置失败: task=%s, err=%v", def.Name, err)
		return nil, fmt.Errorf("解析业务比对任务配置失败: %w", err)
	}

	deviceIPs := c.normalizeDeviceIPs(config.DeviceIPs)
	if len(deviceIPs) == 0 {
		return nil, fmt.Errorf("业务比对任务需要至少一台设备")
	}

	sceneMgr := bizcompare.GetGlobalSceneManager()
	scene, ok := sceneMgr.Get(config.SceneID)
	if !ok {
		// 回退到该域首个场景
		scenes := sceneMgr.ListByDomain(config.Domain)
		if len(scenes) > 0 {
			scene = scenes[0]
		}
	}
	if scene == nil {
		return nil, fmt.Errorf("未找到有效比对场景: domain=%s, sceneId=%s", config.Domain, config.SceneID)
	}

	timeoutSec := config.TimeoutSec
	if timeoutSec <= 0 {
		timeoutSec = 60
	}

	units := make([]UnitPlan, 0, len(deviceIPs))
	for i, deviceIP := range deviceIPs {
		steps := make([]StepPlan, 0, len(scene.Commands))
		for sIdx, cmd := range scene.Commands {
			steps = append(steps, StepPlan{
				ID:         fmt.Sprintf("step-%d", sIdx),
				Kind:       "bizcompare_collect",
				Name:       cmd.CommandKey,
				CommandKey: cmd.CommandKey,
				Params: map[string]string{
					"command":  cmd.Command,
					"category": cmd.Category,
					"domain":   scene.Domain,
					"sceneId":  scene.ID,
					"phase":    config.Phase,
				},
			})
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
		Kind:        string(StageKindBizCompareCollect),
		Name:        fmt.Sprintf("业务快照采集 [%s/%s]", scene.Domain, config.Phase),
		Order:       1,
		Concurrency: concurrency,
		Units:       units,
	}

	plan := &ExecutionPlan{
		RunKind: string(RunKindBizCompare),
		Name:    def.Name,
		Stages:  []StagePlan{stage},
	}

	return plan, nil
}

func (c *BizCompareTaskCompiler) normalizeDeviceIPs(ips []string) []string {
	seen := make(map[string]struct{})
	result := make([]string, 0, len(ips))
	for _, ip := range ips {
		trimmed := strings.TrimSpace(ip)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; !exists {
			seen[trimmed] = struct{}{}
			result = append(result, trimmed)
		}
	}
	return result
}
