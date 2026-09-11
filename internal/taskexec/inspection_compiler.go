package taskexec

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/NetWeaverGo/core/internal/inspection"
	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/models"
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

	// 3. 构建 Unit 与 Steps
	units := make([]UnitPlan, 0, len(deviceIPs))
	for i, deviceIP := range deviceIPs {
		steps := make([]StepPlan, 0, len(items))
		for stepIdx, item := range items {
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

	concurrency := config.Concurrency
	if concurrency <= 0 {
		concurrency = c.options.DefaultConcurrency
	}
	if concurrency <= 0 {
		concurrency = 10
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
