package report

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/NetWeaverGo/core/internal/config"
	"github.com/NetWeaverGo/core/internal/logger"
)

// DeviceSummary 存储单台设备的最终汇总信息
type DeviceSummary struct {
	IP        string
	Status    string // "Success", "Failed", "Aborted", "Warning"
	TotalCmds int
	ExecCmds  int
	ErrorMsg  string
}

// DeviceViewState 单设备视图状态（用于前端快照）
type DeviceViewState struct {
	IP        string   `json:"ip"`
	Status    string   `json:"status"`    // running/success/error/aborted/waiting
	Logs      []string `json:"logs"`      // 已截断的日志数组
	LogCount  int      `json:"logCount"`  // 原始日志总条数
	Truncated bool     `json:"truncated"` // 是否已截断标记
	CmdIndex  int      `json:"cmdIndex"`  // 当前执行命令索引
	TotalCmd  int      `json:"totalCmd"`  // 总命令数
	Message   string   `json:"message"`   // 当前状态消息
}

// ExecutionSnapshot 执行快照（前端直接绑定渲染）
type ExecutionSnapshot struct {
	TaskName      string            `json:"taskName"`
	TotalDevices  int               `json:"totalDevices"`
	FinishedCount int               `json:"finishedCount"`
	Progress      int               `json:"progress"` // 0-100
	IsRunning     bool              `json:"isRunning"`
	StartTime     string            `json:"startTime"`
	Devices       []DeviceViewState `json:"devices"`
}

// getMaxLogsPerDevice 获取每设备最大日志数（从运行时配置）
func getMaxLogsPerDevice() int {
	manager := config.GetRuntimeManager()
	return manager.GetMaxLogsPerDevice()
}

// getMaxLogLength 获取单条日志最大长度（从运行时配置）
func getMaxLogLength() int {
	manager := config.GetRuntimeManager()
	return manager.GetMaxLogLength()
}

// ProgressTracker 终端进度盘面板与报告收集器
type ProgressTracker struct {
	EventBus chan ExecutorEvent

	mu       sync.Mutex
	status   map[string]*DeviceSummary // 用于大盘展示各设备状态
	finished int                       // 已经彻底跑完的设备数量
	total    int                       // 总设备数量
	paused   bool                      // 是否因交互被挂起

	// 新增：状态树管理
	taskName   string
	startTime  time.Time
	logStorage *LogStorage    // 磁盘日志存储
	logCounts  map[string]int // 原始日志计数
	sortedIPs  []string       // 有序的 IP 列表
}

func NewProgressTracker(totalDevices int) *ProgressTracker {
	logger.DebugAll("Report", "-", "生成与编排新的终端任务信息收集进度板，目标设备总量规模: %d 台", totalDevices)

	// 创建日志存储
	storage, err := NewLogStorage()
	if err != nil {
		logger.Error("Report", "-", "创建日志存储失败: %v", err)
		// 降级处理：storage 为 nil 时会在后续方法中处理
	}

	return &ProgressTracker{
		EventBus:   make(chan ExecutorEvent, 1000), // 留足缓冲
		status:     make(map[string]*DeviceSummary),
		total:      totalDevices,
		taskName:   "任务执行",
		startTime:  time.Now(),
		logStorage: storage,
		logCounts:  make(map[string]int),
		sortedIPs:  make([]string, 0, totalDevices),
	}
}

// SetTaskName 设置任务名称
func (p *ProgressTracker) SetTaskName(name string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.taskName = name
}

// GetStartTime 获取开始时间
func (p *ProgressTracker) GetStartTime() time.Time {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.startTime
}

// Listen 开始持续监听总线的事件并刷新屏幕
func (p *ProgressTracker) Listen(ctx context.Context) {
	logger.Debug("Report", "-", "总线收信机(Report-Collector)已进入工作循环，等待设备事件通报以生成最终大盘.")
	for {
		select {
		case <-ctx.Done():
			logger.Debug("Report", "-", "收到上级 Context 被中止信标，准备完成最后一次状态统筹汇算.")
			p.renderFinal()
			return
		case evt, ok := <-p.EventBus:
			if !ok {
				// 通道关闭，正常谢幕
				logger.DebugAll("Report", "-", "EventBus 频道已经下播，触发最终报表渲染并退出监听循环...")
				p.renderFinal()
				return
			}
			p.handleEvent(evt)
		}
	}
}

// Suspend 暂停界面刷新（用于交接控制权给 fmt.Scan 等标准输入）
func (p *ProgressTracker) Suspend() {
	p.mu.Lock()
	defer p.mu.Unlock()
	logger.DebugAll("Report", "-", "用户触发了互动式阻断，现已挂起大盘信息收集的定时排版状态...")
	p.paused = true
}

// CollectEvent 直接收集事件（用于从 worker 接收事件）
func (p *ProgressTracker) CollectEvent(evt ExecutorEvent) {
	p.handleEvent(evt)
}

// Resume 恢复界面刷新
func (p *ProgressTracker) Resume() {
	p.mu.Lock()
	defer p.mu.Unlock()
	logger.DebugAll("Report", "-", "用户输入处置结束，大盘状态监控解封，业务继续推进执行.")
	p.paused = false
}

func (p *ProgressTracker) handleEvent(evt ExecutorEvent) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 【修复】自动注册未记录的设备
	summary, exists := p.status[evt.IP]
	if !exists {
		p.registerDeviceLocked(evt.IP, evt.TotalCmd)
		summary = p.status[evt.IP]
	}

	logger.DebugAll("Report", evt.IP, "EventBus接收到事件: Type=%v, Message=%s", evt.Type, evt.Message)

	// 格式化日志消息
	var logMessage string
	switch evt.Type {
	case EventDeviceStart:
		summary.Status = "Running"
		summary.TotalCmds = evt.TotalCmd
		logMessage = fmt.Sprintf("[START] 开始执行，总命令数: %d", evt.TotalCmd)
		logger.Info("Report", evt.IP, "开始执行设备任务，总命令数: %d", evt.TotalCmd)
	case EventDeviceCmd:
		summary.Status = "Running"
		summary.ExecCmds = evt.CmdIndex
		summary.ErrorMsg = evt.Message // 借用记录当前命令
		logMessage = fmt.Sprintf("[CMD] %s [%d/%d]", evt.Message, evt.CmdIndex, evt.TotalCmd)
		logger.Info("Report", evt.IP, "正在执行: %s [%d/%d]", summary.ErrorMsg, summary.ExecCmds, summary.TotalCmds)
	case EventDeviceSuccess:
		summary.Status = "Success"
		summary.ExecCmds = summary.TotalCmds
		summary.ErrorMsg = "ALL Done"
		p.finished++
		logMessage = "[SUCCESS] 执行完成"
		logger.Info("Report", evt.IP, "设备任务执行成功")
	case EventDeviceError:
		summary.Status = "Error"
		summary.ErrorMsg = evt.Message
		logMessage = fmt.Sprintf("[ERROR] %s", evt.Message)
		logger.Error("Report", evt.IP, "设备任务执行出错: %s", evt.Message)
		// 注意：Error 不是终态事件——后续引擎会根据用户或策略选择发出 Abort 或 Skip。
		// 仅 Abort 和 Success 为终态，p.finished++ 统一在各自分支处理。
		// 边界情况：如果 SSH 流关闭导致 ExecutePlaybook 直接返回 nil（未发 Abort），
		// 引擎不会发 Success/Abort，finished 会少计 1。此类情况极罕见，可在 engine.worker
		// 结束时补发兜底事件来解决（已记录为优化待办）。
	case EventDeviceSkip:
		summary.Status = "Warning"
		summary.ErrorMsg = "Skip: " + evt.Message
		logMessage = fmt.Sprintf("[SKIP] %s", evt.Message)
		logger.Info("Report", evt.IP, "跳过节点: %s", evt.Message)
	case EventDeviceAbort:
		summary.Status = "Aborted"
		summary.ErrorMsg = "Aborted: " + evt.Message
		p.finished++
		logMessage = fmt.Sprintf("[ABORT] %s", evt.Message)
		logger.Error("Report", evt.IP, "设备任务被终止: %s", evt.Message)
	default:
		logMessage = fmt.Sprintf("[UNKNOWN] %s", evt.Message)
	}

	// 添加日志到设备日志存储（无锁版本，因为已持有锁）
	p.addDeviceLogLocked(evt.IP, logMessage)
}

// registerDeviceLocked 内部注册设备（必须在持有锁时调用）
func (p *ProgressTracker) registerDeviceLocked(ip string, totalCmd int) {
	logger.DebugAll("Report", ip, "自动注册设备到进度追踪器")

	// 添加到状态映射
	p.status[ip] = &DeviceSummary{
		IP:        ip,
		Status:    "Init",
		TotalCmds: totalCmd,
		ExecCmds:  0,
	}

	// 添加到有序列表
	p.sortedIPs = append(p.sortedIPs, ip)
	sort.Strings(p.sortedIPs)

	// 初始化磁盘日志存储
	if p.logStorage != nil {
		if err := p.logStorage.InitDevice(ip); err != nil {
			logger.Error("Report", ip, "初始化设备日志存储失败: %v", err)
		}
	}

	// 初始化计数
	if p.logCounts == nil {
		p.logCounts = make(map[string]int)
	}
	p.logCounts[ip] = 0
}

// addDeviceLogLocked 添加日志到设备（必须在持有锁时调用）
func (p *ProgressTracker) addDeviceLogLocked(ip string, message string) {
	// 截断过长日志
	maxLen := getMaxLogLength()
	if len(message) > maxLen {
		message = message[:maxLen] + "...[截断]"
	}

	// 【修复】添加 nil 检查
	if p.logStorage != nil {
		if err := p.logStorage.AppendLog(ip, message); err != nil {
			logger.Error("Report", ip, "写入日志失败: %v", err)
		}
	}

	// 【修复】更新计数时添加 nil 检查
	if p.logCounts == nil {
		p.logCounts = make(map[string]int)
	}
	if p.logStorage != nil {
		p.logCounts[ip] = p.logStorage.GetLogCount(ip)
	} else {
		p.logCounts[ip] = 0 // 降级处理：无日志存储时计数为 0
	}
}

// renderDisplay 简单的静态打印大盘（不再清屏）
func (p *ProgressTracker) renderDisplay() {
	p.mu.Lock()
	defer p.mu.Unlock()

	logger.DebugAll("Report", "-", "触发输出并生成新一期阶段设备态势与健康简报...")
	logger.Info("Report", "-", "=== 终端汇总大盘: 完成 [%d/%d] ===", p.finished, p.total)

	var ips []string
	for ip := range p.status {
		ips = append(ips, ip)
	}
	sort.Strings(ips)

	for _, ip := range ips {
		s := p.status[ip]
		bar := renderProgressBar(s.ExecCmds, s.TotalCmds)
		statusColor := s.Status
		if s.Status == "Success" {
			statusColor = "√ Success"
		} else if s.Status == "Error" || s.Status == "Aborted" {
			statusColor = "x " + s.Status
		}

		dispMsg := strings.ReplaceAll(s.ErrorMsg, "\r", "")
		dispMsg = strings.ReplaceAll(dispMsg, "\n", " ")
		dispMsg = truncateDisplayString(dispMsg, 40)

		logger.Info("Report", "-", " > %-15s [%s] %d/%d | %-9s | %s",
			ip, bar, s.ExecCmds, s.TotalCmds, statusColor, dispMsg)
	}
	logger.Info("Report", "-", "========================================")
}

func (p *ProgressTracker) renderFinal() {
	p.renderDisplay()                  // 最后一帧
	time.Sleep(100 * time.Millisecond) // 刷新等待时间
}

func renderProgressBar(current, total int) string {
	if total == 0 {
		return ".........."
	}
	percent := float64(current) / float64(total)
	filled := int(percent * 10)
	if filled > 10 {
		filled = 10
	}
	bar := ""
	for i := 0; i < filled; i++ {
		bar += "█"
	}
	for i := filled; i < 10; i++ {
		bar += "░"
	}
	return bar
}

// ExportCSV 生成小票结尾结算文档
func (p *ProgressTracker) ExportCSV(outputDir string) {
	logger.Debug("Report", "-", "开始生成结算 CSV 报表 -> %s", outputDir)
	p.mu.Lock()
	defer p.mu.Unlock()

	// 确保输出目录存在
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		logger.Error("Report", "-", "无法创建报告目录 %s: %v", outputDir, err)
		return
	}

	reportName := fmt.Sprintf("report_%s.csv", time.Now().Format("20060102_150405"))
	reportPath := filepath.Join(outputDir, reportName)

	file, err := os.Create(reportPath)
	if err != nil {
		logger.Error("Report", "-", "生成报告文件失败: %v", err)
		return
	}
	defer file.Close()

	// 写入 UTF-8 BOM，以防止在中文系统的 Excel 中被默认以 ANSI/GBK 格式打开导致乱码
	file.Write([]byte{0xEF, 0xBB, 0xBF})

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// 写入表头
	writer.Write([]string{"Target IP", "Final Status", "Commands Executed", "Total Commands", "Message/Error"})

	var ips []string
	for ip := range p.status {
		ips = append(ips, ip)
	}
	sort.Strings(ips)

	for _, ip := range ips {
		s := p.status[ip]
		writer.Write([]string{
			s.IP,
			s.Status,
			fmt.Sprintf("%d", s.ExecCmds),
			fmt.Sprintf("%d", s.TotalCmds),
			strings.ReplaceAll(strings.ReplaceAll(s.ErrorMsg, "\r", ""), "\n", " | "),
		})
	}

	logger.Info("Report", "-", "\n[结算报表已生成] -> %s", reportPath)
}

// truncateDisplayString 根据终端字符显示宽度来截断中英文字符串，避免半角全角混合下的等宽对齐偏离
func truncateDisplayString(s string, maxCol int) string {
	totalWidth := 0
	for _, r := range s {
		if isWideRune(r) {
			totalWidth += 2
		} else {
			totalWidth += 1
		}
	}
	if totalWidth <= maxCol {
		return s
	}

	w := 0
	var res []rune
	for _, r := range s {
		cw := 1
		if isWideRune(r) {
			cw = 2
		}
		if w+cw > maxCol-2 { // 空留2个位置给 ".."
			return string(res) + ".."
		}
		w += cw
		res = append(res, r)
	}
	return string(res) + ".."
}

func isWideRune(r rune) bool {
	// 中文、全角标点、日韩文等宽字符均算两列
	if (r >= 0x4E00 && r <= 0x9FFF) ||
		(r > 0xFF00 && r < 0xFFEF) ||
		(r >= 0x3000 && r <= 0x303F) ||
		(r >= 0x2000 && r <= 0x206F) {
		return true
	}
	return false
}

// ================== 状态树快照方法（前端数据源） ==================

// GetSnapshot 获取当前执行状态的完整快照
// 前端无需任何计算，直接绑定渲染即可
func (p *ProgressTracker) GetSnapshot() *ExecutionSnapshot {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 计算进度百分比
	progress := 0
	if p.total > 0 {
		progress = int(float64(p.finished) / float64(p.total) * 100)
		// 限制最大 95%，完成时由前端设置为 100
		if progress > 95 {
			progress = 95
		}
	}

	// 构建设备视图状态列表
	devices := make([]DeviceViewState, 0, len(p.status))
	for _, ip := range p.sortedIPs {
		summary, exists := p.status[ip]
		if !exists {
			continue
		}

		// 获取设备日志（已截断）
		logs, truncated := p.getDeviceLogsLocked(ip)

		// 状态转换：后端状态 -> 前端状态
		status := strings.ToLower(summary.Status)
		switch status {
		case "running":
			status = "running"
		case "success":
			status = "success"
		case "error":
			status = "error"
		case "aborted":
			status = "error"
		case "warning":
			status = "success" // Skip 视为成功
		case "init":
			status = "waiting"
		default:
			status = "waiting"
		}

		deviceState := DeviceViewState{
			IP:        summary.IP,
			Status:    status,
			Logs:      logs,
			LogCount:  p.logCounts[ip],
			Truncated: truncated,
			CmdIndex:  summary.ExecCmds,
			TotalCmd:  summary.TotalCmds,
			Message:   summary.ErrorMsg,
		}
		devices = append(devices, deviceState)
	}

	return &ExecutionSnapshot{
		TaskName:      p.taskName,
		TotalDevices:  p.total,
		FinishedCount: p.finished,
		Progress:      progress,
		IsRunning:     !p.paused,
		StartTime:     p.startTime.Format(time.RFC3339),
		Devices:       devices,
	}
}

// GetDeviceSnapshot 获取单个设备的快照
func (p *ProgressTracker) GetDeviceSnapshot(ip string) *DeviceViewState {
	p.mu.Lock()
	defer p.mu.Unlock()

	summary, exists := p.status[ip]
	if !exists {
		return nil
	}

	logs, truncated := p.getDeviceLogsLocked(ip)

	status := strings.ToLower(summary.Status)
	switch status {
	case "running":
		status = "running"
	case "success":
		status = "success"
	case "error", "aborted":
		status = "error"
	case "warning":
		status = "success"
	case "init":
		status = "waiting"
	default:
		status = "waiting"
	}

	return &DeviceViewState{
		IP:        summary.IP,
		Status:    status,
		Logs:      logs,
		LogCount:  p.logCounts[ip],
		Truncated: truncated,
		CmdIndex:  summary.ExecCmds,
		TotalCmd:  summary.TotalCmds,
		Message:   summary.ErrorMsg,
	}
}

// getDeviceLogsLocked 获取设备日志（已截断），必须在持有锁时调用
func (p *ProgressTracker) getDeviceLogsLocked(ip string) ([]string, bool) {
	// 从磁盘存储读取日志
	if p.logStorage == nil {
		return []string{}, false
	}

	maxLogs := getMaxLogsPerDevice()
	totalCount := p.logStorage.GetLogCount(ip)

	if totalCount == 0 {
		return []string{}, false
	}

	// 读取最新的日志
	logs, err := p.logStorage.GetLastLogs(ip, maxLogs)
	if err != nil {
		logger.Error("Report", ip, "读取日志失败: %v", err)
		return []string{}, false
	}

	truncated := totalCount > maxLogs
	return logs, truncated
}

// AddDeviceLog 添加日志到设备（带上限控制）
// 此方法由事件处理器调用
func (p *ProgressTracker) AddDeviceLog(ip string, message string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 截断过长日志
	maxLen := getMaxLogLength()
	if len(message) > maxLen {
		message = message[:maxLen] + "...[截断]"
	}

	// 【修复】添加 nil 检查
	if p.logStorage != nil {
		if err := p.logStorage.AppendLog(ip, message); err != nil {
			logger.Error("Report", ip, "写入日志失败: %v", err)
		}
	}

	// 【修复】更新计数时添加 nil 检查
	if p.logCounts == nil {
		p.logCounts = make(map[string]int)
	}
	if p.logStorage != nil {
		p.logCounts[ip] = p.logStorage.GetLogCount(ip)
	} else {
		p.logCounts[ip] = 0 // 降级处理：无日志存储时计数为 0
	}
}

// RegisterDevice 注册设备到有序列表（公共方法）
func (p *ProgressTracker) RegisterDevice(ip string, totalCmd int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.registerDeviceLocked(ip, totalCmd)
}

// Close 关闭并清理资源
func (p *ProgressTracker) Close() {
	if p.logStorage != nil {
		p.logStorage.Close()
	}
}

// GetStats 获取当前统计信息
func (p *ProgressTracker) GetStats() (total, finished, success, error int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	total = p.total
	finished = p.finished

	for _, summary := range p.status {
		switch summary.Status {
		case "Success", "Warning":
			success++
		case "Error", "Aborted":
			error++
		}
	}

	return
}

// IsFinished 检查是否所有设备都已完成
func (p *ProgressTracker) IsFinished() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.finished >= p.total
}
