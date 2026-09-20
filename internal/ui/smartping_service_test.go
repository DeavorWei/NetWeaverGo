package ui

import (
	"testing"

	"github.com/NetWeaverGo/core/internal/smartping"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSmartPingService_AnalyzePing(t *testing.T) {
	service := NewSmartPingService()
	require.NotNil(t, service)

	// 1. 正常场景
	metricNormal := &smartping.PingHostMetric{
		IP:        "192.168.1.1",
		Alive:     true,
		SentCount: 10,
		RecvCount: 10,
		LossRate:  0.0,
		MinRtt:    1.2,
		MaxRtt:    2.4,
		AvgRtt:    1.8,
	}
	resNormal, err := service.AnalyzePing(metricNormal, "default")
	require.NoError(t, err)
	assert.Equal(t, "healthy", resNormal.Status)
	assert.Equal(t, 100, resNormal.HealthScore)

	// 2. 丢包与离线场景
	metricLoss := &smartping.PingHostMetric{
		IP:          "10.0.0.1",
		Alive:       false,
		SentCount:   10,
		RecvCount:   0,
		FailedCount: 10,
		LossRate:    100.0,
	}
	resLoss, err := service.AnalyzePing(metricLoss, "default")
	require.NoError(t, err)
	assert.Equal(t, "unreachable", resLoss.Status)
	assert.Equal(t, 0, resLoss.HealthScore)
	assert.NotEmpty(t, resLoss.Anomalies)

	// 3. 空参数校验
	_, err = service.AnalyzePing(nil, "default")
	assert.Error(t, err)

	// 4. 获取规则列表
	rules := service.ListRules()
	assert.NotEmpty(t, rules)

	// 5. 批量诊断
	batchRes, err := service.BatchAnalyze([]BatchAnalyzeItem{
		{Metric: *metricNormal, Scene: "lan"},
		{Metric: *metricLoss, Scene: "wan"},
	})
	require.NoError(t, err)
	assert.Len(t, batchRes, 2)
}
