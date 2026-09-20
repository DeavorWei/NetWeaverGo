//go:build windows

package ui

import (
	"testing"

	"github.com/NetWeaverGo/core/internal/icmp"
)

// P1-10：批量 Ping 结果必须接入智能诊断引擎并可通过 Wails 方法读取
func TestPingService_AnalyzeWithSmartPing(t *testing.T) {
	svc := &PingService{}
	progress := &icmp.BatchPingProgress{
		Results: []icmp.PingHostResult{
			{IP: "10.0.0.1", Alive: true, SentCount: 3, RecvCount: 3, LossRate: 0, MinRtt: 1, MaxRtt: 2, AvgRtt: 1.5},
			{IP: "10.0.0.2", Alive: false, SentCount: 3, RecvCount: 0, FailedCount: 3, LossRate: 100, MinRtt: -1, MaxRtt: -1, AvgRtt: -1},
		},
	}

	reports := svc.analyzeWithSmartPing(progress, "default")
	if len(reports) != 2 {
		t.Fatalf("应生成 2 份诊断报告, got %d", len(reports))
	}
	if reports[1] == nil || reports[1].Status == "healthy" {
		t.Fatalf("全丢包主机不应判定为健康: %+v", reports[1])
	}

	svc.setSmartPingReports(reports)
	if got := svc.GetSmartPingReports(); len(got) != 2 {
		t.Fatalf("GetSmartPingReports = %d, want 2", len(got))
	}
}
