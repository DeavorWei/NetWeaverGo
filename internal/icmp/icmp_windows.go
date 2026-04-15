//go:build windows

package icmp

import (
	"encoding/binary"
	"fmt"
	"net"
	"syscall"
	"unsafe"
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
	ret, _, err := procIcmpCreateFile.Call()
	if ret == uintptr(syscall.InvalidHandle) {
		return syscall.InvalidHandle, err
	}
	return syscall.Handle(ret), nil
}

// IcmpCloseHandle closes an ICMP handle.
func IcmpCloseHandle(handle syscall.Handle) error {
	ret, _, _ := procIcmpCloseHandle.Call(uintptr(handle))
	if ret == 0 {
		return fmt.Errorf("failed to close ICMP handle")
	}
	return nil
}

// IcmpSendEcho sends an ICMP echo request and returns the reply.
func IcmpSendEcho(handle syscall.Handle, destAddr uint32, sendData []byte, timeout uint32, ttl uint8) (*ICMP_ECHO_REPLY, []byte, error) {
	// Prepare options
	options := IP_OPTION_INFORMATION32{
		TTL:         ttl,
		Tos:         0,
		Flags:       0,
		OptionsSize: 0,
		OptionsData: 0,
	}

	// Calculate buffer size: reply structure + data
	replySize := uint32(unsafe.Sizeof(ICMP_ECHO_REPLY{})) + uint32(len(sendData))
	replyBuffer := make([]byte, replySize)

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

	if ret == 0 {
		// Check for timeout or other errors
		return nil, nil, err
	}

	// Parse reply
	reply := (*ICMP_ECHO_REPLY)(unsafe.Pointer(&replyBuffer[0]))

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

// PingOne performs a single ICMP echo request to the specified IP address.
func PingOne(ip net.IP, timeout uint32, dataSize uint16) (*PingResult, error) {
	// Convert IP to 4-byte representation
	ip = ip.To4()
	if ip == nil {
		return nil, fmt.Errorf("invalid IPv4 address")
	}

	// Create ICMP handle
	handle, err := IcmpCreateFile()
	if err != nil {
		return nil, fmt.Errorf("failed to create ICMP handle: %w", err)
	}
	defer IcmpCloseHandle(handle)

	// Prepare send data
	sendData := make([]byte, dataSize)
	for i := range sendData {
		sendData[i] = byte(i % 256)
	}

	// Convert IP to network byte order (uint32)
	destAddr := binary.BigEndian.Uint32(ip)

	// Send ICMP echo request with default TTL of 128
	reply, _, err := IcmpSendEcho(handle, destAddr, sendData, timeout, 128)

	result := &PingResult{
		IP: ip.String(),
	}

	if err != nil {
		result.Success = false
		result.Status = "Error"
		result.Error = err.Error()
		return result, nil
	}

	if reply == nil {
		result.Success = false
		result.Status = "No Reply"
		result.Error = "No reply received"
		return result, nil
	}

	result.RoundTripTime = reply.RoundTripTime
	result.TTL = reply.Options.TTL

	if reply.Status == IP_SUCCESS {
		result.Success = true
		result.Status = "Success"
	} else {
		result.Success = false
		result.Status = icmpStatusToString(reply.Status)
		result.Error = result.Status
	}

	return result, nil
}

// PingOneWithTTL performs a single ICMP echo request with specified TTL.
func PingOneWithTTL(ip net.IP, timeout uint32, dataSize uint16, ttl uint8) (*PingResult, error) {
	// Convert IP to 4-byte representation
	ip = ip.To4()
	if ip == nil {
		return nil, fmt.Errorf("invalid IPv4 address")
	}

	// Create ICMP handle
	handle, err := IcmpCreateFile()
	if err != nil {
		return nil, fmt.Errorf("failed to create ICMP handle: %w", err)
	}
	defer IcmpCloseHandle(handle)

	// Prepare send data
	sendData := make([]byte, dataSize)
	for i := range sendData {
		sendData[i] = byte(i % 256)
	}

	// Convert IP to network byte order (uint32)
	destAddr := binary.BigEndian.Uint32(ip)

	// Send ICMP echo request
	reply, _, err := IcmpSendEcho(handle, destAddr, sendData, timeout, ttl)

	result := &PingResult{
		IP: ip.String(),
	}

	if err != nil {
		result.Success = false
		result.Status = "Error"
		result.Error = err.Error()
		return result, nil
	}

	if reply == nil {
		result.Success = false
		result.Status = "No Reply"
		result.Error = "No reply received"
		return result, nil
	}

	result.RoundTripTime = reply.RoundTripTime
	result.TTL = reply.Options.TTL

	if reply.Status == IP_SUCCESS {
		result.Success = true
		result.Status = "Success"
	} else {
		result.Success = false
		result.Status = icmpStatusToString(reply.Status)
		result.Error = result.Status
	}

	return result, nil
}
