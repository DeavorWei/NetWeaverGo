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

	"github.com/NetWeaverGo/core/internal/executor"
	"github.com/NetWeaverGo/core/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// PathProvider 路径提供者接口（避免循环导入）
type PathProvider interface {
	GetDiscoveryRawFilePath(taskID, deviceIP, commandKey string) string
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
		EventBus:    make(chan DiscoveryEvent, 200),
		FrontendBus: make(chan DiscoveryEvent, 200),
		maxWorkers:  32,
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
	if workers > 0 && workers <= 100 {
		r.maxWorkers = workers
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
		profile := GetVendorProfile(effectiveVendor)
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
	now := time.Now()
	r.db.Model(&models.DiscoveryTask{}).Where("id = ?", taskID).Updates(map[string]interface{}{
		"status":     "running",
		"started_at": now,
	})

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
	now := time.Now()
	r.db.Model(&models.DiscoveryTask{}).Where("id = ?", taskID).Updates(map[string]interface{}{
		"status":      "cancelled",
		"finished_at": now,
	})

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
	r.db.Model(&models.DiscoveryDevice{}).Where("task_id = ? AND status = ?", taskID, "failed").Update("status", "pending")

	// 初始化 context
	r.ctx, r.cancel = context.WithCancel(ctx)
	r.runningTask = taskID

	// 更新任务状态为运行中
	r.db.Model(&models.DiscoveryTask{}).Where("id = ?", taskID).Updates(map[string]interface{}{
		"status": "running",
	})

	// 启动后台任务执行
	go r.runDiscovery(r.ctx, taskID, devices, task.Vendor, task.TimeoutSec, task.MaxWorkers)

	return nil
}

// runDiscovery 执行发现任务
func (r *Runner) runDiscovery(ctx context.Context, taskID string, devices []models.DeviceInfo, requestedVendor string, timeoutSec int, maxWorkers int) {
	defer func() {
		r.mu.Lock()
		r.runningTask = ""
		r.mu.Unlock()
	}()

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
	var countMu sync.Mutex

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
				<-sem
				wg.Done()
			}()

			// 增加抖动
			time.Sleep(time.Duration(rand.Intn(500)) * time.Millisecond)

			effectiveVendor := resolveDiscoveryVendor(requestedVendor, device.Vendor)
			profile := GetVendorProfile(effectiveVendor)

			deviceCtx := ctx
			cancel := func() {}
			if perDeviceTimeout > 0 {
				deviceCtx, cancel = context.WithTimeout(ctx, perDeviceTimeout)
			}
			defer cancel()

			// 执行设备发现
			err := r.discoverDevice(deviceCtx, taskID, device, effectiveVendor, profile, connectTimeout, taskCommandTimeout)

			countMu.Lock()
			if err != nil {
				failedCount++
			} else {
				successCount++
			}
			countMu.Unlock()
		}(dev)
	}

	wg.Wait()

	// 更新任务状态
	now := time.Now()
	status := "completed"
	if failedCount > 0 && successCount == 0 {
		status = "failed"
	} else if failedCount > 0 {
		status = "partial"
	}
	if ctx.Err() != nil {
		status = "cancelled"
	}

	var currentTask models.DiscoveryTask
	if err := r.db.Select("status").Where("id = ?", taskID).Take(&currentTask).Error; err == nil {
		if strings.EqualFold(currentTask.Status, "cancelled") {
			status = "cancelled"
		}
	}

	r.db.Model(&models.DiscoveryTask{}).Where("id = ?", taskID).Updates(map[string]interface{}{
		"status":        status,
		"finished_at":   now,
		"success_count": successCount,
		"failed_count":  failedCount,
	})

	// 发送完成事件
	r.emitEvent(DiscoveryEvent{
		TaskID:    taskID,
		Type:      "completed",
		Message:   fmt.Sprintf("发现任务完成: 成功 %d, 失败 %d", successCount, failedCount),
		Timestamp: time.Now().UnixMilli(),
	})
}

// discoverDevice 执行单设备发现
func (r *Runner) discoverDevice(ctx context.Context, taskID string, device models.DeviceInfo, vendor string, profile *VendorCommandProfile, connectTimeout time.Duration, taskCommandTimeout time.Duration) error {
	// 更新设备状态
	now := time.Now()
	r.db.Model(&models.DiscoveryDevice{}).Where("task_id = ? AND device_ip = ?", taskID, device.IP).Updates(map[string]interface{}{
		"status":     "running",
		"started_at": now,
		"vendor":     vendor,
	})

	// 发送开始事件
	r.emitEvent(DiscoveryEvent{
		TaskID:    taskID,
		DeviceIP:  device.IP,
		Type:      "start",
		Message:   "开始发现设备",
		Timestamp: time.Now().UnixMilli(),
	})

	// 创建执行器
	exec := executor.NewDeviceExecutor(device.IP, device.Port, device.Username, device.Password, nil, nil)

	// 连接设备
	if err := exec.Connect(ctx, connectTimeout); err != nil {
		r.updateDeviceError(taskID, device.IP, fmt.Sprintf("SSH连接失败: %v", err))
		return err
	}
	defer exec.Close()

	// 执行命令并保存输出
	var lastErr error
	cmdSuccess := 0
	cmdFailed := 0
	for _, cmd := range profile.Commands {
		select {
		case <-ctx.Done():
			r.updateDeviceError(taskID, device.IP, "任务已取消")
			return fmt.Errorf("任务已取消")
		default:
		}

		// 发送命令事件
		r.emitEvent(DiscoveryEvent{
			TaskID:    taskID,
			DeviceIP:  device.IP,
			Type:      "cmd",
			Message:   fmt.Sprintf("执行命令: %s", cmd.Command),
			Timestamp: time.Now().UnixMilli(),
		})

		// 执行命令并收集输出 (使用 ExecuteCommandSync)
		commandTimeout := resolveCommandTimeout(cmd.TimeoutSec, taskCommandTimeout)
		output, err := exec.ExecuteCommandSync(ctx, cmd.Command, commandTimeout)
		if err != nil {
			lastErr = err
			cmdFailed++
			// 保存错误信息
			r.saveRawOutput(taskID, device.IP, cmd.CommandKey, cmd.Command, "", "failed", err.Error(), 0)
			continue
		}
		cmdSuccess++

		// 保存原始输出
		r.saveRawOutput(taskID, device.IP, cmd.CommandKey, cmd.Command, output, "success", "", int64(len(output)))

		// 如果是 version 命令，尝试解析设备信息
		if cmd.CommandKey == "version" {
			r.parseAndUpdateDeviceInfo(taskID, device.IP, vendor, output)
		}
	}

	// 更新设备状态
	finishedAt := time.Now()
	deviceStatus := "success"
	deviceErr := ""
	if cmdFailed > 0 && cmdSuccess > 0 {
		deviceStatus = "partial"
		if lastErr != nil {
			deviceErr = lastErr.Error()
		}
	} else if cmdFailed > 0 && cmdSuccess == 0 {
		deviceStatus = "failed"
		if lastErr != nil {
			deviceErr = lastErr.Error()
		}
	}
	r.db.Model(&models.DiscoveryDevice{}).Where("task_id = ? AND device_ip = ?", taskID, device.IP).Updates(map[string]interface{}{
		"status":        deviceStatus,
		"error_message": deviceErr,
		"finished_at":   finishedAt,
	})

	// 发送成功事件
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
	if lastErr != nil {
		return lastErr
	}
	return fmt.Errorf("设备发现失败: %s", deviceStatus)
}

// updateDeviceError 更新设备错误状态
func (r *Runner) updateDeviceError(taskID, deviceIP, errMsg string) {
	now := time.Now()
	r.db.Model(&models.DiscoveryDevice{}).Where("task_id = ? AND device_ip = ?", taskID, deviceIP).Updates(map[string]interface{}{
		"status":        "failed",
		"error_message": errMsg,
		"finished_at":   now,
	})

	r.emitEvent(DiscoveryEvent{
		TaskID:    taskID,
		DeviceIP:  deviceIP,
		Type:      "error",
		Message:   errMsg,
		Timestamp: time.Now().UnixMilli(),
	})
}

// saveRawOutput 保存原始命令输出
func (r *Runner) saveRawOutput(taskID, deviceIP, commandKey, command, output, status, errMsg string, size int64) {
	// 保存到文件
	var filePath string
	if output != "" && r.pathProvider != nil {
		filePath = r.pathProvider.GetDiscoveryRawFilePath(taskID, deviceIP, commandKey)

		// 确保目录存在（权限0700：仅所有者可读写执行）
		dir := filepath.Dir(filePath)
		if err := os.MkdirAll(dir, 0700); err == nil {
			// 文件权限0600：仅所有者可读写
			os.WriteFile(filePath, []byte(output), 0600)
		}
	}

	// 更新数据库记录
	r.db.Model(&models.RawCommandOutput{}).Where(
		"task_id = ? AND device_ip = ? AND command_key = ?",
		taskID, deviceIP, commandKey,
	).Updates(map[string]interface{}{
		"file_path":     filePath,
		"status":        status,
		"error_message": errMsg,
		"parse_status":  "pending",
		"parse_error":   "",
		"output_size":   size,
	})
}

// parseAndUpdateDeviceInfo 解析并更新设备信息（简单版本，后续由 parser 模块处理）
func (r *Runner) parseAndUpdateDeviceInfo(taskID, deviceIP, vendor, output string) {
	// 这里只是轻量预判，详细解析由 parser 模块完成。
	effectiveVendor := resolveDiscoveryVendor(vendor, "")
	detectedVendor := detectVendorFromVersion(output)
	if detectedVendor != "" {
		effectiveVendor = detectedVendor
	}
	r.db.Model(&models.DiscoveryDevice{}).Where("task_id = ? AND device_ip = ?", taskID, deviceIP).Update("vendor", effectiveVendor)
}

// getDevicesForDiscovery 获取用于发现的设备列表
func (r *Runner) getDevicesForDiscovery(req models.StartDiscoveryRequest) ([]models.DeviceInfo, error) {
	var devices []models.DeviceInfo

	// 定义一个临时结构来接收数据库查询结果
	type DeviceAssetRow struct {
		ID          uint   `gorm:"column:id"`
		IP          string `gorm:"column:ip"`
		Port        int    `gorm:"column:port"`
		Username    string `gorm:"column:username"`
		Password    string `gorm:"column:password"`
		Vendor      string `gorm:"column:vendor"`
		DisplayName string `gorm:"column:display_name"`
		Role        string `gorm:"column:role"`
		Site        string `gorm:"column:site"`
		Group       string `gorm:"column:group_name"`
	}

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
		devices = append(devices, models.DeviceInfo{
			ID:          row.ID,
			IP:          row.IP,
			Port:        row.Port,
			Username:    row.Username,
			Password:    row.Password,
			Vendor:      row.Vendor,
			DisplayName: row.DisplayName,
			Role:        row.Role,
			Site:        row.Site,
		})
	}

	return devices, nil
}

// getDevicesByIPs 根据IP列表获取设备信息
func (r *Runner) getDevicesByIPs(ips []string) ([]models.DeviceInfo, error) {
	type DeviceAssetRow struct {
		ID          uint   `gorm:"column:id"`
		IP          string `gorm:"column:ip"`
		Port        int    `gorm:"column:port"`
		Username    string `gorm:"column:username"`
		Password    string `gorm:"column:password"`
		Vendor      string `gorm:"column:vendor"`
		DisplayName string `gorm:"column:display_name"`
		Role        string `gorm:"column:role"`
		Site        string `gorm:"column:site"`
	}

	var rows []DeviceAssetRow
	if err := r.db.Table("device_assets").Where("ip IN ?", ips).Find(&rows).Error; err != nil {
		return nil, err
	}

	devices := make([]models.DeviceInfo, len(rows))
	for i, row := range rows {
		devices[i] = models.DeviceInfo{
			ID:          row.ID,
			IP:          row.IP,
			Port:        row.Port,
			Username:    row.Username,
			Password:    row.Password,
			Vendor:      row.Vendor,
			DisplayName: row.DisplayName,
			Role:        row.Role,
			Site:        row.Site,
		}
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
