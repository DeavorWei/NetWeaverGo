package sftputil

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/sshutil"
	"github.com/pkg/sftp"
)

// SFTPClient 包装了 pkg/sftp 客户端
type SFTPClient struct {
	client *sftp.Client
	ip     string
}

// NewSFTPClient 在一个全新的 SSH 连接上初始化 SFTP 连接。
// 注意：许多网络设备 (华为/华三) 拒绝在
// 已经有活跃的 "shell" 或 "pty" 通道的 SSH 连接上打开 "sftp" 子系统通道。
// 因此，我们必须专门为 SFTP 创建一个全新的 SSH 连接。
func NewSFTPClient(ctx context.Context, cfg sshutil.Config) (*SFTPClient, error) {
	// 1. 创建一个专用的原始 SSH 连接，不请求 PTY/Shell
	sshClient, err := sshutil.NewRawSSHClient(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("SFTP专属SSH建连失败: %w", err)
	}

	// 2. 在这个干净的连接上初始化 SFTP 子系统
	client, err := sftp.NewClient(sshClient.Client)
	if err != nil {
		sshClient.Close()
		return nil, fmt.Errorf("failed to create sftp client: %w", err)
	}

	return &SFTPClient{
		client: client,
		ip:     cfg.IP,
	}, nil
}

// DownloadFile 从远程路径下载文件到本地路径。
// 如果本地父目录不存在，将会自动创建。
func (s *SFTPClient) DownloadFile(remotePath, localPath string) error {
	remoteFile, err := s.client.Open(remotePath)
	if err != nil {
		return fmt.Errorf("failed to open remote file %s: %w", remotePath, err)
	}
	defer remoteFile.Close()

	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return fmt.Errorf("failed to create local directory for %s: %w", localPath, err)
	}

	localFile, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("failed to create local file %s: %w", localPath, err)
	}
	defer localFile.Close()

	bytesCopied, err := io.Copy(localFile, remoteFile)
	if err != nil {
		return fmt.Errorf("failed to download file (copied %d bytes): %w", bytesCopied, err)
	}

	logger.Info("[%s] 成功下载文件: %s -> %s (大小: %d 字节)", s.ip, remotePath, localPath, bytesCopied)
	return nil
}

// Close 关闭 SFTP 客户端
func (s *SFTPClient) Close() error {
	if s.client != nil {
		return s.client.Close()
	}
	return nil
}
