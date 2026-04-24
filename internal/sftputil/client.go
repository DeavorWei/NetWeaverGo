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
	client    *sftp.Client
	sshClient *sshutil.SSHClient // 保存底层 SSH 连接，以便 Close 时一并释放
	ip        string
}

// NewSFTPClient 在一个全新的 SSH 连接上初始化 SFTP 连接。
// 注意：许多网络设备 (华为/华三) 拒绝在
// 已经有活跃的 "shell" 或 "pty" 通道的 SSH 连接上打开 "sftp" 子系统通道。
// 因此，我们必须专门为 SFTP 创建一个全新的 SSH 连接。
func NewSFTPClient(ctx context.Context, cfg sshutil.Config) (*SFTPClient, error) {
	logger.Debug("SFTP", cfg.IP, "开始初始化专门的 SFTP 连接")
	// 1. 创建一个专用的原始 SSH 连接，不请求 PTY/Shell
	sshClient, err := sshutil.NewRawSSHClient(ctx, cfg)
	if err != nil {
		logger.Debug("SFTP", cfg.IP, "原始 SSH 底层建连失败: %v", err)
		return nil, fmt.Errorf("SFTP专属SSH建连失败: %w", err)
	}

	// 2. 在这个干净的连接上初始化 SFTP 子系统
	client, err := sftp.NewClient(sshClient.Client)
	if err != nil {
		logger.Debug("SFTP", cfg.IP, "sftp.NewClient 初始化失败: %v", err)
		sshClient.Close()
		return nil, fmt.Errorf("创建 SFTP 客户端失败: %w", err)
	}
	logger.Debug("SFTP", cfg.IP, "SFTP 子系统挂载成功")

	return &SFTPClient{
		client:    client,
		sshClient: sshClient, // 保存引用
		ip:        cfg.IP,
	}, nil
}

// DownloadFile 从远程路径下载文件到本地路径。
// 如果本地父目录不存在，将会自动创建。
// 采用原子写入策略：先写入临时文件，下载成功后重命名为目标文件，
// 避免下载中断时留下不完整文件。
func (s *SFTPClient) DownloadFile(remotePath, localPath string) error {
	logger.Debug("SFTP", s.ip, "开始下载文件 %s -> %s", remotePath, localPath)
	remoteFile, err := s.client.Open(remotePath)
	if err != nil {
		logger.Debug("SFTP", s.ip, "打开远端文件 %s 失败: %v", remotePath, err)
		return fmt.Errorf("打开远端文件 %s 失败: %w", remotePath, err)
	}
	defer remoteFile.Close()

	localDir := filepath.Dir(localPath)
	if err := os.MkdirAll(localDir, 0755); err != nil {
		return fmt.Errorf("创建本地目录 %s 失败: %w", localDir, err)
	}

	// 原子写入：先写临时文件，成功后重命名
	tmpPath := localPath + ".tmp"
	localFile, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("创建临时文件 %s 失败: %w", tmpPath, err)
	}

	bytesCopied, err := io.Copy(localFile, remoteFile)
	// 无论 io.Copy 是否成功，都必须先关闭文件，否则 os.Rename 在 Windows 上可能失败
	if closeErr := localFile.Close(); closeErr != nil && err == nil {
		err = closeErr
	}

	if err != nil {
		// 下载失败：清理临时文件，避免留下不完整内容
		if removeErr := os.Remove(tmpPath); removeErr != nil && !os.IsNotExist(removeErr) {
			logger.Warn("SFTP", s.ip, "清理临时文件失败: %v", removeErr)
		}
		return fmt.Errorf("下载文件失败 (已复制 %d 字节): %w", bytesCopied, err)
	}

	// 下载成功：校验文件非空（空文件通常意味着远端路径错误或设备配置异常）
	if bytesCopied == 0 {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("下载文件为空: %s", remotePath)
	}

	// 原子重命名：在同一文件系统上这是原子操作
	if err := os.Rename(tmpPath, localPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("重命名临时文件到目标文件失败: %w", err)
	}

	logger.Info("SFTP", s.ip, "成功下载文件: %s -> %s (大小: %d 字节)", remotePath, localPath, bytesCopied)
	return nil
}

// Close 关闭 SFTP 客户端及其底层 SSH 连接，防止连接泄漏
func (s *SFTPClient) Close() error {
	logger.Verbose("SFTP", s.ip, "正在关闭 SFTP Client 及底层 SSH 连接")
	var firstErr error

	// 先关闭 SFTP 协议层
	if s.client != nil {
		if err := s.client.Close(); err != nil {
			firstErr = err
		}
	}

	// 再关闭底层 SSH 连接（修复：原实现遗漏此步骤导致连接泄漏）
	if s.sshClient != nil {
		if err := s.sshClient.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	return firstErr
}
