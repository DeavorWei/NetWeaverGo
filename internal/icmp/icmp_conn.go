//go:build !rawicmp

package icmp

import (
	"fmt"
	"sync"

	"golang.org/x/net/icmp"

	"github.com/NetWeaverGo/core/internal/logger"
)

type connManager struct {
	mu   sync.Mutex
	conn *icmp.PacketConn
}

func (m *connManager) getOrCreate() (*icmp.PacketConn, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.conn != nil {
		return m.conn, nil
	}
	c, err := icmp.ListenPacket("ip4:icmp", "0.0.0.0")
	if err != nil {
		return nil, fmt.Errorf("failed to listen on ip4:icmp: %w", err)
	}
	m.conn = c
	logger.Info("ICMP", "-", "ICMP raw socket 连接创建成功")
	return c, nil
}

func (m *connManager) close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.conn == nil {
		return nil
	}
	err := m.conn.Close()
	m.conn = nil
	logger.Info("ICMP", "-", "ICMP raw socket 连接已关闭")
	return err
}

func (m *connManager) reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.conn != nil {
		m.conn.Close()
		m.conn = nil
	}
	logger.Info("ICMP", "-", "ICMP raw socket 连接已重置")
}
