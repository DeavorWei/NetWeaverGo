package sshutil

import (
	"context"
	"fmt"
	"net"

	"github.com/NetWeaverGo/core/internal/logger"
	"golang.org/x/crypto/ssh"
	"golang.org/x/net/proxy"
)

// DialWithProxy 通过直连或 SOCKS5 代理建立底层 TCP 与 SSH 连接
func DialWithProxy(ctx context.Context, target string, sshConfig *ssh.ClientConfig, proxyAddr string) (*ssh.Client, net.Conn, error) {
	var conn net.Conn
	var err error

	if proxyAddr != "" {
		logger.Verbose("SSH", target, "使用 SOCKS5 代理 %s 进行拨号", proxyAddr)
		dialer, pErr := proxy.SOCKS5("tcp", proxyAddr, nil, proxy.Direct)
		if pErr != nil {
			return nil, nil, fmt.Errorf("创建 SOCKS5 代理拨号器失败: %w", pErr)
		}

		if contextDialer, ok := dialer.(proxy.ContextDialer); ok {
			conn, err = contextDialer.DialContext(ctx, "tcp", target)
		} else {
			conn, err = dialer.Dial("tcp", target)
		}
		if err != nil {
			return nil, nil, fmt.Errorf("SOCKS5 代理连接 %s 失败: %w", target, err)
		}
	} else {
		d := net.Dialer{Timeout: sshConfig.Timeout}
		conn, err = d.DialContext(ctx, "tcp", target)
		if err != nil {
			return nil, nil, fmt.Errorf("TCP 拨号 %s 失败: %w", target, err)
		}
	}

	c, chans, reqs, err := ssh.NewClientConn(conn, target, sshConfig)
	if err != nil {
		conn.Close()
		return nil, nil, fmt.Errorf("SSH 握手失败: %w", err)
	}

	client := ssh.NewClient(c, chans, reqs)
	return client, conn, nil
}
