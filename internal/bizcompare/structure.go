package bizcompare

import (
	"fmt"
	"regexp"
	"strings"
)

// 结构化键值行模式：`key: value` / `key = value` / `"key": "value"`
var (
	kvJSONRe     = regexp.MustCompile(`^"([A-Za-z0-9_\-\.\/]{1,48})"\s*:\s*(.+?),?$`)
	kvColonRe    = regexp.MustCompile(`^([A-Za-z0-9_\-\.\/]{1,48})\s*[:=]\s*(.+)$`)
	multiSpaceRe = regexp.MustCompile(`\s{2,}`)
)

// ExtractStructuredItems 将命令回显提取为结构化快照项（P1-3）。
//
// 语义对齐方案 §3.5.2 的"采集项→命令→模板→归一化"：
//  1. 显式 key: value / key = value 行 → `命令键.字段名` 归一化指标项；
//  2. 其余有效行 → 折叠空白后的整行项（消除纯格式噪声造成的假差异）；
//  3. 跳过空行、提示符行与命令行回显行。
func ExtractStructuredItems(commandKey, category string, lines []string) []SnapshotItem {
	items := make([]SnapshotItem, 0, len(lines))
	counters := make(map[string]int, len(lines))
	prefix := strings.TrimSpace(commandKey) + "."

	add := func(field, value string) {
		base := prefix + field
		n := counters[base]
		counters[base] = n + 1
		key := base
		if n > 0 {
			key = fmt.Sprintf("%s#%d", base, n+1)
		}
		items = append(items, SnapshotItem{
			Key:      key,
			Category: category,
			Value:    value,
		})
	}

	for _, raw := range lines {
		line := normalizeEchoLine(raw)
		if line == "" {
			continue
		}
		if field, value, ok := parseKeyValueLine(line); ok {
			add(field, value)
			continue
		}
		add(line, line)
	}
	return items
}

// normalizeEchoLine 折叠空白并过滤提示符/命令行回显行
func normalizeEchoLine(raw string) string {
	line := strings.TrimSpace(strings.ReplaceAll(raw, "\t", " "))
	if line == "" {
		return ""
	}
	line = multiSpaceRe.ReplaceAllString(line, " ")
	if strings.HasPrefix(line, "<") || strings.HasPrefix(line, "[") {
		return ""
	}
	if (strings.HasSuffix(line, "#") || strings.HasSuffix(line, ">")) && !strings.Contains(line, " ") {
		return ""
	}
	return line
}

// parseKeyValueLine 解析显式键值行（JSON 风格优先）
func parseKeyValueLine(line string) (string, string, bool) {
	if m := kvJSONRe.FindStringSubmatch(line); len(m) == 3 {
		value := strings.Trim(strings.TrimSpace(m[2]), `",`)
		if value == "" {
			return "", "", false
		}
		return strings.Trim(m[1], `"`), value, true
	}
	if m := kvColonRe.FindStringSubmatch(line); len(m) == 3 {
		value := strings.TrimSpace(m[2])
		if value == "" {
			return "", "", false
		}
		return strings.TrimSpace(m[1]), value, true
	}
	return "", "", false
}
