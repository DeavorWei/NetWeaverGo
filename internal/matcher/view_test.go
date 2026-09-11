package matcher

import "testing"

func TestResolveView_HuaweiH3C(t *testing.T) {
	cases := []struct {
		prompt string
		want   View
	}{
		{"<SW1>", ViewUser},
		{"[SW1]", ViewSystem},
		{"[SW1-diagnose]", ViewDiagnose},
		{"HRP_M[FW-1]", ViewSystem},
		{"[SW1-GigabitEthernet0/0/1]", ViewInterface},
		{"[SW1-GE0/0/1]", ViewInterface},
		{"[SW1-Vlanif10]", ViewInterface},
		{"[SW1-Eth-Trunk1]", ViewInterface},
		{"[SW1-ospf-1]", ViewRouting},
		{"[SW1-bgp]", ViewRouting},
		{"[SW1-aaa-domain]", ViewSubview},
		{"SW1>", ViewUser},
		{"SW1#", ViewSystem},
	}
	for _, c := range cases {
		if got := ResolveView("huawei", c.prompt); got != c.want {
			t.Errorf("ResolveView(huawei, %q) = %q, want %q", c.prompt, got, c.want)
		}
		if got := ResolveView("h3c", c.prompt); got != c.want {
			t.Errorf("ResolveView(h3c, %q) = %q, want %q", c.prompt, got, c.want)
		}
	}
}

func TestResolveView_CiscoRuijie(t *testing.T) {
	cases := []struct {
		prompt string
		want   View
	}{
		{"Router>", ViewUser},
		{"Router#", ViewSystem},
		{"Router(config)#", ViewConfig},
		{"Router(config-if)#", ViewInterface},
		{"Router(config-subif)#", ViewInterface},
		{"Router(config-router)#", ViewRouting},
	}
	for _, c := range cases {
		if got := ResolveView("cisco", c.prompt); got != c.want {
			t.Errorf("ResolveView(cisco, %q) = %q, want %q", c.prompt, got, c.want)
		}
		if got := ResolveView("ruijie", c.prompt); got != c.want {
			t.Errorf("ResolveView(ruijie, %q) = %q, want %q", c.prompt, got, c.want)
		}
	}
}

func TestResolveView_UnknownVendorAndEmpty(t *testing.T) {
	// 未知厂商：先尝试 VRP 括号风格
	if got := ResolveView("", "<SW1>"); got != ViewUser {
		t.Errorf("未知厂商应能识别 VRP 用户视图，实际 %q", got)
	}
	// 未知厂商：回退 Cisco 风格
	if got := ResolveView("", "Router#"); got != ViewSystem {
		t.Errorf("未知厂商应回退识别 Cisco 特权视图，实际 %q", got)
	}
	// 空提示符
	if got := ResolveView("huawei", ""); got != ViewUnknown {
		t.Errorf("空提示符应返回 unknown，实际 %q", got)
	}
	// 完全无法识别
	if got := ResolveView("huawei", "Some random text"); got != ViewUnknown {
		t.Errorf("无法识别应返回 unknown，实际 %q", got)
	}
}
