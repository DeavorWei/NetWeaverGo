package ceas

import "sort"

// NormalizeTreeForGolden 生成稳定可比的硬件树快照，供 golden 测试使用。
//
// Go 的 map 遍历顺序是伪随机的，直接序列化硬件树会因 Attrs 字段顺序不同而产生
// 跨运行/跨平台的比对抖动（flaky）。本函数做三件事保证结果确定：
//  1. 每层子节点按 (Path, ID) 排序；
//  2. 节点 Attrs 按 key 字典序重排；
//  3. 按排序后的树重新推导 AllNodes，保证扁平列表顺序同样确定。
func NormalizeTreeForGolden(t *HardwareTree) *HardwareTree {
	if t == nil {
		return nil
	}

	roots := make([]*Node, 0, len(t.Roots))
	for _, r := range t.Roots {
		roots = append(roots, normalizeNodeForGolden(r))
	}
	sortNodes(roots)

	allNodes := make([]*Node, 0, len(t.AllNodes))
	var walk func(n *Node)
	walk = func(n *Node) {
		if n == nil {
			return
		}
		allNodes = append(allNodes, n)
		for _, c := range n.Children {
			walk(c)
		}
	}
	for _, r := range roots {
		walk(r)
	}

	return &HardwareTree{
		DeviceIP:   t.DeviceIP,
		ChassisESN: t.ChassisESN,
		Roots:      roots,
		AllNodes:   allNodes,
	}
}

// normalizeNodeForGolden 深拷贝节点并规范化子节点顺序与 Attrs 顺序
func normalizeNodeForGolden(n *Node) *Node {
	if n == nil {
		return nil
	}

	cp := *n
	cp.Children = nil
	cp.Attrs = normalizeAttrs(n.Attrs)

	for _, c := range n.Children {
		cp.Children = append(cp.Children, normalizeNodeForGolden(c))
	}
	sortNodes(cp.Children)

	return &cp
}

// normalizeAttrs 返回按 key 排序的属性副本；空 map 归一化为 nil，避免 "{}" 与 null 的伪差异
func normalizeAttrs(attrs map[string]string) map[string]string {
	if len(attrs) == 0 {
		return nil
	}
	keys := make([]string, 0, len(attrs))
	for k := range attrs {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	normalized := make(map[string]string, len(keys))
	for _, k := range keys {
		normalized[k] = attrs[k]
	}
	return normalized
}

// sortNodes 按 (Path, ID) 稳定排序
func sortNodes(nodes []*Node) {
	sort.SliceStable(nodes, func(i, j int) bool {
		if nodes[i].Path != nodes[j].Path {
			return nodes[i].Path < nodes[j].Path
		}
		return nodes[i].ID < nodes[j].ID
	})
}
