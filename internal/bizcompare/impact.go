package bizcompare

import (
	"fmt"
	"strings"

	"github.com/NetWeaverGo/core/internal/models"
)

// ImpactAnalyzer 变更影响分析器
type ImpactAnalyzer struct{}

// NewImpactAnalyzer 创建影响分析器
func NewImpactAnalyzer() *ImpactAnalyzer {
	return &ImpactAnalyzer{}
}

// Analyze 分析单条比对差异项的风险级别与影响范围
func (a *ImpactAnalyzer) Analyze(item *models.BizCompareItem) {
	if item == nil {
		return
	}

	key := strings.ToLower(item.ItemKey)
	cat := strings.ToLower(item.ItemCategory)
	before := strings.ToLower(strings.TrimSpace(item.BeforeValue))
	after := strings.ToLower(strings.TrimSpace(item.AfterValue))

	// 1. 路由协议影响 (BGP / OSPF / ISIS)
	if cat == "route" || strings.Contains(key, "bgp") || strings.Contains(key, "ospf") || strings.Contains(key, "isis") {
		if item.DiffType == "deleted" || (strings.Contains(before, "established") && !strings.Contains(after, "established")) || (strings.Contains(before, "full") && !strings.Contains(after, "full")) {
			item.ImpactLevel = "critical"
			item.ImpactScope = fmt.Sprintf("关键路由邻居中断: %s (原状态: %s -> 现状态: %s)", item.ItemKey, item.BeforeValue, item.AfterValue)
			return
		}
		if item.DiffType == "added" || strings.Contains(after, "established") || strings.Contains(after, "full") {
			item.ImpactLevel = "info"
			item.ImpactScope = fmt.Sprintf("路由邻居建立或新增: %s", item.ItemKey)
			return
		}
		item.ImpactLevel = "major"
		item.ImpactScope = fmt.Sprintf("路由协议属性变更: %s", item.ItemKey)
		return
	}

	// 2. 接口状态影响 (interface)
	if cat == "interface" || strings.Contains(key, "status") || strings.Contains(key, "interface") {
		// UP -> DOWN 属于严重事故
		if (before == "up" || strings.HasPrefix(before, "up")) && (after == "down" || strings.HasPrefix(after, "down")) {
			item.ImpactLevel = "critical"
			item.ImpactScope = fmt.Sprintf("接口物理/协议掉线: %s (UP -> DOWN)", item.ItemKey)
			return
		}
		// DOWN -> UP 恢复
		if (before == "down" || strings.HasPrefix(before, "down")) && (after == "up" || strings.HasPrefix(after, "up")) {
			item.ImpactLevel = "info"
			item.ImpactScope = fmt.Sprintf("接口状态恢复: %s (DOWN -> UP)", item.ItemKey)
			return
		}
		if item.DiffType == "deleted" {
			item.ImpactLevel = "major"
			item.ImpactScope = fmt.Sprintf("接口配置或实例被删除: %s", item.ItemKey)
			return
		}
		item.ImpactLevel = "minor"
		item.ImpactScope = fmt.Sprintf("接口属性发生变化: %s", item.ItemKey)
		return
	}

	// 3. 数据中心网络与大二层 (VXLAN / STP / LLDP)
	if cat == "vxlan" || cat == "stp" || cat == "lldp" {
		if item.DiffType == "deleted" {
			item.ImpactLevel = "major"
			item.ImpactScope = fmt.Sprintf("二层/VXLAN拓扑拓扑关系缺失: %s", item.ItemKey)
			return
		}
		item.ImpactLevel = "minor"
		item.ImpactScope = fmt.Sprintf("二层/VXLAN状态更新: %s", item.ItemKey)
		return
	}

	// 4. 表项漂移 (MAC / ARP)
	if cat == "mac" || cat == "arp" {
		item.ImpactLevel = "minor"
		if item.DiffType == "added" {
			item.ImpactScope = fmt.Sprintf("新增终端/MAC/ARP学习项: %s", item.ItemKey)
		} else if item.DiffType == "deleted" {
			item.ImpactScope = fmt.Sprintf("老终端/MAC/ARP学习项老化或离线: %s", item.ItemKey)
		} else {
			item.ImpactScope = fmt.Sprintf("终端学习项迁移或漂移: %s", item.ItemKey)
		}
		return
	}

	// 默认兜底
	item.ImpactLevel = "info"
	item.ImpactScope = fmt.Sprintf("常规业务项状态变化: %s", item.ItemKey)
}
