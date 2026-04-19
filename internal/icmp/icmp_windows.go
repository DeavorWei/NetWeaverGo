//go:build windows

package icmp

import (
	"encoding/binary"
	"fmt"
	"net"
	"syscall"
	"time"
	"unsafe"

	"github.com/NetWeaverGo/core/internal/logger"
)

// Windows ICMP API constants
const (
	IP_SUCCESS             = 0
	IP_BUF_TOO_SMALL       = 11001
	IP_DEST_NET_UNREACHABLE = 11002
	IP_DEST_HOST_UNREACHABLE = 11003
	IP_DEST_PROT_UNREACHABLE = 11004
	IP_DEST_PORT_UNREACHABLE = 11005
	IP_NO_RESOURCES        = 11006
	IP_BAD_OPTION          = 11007
	IP_HW_ERROR            = 11008
	IP_PACKET_TOO_BIG      = 11009
	IP_REQ_TIMED_OUT       = 11010
	IP_BAD_REQ             = 11011
	IP_BAD_ROUTE           = 11012
	IP_TTL_EXPIRED_TRANSIT = 11013
	IP_TTL_EXPIRED_REASSEM = 11014
	IP_PARAM_PROBLEM       = 11015
	IP_SOURCE_QUENCH       = 11016
	IP_OPTION_TOO_BIG      = 11017
	IP_BAD_DESTINATION     = 11018
	IP_GENERAL_FAILURE     = 11050

	// Buffer size constants for IcmpSendEcho
	// Windows IcmpSendEcho requires more buffer space than the theoretical calculation
	// Reference: https://docs.microsoft.com/en-us/windows/win32/api/icmpapi/nf-icmpapi-icmpsendecho
	minBufferSize = 256  // Minimum recommended buffer size
	extraPadding  = 128  // Extra padding for IP headers and internal processing
	alignment     = 8    // 8-byte alignment for Windows API compatibility
)

// IP_OPTION_INFORMATION32 - 32-bit version for 64-bit Windows compatibility
// ⚠️ Critical: Must use 32-bit aligned structure on 64-bit Windows
type IP_OPTION_INFORMATION32 struct {
	TTL         uint8
	Tos         uint8
	Flags       uint8
	OptionsSize uint8
	OptionsData uint32 // Must be uint32, not uintptr
}

// ICMP_ECHO_REPLY - 32-bit version for 64-bit Windows compatibility
// ⚠️ Critical: Must use 32-bit aligned structure on 64-bit Windows
type ICMP_ECHO_REPLY struct {
	Address       uint32 // Network byte order
	Status        uint32
	RoundTripTime uint32
	DataSize      uint16
	Reserved      uint16
	DataPointer   uint32 // Must be uint32, not uintptr
	Options       IP_OPTION_INFORMATION32
}

// Windows DLL and function pointers
var (
	iphlpapi          = syscall.NewLazyDLL("iphlpapi.dll")
	procIcmpCreateFile = iphlpapi.NewProc("IcmpCreateFile")
	procIcmpSendEcho   = iphlpapi.NewProc("IcmpSendEcho")
	procIcmpCloseHandle = iphlpapi.NewProc("IcmpCloseHandle")
)

// icmpStatusToString converts ICMP status code to human-readable string.
func icmpStatusToString(status uint32) string {
	switch status {
	case IP_SUCCESS:
		return "Success"
	case IP_BUF_TOO_SMALL:
		return "Buffer Too Small"
	case IP_DEST_NET_UNREACHABLE:
		return "Destination Network Unreachable"
	case IP_DEST_HOST_UNREACHABLE:
		return "Destination Host Unreachable"
	case IP_DEST_PROT_UNREACHABLE:
		return "Destination Protocol Unreachable"
	case IP_DEST_PORT_UNREACHABLE:
		return "Destination Port Unreachable"
	case IP_NO_RESOURCES:
		return "No Resources"
	case IP_BAD_OPTION:
		return "Bad Option"
	case IP_HW_ERROR:
		return "Hardware Error"
	case IP_PACKET_TOO_BIG:
		return "Packet Too Big"
	case IP_REQ_TIMED_OUT:
		return "Request Timed Out"
	case IP_BAD_REQ:
		return "Bad Request"
	case IP_BAD_ROUTE:
		return "Bad Route"
	case IP_TTL_EXPIRED_TRANSIT:
		return "TTL Expired in Transit"
	case IP_TTL_EXPIRED_REASSEM:
		return "TTL Expired During Reassembly"
	case IP_PARAM_PROBLEM:
		return "Parameter Problem"
	case IP_SOURCE_QUENCH:
		return "Source Quench"
	case IP_OPTION_TOO_BIG:
		return "Option Too Big"
	case IP_BAD_DESTINATION:
		return "Bad Destination"
	case IP_GENERAL_FAILURE:
		return "General Failure"
	default:
		return fmt.Sprintf("Unknown Error (%d)", status)
	}
}

// IcmpCreateFile creates a handle for sending ICMP echo requests.
func IcmpCreateFile() (syscall.Handle, error) {
	logger.Verbose("ICMP", "-", "调用 IcmpCreateFile()")
	ret, _, err := procIcmpCreateFile.Call()
	if ret == uintptr(syscall.InvalidHandle) {
		logger.Debug("ICMP", "-", "IcmpCreateFile 失败: %v", err)
		return syscall.InvalidHandle, err
	}
	logger.Verbose("ICMP", "-", "IcmpCreateFile 成功: handle=%v", ret)
	return syscall.Handle(ret), nil
}

// IcmpCloseHandle closes an ICMP handle.
func IcmpCloseHandle(handle syscall.Handle) error {
	logger.Verbose("ICMP", "-", "调用 IcmpCloseHandle: handle=%v", handle)
	ret, _, _ := procIcmpCloseHandle.Call(uintptr(handle))
	if ret == 0 {
		logger.Debug("ICMP", "-", "IcmpCloseHandle 失败")
		return fmt.Errorf("failed to close ICMP handle")
	}
	logger.Verbose("ICMP", "-", "IcmpCloseHandle 成功")
	return nil
}

// IcmpSendEcho sends an ICMP echo request and returns the reply.
func IcmpSendEcho(handle syscall.Handle, destAddr uint32, sendData []byte, timeout uint32, ttl uint8) (*ICMP_ECHO_REPLY, []byte, error) {
	logger.Verbose("ICMP", "-", "调用 IcmpSendEcho: destAddr=%08x, dataSize=%d, timeout=%dms, ttl=%d",
		destAddr, len(sendData), timeout, ttl)

	// Prepare options
	options := IP_OPTION_INFORMATION32{
		TTL:         ttl,
		Tos:         0,
		Flags:       0,
		OptionsSize: 0,
		OptionsData: 0,
	}

	// Calculate buffer size with extra padding and alignment
	// Windows IcmpSendEcho requires more buffer space than the theoretical calculation
	// Reference: https://docs.microsoft.com/en-us/windows/win32/api/icmpapi/nf-icmpapi-icmpsendecho

	// Calculate base size: reply structure + data + ICMP header overhead (8 bytes)
	baseSize := uint32(unsafe.Sizeof(ICMP_ECHO_REPLY{})) + uint32(len(sendData)) + 8

	// Add extra padding for Windows internal processing (IP headers, error messages, etc.)
	calculatedSize := baseSize + extraPadding

	// Align to 8-byte boundary for Windows API compatibility
	alignedSize := (calculatedSize + alignment - 1) &^ (alignment - 1)

	// Ensure minimum buffer size
	replySize := alignedSize
	if replySize < minBufferSize {
		replySize = minBufferSize
	}
	replyBuffer := make([]byte, replySize)

	logger.Verbose("ICMP", "-", "IcmpSendEcho 缓冲区计算: baseSize=%d (结构体=%d + 数据=%d + ICMP头=8), extraPadding=%d, alignedSize=%d, finalSize=%d",
		baseSize, unsafe.Sizeof(ICMP_ECHO_REPLY{}), len(sendData), extraPadding, alignedSize, replySize)

	// Send ICMP echo request
	ret, _, err := procIcmpSendEcho.Call(
		uintptr(handle),
		uintptr(destAddr),
		uintptr(unsafe.Pointer(&sendData[0])),
		uintptr(len(sendData)),
		uintptr(unsafe.Pointer(&options)),
		uintptr(unsafe.Pointer(&replyBuffer[0])),
		uintptr(replySize),
		uintptr(timeout),
	)

	logger.Debug("ICMP", "-", "IcmpSendEcho 返回: ret=%d, err=%v", ret, err)

	if ret == 0 {
		// Even when ret == 0, we may have reply data with error status
		// Parse reply buffer to get detailed error status
		if len(replyBuffer) >= int(unsafe.Sizeof(ICMP_ECHO_REPLY{})) {
			reply := (*ICMP_ECHO_REPLY)(unsafe.Pointer(&replyBuffer[0]))
			statusStr := icmpStatusToString(reply.Status)
			logger.Debug("ICMP", "-", "IcmpSendEcho 失败: status=%d(%s)", reply.Status, statusStr)
			return nil, nil, fmt.Errorf("ICMP error: %s (code: %d)", statusStr, reply.Status)
		}
		// No valid reply data, return syscall error
		if err != nil {
			logger.Debug("ICMP", "-", "IcmpSendEcho 无响应或错误: err=%v", err)
			return nil, nil, fmt.Errorf("ICMP send failed: %w", err)
		}
		logger.Debug("ICMP", "-", "IcmpSendEcho 无响应")
		return nil, nil, fmt.Errorf("ICMP send failed: no reply")
	}

	// Parse reply
	reply := (*ICMP_ECHO_REPLY)(unsafe.Pointer(&replyBuffer[0]))
	// 将 reply.Address 转换为可读 IP 用于日志
	// reply.Address 是 in_addr 结构，在 x86 小端序上 Go 读取后为小端序 uint32，
	// 需要用 LittleEndian 反向解析回 IP 字节
	replyIP := make(net.IP, 4)
	binary.LittleEndian.PutUint32(replyIP, reply.Address)
	logger.Debug("ICMP", "-", "IcmpSendEcho 响应: status=%d(%s), rtt=%dms, ttl=%d, replyAddr=%s (%08x)",
		reply.Status, icmpStatusToString(reply.Status), reply.RoundTripTime, reply.Options.TTL, replyIP.String(), reply.Address)

	// Extract reply data
	var replyData []byte
	if reply.DataSize > 0 && reply.DataPointer != 0 {
		dataOffset := reply.DataPointer - uint32(uintptr(unsafe.Pointer(&replyBuffer[0])))
		if dataOffset+uint32(reply.DataSize) <= uint32(len(replyBuffer)) {
			replyData = replyBuffer[dataOffset : dataOffset+uint32(reply.DataSize)]
		}
	}

	return reply, replyData, nil
}

// prepareSendData 准备 ICMP 发送数据
// 使用固定填充模式 + 时间戳，模拟标准 ping 行为
func prepareSendData(dataSize uint16) []byte {
	sendData := make([]byte, dataSize)

	if dataSize == 0 {
		return sendData
	}

	// 前 8 字节存储时间戳（用于验证响应）
	timestamp := time.Now().UnixNano()
	for i := 0; i < 8 && i < int(dataSize); i++ {
		sendData[i] = byte(timestamp >> (i * 8))
	}

	// 剩余部分使用固定填充模式 (模拟 Windows ping 的 ABCD... 模式)
	pattern := []byte("ABCDEFGHIJKLMNOPQRSTUVWXYZ")
	for i := 8; i < int(dataSize); i++ {
		sendData[i] = pattern[(i-8)%len(pattern)]
	}

	return sendData
}

// PingOne performs a single ICMP echo request to the specified IP address.
func PingOne(ip net.IP, timeout uint32, dataSize uint16) (*PingResult, error) {
	// 防止 dataSize=0 导致 prepareSendData 返回空切片，进而引发 IcmpSendEcho panic
	if dataSize == 0 {
		dataSize = 32
	}

	ipStr := ip.String()
	logger.Verbose("ICMP", ipStr, "开始 Ping: timeout=%dms, dataSize=%d", timeout, dataSize)

	// Convert IP to 4-byte representation
	ip = ip.To4()
	if ip == nil {
		logger.Error("ICMP", ipStr, "无效的 IPv4 地址")
		return nil, fmt.Errorf("invalid IPv4 address")
	}

	// Create ICMP handle
	handle, err := IcmpCreateFile()
	if err != nil {
		logger.Error("ICMP", ipStr, "创建 ICMP 句柄失败: %v", err)
		return nil, fmt.Errorf("failed to create ICMP handle: %w", err)
	}
	defer IcmpCloseHandle(handle)

	// Prepare send data
	sendData := prepareSendData(dataSize)
	logger.Verbose("ICMP", ipStr, "准备发送数据: size=%d", len(sendData))

	// Convert IP to uint32 in the format expected by Windows IcmpSendEcho.
	// IcmpSendEcho's DestinationAddress parameter is an in_addr structure (network byte order),
	// but when passed as a uintptr on little-endian x86, the bytes in memory must be
	// laid out as [byte0, byte1, byte2, byte3] where the IP is byte0.byte1.byte2.byte3.
	// Using LittleEndian.Uint32 achieves this correct memory layout on x86.
	destAddr := binary.LittleEndian.Uint32(ip)

	// Send ICMP echo request with default TTL of 128
	logger.Debug("ICMP", ipStr, "发送 ICMP 请求: destAddr=%08x, timeout=%dms, ttl=128", destAddr, timeout)
	reply, _, err := IcmpSendEcho(handle, destAddr, sendData, timeout, 128)

	result := &PingResult{
		IP: ip.String(),
	}

	// Handle IcmpSendEcho errors
	if err != nil {
		result.Success = false
		result.Status = "Error"
		result.Error = err.Error()
		logger.Info("ICMP", ipStr, "Ping 失败 (API错误): status=%s, error=%s", result.Status, result.Error)
		return result, nil
	}

	// Handle nil reply (should not happen if err is nil, but just in case)
	if reply == nil {
		result.Success = false
		result.Status = "No Reply"
		result.Error = "No reply received"
		logger.Info("ICMP", ipStr, "Ping 失败 (无响应): status=%s, error=%s", result.Status, result.Error)
		return result, nil
	}

	// Extract reply data
	result.RoundTripTime = float64(reply.RoundTripTime)
	result.TTL = reply.Options.TTL

	// Check reply status
	if reply.Status == IP_SUCCESS {
		// 校验响应地址是否与请求目标匹配
		// 修复高并发场景下的响应交叉投递问题
		if reply.Address != destAddr {
			replyIP := make(net.IP, 4)
			binary.LittleEndian.PutUint32(replyIP, reply.Address)
			result.Success = false
			result.Status = "Address Mismatch"
			result.Error = fmt.Sprintf("响应地址不匹配: 期望 %s, 实际 %s", ip.String(), replyIP.String())
			logger.Warn("ICMP", ipStr, "Ping 响应地址不匹配: expected=%s, actual=%s (%08x), rtt=%dms",
				ip.String(), replyIP.String(), reply.Address, reply.RoundTripTime)
			return result, nil
		}
		result.Success = true
		result.Status = "Success"
		logger.Info("ICMP", ipStr, "Ping 成功: rtt=%.2fms, ttl=%d", result.RoundTripTime, result.TTL)
	} else {
		result.Success = false
		result.Status = icmpStatusToString(reply.Status)
		// Critical fix: Always set Error field with status description
		result.Error = result.Status
		logger.Info("ICMP", ipStr, "Ping 失败 (状态码错误): status=%s, code=%d", result.Status, reply.Status)
	}

	return result, nil
}

// PingOneWithTTL performs a single ICMP echo request with specified TTL.
func PingOneWithTTL(ip net.IP, timeout uint32, dataSize uint16, ttl uint8) (*PingResult, error) {
	// 防止 dataSize=0 导致 prepareSendData 返回空切片，进而引发 IcmpSendEcho panic
	if dataSize == 0 {
		dataSize = 32
	}

	ipStr := ip.String()
	logger.Verbose("ICMP", ipStr, "开始 Ping (带TTL): timeout=%dms, dataSize=%d, ttl=%d", timeout, dataSize, ttl)

	// Convert IP to 4-byte representation
	ip = ip.To4()
	if ip == nil {
		logger.Error("ICMP", ipStr, "无效的 IPv4 地址")
		return nil, fmt.Errorf("invalid IPv4 address")
	}

	// Create ICMP handle
	handle, err := IcmpCreateFile()
	if err != nil {
		logger.Error("ICMP", ipStr, "创建 ICMP 句柄失败: %v", err)
		return nil, fmt.Errorf("failed to create ICMP handle: %w", err)
	}
	defer IcmpCloseHandle(handle)

	// Prepare send data
	sendData := prepareSendData(dataSize)

	// Convert IP to uint32 in the format expected by Windows IcmpSendEcho.
	// IcmpSendEcho's DestinationAddress parameter is an in_addr structure (network byte order),
	// but when passed as a uintptr on little-endian x86, the bytes in memory must be
	// laid out as [byte0, byte1, byte2, byte3] where the IP is byte0.byte1.byte2.byte3.
	// Using LittleEndian.Uint32 achieves this correct memory layout on x86.
	destAddr := binary.LittleEndian.Uint32(ip)

	// Send ICMP echo request
	logger.Debug("ICMP", ipStr, "发送 ICMP 请求: destAddr=%08x, timeout=%dms, ttl=%d", destAddr, timeout, ttl)
	reply, _, err := IcmpSendEcho(handle, destAddr, sendData, timeout, ttl)

	result := &PingResult{
		IP: ip.String(),
	}

	// Handle IcmpSendEcho errors
	if err != nil {
		result.Success = false
		result.Status = "Error"
		result.Error = err.Error()
		logger.Info("ICMP", ipStr, "Ping 失败 (API错误): status=%s, error=%s", result.Status, result.Error)
		return result, nil
	}

	// Handle nil reply
	if reply == nil {
		result.Success = false
		result.Status = "No Reply"
		result.Error = "No reply received"
		logger.Info("ICMP", ipStr, "Ping 失败 (无响应): status=%s, error=%s", result.Status, result.Error)
		return result, nil
	}

	// Extract reply data
	result.RoundTripTime = float64(reply.RoundTripTime)
	result.TTL = reply.Options.TTL

	// Check reply status
	if reply.Status == IP_SUCCESS {
		// 校验响应地址是否与请求目标匹配
		// 修复高并发场景下的响应交叉投递问题
		if reply.Address != destAddr {
			replyIP := make(net.IP, 4)
			binary.LittleEndian.PutUint32(replyIP, reply.Address)
			result.Success = false
			result.Status = "Address Mismatch"
			result.Error = fmt.Sprintf("响应地址不匹配: 期望 %s, 实际 %s", ip.String(), replyIP.String())
			logger.Warn("ICMP", ipStr, "Ping 响应地址不匹配: expected=%s, actual=%s (%08x), rtt=%dms",
				ip.String(), replyIP.String(), reply.Address, reply.RoundTripTime)
			return result, nil
		}
		result.Success = true
		result.Status = "Success"
		logger.Info("ICMP", ipStr, "Ping 成功: rtt=%.2fms, ttl=%d", result.RoundTripTime, result.TTL)
	} else {
		result.Success = false
		result.Status = icmpStatusToString(reply.Status)
		// Critical fix: Always set Error field with status description
		result.Error = result.Status
		logger.Info("ICMP", ipStr, "Ping 失败 (状态码错误): status=%s, code=%d", result.Status, reply.Status)
	}

	return result, nil
}
