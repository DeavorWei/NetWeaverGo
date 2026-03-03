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

// SFTPClient wraps the pkg/sftp client
type SFTPClient struct {
	client *sftp.Client
	ip     string
}

// NewSFTPClient initializes an SFTP connection over a fresh SSH connection.
// Note: Many network devices (Huawei/H3C) reject opening an "sftp" subsystem channel
// on an SSH connection that already has an active "shell" or "pty" channel.
// Therefore, we must create a completely new SSH connection specifically for SFTP.
func NewSFTPClient(ctx context.Context, cfg sshutil.Config) (*SFTPClient, error) {
	// 1. Create a dedicated raw SSH connection without requesting PTY/Shell
	sshClient, err := sshutil.NewRawSSHClient(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("SFTP专属SSH建连失败: %w", err)
	}

	// 2. Initialize the SFTP subsystem on this clean connection
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

// DownloadFile downloads a file from the remote path to the local path.
// It creates the local parent directories if they don't exist.
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

// Close closes the SFTP client
func (s *SFTPClient) Close() error {
	if s.client != nil {
		return s.client.Close()
	}
	return nil
}
