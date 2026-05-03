//go:build windows

package icmp

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"

	"github.com/NetWeaverGo/core/internal/logger"
)

var (
	shell32              = syscall.NewLazyDLL("shell32.dll")
	procShellExecuteW    = shell32.NewProc("ShellExecuteW")
	advapi32             = syscall.NewLazyDLL("advapi32.dll")
	procAllocateAndInitializeSid = advapi32.NewProc("AllocateAndInitializeSid")
	procFreeSid          = advapi32.NewProc("FreeSid")
	procCheckTokenMembership = advapi32.NewProc("CheckTokenMembership")
)

const (
	SECURITY_NT_AUTHORITY = 5
	SECURITY_BUILTIN_DOMAIN_RID = 0x00000020
	DOMAIN_ALIAS_RID_ADMINS     = 0x00000220
)

// SID_IDENTIFIER_AUTHORITY corresponds to Windows SID_IDENTIFIER_AUTHORITY
type SID_IDENTIFIER_AUTHORITY struct {
	Value [6]byte
}

// IsAdmin 检测当前进程是否以管理员权限运行
func IsAdmin() bool {
	authority := SID_IDENTIFIER_AUTHORITY{}
	authority.Value[5] = SECURITY_NT_AUTHORITY

	var sid *syscall.SID

	// AllocateAndInitializeSid
	ret, _, err := procAllocateAndInitializeSid.Call(
		uintptr(unsafe.Pointer(&authority)),
		2,
		SECURITY_BUILTIN_DOMAIN_RID,
		DOMAIN_ALIAS_RID_ADMINS,
		0, 0, 0, 0, 0, 0,
		uintptr(unsafe.Pointer(&sid)),
	)
	if ret == 0 {
		logger.Debug("ICMP", "-", "AllocateAndInitializeSid 失败: %v", err)
		return false
	}
	defer procFreeSid.Call(uintptr(unsafe.Pointer(sid)))

	// CheckTokenMembership
	var isMember int32
	ret, _, err = procCheckTokenMembership.Call(
		0, // 使用当前进程 token
		uintptr(unsafe.Pointer(sid)),
		uintptr(unsafe.Pointer(&isMember)),
	)
	if ret == 0 {
		logger.Debug("ICMP", "-", "CheckTokenMembership 失败: %v", err)
		return false
	}

	return isMember != 0
}

// RequestElevation 请求 UAC 提权，重新以管理员身份启动当前程序
// 调用成功后当前进程会退出，新的管理员进程将启动
func RequestElevation() error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("获取可执行文件路径失败: %w", err)
	}

	verb, _ := syscall.UTF16PtrFromString("runas")
	file, _ := syscall.UTF16PtrFromString(exe)

	logger.Info("ICMP", "-", "正在请求 UAC 提权重新启动程序...")

	ret, _, lastErr := procShellExecuteW.Call(
		0,                              // hwnd
		uintptr(unsafe.Pointer(verb)),  // "runas"
		uintptr(unsafe.Pointer(file)),  // 可执行文件路径
		0,                              // 参数
		0,                              // 工作目录
		1,                              // SW_SHOWNORMAL
	)

	// ShellExecuteW 返回值 > 32 表示成功
	if ret <= 32 {
		return fmt.Errorf("UAC 提权请求失败 (code=%d, err=%v)", ret, lastErr)
	}

	logger.Info("ICMP", "-", "UAC 提权请求已发送，当前进程即将退出")
	os.Exit(0)
	return nil
}
