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
	"github.com/gosuri/uilive"
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
	writer   *uilive.Writer

	mu       sync.Mutex
	status   map[string]*DeviceSummary // 用于大盘展示各设备状态
	finished int                       // 已经彻底跑完的设备数量
	total    int                       // 总设备数量
	paused   bool                      // 是否因交互被挂起
}

func NewProgressTracker(totalDevices int) *ProgressTracker {
	writer := uilive.New()

	return &ProgressTracker{
		EventBus: make(chan ExecutorEvent, 1000), // 留足缓冲
		writer:   writer,
		status:   make(map[string]*DeviceSummary),
		total:    totalDevices,
	}
}

// Listen 开始持续监听总线的事件并刷新屏幕
func (p *ProgressTracker) Listen(ctx context.Context) {
	ticker := time.NewTicker(200 * time.Millisecond) // 每秒刷新5次
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			p.renderFinal()
			return
		case evt, ok := <-p.EventBus:
			if !ok {
				// 通道关闭，正常谢幕
				p.renderFinal()
				return
			}
			p.handleEvent(evt)
		case <-ticker.C:
			p.mu.Lock()
			isPaused := p.paused
			p.mu.Unlock()
			if !isPaused {
				p.renderDisplay()
			}
		}
	}
}

// Suspend 暂停界面刷新（用于交接控制权给 fmt.Scan 等标准输入）
func (p *ProgressTracker) Suspend() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.paused = true
}

// Resume 恢复界面刷新
func (p *ProgressTracker) Resume() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.writer = uilive.New() // 重新生成 writer，让下一次排版行数清零，在底部新开画板！
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
	case EventDeviceCmd:
		summary.Status = "Running"
		summary.ExecCmds = evt.CmdIndex
		summary.ErrorMsg = evt.Message // 借用记录当前命令
	case EventDeviceSuccess:
		summary.Status = "Success"
		summary.ExecCmds = summary.TotalCmds
		summary.ErrorMsg = "ALL Done"
		p.finished++
	case EventDeviceError:
		summary.Status = "Error"
		summary.ErrorMsg = evt.Message
		// 注意：Error 不是终态事件——后续引擎会根据用户或策略选择发出 Abort 或 Skip。
		// 仅 Abort 和 Success 为终态，p.finished++ 统一在各自分支处理。
		// 边界情况：如果 SSH 流关闭导致 ExecutePlaybook 直接返回 nil（未发 Abort），
		// 引擎不会发 Success/Abort，finished 会少计 1。此类情况极罕见，可在 engine.worker
		// 结束时补发兜底事件来解决（已记录为优化待办）。
	case EventDeviceSkip:
		summary.Status = "Warning"
		summary.ErrorMsg = "Skip: " + evt.Message
	case EventDeviceAbort:
		summary.Status = "Aborted"
		summary.ErrorMsg = "Aborted: " + evt.Message
		p.finished++
	}
}

// renderDisplay 清屏重绘当前的活动设备列表大盘
func (p *ProgressTracker) renderDisplay() {
	p.mu.Lock()
	defer p.mu.Unlock()

	var buf strings.Builder
	// 取消前后的 \n，防止终端自动滚动导致 uilive 的往上回退产生错位（即“残留的悬崖”）
	fmt.Fprintf(&buf, "\r[ NetWeaverGo ] 进度大盘: 完成 [%d/%d]\n", p.finished, p.total)
	fmt.Fprintf(&buf, "--------------------------------------------------------")

	// 抽出所有 IP 以稳定行序展现
	var ips []string
	for ip := range p.status {
		ips = append(ips, ip)
	}
	sort.Strings(ips)

	// 最多显示最活跃的15台设备以防止终端过长
	displayCount := 0
	for _, ip := range ips {
		s := p.status[ip]
		if displayCount >= 15 && s.Status != "Running" && s.Status != "Error" && s.Status != "Init" {
			continue // 隐藏成功并且排在后面的条目
		}

		// 进度条渲染 -> [██████░░░░]
		bar := renderProgressBar(s.ExecCmds, s.TotalCmds)

		statusColor := s.Status
		if s.Status == "Success" {
			statusColor = "√ Success"
		} else if s.Status == "Error" || s.Status == "Aborted" {
			statusColor = "x " + s.Status
		}

		// 严格控制 dispMsg 长度，防止在此行发生终端换行（Wrap）导致被动滚动
		dispMsg := strings.ReplaceAll(s.ErrorMsg, "\r", "")
		dispMsg = strings.ReplaceAll(dispMsg, "\n", " ")
		dispMsg = truncateDisplayString(dispMsg, 20) // 给20列的最大宽度，前缀大约占用55列，总计不超过80列

		// 保证此行的物理字符宽度 < 80 列
		fmt.Fprintf(&buf, "\n > %-15s [%s] %d/%d | %-9s | %s",
			ip, bar, s.ExecCmds, s.TotalCmds, statusColor, dispMsg)

		displayCount++
	}

	// 核心修复：恒定高度占位符！
	// 动态增加的大盘会引起终端卷动，从而导致 uilive 的向上清屏发生错位。
	// 这里预先输出足够多的空行，在一开始就把终端空间占住，使得渲染高度永远是恒定的。
	targetHeight := p.total
	if targetHeight > 15 {
		targetHeight = 15
	}
	for displayCount < targetHeight {
		fmt.Fprintf(&buf, "\n")
		displayCount++
	}

	fmt.Fprintf(&buf, "\n--------------------------------------------------------")

	p.writer.Write([]byte(buf.String()))
	p.writer.Flush()
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
