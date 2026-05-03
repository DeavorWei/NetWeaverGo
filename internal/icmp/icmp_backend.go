package icmp

import "net"

type BackendType int

const (
	BackendAuto       BackendType = iota
	BackendWindowsAPI
	BackendRawSocket
)

type icmpBackend interface {
	pingOne(ip net.IP, timeout uint32, dataSize uint16) (*PingResult, error)
	pingOneWithTTL(ip net.IP, timeout uint32, dataSize uint16, ttl uint8) (*PingResult, error)
	close() error
	name() string
}

var (
	currentBackend icmpBackend
	backendType    BackendType = BackendAuto
)

func PingOne(ip net.IP, timeout uint32, dataSize uint16) (*PingResult, error) {
	return currentBackend.pingOne(ip, timeout, dataSize)
}

func PingOneWithTTL(ip net.IP, timeout uint32, dataSize uint16, ttl uint8) (*PingResult, error) {
	return currentBackend.pingOneWithTTL(ip, timeout, dataSize, ttl)
}

func GetBackend() BackendType {
	return backendType
}

func GetBackendName() string {
	if currentBackend != nil {
		return currentBackend.name()
	}
	return "none"
}

// errorBackend 是当后端初始化失败时的降级后端
// 所有操作都会返回错误，用于程序无法获取管理员权限等场景
type errorBackend struct {
	err string
}

func (b *errorBackend) pingOne(ip net.IP, timeout uint32, dataSize uint16) (*PingResult, error) {
	return &PingResult{
		IP:     ip.String(),
		Status: "Error",
		Error:  b.err,
	}, nil
}

func (b *errorBackend) pingOneWithTTL(ip net.IP, timeout uint32, dataSize uint16, ttl uint8) (*PingResult, error) {
	return &PingResult{
		IP:     ip.String(),
		Status: "Error",
		Error:  b.err,
	}, nil
}

func (b *errorBackend) close() error {
	return nil
}

func (b *errorBackend) name() string {
	return "ErrorBackend(" + b.err + ")"
}
