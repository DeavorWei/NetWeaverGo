package executor

import (
	"github.com/NetWeaverGo/core/internal/config"
	"github.com/NetWeaverGo/core/internal/matcher"
)

// Initializer 会话初始化器配置访问。
// 注意：初始化执行已并入 ExecutePlan/StreamEngine 统一路径。
// 本类型仅保留配置访问功能，执行相关方法已删除。
type Initializer struct {
	profile *config.DeviceProfile
	matcher *matcher.StreamMatcher
}

// NewInitializer 创建初始化器（仅用于配置访问）。
func NewInitializer(profile *config.DeviceProfile) *Initializer {
	if profile == nil {
		profile = config.GetDeviceProfile("huawei")
	}

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
func NewInitializerWithMatcher(profile *config.DeviceProfile, m *matcher.StreamMatcher) *Initializer {
	if profile == nil {
		profile = config.GetDeviceProfile("huawei")
	}

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

// GetProfile 获取设备画像。
func (i *Initializer) GetProfile() *config.DeviceProfile {
	return i.profile
}

// GetMatcher 获取匹配器。
func (i *Initializer) GetMatcher() *matcher.StreamMatcher {
	return i.matcher
}
