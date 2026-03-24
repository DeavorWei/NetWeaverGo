package executor

import (
	"context"
	"fmt"
	"time"

	"github.com/NetWeaverGo/core/internal/config"
	"github.com/NetWeaverGo/core/internal/matcher"
	"github.com/NetWeaverGo/core/internal/sshutil"
)

// Initializer 会话初始化器（兼容壳）。
// Deprecated: 初始化已并入 ExecutePlan/StreamEngine 统一路径，本类型仅保留配置访问与旧接口兼容。
type Initializer struct {
	profile *config.DeviceProfile
	matcher *matcher.StreamMatcher
}

// PromptTracker 初始化阶段的最小提示符跟踪接口（兼容保留）。
// Deprecated: 初始化流程不再使用该接口。
type PromptTracker interface {
	TrackChunk(chunk string)
	ActiveLine() string
	Lines() []string
}

// NewInitializer 创建初始化器。
// Deprecated: 请通过 DeviceExecutor.ExecutePlan 触发统一初始化流程。
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

// NewInitializerWithMatcher 创建初始化器（使用现有匹配器）。
// Deprecated: 请通过 DeviceExecutor.ExecutePlan 触发统一初始化流程。
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

// GetProfile 获取设备画像（兼容保留）。
func (i *Initializer) GetProfile() *config.DeviceProfile {
	return i.profile
}

// GetMatcher 获取匹配器（兼容保留）。
func (i *Initializer) GetMatcher() *matcher.StreamMatcher {
	return i.matcher
}

// InitResult 初始化结果（兼容保留）。
type InitResult struct {
	Success       bool
	ErrorMessage  string
	PromptFound   bool
	PagerDisabled bool
	Duration      time.Duration
}

const deprecatedInitializerPathMessage = "Initializer 直连 SSH 初始化路径已废弃，请使用 ExecutePlan/StreamEngine 统一执行路径"

// Run 已废弃。
// 方案三中初始化已并入统一状态机执行路径，禁止再通过 Initializer 直接读写 SSH。
func (i *Initializer) Run(ctx context.Context, client *sshutil.SSHClient, tracker PromptTracker) error {
	_ = ctx
	_ = client
	_ = tracker
	result := i.RunWithResult(ctx, client, tracker)
	if !result.Success {
		return fmt.Errorf("初始化失败: %s", result.ErrorMessage)
	}
	return nil
}

// RunWithResult 已废弃。
// 保留该方法仅用于兼容旧调用，固定返回失败，避免引入旁路命令下发。
func (i *Initializer) RunWithResult(ctx context.Context, client *sshutil.SSHClient, tracker PromptTracker) *InitResult {
	_ = ctx
	_ = client
	_ = tracker
	return &InitResult{
		Success:      false,
		ErrorMessage: deprecatedInitializerPathMessage,
		Duration:     0,
	}
}

// QuickInit 已废弃。
func (i *Initializer) QuickInit(ctx context.Context, client *sshutil.SSHClient, tracker PromptTracker) error {
	_ = ctx
	_ = client
	_ = tracker
	return fmt.Errorf(deprecatedInitializerPathMessage)
}

// TrackChunk 将输入喂给适配器，仅用于初始化阶段的提示符跟踪
// Deprecated: 统一执行路径下不再使用此方法。
func (a *SessionAdapter) TrackChunk(chunk string) {
	a.FeedSessionActions(chunk)
}

// SendPagerContinue 已废弃。
func (i *Initializer) SendPagerContinue(client *sshutil.SSHClient) error {
	_ = client
	return fmt.Errorf(deprecatedInitializerPathMessage)
}
