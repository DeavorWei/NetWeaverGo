package ui

import (
	"testing"

	"github.com/NetWeaverGo/core/internal/optical"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpticalService_CheckTransceiver(t *testing.T) {
	service := NewOpticalService()
	require.NotNil(t, service)

	// 正常光模块指标
	metric := optical.TransceiverMetric{
		DeviceIP:        "192.168.1.1",
		Port:            "10GE1/0/1",
		Vendor:          "huawei",
		TransceiverType: "10GE-LR",
		TxPower:         -2.5,
		RxPower:         -8.0,
		Temperature:     40.0,
		Voltage:         3.3,
		BiasCurrent:     35.0,
	}

	result, err := service.CheckTransceiver(metric)
	require.NoError(t, err)
	assert.Equal(t, optical.LevelNormal, result.Level)
	assert.False(t, result.IsWeakOptical)
	assert.False(t, result.IsLOS)

	// 弱光收光过低指标
	weakMetric := optical.TransceiverMetric{
		DeviceIP:        "192.168.1.1",
		Port:            "10GE1/0/2",
		Vendor:          "huawei",
		TransceiverType: "10GE-LR",
		TxPower:         -2.5,
		RxPower:         -25.0, // 远低于预警阈值
		Temperature:     40.0,
		Voltage:         3.3,
		BiasCurrent:     35.0,
	}

	resultWeak, err := service.CheckTransceiver(weakMetric)
	require.NoError(t, err)
	assert.True(t, resultWeak.IsWeakOptical)
}
