//go:build !windows

package icmp

import (
	"fmt"
	"os"

	"github.com/NetWeaverGo/core/internal/logger"
)

// IsAdmin 检测当前进程是否以管理员/root权限运行
func IsAdmin() bool {
	return os.Geteuid() == 0
}

// RequestElevation 请求提权（非Windows平台无法自动提权）
func RequestElevation() error {
	logger.Error("ICMP", "-", "当前非管理员权限运行，ICMP 操作需要 root 权限")
	return fmt.Errorf("请使用管理员权限运行此程序 (e.g., sudo)")
}
