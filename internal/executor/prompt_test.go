package executor

import (
	"os"
	"strings"
	"testing"

	"github.com/NetWeaverGo/core/internal/config"
	"github.com/NetWeaverGo/core/internal/matcher"
)

func TestPrompt_LinuxAndCustomProfile(t *testing.T) {
	// 验证打通断头路 #1：通过 Profile 注入自定义提示符（如 user@host:~$）
	m := matcher.NewStreamMatcher()

	// 模拟针对 Linux 服务器或特殊自定义提示符设备的画像配置注入
	promptSuffixes := []string{"$", "#", ">"}
	promptPatterns := []string{`[\w\-]+@[\w\-]+:[~/\w\-]*\$\s*$`}
	pagerPatterns := []string{"--More--"}

	m.ConfigureFromProfile(promptSuffixes, promptPatterns, pagerPatterns)

	// 验证正则模式成功编译并匹配
	linuxPrompt := "user@ubuntu-server:~$ "
	if !m.IsPrompt(linuxPrompt) {
		t.Fatalf("期望命中注入正则提示符: %s", linuxPrompt)
	}
	if !m.IsPromptStrict("user@ubuntu-server:~$") {
		t.Fatalf("期望严格命中注入正则提示符: user@ubuntu-server:~$")
	}

	// 验证 Detector 能够提取该提示符并驱动事件
	detector := NewSessionDetector(m)
	events := detector.Detect([]string{"total 24", "-rw-r--r-- 1 root root 120 Jan 1 10:00 test.txt"}, "user@ubuntu-server:~$")
	if len(events) < 1 {
		t.Fatalf("期望检测到活动行提示符事件，实际未检测到")
	}

	lastEv := events[len(events)-1]
	promptEv, ok := lastEv.(EvActivePromptSeen)
	if !ok {
		t.Fatalf("期望最后事件为 EvActivePromptSeen，实际得到 %T", lastEv)
	}
	if promptEv.Prompt != "user@ubuntu-server:~$" {
		t.Errorf("期望提取提示符 'user@ubuntu-server:~$'，实际得到 %s", promptEv.Prompt)
	}
}

func TestPrompt_H3CAndHuaweiAngleBrackets(t *testing.T) {
	m := matcher.NewStreamMatcher()
	m.ConfigureFromProfile([]string{">", "]", "#"}, nil, []string{"---- More ----"})

	huaweiPrompt := "<HUAWEI>"
	if !m.IsPromptStrict(huaweiPrompt) {
		t.Errorf("期望严格识别华为尖括号提示符: %s", huaweiPrompt)
	}

	h3cSysPrompt := "[H3C-GigabitEthernet0/0/1]"
	if !m.IsPromptStrict(h3cSysPrompt) {
		t.Errorf("期望严格识别方括号系统视图提示符: %s", h3cSysPrompt)
	}
}

func TestPrompt_LinuxTestdataNoFalsePagination(t *testing.T) {
	data, err := os.ReadFile("../../testdata/executor/linux_echo.txt")
	if err != nil {
		t.Fatalf("读取 linux_echo.txt 失败: %v", err)
	}

	m := matcher.NewStreamMatcher()
	// 使用 Linux 真实画像规则
	linuxProfile := config.GetDeviceProfile("linux")
	if linuxProfile == nil {
		t.Fatalf("未找到 linux 画像配置")
	}
	m.ConfigureFromProfile(
		linuxProfile.Prompt.Suffixes,
		linuxProfile.Prompt.Patterns,
		linuxProfile.Pager.Patterns,
	)

	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		// 严禁包含冒号的行（如 1: lo: <LOOPBACK...>）被误判为分页
		if m.IsPaginationPrompt(trimmed) {
			t.Fatalf("行 %d: %q 包含冒号，被错误识别为分页符！", i+1, trimmed)
		}
	}
}

func TestPrompt_HuaweiTestdataConfirm(t *testing.T) {
	data, err := os.ReadFile("../../testdata/executor/huawei_confirm.txt")
	if err != nil {
		t.Fatalf("读取 huawei_confirm.txt 失败: %v", err)
	}

	m := matcher.NewStreamMatcher()
	m.SetConfirmPatterns(matcher.DefaultConfirmPatterns)

	lines := strings.Split(string(data), "\n")
	var lastLine string
	for i := len(lines) - 1; i >= 0; i-- {
		t := strings.TrimSpace(lines[i])
		if t != "" {
			lastLine = t
			break
		}
	}

	matched, prompt := m.CheckConfirmPrompt(lastLine)
	if !matched {
		t.Fatalf("期望华为确认行被识别，实际未识别: %s", lastLine)
	}
	if prompt == "" {
		t.Errorf("期望识别到提示符文本")
	}
}

