package connutil

import (
	"strings"
	"testing"
)

func TestJumpConnector_EmbeddedTemplates(t *testing.T) {
	connector := GetDefaultJumpConnector()
	if connector == nil {
		t.Fatalf("GetDefaultJumpConnector() returned nil")
	}

	templates := connector.ListTemplates()
	if len(templates) < 13 {
		t.Errorf("expected at least 13 templates, got %d", len(templates))
	}

	// 验证核心模板存在
	tpls := []string{
		"ssh_standard",
		"ssh_legacy_ciphers",
		"ssh_insecure_hostkey",
		"ssh_forced_pty",
		"ssh_proxy_command",
		"telnet_standard",
		"telnet_escape_ctrl",
		"telnet_binary_mode",
		"huawei_stelnet",
		"huawei_telnet",
		"cisco_ssh",
		"h3c_ssh2",
		"privilege_escalation",
	}

	for _, id := range tpls {
		tpl, err := connector.FindTemplate(id)
		if err != nil || tpl == nil {
			t.Errorf("template %s not found: %v", id, err)
		}
	}
}

func TestJumpConnector_BuildJumpCommand(t *testing.T) {
	connector := GetDefaultJumpConnector()

	// 1. 标准 SSH 渲染
	cmd1, err := connector.BuildJumpCommand(JumpHostConfig{
		TargetIP:       "192.168.1.100",
		TargetPort:     2222,
		TargetUsername: "admin",
		TargetProtocol: "ssh",
	})
	if err != nil {
		t.Fatalf("BuildJumpCommand failed: %v", err)
	}
	expected1 := "ssh -p 2222 -l admin 192.168.1.100"
	if cmd1 != expected1 {
		t.Errorf("expected %q, got %q", expected1, cmd1)
	}

	// 2. 华为 STelnet 渲染
	cmd2, err := connector.BuildJumpCommand(JumpHostConfig{
		TargetIP:   "10.10.10.1",
		TemplateID: "huawei_stelnet",
	})
	if err != nil {
		t.Fatalf("BuildJumpCommand for huawei_stelnet failed: %v", err)
	}
	expected2 := "stelnet 10.10.10.1 22"
	if cmd2 != expected2 {
		t.Errorf("expected %q, got %q", expected2, cmd2)
	}

	// 3. ProxyCommand 渲染
	cmd3, err := connector.BuildJumpCommand(JumpHostConfig{
		TargetIP:       "172.16.1.1",
		TargetPort:     22,
		TargetUsername: "root",
		TemplateID:     "ssh_proxy_command",
		ProxyHost:      "127.0.0.1",
		ProxyPort:      1080,
	})
	if err != nil {
		t.Fatalf("BuildJumpCommand for ssh_proxy_command failed: %v", err)
	}
	if !strings.Contains(cmd3, "127.0.0.1:1080") || !strings.Contains(cmd3, "root") {
		t.Errorf("expected proxy command with host:port and username, got %q", cmd3)
	}
}
