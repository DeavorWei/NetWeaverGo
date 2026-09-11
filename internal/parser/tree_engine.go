package parser

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// TreeEngine 声明式规则树解析引擎
type TreeEngine struct{}

// NewTreeEngine 创建树形解析引擎实例
func NewTreeEngine() *TreeEngine {
	return &TreeEngine{}
}

// ParseWithTemplate 执行树形解析并拍平为关系型表格行（实现 CliParser 契约）
func (e *TreeEngine) ParseWithTemplate(tpl *CompiledTemplate, rawText string) ([]map[string]string, error) {
	if tpl == nil {
		return nil, ErrNilTemplate
	}
	if len(tpl.TreeRootRules) == 0 {
		return nil, fmt.Errorf("%w: 缺少根规则", ErrInvalidTreeRule)
	}

	ruleMap := make(map[string]*CompiledTreeRule, len(tpl.CompiledTreeRules))
	for _, r := range tpl.CompiledTreeRules {
		ruleMap[r.ParseItem] = r
	}

	// 1. 递归构建数据结果树
	var rootNodes []*ResultNode
	for _, rootRule := range tpl.TreeRootRules {
		nodes := fillSubResult(rawText, rootRule)
		rootNodes = append(rootNodes, nodes...)
	}

	maxLevel := 0
	if tpl.TreeConfig != nil {
		maxLevel = tpl.TreeConfig.MaxOutputLevel
	}

	// 2. 笛卡尔拍平展开
	rows := TileResultTree(rootNodes, ruleMap, maxLevel)
	return rows, nil
}

// ParseTree 执行树形解析并输出层级结构 ResultNode（供 CEAS 等层级消费场景）
func (e *TreeEngine) ParseTree(tpl *CompiledTemplate, rawText string) ([]*ResultNode, error) {
	if tpl == nil {
		return nil, ErrNilTemplate
	}
	if len(tpl.TreeRootRules) == 0 {
		return nil, fmt.Errorf("%w: 缺少根规则", ErrInvalidTreeRule)
	}

	ruleMap := make(map[string]*CompiledTreeRule, len(tpl.CompiledTreeRules))
	for _, r := range tpl.CompiledTreeRules {
		ruleMap[r.ParseItem] = r
	}

	var rootNodes []*ResultNode
	for _, rootRule := range tpl.TreeRootRules {
		nodes := fillSubResult(rawText, rootRule)
		rootNodes = append(rootNodes, nodes...)
	}

	return rootNodes, nil
}

// CompileTreeRules 校验并编译规则树，返回平铺列表与根规则列表
func CompileTreeRules(rules []TreeRule) ([]*CompiledTreeRule, []*CompiledTreeRule, error) {
	if len(rules) == 0 {
		return nil, nil, fmt.Errorf("%w: 规则列表为空", ErrInvalidTreeRule)
	}

	compiledList := make([]*CompiledTreeRule, 0, len(rules))
	ruleMap := make(map[string]*CompiledTreeRule, len(rules))

	// 1. 编译各条独立规则的正则
	for i := range rules {
		r := rules[i]
		if r.ParseItem == "" {
			return nil, nil, fmt.Errorf("%w: 存在空字段名的规则", ErrInvalidTreeRule)
		}
		if _, exists := ruleMap[r.ParseItem]; exists {
			return nil, nil, fmt.Errorf("%w: 重复的字段名 '%s'", ErrInvalidTreeRule, r.ParseItem)
		}

		compiled := &CompiledTreeRule{
			TreeRule: r,
		}

		// 编译 ParseRegex
		if r.ParseRegex != "" {
			pat := buildRegexWithFlags(r.ParseRegex, r.ParseFlags)
			re, err := regexp.Compile(pat)
			if err != nil {
				return nil, nil, fmt.Errorf("%w: 字段 '%s' 的 parseRegex 编译失败: %v", ErrInvalidTreeRule, r.ParseItem, err)
			}
			compiled.CompiledParseRegex = re
		}

		// 编译 SplitRegex
		if r.SplitRegex != "" {
			pat := buildRegexWithFlags(r.SplitRegex, r.SplitFlags)
			re, err := regexp.Compile(pat)
			if err != nil {
				return nil, nil, fmt.Errorf("%w: 字段 '%s' 的 splitRegex 编译失败: %v", ErrInvalidTreeRule, r.ParseItem, err)
			}
			compiled.CompiledSplitRegex = re
		}

		compiledList = append(compiledList, compiled)
		ruleMap[r.ParseItem] = compiled
	}

	// 2. 构建父子层级关系，并检查环路
	var rootRules []*CompiledTreeRule
	for _, r := range compiledList {
		if r.ParentItem == "" {
			rootRules = append(rootRules, r)
		} else {
			parent, exists := ruleMap[r.ParentItem]
			if !exists {
				return nil, nil, fmt.Errorf("%w: 字段 '%s' 指定的父字段 '%s' 不存在", ErrInvalidTreeRule, r.ParseItem, r.ParentItem)
			}
			parent.Children = append(parent.Children, r)
		}
	}

	if len(rootRules) == 0 {
		return nil, nil, fmt.Errorf("%w: 没有根规则（所有规则均指定了父节点）", ErrCyclicTreeRule)
	}

	// 拓扑有向无环图（DAG）验证
	visited := make(map[string]int) // 0: unvisited, 1: visiting, 2: visited
	var checkCycle func(r *CompiledTreeRule) error
	checkCycle = func(r *CompiledTreeRule) error {
		visited[r.ParseItem] = 1
		for _, child := range r.Children {
			if visited[child.ParseItem] == 1 {
				return fmt.Errorf("%w: 在字段 '%s' 处检测到依赖环路", ErrCyclicTreeRule, child.ParseItem)
			}
			if visited[child.ParseItem] == 0 {
				if err := checkCycle(child); err != nil {
					return err
				}
			}
		}
		visited[r.ParseItem] = 2
		return nil
	}

	for _, root := range rootRules {
		if err := checkCycle(root); err != nil {
			return nil, nil, err
		}
	}

	// 3. 子节点按 Order 排序
	var sortChildren func(r *CompiledTreeRule)
	sortChildren = func(r *CompiledTreeRule) {
		sort.SliceStable(r.Children, func(i, j int) bool {
			return r.Children[i].Order < r.Children[j].Order
		})
		for _, c := range r.Children {
			sortChildren(c)
		}
	}
	sort.SliceStable(rootRules, func(i, j int) bool {
		return rootRules[i].Order < rootRules[j].Order
	})
	for _, root := range rootRules {
		sortChildren(root)
	}

	// 4. 编译期一次性固化 IsPath 标记（解决 M2：解析期并发写快照导致的数据竞争）
	markPathNodes(ruleMap)

	return compiledList, rootRules, nil
}

// markPathNodes 回溯标记：从所有 IsOutput=true 的规则向上回溯其祖先，将 IsPath 标记为 true。
// 编译期一次性执行，使 CompiledTreeRule 在解析期为只读，保证多并发安全。
func markPathNodes(ruleMap map[string]*CompiledTreeRule) {
	for _, rule := range ruleMap {
		if rule.IsOutput {
			rule.IsPath = true
			curr := rule
			for curr.ParentItem != "" {
				parent, exists := ruleMap[curr.ParentItem]
				if !exists || parent.IsPath {
					break
				}
				parent.IsPath = true
				curr = parent
			}
		}
	}
}

// 拼接内联正则修饰符 (?mi)
func buildRegexWithFlags(pattern, flags string) string {
	if flags == "" {
		return pattern
	}
	cleanFlags := strings.ToLower(strings.TrimSpace(flags))
	validFlags := ""
	for _, c := range cleanFlags {
		if c == 'm' || c == 'i' || c == 's' {
			if !strings.ContainsRune(validFlags, c) {
				validFlags += string(c)
			}
		}
	}
	if validFlags != "" && !strings.HasPrefix(pattern, "(?") {
		return "(?" + validFlags + ")" + pattern
	}
	return pattern
}

// fillSubResult 核心抽取算法：递归提取并成树
func fillSubResult(
	text string,
	rule *CompiledTreeRule,
) []*ResultNode {
	if len(text) == 0 {
		return nil
	}

	// 场景 A：列表节点且包含分块正则（如 Slot 分块）
	if rule.IsList && rule.CompiledSplitRegex != nil {
		blocks := SplitBlocks(text, rule.CompiledSplitRegex, true)
		if len(blocks) == 0 {
			return nil
		}

		var nodes []*ResultNode
		for _, blk := range blocks {
			node := NewResultNode(rule.ParseItem, blk.Body)

			// 优先从分块正则捕获组提取值（例如 Slot 序号）
			groupIdx := rule.GroupIndex
			if groupIdx <= 0 {
				groupIdx = 1
			}
			splitMatches := rule.CompiledSplitRegex.FindStringSubmatch(blk.Match)
			if len(splitMatches) > groupIdx {
				node.Attrs[rule.ParseItem] = splitMatches[groupIdx]
			} else if rule.CompiledParseRegex != nil {
				parseMatches := rule.CompiledParseRegex.FindStringSubmatch(blk.Body)
				if len(parseMatches) > groupIdx {
					node.Attrs[rule.ParseItem] = parseMatches[groupIdx]
				}
			}

			// 递归提取所有子规则
			for _, childRule := range rule.Children {
				if !childRule.IsList && len(childRule.Children) == 0 {
					// 叶子单值规则：直接在当前块抽取属性，并入 node.Attrs
					extractScalarAttr(blk.Body, childRule, node.Attrs)
				} else {
					// 具有下级或列表的子规则，递归生成子 ResultNode
					childNodes := fillSubResult(blk.Body, childRule)
					node.Children = append(node.Children, childNodes...)
				}
			}

			nodes = append(nodes, node)
		}
		return nodes
	}

	// 场景 B：列表节点但无分块正则（多匹配提取列表，如 ports 或按行记录）
	if rule.IsList && rule.CompiledParseRegex != nil {
		groupIdx := rule.GroupIndex
		if groupIdx <= 0 {
			groupIdx = 1
		}
		allIndices := rule.CompiledParseRegex.FindAllStringSubmatchIndex(text, -1)
		if len(allIndices) == 0 {
			return nil
		}

		var nodes []*ResultNode
		for _, idxs := range allIndices {
			matchFullText := text[idxs[0]:idxs[1]]
			node := NewResultNode(rule.ParseItem, matchFullText)
			if len(idxs) > 2*groupIdx+1 && idxs[2*groupIdx] >= 0 {
				node.Attrs[rule.ParseItem] = text[idxs[2*groupIdx]:idxs[2*groupIdx+1]]
			}
			for _, childRule := range rule.Children {
				if !childRule.IsList && len(childRule.Children) == 0 {
					extractScalarAttr(matchFullText, childRule, node.Attrs)
				} else {
					childNodes := fillSubResult(matchFullText, childRule)
					node.Children = append(node.Children, childNodes...)
				}
			}
			nodes = append(nodes, node)
		}
		return nodes
	}

	// 场景 C：单项节点（isList = false）
	node := NewResultNode(rule.ParseItem, text)
	if rule.CompiledParseRegex != nil {
		groupIdx := rule.GroupIndex
		if groupIdx <= 0 {
			groupIdx = 1
		}
		matches := rule.CompiledParseRegex.FindStringSubmatch(text)
		if len(matches) > groupIdx {
			node.Attrs[rule.ParseItem] = matches[groupIdx]
		}
	}

	// 递归抽取挂在其下的子规则
	for _, childRule := range rule.Children {
		if !childRule.IsList && len(childRule.Children) == 0 {
			extractScalarAttr(text, childRule, node.Attrs)
		} else {
			childNodes := fillSubResult(text, childRule)
			node.Children = append(node.Children, childNodes...)
		}
	}

	return []*ResultNode{node}
}

func extractScalarAttr(text string, rule *CompiledTreeRule, attrs map[string]string) {
	if rule.CompiledParseRegex != nil {
		groupIdx := rule.GroupIndex
		if groupIdx <= 0 {
			groupIdx = 1
		}
		matches := rule.CompiledParseRegex.FindStringSubmatch(text)
		if len(matches) > groupIdx {
			attrs[rule.ParseItem] = matches[groupIdx]
			return
		}
	}
	if rule.DefaultValue != "" {
		attrs[rule.ParseItem] = rule.DefaultValue
	} else {
		attrs[rule.ParseItem] = ""
	}
}
