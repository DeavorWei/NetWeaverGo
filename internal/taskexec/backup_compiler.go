package taskexec

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/NetWeaverGo/core/internal/logger"
)

// BackupTaskCompiler 备份任务编译器
type BackupTaskCompiler struct {
	options *CompileOptions
}

// NewBackupTaskCompiler 创建备份任务编译器
func NewBackupTaskCompiler(options *CompileOptions) *BackupTaskCompiler {
	if options == nil {
		options = DefaultCompileOptions()
	}
	return &BackupTaskCompiler{options: options}
}

// Supports 返回是否支持
func (c *BackupTaskCompiler) Supports(kind string) bool {
	return kind == string(RunKindBackup)
}

// Compile 编译备份任务定义
func (c *BackupTaskCompiler) Compile(ctx context.Context, def *TaskDefinition) (*ExecutionPlan, error) {
	var config BackupTaskConfig
	if err := json.Unmarshal(def.Config, &config); err != nil {
		logger.Error("TaskCompiler", "-", "解析备份任务配置失败: task=%s, err=%v", def.Name, err)
		return nil, fmt.Errorf("解析备份任务配置失败: %w", err)
	}

	deviceIPs := c.normalizeDeviceIPs(config.DeviceIPs)
	if len(deviceIPs) == 0 {
		return nil, fmt.Errorf("备份任务需要至少一台设备")
	}

	units := make([]UnitPlan, 0, len(deviceIPs))
	for i, deviceIP := range deviceIPs {
		steps := []StepPlan{
			{
				ID:         "step-0",
				Kind:       "backup_query_startup",
				Name:       "查询配置路径",
				CommandKey: "query_startup",
				Params: map[string]string{
					"startupCommand": config.StartupCommand,
				},
			},
			{
				ID:         "step-1",
				Kind:       "backup_sftp_download",
				Name:       "下载配置文件",
				CommandKey: "sftp_download",
				Params: map[string]string{
					"saveRootPath":    config.SaveRootPath,
					"dirNamePattern":  config.DirNamePattern,
					"fileNamePattern": config.FileNamePattern,
				},
			},
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
		Kind:        string(StageKindBackupCollect),
		Name:        "配置备份",
		Order:       1,
		Concurrency: concurrency,
		Units:       units,
	}

	return &ExecutionPlan{
		RunKind: string(RunKindBackup),
		Name:    def.Name,
		Stages:  []StagePlan{stage},
	}, nil
}

func (c *BackupTaskCompiler) normalizeDeviceIPs(deviceIPs []string) []string {
	result := make([]string, 0, len(deviceIPs))
	for _, ip := range deviceIPs {
		if ip != "" {
			result = append(result, ip)
		}
	}
	return result
}
