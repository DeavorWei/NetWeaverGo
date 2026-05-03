//go:build !rawicmp

package icmp

import (
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"sync/atomic"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"

	"github.com/NetWeaverGo/core/internal/logger"
)

const (
	protocolICMP   = 1
	maxMessageSize = 1500
)

type rawSocketBackend struct{}

var globalSeq atomic.Uint32

func nextSeq() int {
	return int(globalSeq.Add(1))
}

func icmpID() int {
	return os.Getpid() & 0xffff
}

func initRawSocketBackend() (icmpBackend, error) {
	conn, err := icmp.ListenPacket("ip4:icmp", "0.0.0.0")
	if err != nil {
		return nil, fmt.Errorf("failed to listen on ip4:icmp (需要管理员/root 权限): %w", err)
	}
	conn.Close()
	return &rawSocketBackend{}, nil
}

func (b *rawSocketBackend) pingOne(ip net.IP, timeout uint32, dataSize uint16) (*PingResult, error) {
	return b.pingOneRaw(ip, timeout, dataSize, 128)
}

func (b *rawSocketBackend) pingOneWithTTL(ip net.IP, timeout uint32, dataSize uint16, ttl uint8) (*PingResult, error) {
	return b.pingOneRaw(ip, timeout, dataSize, ttl)
}

func (b *rawSocketBackend) close() error {
	return nil
}

func (b *rawSocketBackend) name() string {
	return "RawSocket(golang.org/x/net/icmp)"
}

func prepareSendDataRaw(dataSize uint16) []byte {
	sendData := make([]byte, dataSize)
	if dataSize == 0 {
		return sendData
	}
	timestamp := time.Now().UnixNano()
	for i := 0; i < 8 && i < int(dataSize); i++ {
		sendData[i] = byte(timestamp >> (i * 8))
	}
	pattern := []byte("ABCDEFGHIJKLMNOPQRSTUVWXYZ")
	for i := 8; i < int(dataSize); i++ {
		sendData[i] = pattern[(i-8)%len(pattern)]
	}
	return sendData
}

func (b *rawSocketBackend) pingOneRaw(ip net.IP, timeout uint32, dataSize uint16, ttl uint8) (*PingResult, error) {
	if dataSize == 0 {
		dataSize = 32
	}

	ipStr := ip.String()
	ip = ip.To4()
	if ip == nil {
		return nil, fmt.Errorf("invalid IPv4 address")
	}

	conn, err := icmp.ListenPacket("ip4:icmp", "0.0.0.0")
	if err != nil {
		return nil, fmt.Errorf("failed to listen on ip4:icmp: %w", err)
	}
	defer conn.Close()

	pconn := conn.IPv4PacketConn()
	if err := pconn.SetTTL(int(ttl)); err != nil {
		return nil, fmt.Errorf("failed to set TTL: %w", err)
	}

	seq := nextSeq()
	msg := icmp.Message{
		Type: ipv4.ICMPTypeEcho,
		Code: 0,
		Body: &icmp.Echo{
			ID:   icmpID(),
			Seq:  seq,
			Data: prepareSendDataRaw(dataSize),
		},
	}
	wb, err := msg.Marshal(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal ICMP message: %w", err)
	}

	dst := &net.IPAddr{IP: ip}
	sendTime := time.Now()
	if _, err := conn.WriteTo(wb, dst); err != nil {
		return nil, fmt.Errorf("failed to send ICMP: %w", err)
	}

	deadline := sendTime.Add(time.Duration(timeout) * time.Millisecond)
	conn.SetReadDeadline(deadline)

	rb := make([]byte, maxMessageSize)
	for {
		n, peer, err := conn.ReadFrom(rb)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				return &PingResult{
					IP:     "*",
					Status: "Request Timed Out",
					Error:  "Request Timed Out",
				}, nil
			}
			return &PingResult{
				IP:     "*",
				Status: "Error",
				Error:  err.Error(),
			}, nil
		}

		rm, err := icmp.ParseMessage(protocolICMP, rb[:n])
		if err != nil {
			continue
		}

		switch rm.Type {
		case ipv4.ICMPTypeEchoReply:
			reply, ok := rm.Body.(*icmp.Echo)
			if !ok || reply.ID != icmpID() || reply.Seq != seq {
				continue
			}
			rtt := time.Since(sendTime).Milliseconds()
			replyIP := extractIP(peer)

			logger.Info("ICMP", ipStr, "Ping 成功 (raw): rtt=%dms, ttl=%d, replyIP=%s", rtt, ttl, replyIP)

			return &PingResult{
				IP:            replyIP,
				Success:       true,
				RoundTripTime: float64(rtt),
				TTL:           ttl,
				Status:        "Success",
			}, nil

		case ipv4.ICMPTypeTimeExceeded:
			if !matchTimeExceeded(rm, icmpID(), seq) {
				continue
			}
			rtt := time.Since(sendTime).Milliseconds()
			replyIP := extractIP(peer)

			logger.Info("ICMP", ipStr, "TTL 过期 (raw): 中间路由器=%s, rtt=%dms", replyIP, rtt)

			return &PingResult{
				IP:            replyIP,
				Success:       false,
				RoundTripTime: float64(rtt),
				TTL:           ttl,
				Status:        "TTLExpired",
			}, nil

		case ipv4.ICMPTypeDestinationUnreachable:
			if !matchDestUnreachable(rm, icmpID(), seq) {
				continue
			}
			return &PingResult{
				IP:     "*",
				Status: "Destination Host Unreachable",
				Error:  "Destination Host Unreachable",
			}, nil

		default:
			continue
		}
	}
}

func extractIP(peer net.Addr) string {
	s := peer.String()
	if addr, _, err := net.SplitHostPort(s); err == nil {
		return addr
	}
	return s
}

func matchTimeExceeded(rm *icmp.Message, expectID, expectSeq int) bool {
	body, ok := rm.Body.(*icmp.TimeExceeded)
	if !ok {
		return false
	}
	if len(body.Data) >= 28 {
		origID := int(binary.BigEndian.Uint16(body.Data[24:26]))
		origSeq := int(binary.BigEndian.Uint16(body.Data[26:28]))
		return origID == expectID && origSeq == expectSeq
	}
	return false
}

func matchDestUnreachable(rm *icmp.Message, expectID, expectSeq int) bool {
	body, ok := rm.Body.(*icmp.DstUnreach)
	if !ok {
		return false
	}
	if len(body.Data) >= 28 {
		origID := int(binary.BigEndian.Uint16(body.Data[24:26]))
		origSeq := int(binary.BigEndian.Uint16(body.Data[26:28]))
		return origID == expectID && origSeq == expectSeq
	}
	return false
}
