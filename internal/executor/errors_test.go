package executor

import (
	"errors"
	"testing"

	"github.com/NetWeaverGo/core/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClassifyErrorReason(t *testing.T) {
	tests := []struct {
		err  error
		want string
	}{
		{errors.New("ssh: handshake failed: ssh: unable to authenticate"), ErrCategoryAuthFailed},
		{errors.New("i/o timeout while waiting for command response"), ErrCategoryTimeout},
		{errors.New("dial tcp 192.168.1.1:22: connect: connection refused"), ErrCategoryNetworkUnreachable},
		{errors.New("Error: Unrecognized command found at '^' position"), ErrCategoryCommandNotFound},
		{errors.New("Error: Incomplete command found at '^' position"), ErrCategorySyntaxError},
		{errors.New("Permission denied (publickey,password)"), ErrCategoryAuthFailed},
		{errors.New("Error: Privilege level too low"), ErrCategoryPermissionDenied},
		{errors.New("runtime error: out of memory"), ErrCategorySystemFault},
		{errors.New("some unexpected random string"), ErrCategoryUnknown},
	}

	for _, tt := range tests {
		got := ClassifyErrorReason(tt.err)
		assert.Equal(t, tt.want, got, "err: %v", tt.err)
	}
}

func TestDeviceExecutor_MatchPolicyAndCharsetWiring(t *testing.T) {
	profile := &config.DeviceProfile{
		Vendor:     "huawei",
		DeviceType: "switch",
		Charset:    "gb18030",
	}

	opts := ExecutorOptions{
		Vendor:        "huawei",
		DeviceProfile: profile,
		ProxyAddr:     "127.0.0.1:1080",
	}

	exec := NewDeviceExecutor("192.168.1.1", 22, "admin", "admin", opts)
	require.NotNil(t, exec)
	assert.Equal(t, "gb18030", exec.charset)
	assert.Equal(t, "127.0.0.1:1080", exec.proxyAddr)
	assert.NotNil(t, exec.Matcher)

	// 验证策略已接入
	assert.NotNil(t, exec.Matcher.Policy, "MatchPolicy 应成功接入 Matcher")
	assert.Equal(t, "huawei", exec.Matcher.Policy.Vendor)

	// 验证 StreamEngine 继承 Matcher
	engine := NewStreamEngine(exec, nil, []string{"display version"}, 80)
	require.NotNil(t, engine)
	assert.Same(t, exec.Matcher, engine.matcher, "StreamEngine 应当直接继承复用 DeviceExecutor.Matcher")
}
