package executor

import (
	"strings"
	"testing"
)

func TestCommandCache_BasicAndLRU(t *testing.T) {
	// 创建一个小容量缓存用于精准测试 LRU：上限 3 条，单条 10KB，总容量 100KB
	cache := NewCommandCache(3, 10*1024, 100*1024)

	res1 := &CommandResult{Command: "display version", RawText: "VRP 5.170", RawSize: 9}
	res2 := &CommandResult{Command: "display interface brief", RawText: "GE0/0/1 up", RawSize: 10}
	res3 := &CommandResult{Command: "display ip routing-table", RawText: "0.0.0.0/0", RawSize: 9}

	cache.Put("display version", res1)
	cache.Put("display interface brief", res2)
	cache.Put("display ip routing-table", res3)

	if cache.Len() != 3 {
		t.Fatalf("期望缓存条目为 3，实际为 %d", cache.Len())
	}

	// 访问 res1，提升其为 MRU
	got, found := cache.Get("display version")
	if !found || got.Command != "display version" {
		t.Fatalf("期望命中 display version")
	}

	// 插入第 4 条命令，此时最久未使用的应该是 res2 ("display interface brief")
	res4 := &CommandResult{Command: "display lldp neighbor", RawText: "neighbor SW2", RawSize: 12}
	cache.Put("display lldp neighbor", res4)

	if cache.Len() != 3 {
		t.Fatalf("期望缓存条目保持上限 3，实际为 %d", cache.Len())
	}

	// res2 应该已被 LRU 驱逐
	_, found = cache.Get("display interface brief")
	if found {
		t.Errorf("期望 display interface brief 已被 LRU 淘汰，但依然存在")
	}

	// res1 和 res4 必须存在
	if _, found := cache.Get("display version"); !found {
		t.Errorf("display version 应该被保全")
	}
	if _, found := cache.Get("display lldp neighbor"); !found {
		t.Errorf("display lldp neighbor 应该存在")
	}
}

func TestCommandCache_LargeEntrySkip(t *testing.T) {
	// 单条超过 512KB 禁止入缓存
	cache := DefaultCommandCache()

	largeRes := &CommandResult{
		Command: "display logbuffer",
		RawText: strings.Repeat("A", 600*1024),
		RawSize: 600 * 1024,
	}

	cache.Put("display logbuffer", largeRes)
	if cache.Len() != 0 {
		t.Errorf("超过 512KB 的单条回显不应入缓存，实际条目数: %d", cache.Len())
	}
}

func TestCommandContext_EchoConsumedAndTruncated(t *testing.T) {
	// 1. 验证 EchoConsumed 剥离首行命令行
	ctx := NewCommandContext(0, "display version")
	ctx.SetCommand("display version")

	// 设备回显首行通常带回显命令
	ctx.AddNormalizedLine("display version")
	ctx.AddNormalizedLine("Huawei Versatile Routing Platform Software")
	ctx.AddNormalizedLine("VRP (R) software, Version 5.170")

	if !ctx.EchoConsumed {
		t.Fatalf("期望首行匹配命令时自动标记 EchoConsumed = true")
	}

	effective := ctx.EffectiveLines()
	if len(effective) != 2 {
		t.Fatalf("期望剥离首行后剩余 2 行，实际得到 %d 行", len(effective))
	}
	if effective[0] != "Huawei Versatile Routing Platform Software" {
		t.Errorf("首行未能正确剥离: %s", effective[0])
	}

	normText := ctx.NormalizedText()
	if strings.HasPrefix(normText, "display version") {
		t.Errorf("NormalizedText 未剥离首行 echo: %s", normText)
	}

	result := ctx.ToResult()
	if !result.EchoConsumed {
		t.Errorf("CommandResult 必须同步 EchoConsumed 标记")
	}
	if len(result.NormalizedLines) != 2 {
		t.Errorf("CommandResult.NormalizedLines 必须是去除 echo 后的有效行")
	}

	// 2. 验证 8MB 大回显内存截断治理与 Truncated 标记
	ctx2 := NewCommandContext(1, "display current-configuration")
	largeChunk := make([]byte, 5*1024*1024) // 5MB
	ctx2.AppendRawData(largeChunk)
	if ctx2.Truncated {
		t.Fatalf("5MB 未达 8MB 上限，不应标记 Truncated")
	}

	// 再追加 5MB，总共 10MB，应触发 8MB 截断
	ctx2.AppendRawData(largeChunk)
	if !ctx2.Truncated {
		t.Fatalf("超过 8MB 应标记 Truncated = true")
	}
	if len(ctx2.RawBuffer) > MaxRawBufferSize {
		t.Fatalf("RawBuffer 内存长度 %d 超过了 8MB 上限", len(ctx2.RawBuffer))
	}

	res2 := ctx2.ToResult()
	if !res2.Truncated {
		t.Errorf("CommandResult 应标记 Truncated = true")
	}
}

func TestCommandCache_DeviceIsolation(t *testing.T) {
	// 验证不同设备互不影响
	dev1 := &DeviceExecutor{IP: "192.168.1.1", commandCache: DefaultCommandCache()}
	dev2 := &DeviceExecutor{IP: "192.168.1.2", commandCache: DefaultCommandCache()}

	dev1.GetCommandCache().Put("display version", &CommandResult{Command: "display version", RawText: "SW1"})

	if _, found := dev2.GetCommandCache().Get("display version"); found {
		t.Fatalf("不同设备的命令缓存必须物理隔离，不可互相串扰")
	}
}
