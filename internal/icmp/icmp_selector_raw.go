//go:build !windows

package icmp

import (
	"github.com/NetWeaverGo/core/internal/logger"
)

func init() {
	backend, err := initRawSocketBackend()
	if err != nil {
		logger.Error("ICMP", "-", "Raw Socket 后端初始化失败: %v", err)
		panic("ICMP Raw Socket 后端初始化失败: " + err.Error())
	}
	currentBackend = backend
	backendType = BackendRawSocket
	logger.Info("ICMP", "-", "ICMP 后端初始化: %s", currentBackend.name())
}
