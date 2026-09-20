package taskexec

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/NetWeaverGo/core/internal/config"
	"github.com/NetWeaverGo/core/internal/inspection"
	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/models"
	"github.com/NetWeaverGo/core/internal/repository"
	"gorm.io/gorm"
)

// InspectionTaskConfig 巡检任务执行配置
type InspectionTaskConfig struct {
	DeviceIPs   []string `json:"deviceIps"`
	TemplateID  string   `json:"templateId"`
	Concurrency int      `json:"concurrency"`
	TimeoutSec  int      `json:"timeoutSec"`
}

// InspectionTaskCompiler 巡检任务编译器
type InspectionTaskCompiler struct {
	options *CompileOptions
	db      *gorm.DB
}

// NewInspectionTaskCompiler 创建巡检任务编译器
func NewInspectionTaskCompiler(options *CompileOptions, db *gorm.DB) *InspectionTaskCompiler {
	if options == nil {
		options = DefaultCompileOptions()
	}
	return &InspectionTaskCompiler{
		options: options,
		db:      db,
	}
}

// Supports 返回是否支持指定任务类型
func (c *InspectionTaskCompiler) Supports(kind string) bool {
	return kind == string(RunKindInspection)
}

// Compile 编译巡检任务定义为执行计划
func (c *InspectionTaskCompiler) Compile(ctx context.Context, def *TaskDefinition) (*ExecutionPlan, error) {
	var config InspectionTaskConfig
	if err := json.Unmarshal(def.Config, &config); err != nil {
		logger.Error("TaskCompiler", "-", "解析巡检任务配置失败: task=%s, err=%v", def.Name, err)
		return nil, fmt.Errorf("解析巡检任务配置失败: %w", err)
	}

	deviceIPs := c.normalizeDeviceIPs(config.DeviceIPs)
	if len(deviceIPs) == 0 {
		return nil, fmt.Errorf("巡检任务需要至少一台设备")
	}

	templateID := strings.TrimSpace(config.TemplateID)
	if templateID == "" {
		templateID = "tpl-huawei-general"
	}

	// 1. 获取巡检检查项
	items := c.resolveItems(templateID)
	if len(items) == 0 {
		return nil, fmt.Errorf("未找到模板 [%s] 下启用的巡检检查项", templateID)
	}

	// 2. 区分并排查前置采集项与普通项（对齐 P1-3 与 P4-1: IsPreCollect 优先执行）
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].IsPreCollect != items[j].IsPreCollect {
			return items[i].IsPreCollect // IsPreCollect=true 排在前面
		}
		return items[i].Order < items[j].Order
	})

	timeoutSec := config.TimeoutSec
	if timeoutSec <= 0 {
		timeoutSec = 60
	}

	// 3. 计算并发度
	concurrency := config.Concurrency
	if concurrency <= 0 {
		concurrency = c.options.DefaultConcurrency
	}
	if concurrency <= 0 {
		concurrency = 10
	}

	// 4. 依据编排模式产出执行计划（方案 §5.3.1：single 为默认灰度档）
	if useThreeStagePipeline() {
		return c.compileThreeStage(def.Name, deviceIPs, items, templateID, concurrency, timeoutSec)
	}

	devRepo := repository.NewDeviceRepositoryWithDB(c.db)
	devList, _ := devRepo.FindByIPs(deviceIPs)
	devMap := make(map[string]*models.DeviceAsset, len(devList))
	for idx := range devList {
		devMap[devList[idx].IP] = &devList[idx]
	}

	// 单阶段模式：采集 + 解析 + 判定内联在 inspection_check 内
	units := make([]UnitPlan, 0, len(deviceIPs))
	for i, deviceIP := range deviceIPs {
		dev := devMap[deviceIP]
		eligible, reason := true, ""
		if dev != nil {
			eligible, reason = CheckDeviceEligibility(dev, "inspection")
		}
		if !eligible {
			units = append(units, UnitPlan{
				ID:            fmt.Sprintf("unit-%d", i),
				Kind:          string(UnitKindDevice),
				Target:        TargetRef{Type: "device_ip", Key: deviceIP},
				Timeout:       time.Duration(timeoutSec) * time.Second,
				InitialStatus: string(UnitStatusUnsupported),
				ErrorMessage:  reason,
				Steps:         nil,
			})
			continue
		}

		steps := make([]StepPlan, 0, len(items))
		for stepIdx, item := range items {
			// 形态判定：软件形态跳过 optical / hardware 检查项
			if dev != nil && strings.EqualFold(dev.FormFactor, "software") {
				if strings.EqualFold(item.Category, "optical") || strings.EqualFold(item.Category, "hardware") {
					continue
				}
			}
			steps = append(steps, StepPlan{
				ID:      fmt.Sprintf("step-%d", stepIdx),
				Kind:    "inspection_item",
				Name:    item.Name,
				Command: item.CommandKey,
				Params: map[string]string{
					"itemCode":     item.Code,
					"isPreCollect": fmt.Sprintf("%t", item.IsPreCollect),
					"templateId":   templateID,
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

	stage := StagePlan{
		ID:          newStageID(),
		Kind:        string(StageKindInspectionCheck),
		Name:        "设备指标采集与规则判定",
		Order:       1,
		Concurrency: concurrency,
		Units:       units,
	}

	return &ExecutionPlan{
		RunKind: string(RunKindInspection),
		Name:    def.Name,
		Stages:  []StagePlan{stage},
	}, nil
}

// useThreeStagePipeline 读取全局设置判断是否启用巡检三阶段编排
func useThreeStagePipeline() bool {
	if st := config.GetGlobalSettings(); st != nil {
		return strings.TrimSpace(st.InspectionPipelineMode) == "three_stage"
	}
	return false
}

// compileThreeStage 产出 inspection_collect → inspection_parse → inspection_check 三阶段计划。
// 命令按检查项顺序去重（IsPreCollect 已在 resolveItems 中前置排序），
// 中间产物经内存快照传递，不写入任何中间实体表。
func (c *InspectionTaskCompiler) compileThreeStage(
	name string,
	deviceIPs []string,
	items []models.InspectionItem,
	templateID string,
	concurrency int,
	timeoutSec int,
) (*ExecutionPlan, error) {
	// 命令去重（保持顺序）
	commands := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, it := range items {
		cmd := strings.TrimSpace(it.CommandKey)
		if cmd == "" {
			continue
		}
		if _, ok := seen[cmd]; ok {
			continue
		}
		seen[cmd] = struct{}{}
		commands = append(commands, cmd)
	}
	if len(commands) == 0 {
		return nil, fmt.Errorf("巡检检查项未绑定任何采集命令，无法编排三阶段")
	}

	timeout := time.Duration(timeoutSec) * time.Second
	buildUnits := func(stepsBuilder func() []StepPlan) []UnitPlan {
		units := make([]UnitPlan, 0, len(deviceIPs))
		for i, deviceIP := range deviceIPs {
			units = append(units, UnitPlan{
				ID:      fmt.Sprintf("unit-%d", i),
				Kind:    string(UnitKindDevice),
				Target:  TargetRef{Type: "device_ip", Key: deviceIP},
				Timeout: timeout,
				Steps:   stepsBuilder(),
			})
		}
		return units
	}

	collectSteps := func() []StepPlan {
		steps := make([]StepPlan, 0, len(commands))
		for idx, cmd := range commands {
			steps = append(steps, StepPlan{
				ID:      fmt.Sprintf("cmd-%d", idx),
				Kind:    "command",
				Name:    cmd,
				Command: cmd,
				Params:  map[string]string{"templateId": templateID},
			})
		}
		return steps
	}
	parseSteps := func() []StepPlan {
		steps := make([]StepPlan, 0, len(commands))
		for idx, cmd := range commands {
			steps = append(steps, StepPlan{
				ID:      fmt.Sprintf("parse-%d", idx),
				Kind:    "parse",
				Name:    cmd,
				Command: cmd,
				Params:  map[string]string{"templateId": templateID},
			})
		}
		return steps
	}
	checkSteps := func() []StepPlan {
		steps := make([]StepPlan, 0, len(items))
		for idx, item := range items {
			steps = append(steps, StepPlan{
				ID:      fmt.Sprintf("step-%d", idx),
				Kind:    "inspection_item",
				Name:    item.Name,
				Command: item.CommandKey,
				Params: map[string]string{
					"itemCode":     item.Code,
					"isPreCollect": fmt.Sprintf("%t", item.IsPreCollect),
					"templateId":   templateID,
				},
			})
		}
		return steps
	}

	return &ExecutionPlan{
		RunKind: string(RunKindInspection),
		Name:    name,
		Stages: []StagePlan{
			{
				ID:          newStageID(),
				Kind:        string(StageKindInspectionCollect),
				Name:        "设备指标采集",
				Order:       1,
				Concurrency: concurrency,
				Units:       buildUnits(collectSteps),
			},
			{
				ID:          newStageID(),
				Kind:        string(StageKindInspectionParse),
				Name:        "回显结构化解析",
				Order:       2,
				Concurrency: concurrency,
				Units:       buildUnits(parseSteps),
			},
			{
				ID:          newStageID(),
				Kind:        string(StageKindInspectionCheck),
				Name:        "巡检规则判定",
				Order:       3,
				Concurrency: concurrency,
				Units:       buildUnits(checkSteps),
			},
		},
	}, nil
}

// ResolveInspectionItems 查询指定模板启用的巡检项，若数据库未就绪则兜底使用内存种子
func ResolveInspectionItems(db *gorm.DB, templateID string) []models.InspectionItem {
	if db != nil {
		var dbItems []models.InspectionItem
		if err := db.Where("template_id = ? AND enabled = ?", templateID, true).Order("order_num asc, id asc").Find(&dbItems).Error; err == nil && len(dbItems) > 0 {
			return dbItems
		}
	}

	// 兜底使用内存默认种子
	var fallback []models.InspectionItem
	for _, it := range inspection.DefaultItems {
		if it.TemplateID == templateID && it.Enabled {
			fallback = append(fallback, it)
		}
	}
	if len(fallback) == 0 {
		// 如果指定模板无匹配，使用通用模板项
		for _, it := range inspection.DefaultItems {
			if it.TemplateID == "tpl-huawei-general" && it.Enabled {
				fallback = append(fallback, it)
			}
		}
	}
	return fallback
}

func (c *InspectionTaskCompiler) resolveItems(templateID string) []models.InspectionItem {
	return ResolveInspectionItems(c.db, templateID)
}

func (c *InspectionTaskCompiler) normalizeDeviceIPs(deviceIPs []string) []string {
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
