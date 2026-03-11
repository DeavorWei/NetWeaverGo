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

// ProgressTracker 终端进度盘面板与报告收集器
type ProgressTracker struct {
	EventBus chan ExecutorEvent

	mu       sync.Mutex
	status   map[string]*DeviceSummary // 用于大盘展示各设备状态
	finished int                       // 已经彻底跑完的设备数量
	total    int                       // 总设备数量
	paused   bool                      // 是否因交互被挂起
}

func NewProgressTracker(totalDevices int) *ProgressTracker {
	logger.DebugAll("Report", "-", "生成与编排新的终端任务信息收集进度板，目标设备总量规模: %d 台", totalDevices)
	return &ProgressTracker{
		EventBus: make(chan ExecutorEvent, 1000), // 留足缓冲
		status:   make(map[string]*DeviceSummary),
		total:    totalDevices,
	}
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

	summary, exists := p.status[evt.IP]
	if !exists {
		logger.DebugAll("Report", evt.IP, "初始化设备状态记录大盘")
		summary = &DeviceSummary{
			IP:        evt.IP,
			Status:    "Init",
			TotalCmds: evt.TotalCmd,
			ExecCmds:  0,
		}
		p.status[evt.IP] = summary
	}

	logger.DebugAll("Report", evt.IP, "EventBus接收到事件: Type=%v, Message=%s", evt.Type, evt.Message)

	switch evt.Type {
	case EventDeviceStart:
		summary.Status = "Running"
		summary.TotalCmds = evt.TotalCmd
		logger.Info("Report", evt.IP, "开始执行设备任务，总命令数: %d", evt.TotalCmd)
	case EventDeviceCmd:
		summary.Status = "Running"
		summary.ExecCmds = evt.CmdIndex
		summary.ErrorMsg = evt.Message // 借用记录当前命令
		logger.Info("Report", evt.IP, "正在执行: %s [%d/%d]", summary.ErrorMsg, summary.ExecCmds, summary.TotalCmds)
	case EventDeviceSuccess:
		summary.Status = "Success"
		summary.ExecCmds = summary.TotalCmds
		summary.ErrorMsg = "ALL Done"
		p.finished++
		logger.Info("Report", evt.IP, "设备任务执行成功")
	case EventDeviceError:
		summary.Status = "Error"
		summary.ErrorMsg = evt.Message
		logger.Error("Report", evt.IP, "设备任务执行出错: %s", evt.Message)
		// 注意：Error 不是终态事件——后续引擎会根据用户或策略选择发出 Abort 或 Skip。
		// 仅 Abort 和 Success 为终态，p.finished++ 统一在各自分支处理。
		// 边界情况：如果 SSH 流关闭导致 ExecutePlaybook 直接返回 nil（未发 Abort），
		// 引擎不会发 Success/Abort，finished 会少计 1。此类情况极罕见，可在 engine.worker
		// 结束时补发兜底事件来解决（已记录为优化待办）。
	case EventDeviceSkip:
		summary.Status = "Warning"
		summary.ErrorMsg = "Skip: " + evt.Message
		logger.Info("Report", evt.IP, "跳过节点: %s", evt.Message)
	case EventDeviceAbort:
		summary.Status = "Aborted"
		summary.ErrorMsg = "Aborted: " + evt.Message
		p.finished++
		logger.Error("Report", evt.IP, "设备任务被终止: %s", evt.Message)
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
