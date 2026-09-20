package taskexec

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFormatTroubleshootMessage(t *testing.T) {
	tests := []struct {
		deviceIP   string
		rawMsg     string
		err        error
		wantSubstr string
	}{
		{
			deviceIP:   "192.168.1.1",
			rawMsg:     "ssh: handshake failed: ssh: unable to authenticate",
			err:        nil,
			wantSubstr: "设备认证失败 (auth_failed",
		},
		{
			deviceIP:   "10.0.0.2",
			rawMsg:     "i/o timeout",
			err:        errors.New("context deadline exceeded"),
			wantSubstr: "超时 (timeout)",
		},
		{
			deviceIP:   "172.16.0.1",
			rawMsg:     "connection refused",
			err:        nil,
			wantSubstr: "网络不可达或端口拒绝 (network_unreachable)",
		},
		{
			deviceIP:   "10.1.1.1",
			rawMsg:     "Error: Unrecognized command found",
			err:        nil,
			wantSubstr: "命令无法识别或款型不支持 (command_not_found)",
		},
	}

	for _, tt := range tests {
		got := FormatTroubleshootMessage(tt.deviceIP, tt.rawMsg, tt.err)
		assert.Contains(t, got, tt.deviceIP)
		assert.Contains(t, got, tt.wantSubstr)
	}
}
