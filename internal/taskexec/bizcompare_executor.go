package taskexec

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/NetWeaverGo/core/internal/bizcompare"
	"github.com/NetWeaverGo/core/internal/config"
	"github.com/NetWeaverGo/core/internal/executor"
	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/models"
	"github.com/NetWeaverGo/core/internal/repository"
	"gorm.io/gorm"
)

// BizCompareExecutor 业务比对阶段执行器
type BizCompareExecutor struct {
	repo     repository.DeviceRepository
	db       *gorm.DB
	settings *models.GlobalSettings
}

// NewBizCompareExecutor 创建业务比对执行器
func NewBizCompareExecutor(repo repository.DeviceRepository, db *gorm.DB) *BizCompareExecutor {
	settings, _, _ := config.LoadSettings()
	return &BizCompareExecutor{
		repo:     repo,
		db:       db,
		settings: settings,
	}
}

// Kind 返回支持的阶段类型
func (e *BizCompareExecutor) Kind() string {
	return string(StageKindBizCompareCollect)
}

// Run 执行业务比对采集阶段
func (e *BizCompareExecutor) Run(ctx RuntimeContext, stage *StagePlan) error {
	logger.Info("BizCompareExecutor", ctx.RunID(), "开始执行业务比对采集阶段: stage=%s, units=%d, concurrency=%d",
		stage.Name, len(stage.Units), stage.Concurrency)

	concurrency := stage.Concurrency
	if concurrency <= 0 {
		concurrency = 10
	}

	semaphore := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	var completedCount, failedCount int
	var mu sync.Mutex

	for _, unit := range stage.Units {
		if ctx.IsCancelled() {
			break
		}

		wg.Add(1)
		semaphore <- struct{}{}
		go func(u UnitPlan) {
			defer wg.Done()
			defer func() { <-semaphore }()

			err := e.executeUnit(ctx, &u)
			mu.Lock()
			if err != nil {
				failedCount++
				logger.Warn("BizCompareExecutor", u.Target.Key, "设备业务快照采集失败: %v", err)
			} else {
				completedCount++
			}
			mu.Unlock()
		}(unit)
	}

	wg.Wait()
	logger.Info("BizCompareExecutor", ctx.RunID(), "业务比对采集完成: total=%d, completed=%d, failed=%d",
		len(stage.Units), completedCount, failedCount)
	return nil
}

func (e *BizCompareExecutor) executeUnit(ctx RuntimeContext, unit *UnitPlan) error {
	deviceIP := unit.Target.Key
	if unit.InitialStatus == string(UnitStatusUnsupported) {
		logger.Info("TaskExec", ctx.RunID(), "设备 %s 不支持业务比对能力，跳过执行: %s", deviceIP, unit.ErrorMessage)
		return nil
	}
	if deviceIP == "" {
		return fmt.Errorf("设备IP为空")
	}

	domain := "S"
	sceneID := "default"
	phase := "before"

	commands := make([]string, 0, len(unit.Steps))
	categoryMap := make(map[string]string)

	for _, step := range unit.Steps {
		cmd := step.Params["command"]
		if cmd == "" {
			cmd = step.CommandKey
		}
		if cmd != "" {
			commands = append(commands, cmd)
			categoryMap[cmd] = step.Params["category"]
		}
		if step.Params["domain"] != "" {
			domain = step.Params["domain"]
		}
		if step.Params["sceneId"] != "" {
			sceneID = step.Params["sceneId"]
		}
		if step.Params["phase"] != "" {
			phase = step.Params["phase"]
		}
	}

	snap := bizcompare.NewDeviceSnapshot(ctx.RunID(), deviceIP, domain, sceneID, phase)

	// 若未接入真实连接则模拟/记录命令执行结果，已接入则走 DeviceExecutor
	var collectErr error
	if e.repo != nil {
		device, err := e.repo.FindByIP(deviceIP)
		if err != nil || device == nil {
			collectErr = fmt.Errorf("设备未找到: %s", deviceIP)
		} else {
			opts := executor.ExecutorOptions{
				Vendor:   device.Vendor,
				Protocol: device.Protocol,
				RunID:    ctx.RunID(),
			}
			exec := executor.NewDeviceExecutor(
				device.IP,
				device.Port,
				device.Username,
				device.Password,
				opts,
			)
			defer exec.Close()
			connTimeout := 30 * time.Second
			if connErr := exec.Connect(ctx.Context(), connTimeout); connErr != nil {
				collectErr = fmt.Errorf("连接设备失败: %w", connErr)
			} else {
				rep, playbookErr := exec.ExecutePlaybookWithReport(ctx.Context(), commands, 30*time.Second, nil)
				if playbookErr != nil {
					collectErr = fmt.Errorf("执行比对采集命令失败: %w", playbookErr)
				} else if rep != nil {
					for _, res := range rep.Results {
						cat := categoryMap[res.Command]
						if cat == "" {
							cat = "general"
						}
						// 提取有效逻辑行入快照
						for _, line := range res.NormalizedLines {
							trimmed := strings.TrimSpace(line)
							if trimmed != "" && !strings.HasPrefix(trimmed, "<") && !strings.HasPrefix(trimmed, "[") {
								key := fmt.Sprintf("%s.%s", res.CommandKey, trimmed)
								snap.AddItem(key, cat, trimmed)
							}
						}
					}
				}
			}
		}
	}

	if collectErr != nil {
		logger.Warn("BizCompareExecutor", deviceIP, "业务快照采集失败: %v", collectErr)
		return collectErr
	}

	store := bizcompare.GetGlobalSnapshotStore()
	if err := store.SaveSnapshot(snap); err != nil {
		return fmt.Errorf("保存快照失败: %w", err)
	}

	return nil
}
