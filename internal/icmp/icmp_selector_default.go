//go:build windows && !rawicmp

package icmp

import (
	"github.com/NetWeaverGo/core/internal/logger"
)

func init() {
	// 默认使用 golang.org/x/net/icmp 的 Raw Socket 后端
	// 如果初始化失败且非管理员权限，会尝试 UAC 提权
	backend, err := initRawSocketBackend()
	if err != nil {
		logger.Warn("ICMP", "-", "Raw Socket 后端初始化失败: %v", err)

		// 检测是否为管理员权限
		if !IsAdmin() {
			logger.Info("ICMP", "-", "检测到非管理员权限，正在请求 UAC 提权...")
			if elevateErr := RequestElevation(); elevateErr != nil {
				logger.Error("ICMP", "-", "UAC 提权请求失败: %v（用户可能拒绝了提权）", elevateErr)
			}
			// 如果提权成功，当前进程会在 RequestElevation 中 os.Exit(0)
			// 如果提权失败（用户拒绝），继续使用 errorBackend
		} else {
			logger.Error("ICMP", "-", "当前已是管理员权限，但 Raw Socket 仍初始化失败: %v", err)
		}

		// 降级为 errorBackend，程序仍可启动但 ICMP 功能不可用
		currentBackend = &errorBackend{err: "ICMP 需要管理员权限: " + err.Error()}
		backendType = BackendRawSocket
		logger.Warn("ICMP", "-", "ICMP 后端降级为 ErrorBackend，ICMP 功能将不可用")
		return
	}

	currentBackend = backend
	backendType = BackendRawSocket
	logger.Info("ICMP", "-", "ICMP 后端初始化: %s", currentBackend.name())
}
