package discovery

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/NetWeaverGo/core/internal/config"
	"github.com/NetWeaverGo/core/internal/executor"
	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/models"
	"github.com/NetWeaverGo/core/internal/report"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// PathProvider 路径提供者接口（避免循环导入）
type PathProvider interface {
	GetDiscoveryRawFilePath(taskID, deviceIP, commandKey string) string
	GetDiscoveryNormalizedFilePath(taskID, deviceIP, commandKey string) string
}

// RuntimeConfigProvider 运行时配置提供者接口
type RuntimeConfigProvider interface {
	GetConnectionTimeout() time.Duration
	GetDiscoveryWorkerCount() int
	GetDiscoveryPerDeviceTimeout() time.Duration
	GetDiscoveryCommandTimeout() time.Duration
}

// Runner 发现任务运行器
type Runner struct {
	db *gorm.DB

	// 状态管理
	mu          sync.RWMutex
	runningTask string
	ctx         context.Context
	cancel      context.CancelFunc

	// 事件通道
	EventBus    chan DiscoveryEvent
	FrontendBus chan DiscoveryEvent

	// 并发控制
	maxWorkers int

	// 外部依赖注入
	pathProvider    PathProvider
	runtimeProvider RuntimeConfigProvider

	// 日志存储（任务级）
	logStore   *report.ExecutionLogStore
	logStoreMu sync.Mutex
}

// DiscoveryEvent 发现事件
type DiscoveryEvent struct {
	TaskID    string `json:"taskId"`
	DeviceIP  string `json:"deviceIp"`
	Type      string `json:"type"` // start, cmd, success, error, abort
	Message   string `json:"message"`
	Timestamp int64  `json:"timestamp"`
}

// NewRunner 创建发现任务运行器
func NewRunner(db *gorm.DB) *Runner {
	return &Runner{
		db:          db,
		EventBus:    make(chan DiscoveryEvent, config.DefaultDiscoveryEventBufferSize),
		FrontendBus: make(chan DiscoveryEvent, config.DefaultDiscoveryEventBufferSize),
		maxWorkers:  config.DefaultDiscoveryMaxWorkers,
	}
}

// SetPathProvider 设置路径提供者
func (r *Runner) SetPathProvider(p PathProvider) {
	r.pathProvider = p
}

// SetRuntimeProvider 设置运行时配置提供者
func (r *Runner) SetRuntimeProvider(p RuntimeConfigProvider) {
	r.runtimeProvider = p
}

// SetMaxWorkers 设置最大并发数
func (r *Runner) SetMaxWorkers(workers int) {
	if workers > 0 && workers <= config.MaxDiscoveryWorkers {
		r.maxWorkers = workers
	}
}

// setPhase 更新任务阶段
// 阶段C修复：添加错误处理
func (r *Runner) setPhase(taskID string, phase models.DiscoveryTaskPhase) {
	now := time.Now()
	if err := r.db.Model(&models.DiscoveryTask{}).Where("id = ?", taskID).Updates(map[string]interface{}{
		"phase":            phase,
		"phase_started_at": now,
		"phase_progress":   0,
	}).Error; err != nil {
		logger.Warn("Discovery", taskID, "更新任务阶段失败: %v", err)
	}
}

// setPhaseProgress 更新阶段进度
// 阶段C修复：添加错误处理
func (r *Runner) setPhaseProgress(taskID string, progress int) {
	if progress > 100 {
		progress = 100
	}
	if err := r.db.Model(&models.DiscoveryTask{}).Where("id = ?", taskID).Update("phase_progress", progress).Error; err != nil {
		logger.Warn("Discovery", taskID, "更新阶段进度失败: %v", err)
	}
}

// Start 启动发现任务
func (r *Runner) Start(ctx context.Context, req models.StartDiscoveryRequest) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 检查是否已有任务在运行
	if r.runningTask != "" {
		return "", fmt.Errorf("已有发现任务正在运行: %s", r.runningTask)
	}

	// 获取设备列表
	devices, err := r.getDevicesForDiscovery(req)
	if err != nil {
		return "", fmt.Errorf("获取设备列表失败: %v", err)
	}

	if len(devices) == 0 {
		return "", fmt.Errorf("没有可用的设备进行发现")
	}

	// 创建任务记录
	taskID := uuid.New().String()[:8]
	task := models.DiscoveryTask{
		ID:         taskID,
		Name:       fmt.Sprintf("发现任务-%s", time.Now().Format("20060102-150405")),
		Status:     "pending",
		TotalCount: len(devices),
		MaxWorkers: req.MaxWorkers,
		TimeoutSec: req.TimeoutSec,
		Vendor:     normalizeTaskVendor(req.Vendor),
	}

	if task.MaxWorkers <= 0 {
		task.MaxWorkers = r.defaultDiscoveryWorkerCount()
	}
	if task.TimeoutSec <= 0 {
		task.TimeoutSec = durationToPositiveSeconds(r.defaultDiscoveryCommandTimeout(), 60)
	}

	// 保存任务到数据库
	if err := r.db.Create(&task).Error; err != nil {
		return "", fmt.Errorf("创建任务记录失败: %v", err)
	}

	// 创建设备发现记录
	var discoveryDevices []models.DiscoveryDevice
	for _, dev := range devices {
		effectiveVendor := resolveDiscoveryVendor(req.Vendor, dev.Vendor)
		discoveryDevices = append(discoveryDevices, models.DiscoveryDevice{
			TaskID:      taskID,
			DeviceIP:    dev.IP,
			DeviceID:    dev.ID,
			Status:      "pending",
			DisplayName: dev.DisplayName,
			Role:        dev.Role,
			Site:        dev.Site,
			Vendor:      effectiveVendor,
		})
	}

	if err := r.db.Create(&discoveryDevices).Error; err != nil {
		return "", fmt.Errorf("创建设备发现记录失败: %v", err)
	}

	// 创建原始命令输出记录
	var rawOutputs []models.RawCommandOutput
	for _, dev := range devices {
		effectiveVendor := resolveDiscoveryVendor(req.Vendor, dev.Vendor)
		profile := config.GetDeviceProfile(effectiveVendor)
		for _, cmd := range profile.Commands {
			rawOutputs = append(rawOutputs, models.RawCommandOutput{
				TaskID:      taskID,
				DeviceIP:    dev.IP,
				CommandKey:  cmd.CommandKey,
				Command:     cmd.Command,
				Status:      "pending",
				ParseStatus: "pending",
			})
		}
	}

	if err := r.db.Create(&rawOutputs).Error; err != nil {
		return "", fmt.Errorf("创建原始输出记录失败: %v", err)
	}

	// 初始化 context
	r.ctx, r.cancel = context.WithCancel(ctx)
	r.runningTask = taskID

	// 更新任务状态为运行中
	// 阶段C修复：添加错误处理
	now := time.Now()
	if err := r.db.Model(&models.DiscoveryTask{}).Where("id = ?", taskID).Updates(map[string]interface{}{
		"status":     "running",
		"started_at": now,
		"phase":      models.PhaseCollecting,
	}).Error; err != nil {
		logger.Warn("Discovery", taskID, "更新任务状态为running失败: %v", err)
	}

	// 启动后台任务执行
	go r.runDiscovery(r.ctx, taskID, devices, req.Vendor, task.TimeoutSec, task.MaxWorkers)

	return taskID, nil
}

// Cancel 取消发现任务
func (r *Runner) Cancel(taskID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.runningTask != taskID {
		return fmt.Errorf("任务 %s 不在运行中", taskID)
	}

	if r.cancel != nil {
		r.cancel()
	}

	// 更新任务状态
	// 阶段C修复：添加错误处理
	now := time.Now()
	if err := r.db.Model(&models.DiscoveryTask{}).Where("id = ?", taskID).Updates(map[string]interface{}{
		"status":      "cancelled",
		"phase":       models.PhaseCancelled,
		"finished_at": now,
	}).Error; err != nil {
		logger.Warn("Discovery", taskID, "更新任务状态为cancelled失败: %v", err)
	}

	r.runningTask = ""
	return nil
}

// RetryFailed 重试失败的设备
func (r *Runner) RetryFailed(ctx context.Context, taskID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.runningTask != "" {
		return fmt.Errorf("已有发现任务正在运行: %s", r.runningTask)
	}

	// 获取失败的设备
	var failedDevices []models.DiscoveryDevice
	if err := r.db.Where("task_id = ? AND status = ?", taskID, "failed").Find(&failedDevices).Error; err != nil {
		return fmt.Errorf("获取失败设备列表失败: %v", err)
	}

	if len(failedDevices) == 0 {
		return fmt.Errorf("没有失败的设备需要重试")
	}

	// 获取任务信息
	var task models.DiscoveryTask
	if err := r.db.First(&task, "id = ?", taskID).Error; err != nil {
		return fmt.Errorf("获取任务信息失败: %v", err)
	}

	// 获取设备资产信息
	var deviceIPs []string
	for _, d := range failedDevices {
		deviceIPs = append(deviceIPs, d.DeviceIP)
	}

	devices, err := r.getDevicesByIPs(deviceIPs)
	if err != nil {
		return fmt.Errorf("获取设备资产失败: %v", err)
	}

	// 更新设备状态为 pending
	// 阶段C修复：添加错误处理
	if err := r.db.Model(&models.DiscoveryDevice{}).Where("task_id = ? AND status = ?", taskID, "failed").Update("status", "pending").Error; err != nil {
		logger.Warn("Discovery", taskID, "重置设备状态失败: %v", err)
	}

	// 初始化 context
	r.ctx, r.cancel = context.WithCancel(ctx)
	r.runningTask = taskID

	// 更新任务状态为运行中
	// 阶段C修复：添加错误处理
	if err := r.db.Model(&models.DiscoveryTask{}).Where("id = ?", taskID).Updates(map[string]interface{}{
		"status": "running",
		"phase":  models.PhaseCollecting,
	}).Error; err != nil {
		logger.Warn("Discovery", taskID, "更新任务状态失败: %v", err)
	}

	// 启动后台任务执行
	go r.runDiscovery(r.ctx, taskID, devices, task.Vendor, task.TimeoutSec, task.MaxWorkers)

	return nil
}

// runDiscovery 执行发现任务
func (r *Runner) runDiscovery(ctx context.Context, taskID string, devices []models.DeviceInfo, requestedVendor string, timeoutSec int, maxWorkers int) {
	defer func() {
		if p := recover(); p != nil {
			r.emitEvent(DiscoveryEvent{
				TaskID:    taskID,
				Type:      "error",
				Message:   fmt.Sprintf("发现任务内部错误: %v", p),
				Timestamp: time.Now().UnixMilli(),
			})
			// 更新任务状态为失败
			// 阶段C修复：添加错误处理
			if err := r.db.Model(&models.DiscoveryTask{}).Where("id = ?", taskID).Updates(map[string]interface{}{
				"status":      "failed",
				"phase":       models.PhaseFailed,
				"finished_at": time.Now(),
			}).Error; err != nil {
				logger.Error("Discovery", taskID, "更新任务失败状态失败: %v", err)
			}
		}
		r.mu.Lock()
		r.runningTask = ""
		r.mu.Unlock()
	}()

	var algorithms *config.SSHAlgorithmSettings
	if settings, _, err := config.LoadSettings(); err == nil && settings != nil {
		algorithms = &settings.SSHAlgorithms
	}

	// 获取连接超时配置
	connectTimeout := 30 * time.Second
	perDeviceTimeout := 3 * time.Minute
	if r.runtimeProvider != nil {
		connectTimeout = r.runtimeProvider.GetConnectionTimeout()
		if timeout := r.runtimeProvider.GetDiscoveryPerDeviceTimeout(); timeout > 0 {
			perDeviceTimeout = timeout
		}
	}
	taskCommandTimeout := time.Duration(timeoutSec) * time.Second
	if taskCommandTimeout <= 0 {
		taskCommandTimeout = r.defaultDiscoveryCommandTimeout()
	}

	// 并发控制
	var wg sync.WaitGroup
	if maxWorkers <= 0 {
		maxWorkers = r.defaultDiscoveryWorkerCount()
	}
	sem := make(chan struct{}, maxWorkers)

	// 统计计数
	var successCount int
	var failedCount int
	var completedCount int
	var countMu sync.Mutex
	totalDevices := len(devices)

dispatchLoop:
	for _, dev := range devices {
		select {
		case <-ctx.Done():
			break dispatchLoop
		default:
		}

		wg.Add(1)
		sem <- struct{}{}

		go func(device models.DeviceInfo) {
			defer func() {
				if p := recover(); p != nil {
					// 记录 panic 信息并更新设备状态
					errMsg := fmt.Sprintf("设备发现内部错误: %v", p)
					r.updateDeviceError(taskID, device.IP, errMsg)
					r.emitEvent(DiscoveryEvent{
						TaskID:    taskID,
						DeviceIP:  device.IP,
						Type:      "error",
						Message:   errMsg,
						Timestamp: time.Now().UnixMilli(),
					})
					countMu.Lock()
					failedCount++
					completedCount++
					if totalDevices > 0 {
						progress := int(float64(completedCount) / float64(totalDevices) * 100)
						r.setPhaseProgress(taskID, progress)
					}
					countMu.Unlock()
				}
				<-sem
				wg.Done()
			}()

			// 增加抖动
			time.Sleep(time.Duration(rand.Intn(500)) * time.Millisecond)

			effectiveVendor := resolveDiscoveryVendor(requestedVendor, device.Vendor)

			deviceCtx := ctx
			cancel := func() {}
			if perDeviceTimeout > 0 {
				deviceCtx, cancel = context.WithTimeout(ctx, perDeviceTimeout)
			}
			defer cancel()

			// 执行设备发现
			err := r.discoverDevice(deviceCtx, taskID, device, effectiveVendor, algorithms, connectTimeout, taskCommandTimeout)

			countMu.Lock()
			if err != nil {
				failedCount++
			} else {
				successCount++
			}
			completedCount++
			// 更新采集阶段进度
			if totalDevices > 0 {
				progress := int(float64(completedCount) / float64(totalDevices) * 100)
				r.setPhaseProgress(taskID, progress)
			}
			countMu.Unlock()
		}(dev)
	}

	wg.Wait()

	// 切换到解析阶段
	r.setPhase(taskID, models.PhaseParsing)

	// 更新任务状态
	now := time.Now()
	status := "completed"
	phase := models.PhaseCompleted
	if failedCount > 0 && successCount == 0 {
		status = "failed"
		phase = models.PhaseFailed
	} else if failedCount > 0 {
		status = "partial"
	}
	if ctx.Err() != nil {
		status = "cancelled"
		phase = models.PhaseCancelled
	}

	var currentTask models.DiscoveryTask
	if err := r.db.Select("status").Where("id = ?", taskID).Take(&currentTask).Error; err == nil {
		if strings.EqualFold(currentTask.Status, "cancelled") {
			status = "cancelled"
			phase = models.PhaseCancelled
		}
	}

	// 阶段C修复：添加错误处理
	if err := r.db.Model(&models.DiscoveryTask{}).Where("id = ?", taskID).Updates(map[string]interface{}{
		"status":        status,
		"phase":         phase,
		"finished_at":   now,
		"success_count": successCount,
		"failed_count":  failedCount,
	}).Error; err != nil {
		logger.Error("Discovery", taskID, "更新任务最终状态失败: %v", err)
	}

	// 发送完成事件
	r.emitEvent(DiscoveryEvent{
		TaskID:    taskID,
		Type:      "completed",
		Message:   fmt.Sprintf("发现任务完成: 成功 %d, 失败 %d", successCount, failedCount),
		Timestamp: time.Now().UnixMilli(),
	})

	// 关闭日志存储
	r.logStoreMu.Lock()
	if r.logStore != nil {
		r.logStore.Close()
		r.logStore = nil
		logger.Debug("Discovery", taskID, "日志存储已关闭")
	}
	r.logStoreMu.Unlock()
}

// discoverDevice 执行单设备发现 - 重构后版本
// 使用 ExecutePlan 统一执行计划，修复会话状态丢失问题
// 修复：添加 ExecutionLogStore 支持，生成 live-logs 日志
func (r *Runner) discoverDevice(ctx context.Context, taskID string, device models.DeviceInfo, vendor string, algorithms *config.SSHAlgorithmSettings, connectTimeout time.Duration, taskCommandTimeout time.Duration) error {
	logger.Debug("Discovery", device.IP, "开始发现设备, vendor=%s", vendor)

	// 1. 更新设备状态
	// 阶段C修复：添加错误处理
	now := time.Now()
	if err := r.db.Model(&models.DiscoveryDevice{}).Where("task_id = ? AND device_ip = ?", taskID, device.IP).Updates(map[string]interface{}{
		"status":     "running",
		"started_at": now,
		"vendor":     vendor,
	}).Error; err != nil {
		logger.Warn("Discovery", device.IP, "更新设备running状态失败: %v", err)
	}

	// 发送开始事件
	r.emitEvent(DiscoveryEvent{
		TaskID:    taskID,
		DeviceIP:  device.IP,
		Type:      "start",
		Message:   "开始发现设备",
		Timestamp: time.Now().UnixMilli(),
	})

	// 2. 获取设备画像并构建执行计划
	// 优先使用统一的 config 配置源
	deviceProfile := config.GetDeviceProfile(vendor)
	plan := BuildDiscoveryPlan(deviceProfile, taskCommandTimeout)

	logger.Debug("Discovery", device.IP, "构建执行计划: %s, 命令数=%d", plan.Name, len(plan.Commands))

	// 3. 初始化日志存储（如果尚未初始化）
	r.logStoreMu.Lock()
	if r.logStore == nil {
		var err error
		r.logStore, err = report.NewExecutionLogStore(
			fmt.Sprintf("Discovery-%s", taskID),
			time.Now(),
		)
		if err != nil {
			logger.Warn("Discovery", taskID, "创建日志存储失败: %v", err)
		}
	}
	r.logStoreMu.Unlock()

	// 4. 创建设备日志会话
	var logSession *report.DeviceLogSession
	if r.logStore != nil {
		var err error
		// 获取命令数量用于日志会话初始化
		cmdCount := len(plan.Commands)
		logSession, err = r.logStore.EnsureDevice(device.IP, true) // enableRaw = true 启用原始日志
		if err != nil {
			logger.Warn("Discovery", device.IP, "创建设备日志会话失败: %v", err)
		} else {
			// 写入开始标记到日志
			logSession.WriteSummary(fmt.Sprintf("=== 设备发现任务开始 | 任务ID: %s | 命令数: %d ===", taskID, cmdCount))
		}
	}

	// 5. 创建执行器（传递 LogSession）
	exec := executor.NewDeviceExecutor(device.IP, device.Port, device.Username, device.Password, executor.ExecutorOptions{
		Algorithms: algorithms,
		Vendor:     vendor,
		LogSession: logSession, // 修复：传递日志会话，使执行器可以写入详细日志
	})

	// 6. 连接设备
	if err := exec.Connect(ctx, connectTimeout); err != nil {
		logger.Error("Discovery", device.IP, "SSH连接失败: %v", err)
		r.updateDeviceError(taskID, device.IP, fmt.Sprintf("SSH连接失败: %v", err))
		return err
	}
	defer exec.Close()

	// 7. 执行统一计划（✅ 修复：只创建一次 StreamEngine，只初始化一次）
	execReport, err := exec.ExecutePlan(ctx, plan)

	// 8. 写入完成标记到日志
	if logSession != nil {
		status := "成功"
		if err != nil {
			status = "失败"
		}
		logSession.WriteSummary(fmt.Sprintf("=== 设备发现任务结束 | 状态: %s ===", status))
	}

	// 9. 处理结果
	return r.handleDiscoveryReport(taskID, device, vendor, execReport, err)
}

// handleDiscoveryReport 处理发现执行报告
// 阶段B修复：增加空指针防御和失败统计准确性
func (r *Runner) handleDiscoveryReport(taskID string, device models.DeviceInfo, vendor string, report *executor.ExecutionReport, execErr error) error {
	// 防御：report 为 nil 的情况
	if report == nil {
		logger.Error("Discovery", device.IP, "执行报告为空")
		errMsg := "执行报告为空"
		if execErr != nil {
			errMsg = fmt.Sprintf("执行失败: %v", execErr)
		}
		r.updateDeviceError(taskID, device.IP, errMsg)
		return fmt.Errorf("执行报告为空: %w", execErr)
	}

	cmdSuccess := 0
	cmdFailed := 0

	// 保存每条命令的结果
	// 即使 Results 为空，也要根据 FatalError 和 execErr 判断状态
	for _, result := range report.Results {
		status := "success"
		errMsg := ""
		if result.ErrorMessage != "" {
			status = "failed"
			errMsg = result.ErrorMessage
			cmdFailed++
		} else {
			cmdSuccess++
		}

		// 查找对应的命令文本
		cmdText := result.Command
		if cmdText == "" {
			// 如果 Command 为空，尝试从 profile 查找
			if spec := GetCommandByKey(vendor, result.CommandKey); spec != nil {
				cmdText = spec.Command
			}
		}

		// 保存命令输出
		r.saveCommandOutput(taskID, device.IP, result.CommandKey, cmdText, result, status, errMsg)

		// 如果是 version 命令，解析设备信息
		if result.CommandKey == "version" && status == "success" {
			r.parseAndUpdateDeviceInfo(taskID, device.IP, vendor, result.NormalizedText)
		}

		// 发送命令完成事件
		r.emitEvent(DiscoveryEvent{
			TaskID:    taskID,
			DeviceIP:  device.IP,
			Type:      "cmd",
			Message:   fmt.Sprintf("命令完成: %s (%s)", result.CommandKey, status),
			Timestamp: time.Now().UnixMilli(),
		})
	}

	// 确定设备状态（阶段B修复：统一状态判定规则）
	// 规则：
	// - fatal error => failed
	// - failure>0 && success>0 => partial
	// - failure>0 && success==0 => failed
	// - 全成功 => success
	deviceStatus := "success"
	deviceErr := ""

	// 优先检查会话级致命错误
	if report.FatalError != nil {
		deviceStatus = "failed"
		deviceErr = report.FatalError.Error()
		logger.Debug("Discovery", device.IP, "会话级致命错误: %s", deviceErr)
	} else if execErr != nil {
		// 执行错误（非致命）
		deviceStatus = "failed"
		deviceErr = execErr.Error()
		logger.Debug("Discovery", device.IP, "执行错误: %s", deviceErr)
	} else if cmdFailed > 0 {
		if cmdSuccess > 0 {
			deviceStatus = "partial"
			logger.Debug("Discovery", device.IP, "部分成功: %d 成功, %d 失败", cmdSuccess, cmdFailed)
		} else {
			deviceStatus = "failed"
			deviceErr = "所有命令执行失败"
			logger.Debug("Discovery", device.IP, "所有命令执行失败")
		}
	}

	// 如果 Results 为空但有 FatalError，记录错误
	if len(report.Results) == 0 && report.FatalError != nil {
		deviceStatus = "failed"
		deviceErr = report.FatalError.Error()
		logger.Warn("Discovery", device.IP, "无命令结果且存在致命错误")
	}

	logger.Debug("Discovery", device.IP, "设备发现完成: status=%s, success=%d, failed=%d",
		deviceStatus, cmdSuccess, cmdFailed)

	// 更新数据库
	// 阶段C修复：添加错误处理
	finishedAt := time.Now()
	if err := r.db.Model(&models.DiscoveryDevice{}).Where("task_id = ? AND device_ip = ?", taskID, device.IP).Updates(map[string]interface{}{
		"status":        deviceStatus,
		"error_message": deviceErr,
		"finished_at":   finishedAt,
	}).Error; err != nil {
		logger.Error("Discovery", device.IP, "更新设备状态失败: %v", err)
	}

	// 发送完成事件
	r.emitEvent(DiscoveryEvent{
		TaskID:    taskID,
		DeviceIP:  device.IP,
		Type:      map[bool]string{true: "success", false: "error"}[deviceStatus == "success"],
		Message:   fmt.Sprintf("设备发现完成: status=%s success=%d failed=%d", deviceStatus, cmdSuccess, cmdFailed),
		Timestamp: time.Now().UnixMilli(),
	})

	if deviceStatus == "success" {
		return nil
	}
	if execErr != nil {
		return execErr
	}
	return fmt.Errorf("设备发现失败: %s", deviceStatus)
}

// updateDeviceError 更新设备错误状态
// 阶段C修复：添加错误处理
func (r *Runner) updateDeviceError(taskID, deviceIP, errMsg string) {
	now := time.Now()
	if err := r.db.Model(&models.DiscoveryDevice{}).Where("task_id = ? AND device_ip = ?", taskID, deviceIP).Updates(map[string]interface{}{
		"status":        "failed",
		"error_message": errMsg,
		"finished_at":   now,
	}).Error; err != nil {
		logger.Error("Discovery", deviceIP, "更新设备错误状态失败: %v", err)
	}

	r.emitEvent(DiscoveryEvent{
		TaskID:    taskID,
		DeviceIP:  deviceIP,
		Type:      "error",
		Message:   errMsg,
		Timestamp: time.Now().UnixMilli(),
	})
}

// saveCommandOutput 保存命令输出（规范化输出 + 原始审计输出）
// result 包含 RawText（原始输出）和 NormalizedText（规范化输出）
// 注意：result 可能为 nil（当命令执行失败时），需要做空指针检查
func (r *Runner) saveCommandOutput(taskID, deviceIP, commandKey, command string, result *executor.CommandResult, status, errMsg string) {
	var filePath string
	var rawFilePath string

	if r.pathProvider != nil {
		// 原始审计输出路径
		rawFilePath = r.pathProvider.GetDiscoveryRawFilePath(taskID, deviceIP, commandKey)
		// 规范化输出路径
		filePath = r.pathProvider.GetDiscoveryNormalizedFilePath(taskID, deviceIP, commandKey)

		// 保存原始审计输出（仅当 result 不为 nil 时）
		if result != nil && result.RawText != "" {
			rawDir := filepath.Dir(rawFilePath)
			if err := os.MkdirAll(rawDir, 0700); err == nil {
				os.WriteFile(rawFilePath, []byte(result.RawText), 0600)
			}
		}

		// 保存规范化输出（仅当 result 不为 nil 时）
		if result != nil && result.NormalizedText != "" {
			normalizedDir := filepath.Dir(filePath)
			if err := os.MkdirAll(normalizedDir, 0700); err == nil {
				os.WriteFile(filePath, []byte(result.NormalizedText), 0600)
			}
		}
	}

	// 更新数据库记录
	updates := map[string]interface{}{
		"file_path":     filePath,
		"raw_file_path": rawFilePath,
		"status":        status,
		"error_message": errMsg,
		"parse_status":  "pending",
		"parse_error":   "",
	}

	// 处理 result 可能为 nil 的情况
	if result != nil {
		updates["raw_size"] = result.RawSize
		updates["normalized_size"] = result.NormalizedSize
		updates["line_count"] = result.LineCount()
		updates["pager_count"] = result.PaginationCount
		updates["echo_consumed"] = result.EchoConsumed
		updates["prompt_matched"] = result.PromptMatched
	} else {
		updates["raw_size"] = 0
		updates["normalized_size"] = 0
		updates["line_count"] = 0
		updates["pager_count"] = 0
		updates["echo_consumed"] = false
		updates["prompt_matched"] = false
	}

	// 阶段C修复：添加错误处理
	if err := r.db.Model(&models.RawCommandOutput{}).Where(
		"task_id = ? AND device_ip = ? AND command_key = ?",
		taskID, deviceIP, commandKey,
	).Updates(updates).Error; err != nil {
		logger.Error("Discovery", deviceIP, "更新命令输出记录失败 [%s]: %v", commandKey, err)
	}
}

// parseAndUpdateDeviceInfo 解析并更新设备信息（简单版本，后续由 parser 模块处理）
// 阶段C修复：添加错误处理
func (r *Runner) parseAndUpdateDeviceInfo(taskID, deviceIP, vendor, output string) {
	// 这里只是轻量预判，详细解析由 parser 模块完成。
	effectiveVendor := resolveDiscoveryVendor(vendor, "")
	detectedVendor := detectVendorFromVersion(output)
	if detectedVendor != "" {
		effectiveVendor = detectedVendor
	}
	if err := r.db.Model(&models.DiscoveryDevice{}).Where("task_id = ? AND device_ip = ?", taskID, deviceIP).Update("vendor", effectiveVendor).Error; err != nil {
		logger.Warn("Discovery", deviceIP, "更新设备厂商失败: %v", err)
	}
}

// getDevicesForDiscovery 获取用于发现的设备列表
func (r *Runner) getDevicesForDiscovery(req models.StartDiscoveryRequest) ([]models.DeviceInfo, error) {
	var devices []models.DeviceInfo
	var rows []DeviceAssetRow

	if len(req.DeviceIDs) > 0 {
		// 按设备ID查询
		if err := r.db.Table("device_assets").Where("id IN ?", req.DeviceIDs).Find(&rows).Error; err != nil {
			return nil, err
		}
	} else if len(req.GroupNames) > 0 {
		// 按设备组查询
		if err := r.db.Table("device_assets").Where("group_name IN ?", req.GroupNames).Find(&rows).Error; err != nil {
			return nil, err
		}
	} else {
		// 获取所有设备
		if err := r.db.Table("device_assets").Find(&rows).Error; err != nil {
			return nil, err
		}
	}

	requestedVendor := normalizeTaskVendor(req.Vendor)

	// 转换为 DeviceInfo
	for _, row := range rows {
		// 按厂商过滤
		if requestedVendor != "auto" && IsVendorSupported(requestedVendor) {
			if strings.ToLower(strings.TrimSpace(row.Vendor)) != requestedVendor {
				continue
			}
		}
		if strings.TrimSpace(row.IP) == "" {
			continue
		}
		devices = append(devices, row.ToDeviceInfo())
	}

	return devices, nil
}

// getDevicesByIPs 根据IP列表获取设备信息
func (r *Runner) getDevicesByIPs(ips []string) ([]models.DeviceInfo, error) {
	var rows []DeviceAssetRow
	if err := r.db.Table("device_assets").Where("ip IN ?", ips).Find(&rows).Error; err != nil {
		return nil, err
	}

	devices := make([]models.DeviceInfo, len(rows))
	for i, row := range rows {
		devices[i] = row.ToDeviceInfo()
	}

	return devices, nil
}

// emitEvent 发送事件
func (r *Runner) emitEvent(ev DiscoveryEvent) {
	select {
	case r.FrontendBus <- ev:
	default:
		// 通道已满，跳过
	}

	select {
	case r.EventBus <- ev:
	default:
		// 通道已满，跳过
	}
}

func (r *Runner) defaultDiscoveryWorkerCount() int {
	if r.runtimeProvider != nil {
		if workers := r.runtimeProvider.GetDiscoveryWorkerCount(); workers > 0 {
			return workers
		}
	}
	if r.maxWorkers > 0 {
		return r.maxWorkers
	}
	return 32
}

func (r *Runner) defaultDiscoveryCommandTimeout() time.Duration {
	if r.runtimeProvider != nil {
		if timeout := r.runtimeProvider.GetDiscoveryCommandTimeout(); timeout > 0 {
			return timeout
		}
	}
	return 60 * time.Second
}

func durationToPositiveSeconds(timeout time.Duration, fallback int) int {
	seconds := int(timeout.Seconds())
	if seconds > 0 {
		return seconds
	}
	return fallback
}

func resolveCommandTimeout(specTimeoutSec int, taskTimeout time.Duration) time.Duration {
	specTimeout := time.Duration(specTimeoutSec) * time.Second
	if specTimeout <= 0 {
		if taskTimeout > 0 {
			return taskTimeout
		}
		return 60 * time.Second
	}
	if taskTimeout <= 0 {
		return specTimeout
	}
	if specTimeout > taskTimeout {
		return taskTimeout
	}
	return specTimeout
}

func normalizeTaskVendor(vendor string) string {
	v := strings.ToLower(strings.TrimSpace(vendor))
	if v == "" {
		return "auto"
	}
	return v
}

func resolveDiscoveryVendor(requestedVendor, deviceVendor string) string {
	requested := strings.ToLower(strings.TrimSpace(requestedVendor))
	device := strings.ToLower(strings.TrimSpace(deviceVendor))

	if requested != "" && requested != "auto" {
		if IsVendorSupported(requested) {
			return requested
		}
		return DefaultVendor
	}
	if IsVendorSupported(device) {
		return device
	}
	return DefaultVendor
}

func detectVendorFromVersion(output string) string {
	text := strings.ToLower(output)
	switch {
	case strings.Contains(text, "huawei"):
		return "huawei"
	case strings.Contains(text, "h3c"), strings.Contains(text, "comware"):
		return "h3c"
	case strings.Contains(text, "cisco"):
		return "cisco"
	default:
		return ""
	}
}

// GetTaskStatus 获取任务状态
func (r *Runner) GetTaskStatus(taskID string) (*models.DiscoveryTaskView, error) {
	var task models.DiscoveryTask
	if err := r.db.First(&task, "id = ?", taskID).Error; err != nil {
		return nil, err
	}

	view := &models.DiscoveryTaskView{
		ID:           task.ID,
		Name:         task.Name,
		Status:       task.Status,
		TotalCount:   task.TotalCount,
		SuccessCount: task.SuccessCount,
		FailedCount:  task.FailedCount,
		StartedAt:    task.StartedAt,
		FinishedAt:   task.FinishedAt,
		CreatedAt:    task.CreatedAt,
		MaxWorkers:   task.MaxWorkers,
		Vendor:       task.Vendor,
	}

	return view, nil
}

// ListTasks 列出所有任务
func (r *Runner) ListTasks(limit int) ([]models.DiscoveryTaskView, error) {
	var tasks []models.DiscoveryTask
	if limit <= 0 {
		limit = 50
	}

	if err := r.db.Order("created_at DESC").Limit(limit).Find(&tasks).Error; err != nil {
		return nil, err
	}

	views := make([]models.DiscoveryTaskView, len(tasks))
	for i, t := range tasks {
		views[i] = models.DiscoveryTaskView{
			ID:           t.ID,
			Name:         t.Name,
			Status:       t.Status,
			TotalCount:   t.TotalCount,
			SuccessCount: t.SuccessCount,
			FailedCount:  t.FailedCount,
			StartedAt:    t.StartedAt,
			FinishedAt:   t.FinishedAt,
			CreatedAt:    t.CreatedAt,
			MaxWorkers:   t.MaxWorkers,
			Vendor:       t.Vendor,
		}
	}

	return views, nil
}

// GetTaskDevices 获取任务下的设备列表
func (r *Runner) GetTaskDevices(taskID string) ([]models.DiscoveryDeviceView, error) {
	var devices []models.DiscoveryDevice
	if err := r.db.Where("task_id = ?", taskID).Order("device_ip ASC").Find(&devices).Error; err != nil {
		return nil, err
	}

	views := make([]models.DiscoveryDeviceView, len(devices))
	for i, d := range devices {
		views[i] = models.DiscoveryDeviceView{
			ID:           d.ID,
			TaskID:       d.TaskID,
			DeviceIP:     d.DeviceIP,
			Status:       d.Status,
			ErrorMessage: d.ErrorMessage,
			StartedAt:    d.StartedAt,
			FinishedAt:   d.FinishedAt,
			DisplayName:  d.DisplayName,
			Role:         d.Role,
			Site:         d.Site,
			Vendor:       d.Vendor,
			Model:        d.Model,
			SerialNumber: d.SerialNumber,
			Version:      d.Version,
			Hostname:     d.Hostname,
			MgmtIP:       d.MgmtIP,
			ChassisID:    d.ChassisID,
		}
	}

	return views, nil
}

// GetRawOutput 获取原始命令输出
func (r *Runner) GetRawOutput(taskID, deviceIP, commandKey string) (string, error) {
	var output models.RawCommandOutput
	if err := r.db.Where("task_id = ? AND device_ip = ? AND command_key = ?", taskID, deviceIP, commandKey).First(&output).Error; err != nil {
		return "", err
	}

	if output.FilePath == "" {
		return "", nil
	}

	data, err := os.ReadFile(output.FilePath)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// IsRunning 检查是否有任务在运行
func (r *Runner) IsRunning() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.runningTask != ""
}

// GetCurrentTask 获取当前运行的任务ID
func (r *Runner) GetCurrentTask() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.runningTask
}
