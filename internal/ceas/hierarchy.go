package ceas

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	reSlotClean     = regexp.MustCompile(`(?i)^(?:slot|unit)_?`)
	reCardIndex     = regexp.MustCompile(`(?i)(?:card|daughter_board|sub_board|board)_?(\d+)`)
	rePortIndex     = regexp.MustCompile(`(?i)(?:port|interface)_?(\d+)`)
	reChassisSlot   = regexp.MustCompile(`^(\d+)/(\d+)$`)
)

// GenerateNodeID 生成层级 ID (对齐 CEAS data.csv 语义，如 0, 0_1, 0_1_2)
func GenerateNodeID(parentID string, index int) string {
	if parentID == "" {
		return fmt.Sprintf("%d", index)
	}
	return fmt.Sprintf("%s_%d", parentID, index)
}

// NormalizeSlot 归一化槽位标识 (如 Slot_1 -> 1, Unit_2 -> 2, Slot_1/3 -> 1/3)
func NormalizeSlot(slotRaw string) string {
	s := strings.TrimSpace(slotRaw)
	s = reSlotClean.ReplaceAllString(s, "")
	return s
}

// Format3SegmentPath 生成新一代 3 段式物理位置路径 (ELabelParserPortBased)
// 规范格式：
// - 机框: "0" 或 "1"
// - 槽位/主板: "X/0" (如 Slot 1 -> "1/0")
// - 子卡/扣板: "X/Y" (如 Slot 1 下的 Card 1 -> "1/1")
// - 端口: "X/Y/Z" (如 Slot 1 子卡 1 下的 Port 2 -> "1/1/2")
func Format3SegmentPath(nodeType, nodeName, slot, parentPath string) string {
	t := strings.ToLower(strings.TrimSpace(nodeType))
	name := strings.TrimSpace(nodeName)
	cleanSlot := NormalizeSlot(slot)

	switch t {
	case "frame":
		// 机框直接返回机框号 (默认 0 或解析 BackPlane 编号)
		if strings.Contains(name, "_") {
			parts := strings.Split(name, "_")
			if len(parts) > 1 && parts[1] != "" {
				return parts[1]
			}
		}
		return "0"

	case "slot":
		if cleanSlot == "" {
			cleanSlot = "0"
		}
		// 特例：如果是堆叠/框式复合槽位如 1/3
		if reChassisSlot.MatchString(cleanSlot) {
			return cleanSlot + "/0"
		}
		return cleanSlot + "/0"

	case "mainboard", "motherboard":
		if cleanSlot != "" {
			return cleanSlot + "/0"
		}
		return "0/0"

	case "daughterboard", "card", "ofccard", "subcard":
		subIndex := "1"
		if strings.Contains(name, "/") {
			parts := strings.Split(name, "/")
			if len(parts) > 1 && parts[len(parts)-1] != "" {
				subIndex = parts[len(parts)-1]
			}
		} else if m := reCardIndex.FindStringSubmatch(name); len(m) > 1 {
			subIndex = m[1]
		}
		// 若父级已有三段式路径（如 1/0），替换次级槽位
		if parentPath != "" && strings.Contains(parentPath, "/") {
			parts := strings.Split(parentPath, "/")
			if len(parts) >= 2 {
				return fmt.Sprintf("%s/%s", parts[0], subIndex)
			}
		}
		if cleanSlot != "" {
			return fmt.Sprintf("%s/%s", cleanSlot, subIndex)
		}
		return fmt.Sprintf("0/%s", subIndex)

	case "port":
		portIndex := "1"
		if m := rePortIndex.FindStringSubmatch(name); len(m) > 1 {
			portIndex = m[1]
		}
		// 若父级已有路径如 1/0 或 1/1，追加端口段 -> 1/0/1 或 1/1/1
		if parentPath != "" {
			parts := strings.Split(parentPath, "/")
			if len(parts) == 2 {
				return fmt.Sprintf("%s/%s/%s", parts[0], parts[1], portIndex)
			} else if len(parts) >= 3 {
				return fmt.Sprintf("%s/%s/%s", parts[0], parts[1], portIndex)
			}
		}
		if cleanSlot != "" {
			return fmt.Sprintf("%s/0/%s", cleanSlot, portIndex)
		}
		return fmt.Sprintf("0/0/%s", portIndex)

	case "fanframe", "power":
		index := "1"
		if strings.Contains(name, "_") {
			parts := strings.Split(name, "_")
			if len(parts) > 1 && parts[1] != "" {
				index = parts[1]
			}
		}
		return fmt.Sprintf("%s/%s", t, index)

	default:
		if parentPath != "" {
			return parentPath
		}
		if cleanSlot != "" {
			return cleanSlot + "/0"
		}
		return "0"
	}
}

// BuildHardwareTree 根据扁平节点列表组装层级树
func BuildHardwareTree(deviceIP, chassisESN string, nodes []*Node) *HardwareTree {
	nodeMap := make(map[string]*Node, len(nodes))
	for _, n := range nodes {
		nodeMap[n.ID] = n
	}

	var roots []*Node
	for _, n := range nodes {
		if n.ParentID == "" {
			roots = append(roots, n)
		} else if parent, ok := nodeMap[n.ParentID]; ok {
			parent.Children = append(parent.Children, n)
		} else {
			// 若找不到父节点，作为顶级根节点防御
			roots = append(roots, n)
		}
	}

	return &HardwareTree{
		DeviceIP:   deviceIP,
		ChassisESN: chassisESN,
		Roots:      roots,
		AllNodes:   nodes,
	}
}

// ConvertTreeToVO 转换树为前端 VO 对象
func ConvertTreeToVO(tree *HardwareTree) *HardwareTreeVO {
	if tree == nil {
		return nil
	}
	vo := &HardwareTreeVO{
		DeviceIP:   tree.DeviceIP,
		ChassisESN: tree.ChassisESN,
		TotalNodes: len(tree.AllNodes),
		Roots:      make([]*NodeVO, 0, len(tree.Roots)),
	}

	for _, r := range tree.Roots {
		vo.Roots = append(vo.Roots, convertNodeToVO(r))
	}
	return vo
}

func convertNodeToVO(n *Node) *NodeVO {
	if n == nil {
		return nil
	}
	vo := &NodeVO{
		ID:           n.ID,
		ParentID:     n.ParentID,
		Level:        n.Level,
		Type:         n.Type,
		Name:         n.Name,
		Path:         n.Path,
		Slot:         n.Slot,
		Item:         n.Item,
		BarCode:      n.BarCode,
		Description:  n.Description,
		Manufactured: n.Manufactured,
		VendorName:   n.VendorName,
		BoardType:    n.BoardType,
		Attrs:        n.Attrs,
		Children:     make([]*NodeVO, 0, len(n.Children)),
	}
	for _, child := range n.Children {
		vo.Children = append(vo.Children, convertNodeToVO(child))
	}
	return vo
}
