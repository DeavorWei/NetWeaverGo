package executor

import (
	"testing"
	"time"

	"github.com/NetWeaverGo/core/internal/models"
)

func TestValidateBypass(t *testing.T) {
	tests := []struct {
		name    string
		req     BypassRequest
		wantErr bool
	}{
		{
			name: "未二次确认",
			req: BypassRequest{
				RunID:           "run-1",
				DeviceIP:        "192.168.1.1",
				Command:         "format flash:",
				Operator:        "admin",
				Reason:          "业务紧急割接临时清理",
				SecondConfirmed: false,
			},
			wantErr: true,
		},
		{
			name: "理由过短",
			req: BypassRequest{
				RunID:           "run-1",
				DeviceIP:        "192.168.1.1",
				Command:         "format flash:",
				Operator:        "admin",
				Reason:          "测试",
				SecondConfirmed: true,
			},
			wantErr: true,
		},
		{
			name: "操作人为空",
			req: BypassRequest{
				RunID:           "run-1",
				DeviceIP:        "192.168.1.1",
				Command:         "format flash:",
				Operator:        "",
				Reason:          "业务紧急割接临时清理",
				SecondConfirmed: true,
			},
			wantErr: true,
		},
		{
			name: "有效请求",
			req: BypassRequest{
				RunID:           "run-1",
				DeviceIP:        "192.168.1.1",
				Command:         "format flash:",
				Operator:        "admin",
				Reason:          "业务紧急割接临时清理",
				SecondConfirmed: true,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateBypass(tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateBypass() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRiskValidator_TrustList(t *testing.T) {
	v := NewRiskValidatorFromRules(models.DefaultRiskCommandSeeds())

	// 添加信任清单：允许 display 命令和一条特权命令
	v.ReloadTrustEntries([]models.RiskTrustEntry{
		{
			ID:        1,
			UserID:    "admin",
			Pattern:   `(?i)^reboot\s+fast$`,
			ExpiresAt: time.Now().Add(1 * time.Hour), // 未过期
			Reason:    "快重启临时测试",
		},
		{
			ID:        2,
			UserID:    "admin",
			Pattern:   `(?i)^format\s+sdcard:$`,
			ExpiresAt: time.Now().Add(-1 * time.Hour), // 已过期
			Reason:    "已过期的授权",
		},
	})

	// 命中有效信任
	trusted, entry := v.CheckTrust("reboot fast")
	if !trusted || entry == nil || entry.ID != 1 {
		t.Errorf("预期命中信任清单条目 1")
	}

	// 命中已过期信任应被判定为未信任
	trustedExpired, _ := v.CheckTrust("format sdcard:")
	if trustedExpired {
		t.Errorf("已过期条目不应判定为信任")
	}

	// 未在信任清单中
	trustedOther, _ := v.CheckTrust("reboot normal")
	if trustedOther {
		t.Errorf("未授权命令不应判定为信任")
	}
}
