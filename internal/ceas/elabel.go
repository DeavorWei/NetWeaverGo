package ceas

import (
	"bufio"
	"fmt"
	"regexp"
	"strings"
)

// 块头识别 Handler
type blockHandler struct {
	nodeType string
	regex    *regexp.Regexp
	level    int
}

var (
	// 块切分正则：匹配所有已知块头标志
	reBlockSplit = regexp.MustCompile(`(?m)^\[(?:BackPlane|Slot|Unit|FanSlot|FanFrame|FAN|PowerFrame|PWR|Main_Board|Mother_Board|Daughter_Board|OfcCard|Port)[^\]\r\n]*\]`)

	// 块头有序识别总表（严格按优先级排序，共 10 类）
	blockHandlers = []blockHandler{
		// 1. frame (BackPlane_X)
		{nodeType: "frame", regex: regexp.MustCompile(`(?i)^\[BackPlane_?(\d+)\]`), level: 1},
		// 2. fanframe (FanSlot_X, FanFrame_X, FAN_X)
		{nodeType: "fanframe", regex: regexp.MustCompile(`(?i)^\[\S*((?:FanSlot|FanFrame|FAN)_?(\S+))\]`), level: 2},
		// 3. power (PowerFrame_X, PWR_X)
		{nodeType: "power", regex: regexp.MustCompile(`(?i)^\[\S*((?:PowerFrame|PWR)_?(\S+))\]`), level: 2},
		// 4. mainboard (Main_Board)
		{nodeType: "mainboard", regex: regexp.MustCompile(`(?i)^\[Main_Board[^\d\r\n]*(\d*)\]`), level: 3},
		// 5. motherboard (Mother_Board)
		{nodeType: "motherboard", regex: regexp.MustCompile(`(?i)^\[Mother_Board[^\d\r\n]*(\d*)\]`), level: 3},
		// 6. daughterboard (Daughter_Board_X)
		{nodeType: "daughterboard", regex: regexp.MustCompile(`(?i)^\[(Daughter_Board_[^\]]+)\]`), level: 3},
		// 7. ofccard (OfcCard_X)
		{nodeType: "ofccard", regex: regexp.MustCompile(`(?i)^\[(OfcCard_[^\]]+)\]`), level: 3},
		// 8. port (Port_X)
		{nodeType: "port", regex: regexp.MustCompile(`(?i)^\[(Port_\S+)\]`), level: 3},
		// 9. card (Slot_X Card_Y, Card_X)
		{nodeType: "card", regex: regexp.MustCompile(`(?i)^\[Slot_?\d\S*\s*Card_?(?:\S*\d+/)?(\d+)\]`), level: 3},
		// 10. slot (Slot_X, Unit_X)
		{nodeType: "slot", regex: regexp.MustCompile(`(?i)^\[(?:Slot_|Unit_)(\S+)\]`), level: 2},
	}

	// 属性段标志正则（用于 handle_extra_properties 切分）
	rePropHeader = regexp.MustCompile(`(?m)^\[(?:Board|Sub_Board|DaughterBoard|Daughter_Board|[A-Za-z0-9_]+)\s+Properties\]`)

	// 键值对提取正则
	reKeyValue = regexp.MustCompile(`^\s*([A-Za-z0-9_]+)\s*[:=]\s*(.*?)\s*$`)
)

// RawBlock 拆分出的单个原始文本块
type RawBlock struct {
	Header string
	Lines  []string
}

// SplitBlocks 切分 elabel 文本块，保留块头，尾块不丢
func SplitBlocks(rawText string) []RawBlock {
	var blocks []RawBlock
	scanner := bufio.NewScanner(strings.NewReader(rawText))

	var currentBlock *RawBlock

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if reBlockSplit.MatchString(trimmed) {
			if currentBlock != nil {
				blocks = append(blocks, *currentBlock)
			}
			currentBlock = &RawBlock{
				Header: trimmed,
				Lines:  []string{trimmed},
			}
		} else if currentBlock != nil {
			currentBlock.Lines = append(currentBlock.Lines, line)
		}
	}

	if currentBlock != nil {
		blocks = append(blocks, *currentBlock)
	}
	return blocks
}

// ParseELabel 核心专用 elabel 解析器
func ParseELabel(rawElabel string) *HardwareTree {
	blocks := SplitBlocks(rawElabel)

	var allNodes []*Node
	var rootFrame *Node
	lastSlotNode := (*Node)(nil)
	lastSlot := ""
	nodeIndex := 0

	// 保证根机框始终存在
	getOrCreateRootFrame := func() *Node {
		if rootFrame == nil {
			rootFrame = &Node{
				ID:       GenerateNodeID("", nodeIndex),
				ParentID: "",
				Level:    1,
				Type:     "frame",
				Name:     "BackPlane_0",
				Path:     "0",
				Attrs:    make(map[string]string),
			}
			nodeIndex++
			allNodes = append(allNodes, rootFrame)
		}
		return rootFrame
	}

	for _, block := range blocks {
		nodeType, nodeName, level, matchedSlot := matchBlockHeader(block.Header)
		if nodeType == "" {
			continue
		}

		// 归一化 slot
		if matchedSlot != "" {
			lastSlot = NormalizeSlot(matchedSlot)
		}

		// 检查块内是否包含多个属性段 (handle_extra_properties 拆卡机制)
		subSections := splitSubSections(block.Lines)

		if len(subSections) <= 1 {
			// 单个属性段，正常生成节点
			attrs := parseAttributes(block.Lines)
			node := createNode(nodeType, nodeName, level, lastSlot, attrs)
			attachNode(getOrCreateRootFrame, &rootFrame, &lastSlotNode, node, &nodeIndex, &allNodes)
		} else {
			// 包含多个属性段（handle_extra_properties：主板 + 子板/扣板）
			// 第一个属性段作为主节点
			mainAttrs := parseAttributes(subSections[0].lines)
			mainNode := createNode(nodeType, nodeName, level, lastSlot, mainAttrs)
			attachNode(getOrCreateRootFrame, &rootFrame, &lastSlotNode, mainNode, &nodeIndex, &allNodes)

			// 后续属性段作为其子节点 (daughterboard / subcard)
			for i := 1; i < len(subSections); i++ {
				sec := subSections[i]
				subAttrs := parseAttributes(sec.lines)
				subName := sec.header
				if subName == "" {
					subName = fmt.Sprintf("Sub_Board_%d", i)
				}
				subNode := &Node{
					Level:        mainNode.Level + 1,
					Type:         "daughterboard",
					Name:         subName,
					Slot:         mainNode.Slot,
					Attrs:        subAttrs,
					Item:         subAttrs["Item"],
					BarCode:      subAttrs["BarCode"],
					Description:  subAttrs["Description"],
					Manufactured: subAttrs["Manufactured"],
					VendorName:   subAttrs["VendorName"],
					BoardType:    subAttrs["BoardType"],
				}
				subNode.ParentID = mainNode.ID
				subNode.ID = GenerateNodeID(mainNode.ID, i)
				subNode.Path = Format3SegmentPath("daughterboard", subName, subNode.Slot, mainNode.Path)
				allNodes = append(allNodes, subNode)
			}
		}
	}

	// 最终构建树
	chassisESN := extractChassisESNFromNodes(allNodes)
	return BuildHardwareTree("", chassisESN, allNodes)
}

func matchBlockHeader(header string) (nodeType string, nodeName string, level int, slot string) {
	for _, h := range blockHandlers {
		if m := h.regex.FindStringSubmatch(header); len(m) > 0 {
			nodeType = h.nodeType
			level = h.level
			nodeName = strings.Trim(header, "[]")

			if len(m) > 1 && m[1] != "" {
				if h.nodeType == "slot" || h.nodeType == "card" {
					slot = m[1]
				} else if h.nodeType == "frame" {
					nodeName = "BackPlane_" + m[1]
				}
			}
			return
		}
	}
	return "", "", 0, ""
}

func createNode(nodeType, nodeName string, level int, slot string, attrs map[string]string) *Node {
	return &Node{
		Level:        level,
		Type:         nodeType,
		Name:         nodeName,
		Slot:         slot,
		Item:         attrs["Item"],
		BarCode:      attrs["BarCode"],
		Description:  attrs["Description"],
		Manufactured: attrs["Manufactured"],
		VendorName:   attrs["VendorName"],
		BoardType:    attrs["BoardType"],
		Attrs:        attrs,
	}
}

func attachNode(getRoot func() *Node, rootFrame **Node, lastSlotNode **Node, node *Node, nodeIndex *int, allNodes *[]*Node) {
	switch node.Type {
	case "frame":
		// 机框节点作为顶级节点
		node.ParentID = ""
		node.ID = GenerateNodeID("", *nodeIndex)
		node.Path = Format3SegmentPath("frame", node.Name, node.Slot, "")
		*nodeIndex++
		*rootFrame = node
		*allNodes = append(*allNodes, node)

	case "slot", "fanframe", "power":
		// 槽位/风扇框/电源框挂在根机框下
		parent := getRoot()
		node.ParentID = parent.ID
		node.ID = GenerateNodeID(parent.ID, len(*allNodes))
		node.Path = Format3SegmentPath(node.Type, node.Name, node.Slot, parent.Path)
		if node.Type == "slot" {
			*lastSlotNode = node
		}
		*allNodes = append(*allNodes, node)

	default:
		// 单板、子卡、端口优先挂在上一个 slot 下；若无 slot，则直接挂在根机框下
		var parent *Node
		if *lastSlotNode != nil {
			parent = *lastSlotNode
		} else {
			parent = getRoot()
		}
		node.ParentID = parent.ID
		node.ID = GenerateNodeID(parent.ID, len(*allNodes))
		node.Path = Format3SegmentPath(node.Type, node.Name, node.Slot, parent.Path)
		*allNodes = append(*allNodes, node)
	}
}

type subSection struct {
	header string
	lines  []string
}

// splitSubSections 实现 handle_extra_properties，切分多段属性
func splitSubSections(lines []string) []subSection {
	var sections []subSection
	var currentSec *subSection

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if rePropHeader.MatchString(trimmed) {
			if currentSec != nil && len(currentSec.lines) > 0 {
				sections = append(sections, *currentSec)
			}
			currentSec = &subSection{
				header: strings.Trim(trimmed, "[]"),
				lines:  []string{line},
			}
		} else {
			if currentSec == nil {
				currentSec = &subSection{lines: []string{}}
			}
			currentSec.lines = append(currentSec.lines, line)
		}
	}
	if currentSec != nil && len(currentSec.lines) > 0 {
		sections = append(sections, *currentSec)
	}
	return sections
}

// parseAttributes 提取块内键值对属性
func parseAttributes(lines []string) map[string]string {
	attrs := make(map[string]string)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "[") || strings.HasPrefix(trimmed, "/$") {
			continue
		}
		if m := reKeyValue.FindStringSubmatch(trimmed); len(m) > 2 {
			k := strings.TrimSpace(m[1])
			v := strings.TrimSpace(m[2])
			attrs[k] = v
		}
	}
	return attrs
}

func extractChassisESNFromNodes(nodes []*Node) string {
	// 优先从 BackPlane 取 BarCode
	for _, n := range nodes {
		if n.Type == "frame" && n.BarCode != "" {
			return n.BarCode
		}
	}
	// 兜底从 0 号主板取
	for _, n := range nodes {
		if (n.Type == "mainboard" || n.Type == "motherboard") && n.BarCode != "" {
			return n.BarCode
		}
	}
	// 兜底从第一个非空且长度 >= 8 的 BarCode 取
	for _, n := range nodes {
		if len(n.BarCode) >= 8 {
			return n.BarCode
		}
	}
	return ""
}
