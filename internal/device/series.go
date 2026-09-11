package device

import (
	"regexp"
	"strings"
)

var (
	seriesPattern = regexp.MustCompile(`^(?:(E6)|(AR|AC|AP|AD|R)|(CE|FM|S|AirEngine\s*))(\d+)`)
	nePattern     = regexp.MustCompile(`^(?:(NE|NetEngine\s*))(\d+)`)
	usgPattern    = regexp.MustCompile(`^(?:(USG|AntiDDoS|SecoPath))(\d+)`)
)

// ConvertSeries 将具体款型归一化为产品系列名称
// 严格对齐 eDeskPro collect_ceas.convert_series 算法规范
func ConvertSeries(model string) string {
	raw := strings.TrimSpace(model)
	if raw == "" {
		return ""
	}

	// 特例：包含 9700D 后缀
	is9700D := strings.Contains(strings.ToUpper(raw), "9700D")

	// 1. 标准系列正则匹配：^(?:(E6)|(AR|AC|AP|AD|R)|(CE|FM|S|AirEngine *))(\d+)
	if m := seriesPattern.FindStringSubmatch(raw); len(m) > 4 {
		g1 := m[1] // E6
		g2 := m[2] // AR, AC, AP, AD, R
		g3 := m[3] // CE, FM, S, AirEngine
		num := m[4]

		var prefix string
		var index int

		if g1 != "" {
			// 第 1 组（E6）：前缀为 E，6 连同后置数字全转 0
			return "E" + strings.Repeat("0", len(num)+1)
		} else if g2 != "" {
			// 第 2 组（AR|AC|AP|AD|R）：保留首位数字，其余转 0
			prefix = g2
			index = 1
		} else if g3 != "" {
			// 第 3 组（CE|FM|S|AirEngine）：基准保留前 2 位数字，超出 4 位加偏移
			prefix = strings.TrimSpace(g3)
			index = 2
			offset := len(num) - 4
			if offset > 0 {
				index += offset
			}
		}

		if index > len(num) {
			index = len(num)
		}

		seriesNum := num[:index] + strings.Repeat("0", len(num)-index)
		series := prefix + seriesNum

		if is9700D && !strings.HasSuffix(series, "D") {
			series += "D"
		}
		return series
	}

	// 2. NE / NetEngine 路由器扩展支持
	if m := nePattern.FindStringSubmatch(raw); len(m) > 2 {
		num := m[2]
		// 如果是 NE40E, NE20E, NE80E, NE05E 等 2 位数带 E 型号，系列归一保持为 NE40E 等
		if len(num) <= 2 && strings.HasSuffix(strings.ToUpper(raw), "E") {
			return "NE" + num + "E"
		}
		index := 2
		offset := len(num) - 4
		if offset > 0 {
			index += offset
		}
		if index > len(num) {
			index = len(num)
		}
		seriesNum := num[:index] + strings.Repeat("0", len(num)-index)
		return "NE" + seriesNum
	}

	// 3. USG / 安全防火墙系列支持
	if m := usgPattern.FindStringSubmatch(raw); len(m) > 2 {
		prefix := m[1]
		num := m[2]
		index := 1
		if len(num) >= 4 {
			index = 1
		}
		seriesNum := num[:index] + strings.Repeat("0", len(num)-index)
		return prefix + seriesNum
	}

	// 4. 未匹配时兜底返回原始款型基础名（去除板卡/子款型后缀）
	idx := strings.IndexAny(raw, "-_ /")
	if idx > 0 {
		return raw[:idx]
	}
	return raw
}
