package parser

// TileResultTree 将层次化解析出的 ResultNode 树拍平展开为关系型表格行（[]map[string]string）。
// 包含两大核心能力：
// 1. 父字段继承：下层行自动携带上层所有已抽取的字段；
// 2. 笛卡尔展开：当遇到 IsList 节点时，自动展开为多行。
func TileResultTree(
	rootNodes []*ResultNode,
	ruleMap map[string]*CompiledTreeRule,
	maxOutputLevel int,
) []map[string]string {
	var results []map[string]string

	for _, root := range rootNodes {
		rule, ok := ruleMap[root.ItemName]
		if !ok {
			continue
		}
		tileRecursive(root, rule, 1, maxOutputLevel, make(map[string]string), ruleMap, &results)
	}

	// 补齐规格：保证所有 IsOutput=true 的字段在每一行都有键（缺失时补 DefaultValue 或 ""）
	for _, row := range results {
		for _, r := range ruleMap {
			if r.IsOutput {
				if _, exists := row[r.ParseItem]; !exists {
					if r.DefaultValue != "" {
						row[r.ParseItem] = r.DefaultValue
					} else {
						row[r.ParseItem] = ""
					}
				}
			}
		}
	}

	return results
}

func tileRecursive(
	node *ResultNode,
	rule *CompiledTreeRule,
	level int,
	maxLevel int,
	inherited map[string]string,
	ruleMap map[string]*CompiledTreeRule,
	out *[]map[string]string,
) {
	// 1. 继承父层字段
	current := make(map[string]string, len(inherited)+len(node.Attrs))
	for k, v := range inherited {
		current[k] = v
	}

	// 2. 填入本节点中所有标记为 IsOutput 的属性字段
	for k, v := range node.Attrs {
		if r, ok := ruleMap[k]; ok && r.IsOutput {
			if v != "" {
				current[k] = v
			} else if r.DefaultValue != "" {
				current[k] = r.DefaultValue
			} else {
				current[k] = ""
			}
		}
	}
	if rule.IsOutput {
		if _, exists := current[rule.ParseItem]; !exists {
			if rule.DefaultValue != "" {
				current[rule.ParseItem] = rule.DefaultValue
			} else {
				current[rule.ParseItem] = ""
			}
		}
	}

	// 3. 判断是否达到终止深度
	if maxLevel > 0 && level >= maxLevel {
		*out = append(*out, current)
		return
	}

	// 4. 筛选属于 IsPath 的子节点
	type childPair struct {
		node *ResultNode
		rule *CompiledTreeRule
	}
	var pathChildren []childPair

	for _, child := range node.Children {
		childRule, exists := ruleMap[child.ItemName]
		if exists && childRule.IsPath {
			pathChildren = append(pathChildren, childPair{node: child, rule: childRule})
		}
	}

	// 如果没有后续路径分支，当前层即为最终发射行
	if len(pathChildren) == 0 {
		*out = append(*out, current)
		return
	}

	// 5. 递归下钻展开
	for _, cp := range pathChildren {
		tileRecursive(cp.node, cp.rule, level+1, maxLevel, current, ruleMap, out)
	}
}
