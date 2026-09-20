package bizcompare

import "testing"

// P1-3：快照项应为结构化归一化字段（显式 key:value 提取 + 空白折叠），并过滤提示符行
func TestExtractStructuredItems(t *testing.T) {
	lines := []string{
		"sysname Core-Switch-01",
		"  vlan 100  ",
		"Description: 杭州核心交换机",
		`"mtu": 9000,`,
		"",
		"<Huawei>",
		"interface   GigabitEthernet0/0/1    up",
	}
	items := ExtractStructuredItems("display current-configuration", "config", lines)
	got := make(map[string]string, len(items))
	for _, it := range items {
		got[it.Key] = it.Value
	}

	if got["display current-configuration.Description"] != "杭州核心交换机" {
		t.Fatalf("应提取结构化字段 Description，实际: %+v", got)
	}
	if got["display current-configuration.mtu"] != "9000" {
		t.Fatalf("应提取 JSON 风格字段 mtu，实际: %+v", got)
	}
	if _, ok := got["display current-configuration.<Huawei>"]; ok {
		t.Fatal("提示符行应被过滤")
	}
	if _, ok := got["display current-configuration.interface GigabitEthernet0/0/1 up"]; !ok {
		t.Fatalf("整行项应折叠多余空白，实际: %+v", got)
	}

	dup := ExtractStructuredItems("cmd", "c", []string{"a: 1", "a: 2"})
	if len(dup) != 2 || dup[0].Key == dup[1].Key {
		t.Fatalf("重复字段应生成不同键以避免覆盖: %+v", dup)
	}
}
