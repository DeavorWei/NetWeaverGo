package ceas

import (
	"regexp"
	"strings"
)

var (
	// CLI display esn 常见回显正则
	reESNPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)ESN of (?:master|device|chassis|slot \d+)\s*:\s*([A-Za-z0-9_-]+)`),
		regexp.MustCompile(`(?i)(?:Equipment|Device)\s+Serial\s+Number\s*:\s*([A-Za-z0-9_-]+)`),
		regexp.MustCompile(`(?i)ESN\s*:\s*([A-Za-z0-9_-]+)`),
		regexp.MustCompile(`(?i)SN\s*:\s*([A-Za-z0-9_-]+)`),
	}

	// elabel 回显中的 BarCode 快速抽取
	reElabelBarCode = regexp.MustCompile(`(?i)BarCode\s*=\s*([A-Za-z0-9_-]+)`)
	reBackPlaneBlock = regexp.MustCompile(`(?is)\[BackPlane_\d+\](.*?)(?:\[\S+\]|\z)`)
	reMainBoardBlock = regexp.MustCompile(`(?is)\[(?:Main|Mother)_Board[^\d\r\n]*\d*\](.*?)(?:\[\S+\]|\z)`)
)

// ExtractESN 从 display esn 与 display elabel 联合提取设备主序列号
// 支持 CE / AR / WLAN / Route 及通用多策略回退
func ExtractESN(vendor, series, rawElabel, rawESN string) string {
	// 1. 优先从专用 display esn 命令回显中提取
	if strings.TrimSpace(rawESN) != "" {
		for _, re := range reESNPatterns {
			if m := re.FindStringSubmatch(rawESN); len(m) > 1 {
				esn := cleanESN(m[1])
				if isValidESN(esn) {
					return esn
				}
			}
		}
	}

	// 2. 若专用命令未能提取，根据产品系列从 elabel 针对性提取
	upperSeries := strings.ToUpper(strings.TrimSpace(series))
	if strings.TrimSpace(rawElabel) != "" {
		// CE 数据中心系列与核心交换机：优先机框背板 BackPlane
		if strings.HasPrefix(upperSeries, "CE") || strings.HasPrefix(upperSeries, "S") {
			if m := reBackPlaneBlock.FindStringSubmatch(rawElabel); len(m) > 1 {
				if bm := reElabelBarCode.FindStringSubmatch(m[1]); len(bm) > 1 {
					esn := cleanESN(bm[1])
					if isValidESN(esn) {
						return esn
					}
				}
			}
		}

		// AR/NE 路由器或盒式设备：优先主控板/主板
		if strings.HasPrefix(upperSeries, "AR") || strings.HasPrefix(upperSeries, "NE") || strings.HasPrefix(upperSeries, "USG") {
			if m := reMainBoardBlock.FindStringSubmatch(rawElabel); len(m) > 1 {
				if bm := reElabelBarCode.FindStringSubmatch(m[1]); len(bm) > 1 {
					esn := cleanESN(bm[1])
					if isValidESN(esn) {
						return esn
					}
				}
			}
		}

		// 通用兜底：BackPlane -> MainBoard -> 首个有效 BarCode
		if m := reBackPlaneBlock.FindStringSubmatch(rawElabel); len(m) > 1 {
			if bm := reElabelBarCode.FindStringSubmatch(m[1]); len(bm) > 1 {
				esn := cleanESN(bm[1])
				if isValidESN(esn) {
					return esn
				}
			}
		}
		if m := reMainBoardBlock.FindStringSubmatch(rawElabel); len(m) > 1 {
			if bm := reElabelBarCode.FindStringSubmatch(m[1]); len(bm) > 1 {
				esn := cleanESN(bm[1])
				if isValidESN(esn) {
					return esn
				}
			}
		}

		// 终极遍历兜底
		allMatches := reElabelBarCode.FindAllStringSubmatch(rawElabel, -1)
		for _, bm := range allMatches {
			if len(bm) > 1 {
				esn := cleanESN(bm[1])
				if isValidESN(esn) {
					return esn
				}
			}
		}
	}

	return ""
}

func cleanESN(raw string) string {
	s := strings.TrimSpace(raw)
	s = strings.Trim(s, `"'`)
	return s
}

func isValidESN(esn string) bool {
	if len(esn) < 6 || len(esn) > 64 {
		return false
	}
	upper := strings.ToUpper(esn)
	if upper == "NULL" || upper == "NA" || upper == "NONE" || upper == "UNKNOWN" || strings.HasPrefix(upper, "000000") {
		return false
	}
	return true
}
