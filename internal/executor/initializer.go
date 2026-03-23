package executor

import (
	"context"
	"fmt"
	"time"

	"github.com/NetWeaverGo/core/internal/config"
	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/matcher"
	"github.com/NetWeaverGo/core/internal/sshutil"
)

// Initializer 会话初始化器
type Initializer struct {
	profile *config.DeviceProfile
	matcher *matcher.StreamMatcher
}

// PromptTracker 初始化阶段的最小提示符跟踪接口
type PromptTracker interface {
	TrackChunk(chunk string)
	ActiveLine() string
	Lines() []string
}

// NewInitializer 创建初始化器
func NewInitializer(profile *config.DeviceProfile) *Initializer {
	if profile == nil {
		// 使用默认画像
		profile = config.GetDeviceProfile("huawei")
	}

	// 创建匹配器并配置
	m := matcher.NewStreamMatcher()
	m.ConfigureFromProfile(
		profile.Prompt.Suffixes,
		profile.Prompt.Patterns,
		profile.Pager.Patterns,
	)

	return &Initializer{
		profile: profile,
		matcher: m,
	}
}

// NewInitializerWithMatcher 创建初始化器（使用现有匹配器）
func NewInitializerWithMatcher(profile *config.DeviceProfile, m *matcher.StreamMatcher) *Initializer {
	if profile == nil {
		profile = config.GetDeviceProfile("huawei")
	}

	// 配置匹配器
	m.ConfigureFromProfile(
		profile.Prompt.Suffixes,
		profile.Prompt.Patterns,
		profile.Pager.Patterns,
	)

	return &Initializer{
		profile: profile,
		matcher: m,
	}
}

// GetProfile 获取设备画像
func (i *Initializer) GetProfile() *config.DeviceProfile {
	return i.profile
}

// GetMatcher 获取匹配器
func (i *Initializer) GetMatcher() *matcher.StreamMatcher {
	return i.matcher
}

// InitResult 初始化结果
type InitResult struct {
	Success       bool
	ErrorMessage  string
	PromptFound   bool
	PagerDisabled bool
	Duration      time.Duration
}

// Run 执行会话初始化流程
// 流程：
// 1. 等待首个稳定 prompt
// 2. 发送预热空行
// 3. 再次等待 prompt
// 4. 发送禁分页命令
// 5. 等待 prompt 恢复
func (i *Initializer) Run(ctx context.Context, client *sshutil.SSHClient, tracker PromptTracker) error {
	result := i.RunWithResult(ctx, client, tracker)
	if !result.Success {
		return fmt.Errorf("初始化失败: %s", result.ErrorMessage)
	}
	return nil
}

// RunWithResult 执行会话初始化流程并返回详细结果
func (i *Initializer) RunWithResult(ctx context.Context, client *sshutil.SSHClient, tracker PromptTracker) *InitResult {
	start := time.Now()
	result := &InitResult{
		Success: false,
	}

	// 获取超时配置
	promptTimeout := time.Duration(i.profile.Init.PromptTimeoutSec) * time.Second
	if promptTimeout == 0 {
		promptTimeout = 30 * time.Second
	}

	// 步骤 1: 等待首个稳定 prompt
	logger.Verbose("Initializer", client.IP, "步骤1: 等待首个提示符...")
	if !i.waitForPrompt(ctx, client, tracker, promptTimeout) {
		result.ErrorMessage = "等待首个提示符超时"
		result.Duration = time.Since(start)
		return result
	}
	result.PromptFound = true
	logger.Verbose("Initializer", client.IP, "步骤1完成: 检测到提示符")

	// 步骤 2: 发送预热空行
	logger.Verbose("Initializer", client.IP, "步骤2: 发送预热空行...")
	if err := client.SendCommand(""); err != nil {
		result.ErrorMessage = fmt.Sprintf("发送预热空行失败: %v", err)
		result.Duration = time.Since(start)
		return result
	}

	// 步骤 3: 再次等待 prompt
	logger.Verbose("Initializer", client.IP, "步骤3: 等待提示符恢复...")
	if !i.waitForPrompt(ctx, client, tracker, promptTimeout) {
		result.ErrorMessage = "预热后等待提示符超时"
		result.Duration = time.Since(start)
		return result
	}
	logger.Verbose("Initializer", client.IP, "步骤3完成: 提示符已恢复")

	// 步骤 4: 发送禁分页命令
	if len(i.profile.Init.DisablePagerCommands) > 0 {
		logger.Verbose("Initializer", client.IP, "步骤4: 发送禁分页命令...")
		for _, cmd := range i.profile.Init.DisablePagerCommands {
			if err := client.SendCommand(cmd); err != nil {
				result.ErrorMessage = fmt.Sprintf("发送禁分页命令失败: %v", err)
				result.Duration = time.Since(start)
				return result
			}

			// 等待命令执行完成
			if !i.waitForPrompt(ctx, client, tracker, promptTimeout) {
				result.ErrorMessage = fmt.Sprintf("禁分页命令 '%s' 后等待提示符超时", cmd)
				result.Duration = time.Since(start)
				return result
			}
			logger.Verbose("Initializer", client.IP, "禁分页命令 '%s' 执行成功", cmd)
		}
		result.PagerDisabled = true
	} else {
		logger.Verbose("Initializer", client.IP, "步骤4: 无需发送禁分页命令")
	}

	// 步骤 5: 发送额外初始化命令
	if len(i.profile.Init.ExtraCommands) > 0 {
		logger.Verbose("Initializer", client.IP, "步骤5: 发送额外初始化命令...")
		for _, cmd := range i.profile.Init.ExtraCommands {
			if err := client.SendCommand(cmd); err != nil {
				result.ErrorMessage = fmt.Sprintf("发送额外命令失败: %v", err)
				result.Duration = time.Since(start)
				return result
			}

			if !i.waitForPrompt(ctx, client, tracker, promptTimeout) {
				result.ErrorMessage = fmt.Sprintf("额外命令 '%s' 后等待提示符超时", cmd)
				result.Duration = time.Since(start)
				return result
			}
		}
	}

	result.Success = true
	result.Duration = time.Since(start)
	logger.Verbose("Initializer", client.IP, "初始化完成，耗时 %v", result.Duration)
	return result
}

// waitForPrompt 等待提示符出现
func (i *Initializer) waitForPrompt(ctx context.Context, client *sshutil.SSHClient, tracker PromptTracker, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)

	// 创建读取缓冲区
	buf := make([]byte, 4096)

	for time.Now().Before(deadline) {
		// 检查上下文是否已取消
		select {
		case <-ctx.Done():
			return false
		default:
		}

		// 设置读取超时
		readTimeout := time.Until(deadline)
		if readTimeout <= 0 {
			return false
		}

		// 尝试读取数据
		if err := client.SetReadDeadline(time.Now().Add(1 * time.Second)); err != nil {
			continue
		}

		n, err := client.Read(buf)
		if err != nil {
			// 超时或临时错误，继续等待
			if netErr, ok := err.(interface{ Timeout() bool }); ok && netErr.Timeout() {
				continue
			}
			// 其他错误
			continue
		}

		if n > 0 {
			chunk := string(buf[:n])

			// 使用提示符跟踪器处理
			tracker.TrackChunk(chunk)

			// 【关键修复】不再对原始 chunk 直接 IsPrompt(chunk)
			// 只看规范化后的活动行/最后一行，使用严格判定
			activeLine := tracker.ActiveLine()
			if activeLine != "" && i.matcher.IsPromptStrict(activeLine) {
				logger.Debug("Initializer", "-", "严格模式检测到提示符（活动行）: '%s'", activeLine)
				return true
			}

			// 也检查已提交的最后一行
			lines := tracker.Lines()
			if len(lines) > 0 {
				lastLine := lines[len(lines)-1]
				if i.matcher.IsPromptStrict(lastLine) {
					logger.Debug("Initializer", "-", "严格模式检测到提示符（最后一行）: '%s'", lastLine)
					return true
				}
			}
		}
	}

	return false
}

// QuickInit 快速初始化（仅等待提示符，不发送禁分页命令）
func (i *Initializer) QuickInit(ctx context.Context, client *sshutil.SSHClient, tracker PromptTracker) error {
	promptTimeout := time.Duration(i.profile.Init.PromptTimeoutSec) * time.Second
	if promptTimeout == 0 {
		promptTimeout = 30 * time.Second
	}

	if !i.waitForPrompt(ctx, client, tracker, promptTimeout) {
		return fmt.Errorf("等待提示符超时")
	}

	return nil
}

// TrackChunk 将输入喂给适配器，仅用于初始化阶段的提示符跟踪
func (a *SessionAdapter) TrackChunk(chunk string) {
	a.FeedSessionActions(chunk)
}

// SendPagerContinue 发送分页续页字节
func (i *Initializer) SendPagerContinue(client *sshutil.SSHClient) error {
	if len(i.profile.Pager.ContinueBytes) > 0 {
		_, err := client.Stdin.Write(i.profile.Pager.ContinueBytes)
		return err
	}
	// 默认发送空格
	_, err := client.Stdin.Write([]byte{' '})
	return err
}
