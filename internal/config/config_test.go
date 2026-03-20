package config

import (
	"testing"
)

func TestMergePassword(t *testing.T) {
	tests := []struct {
		name         string
		oldPassword  string
		newPassword  string
		wantPassword string
		wantChanged  bool
	}{
		{
			name:         "空值不修改-保留原密码",
			oldPassword:  "oldpass",
			newPassword:  "",
			wantPassword: "oldpass",
			wantChanged:  false,
		},
		{
			name:         "非空值更新-相同密码",
			oldPassword:  "samepass",
			newPassword:  "samepass",
			wantPassword: "samepass",
			wantChanged:  false,
		},
		{
			name:         "非空值更新-不同密码",
			oldPassword:  "oldpass",
			newPassword:  "newpass",
			wantPassword: "newpass",
			wantChanged:  true,
		},
		{
			name:         "原密码为空-新密码非空",
			oldPassword:  "",
			newPassword:  "newpass",
			wantPassword: "newpass",
			wantChanged:  true,
		},
		{
			name:         "双空密码",
			oldPassword:  "",
			newPassword:  "",
			wantPassword: "",
			wantChanged:  false,
		},
		{
			name:         "带空格的密码",
			oldPassword:  "old pass",
			newPassword:  "new pass",
			wantPassword: "new pass",
			wantChanged:  true,
		},
		{
			name:         "中文密码",
			oldPassword:  "旧密码",
			newPassword:  "新密码",
			wantPassword: "新密码",
			wantChanged:  true,
		},
		{
			name:         "特殊字符密码",
			oldPassword:  "p@ssw0rd!",
			newPassword:  "n3wP@ss#",
			wantPassword: "n3wP@ss#",
			wantChanged:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MergePassword(tt.oldPassword, tt.newPassword)
			if result.Password != tt.wantPassword {
				t.Errorf("MergePassword().Password = %q, want %q", result.Password, tt.wantPassword)
			}
			if result.Changed != tt.wantChanged {
				t.Errorf("MergePassword().Changed = %v, want %v", result.Changed, tt.wantChanged)
			}
			// 验证 OldPassword 正确保存
			if result.OldPassword != tt.oldPassword {
				t.Errorf("MergePassword().OldPassword = %q, want %q", result.OldPassword, tt.oldPassword)
			}
		})
	}
}

func TestPasswordMergeResultStruct(t *testing.T) {
	// 测试结构体字段完整性
	result := MergePassword("old", "new")

	if result.Password != "new" {
		t.Errorf("Password field not set correctly")
	}
	if !result.Changed {
		t.Errorf("Changed field should be true")
	}
	if result.OldPassword != "old" {
		t.Errorf("OldPassword field not set correctly")
	}
}
