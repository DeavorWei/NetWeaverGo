package device

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	// 防火墙优先探测正则
	fwPriorityRegex = regexp.MustCompile(`(?i)(E1000|Eudemon|ET1D2|CE-FW|CE-IPS|NIP|IPS|AntiDDoS|ASG|SVN|USG|SeMG|LE1D2|HiSecEngine\s+Probe)`)

	// 补丁信息匹配正则（支持 Patch Package Version : V200R019SPH005 等格式）
	patchVersionRegex = regexp.MustCompile(`(?i)(?:Patch\s*(?:Package\s*)?Version|Latest\s*patch\s*version|Active\s*patch)\s*[:=]\s*(\S+)`)
	patchInlineRegex  = regexp.MustCompile(`(?i)(V\d{3}R\d{3}SP[HC]\d+)`)

	// VRBD 提取正则
	vrbdRegex = regexp.MustCompile(`(?i)(V\d{3}R\d{3}(?:[BC]\d+)?(?:SP[HC]\d+)?)`)

	// 主机名提取正则
	sysnameRegex = regexp.MustCompile(`(?m)(?:sysname|hostname)\s+([^\r\n]+)`)
)

type regHandler struct {
	pattern *regexp.Regexp
	name    string
	extract func(m []string, verText string) (model, detailType string)
}

// 华为 _REG2HANDLER 有序规则表（严格移植自 eDeskPro device.py:1057）
var reg2HandlerList = []regHandler{
	{
		pattern: regexp.MustCompile(`(?m)VRouter\S*\s+uptime`),
		name:    "VRouter",
		extract: func(m []string, verText string) (string, string) {
			return "VRouter", "VRouter"
		},
	},
	{
		pattern: regexp.MustCompile(`(?mi)(AntiDDoS)([A-Za-z0-9\-]*)\s+[^\r\n]*uptime`),
		name:    "AntiDDoS",
		extract: func(m []string, verText string) (string, string) {
			full := m[1] + m[2]
			return full, full
		},
	},
	{
		pattern: regexp.MustCompile(`(?mi)(SVN)([A-Za-z0-9\-]*)\s+[^\r\n]*uptime`),
		name:    "SVN",
		extract: func(m []string, verText string) (string, string) {
			full := m[1] + m[2]
			return full, full
		},
	},
	{
		pattern: regexp.MustCompile(`(?mi)((?:ET|LE)1D2(?:IPS|FW)0+S\d+)\s+uptime`),
		name:    "ET/LE1D2",
		extract: func(m []string, verText string) (string, string) {
			return m[1], m[1]
		},
	},
	{
		pattern: regexp.MustCompile(`(?mi)(LE1D2FW00S01)\s+uptime`),
		name:    "LE1D2FW00S01",
		extract: func(m []string, verText string) (string, string) {
			return m[1], m[1]
		},
	},
	{
		pattern: regexp.MustCompile(`(?mi)VRP.* Software,\s+Version\s+\d\.\d+,\s+(?:Release|Feature|RELEASE)\s+(\S+)`),
		name:    "VRP-Release",
		extract: func(m []string, verText string) (string, string) {
			return m[1], m[1]
		},
	},
	{
		pattern: regexp.MustCompile(`(?mi)(CE-(?:FW|IPS)A)\s+uptime`),
		name:    "CE-FW/IPS",
		extract: func(m []string, verText string) (string, string) {
			return m[1], m[1]
		},
	},
	{
		pattern: regexp.MustCompile(`(?mi)(NIP6\d+[A-Z0-9\-]*)\s+uptime`),
		name:    "NIP",
		extract: func(m []string, verText string) (string, string) {
			return m[1], m[1]
		},
	},
	{
		pattern: regexp.MustCompile(`(?mi)(IPS6\d+[A-Z0-9\-]*)\s+uptime`),
		name:    "IPS",
		extract: func(m []string, verText string) (string, string) {
			return m[1], m[1]
		},
	},
	{
		pattern: regexp.MustCompile(`(?mi)HUAWEI\s+(SRG\d{4})\s+uptime`),
		name:    "SRG",
		extract: func(m []string, verText string) (string, string) {
			return m[1], m[1]
		},
	},
	{
		pattern: regexp.MustCompile(`(?mi)VRP\s+\(R\)\s+software,\s+Version\s+\d\.\d+\s*\((SRG|AR|NE16EX)(\S*)\s+`),
		name:    "VRP-R-AR-SRG",
		extract: func(m []string, verText string) (string, string) {
			full := m[1] + m[2]
			return full, full
		},
	},
	{
		pattern: regexp.MustCompile(`(?mi)\((ME60[A-Za-z0-9\-]*)\s+(\S+)\)`),
		name:    "ME60",
		extract: func(m []string, verText string) (string, string) {
			return "ME60", m[1]
		},
	},
	{
		pattern: regexp.MustCompile(`(?mi)\((MultiserviceEngine\s+60\S*)\s+(\S+)\)`),
		name:    "MultiserviceEngine",
		extract: func(m []string, verText string) (string, string) {
			return "ME60", m[1]
		},
	},
	{
		pattern: regexp.MustCompile(`(?mi)\((NE9000[A-Za-z0-9\-]*)\s*(\S*)\)`),
		name:    "NE9000",
		extract: func(m []string, verText string) (string, string) {
			return "NE9000", m[1]
		},
	},
	{
		pattern: regexp.MustCompile(`(?mi)\((NE5000[A-Za-z0-9\-]*)\s*(\S*)\)`),
		name:    "NE5000",
		extract: func(m []string, verText string) (string, string) {
			return "NE5000E", m[1]
		},
	},
	{
		pattern: regexp.MustCompile(`(?mi)\((NetEngine|AdtecRouter|TGDataCom)\s*([86]\d00\S*)\s*(\S*)\)`),
		name:    "NetEngine8000/6000",
		extract: func(m []string, verText string) (string, string) {
			model := "NE" + m[2]
			return model, model
		},
	},
	{
		pattern: regexp.MustCompile(`(?mi)\(*\((NE40E&80E|NE40E|NE80E|CX600|NE20[E]|NE05E|CX66\S*)(?:-[A-Za-z0-9]+)?\)?\s+(\S+)\)`),
		name:    "NE40E/CX600",
		extract: func(m []string, verText string) (string, string) {
			detail := m[2]
			model := m[1]
			if strings.Contains(model, "&") {
				model = "NE40E"
			}
			return model, detail
		},
	},
	{
		pattern: regexp.MustCompile(`(?mi)\(([AE]TN\s*\S*)\s+(\S+)\)`),
		name:    "ATN/ETN",
		extract: func(m []string, verText string) (string, string) {
			model := strings.ReplaceAll(m[1], " ", "")
			return model, m[2]
		},
	},
	{
		pattern: regexp.MustCompile(`(?mi)\((PTN\s*\S+)\s+(\S+)\)`),
		name:    "PTN",
		extract: func(m []string, verText string) (string, string) {
			model := strings.ReplaceAll(m[1], " ", "")
			return model, m[2]
		},
	},
	{
		pattern: regexp.MustCompile(`(?mi)\((VNE\s*\S+)\s+(\S+)\)`),
		name:    "VNE",
		extract: func(m []string, verText string) (string, string) {
			model := strings.ReplaceAll(m[1], " ", "")
			return model, m[2]
		},
	},
	{
		pattern: regexp.MustCompile(`(?mi)HUAWEI\s+(SIG\d+)\s+uptime`),
		name:    "SIG",
		extract: func(m []string, verText string) (string, string) {
			return m[1], m[1]
		},
	},
}

// 华为 handle_else 通用兜底正则
var elsePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?mi)HUAWEI\s+(AirEngine\s*[A-Za-z0-9\-]+)\s+uptime`),
	regexp.MustCompile(`(?mi)HUAWEI\s+([A-Za-z0-9\-]+)(?:\s+(?:Main|Routing|Switch)\s+Processing\s+Unit)?[^\r\n]*(?:\r?\n\s*)?uptime`),
	regexp.MustCompile(`(?mi)HUAWEI\s+([A-Za-z0-9\-]+)\s+(?:Main|Routing|Switch)\s+Processing\s+Unit`),
	regexp.MustCompile(`(?mi)Quidway\s+([A-Za-z0-9\-]+)\s+uptime`),
	regexp.MustCompile(`(?mi)CloudEngine\s+([A-Za-z0-9\-]+)\s+uptime`),
	regexp.MustCompile(`(?mi)(AirEngine\s*[A-Za-z0-9\-]+)\s+uptime`),
	regexp.MustCompile(`(?mi)FutureMatrix\s+([A-Za-z0-9\-]+)\s+uptime`),
	regexp.MustCompile(`(?mi)eKitEngine\s+([A-Za-z0-9\-]+)\s+uptime`),
	regexp.MustCompile(`(?mi)\b(S\d{4}[A-Za-z0-9\-]*)\s+uptime`),
	regexp.MustCompile(`(?mi)\b(CE\d{4,5}[A-Za-z0-9\-]*)(?:\s+(?:Main|Routing|Switch)\s+Processing\s+Unit)?[^\r\n]*(?:\r?\n\s*)?uptime`),
	regexp.MustCompile(`(?mi)\b(CE\d{4,5}[A-Za-z0-9\-]*)\s+(?:Main|Routing|Switch)\s+Processing\s+Unit`),
	regexp.MustCompile(`(?mi)\b(AR\d{3,4}[A-Za-z0-9\-]*)\s+uptime`),
	regexp.MustCompile(`(?mi)\b(USG\d{4}[A-Za-z0-9\-]*)\s+uptime`),
	regexp.MustCompile(`(?mi)\((CE\d{4,5}[A-Za-z0-9\-]*)\s+(\S+)\)`),
}

func identifyHuawei(verText, patchText, devText string) (*Identity, error) {
	id := &Identity{
		Vendor:   "huawei",
		Evidence: make([]string, 0),
	}

	// 1. 防火墙优先判定 (fw_regx)
	if fwPriorityRegex.MatchString(verText) || fwPriorityRegex.MatchString(devText) {
		id.Evidence = append(id.Evidence, "命中防火墙优先特征 (fw_priority)")
		// 尝试从 devText 或 verText 中提取款型（devText 硬件回显优先）
		fwModelRe := regexp.MustCompile(`(?i)(USG\d{4}[A-Za-z0-9\-]*|AntiDDoS\d{4}[A-Za-z0-9\-]*|Eudemon\d+[A-Za-z0-9\-]*)`)
		searchTarget := verText
		if devText != "" {
			searchTarget = devText + "\n" + verText
		}
		if m := fwModelRe.FindStringSubmatch(searchTarget); len(m) > 1 {
			id.DetailType = m[1]
			id.Model = handleFinalModel(m[1])
			id.Evidence = append(id.Evidence, fmt.Sprintf("从安全特征提取款型: %s", id.Model))
		}
	}

	// 2. _REG2HANDLER 有序表逐条匹配
	if id.Model == "" {
		for _, handler := range reg2HandlerList {
			if m := handler.pattern.FindStringSubmatch(verText); len(m) > 0 {
				model, detail := handler.extract(m, verText)
				id.Model = handleFinalModel(model)
				id.DetailType = detail
				id.Evidence = append(id.Evidence, fmt.Sprintf("命中 _REG2HANDLER: %s (pattern=%s)", handler.name, handler.pattern.String()))
				break
			}
		}
	}

	// 3. handle_else 通用兜底
	if id.Model == "" {
		for _, re := range elsePatterns {
			if m := re.FindStringSubmatch(verText); len(m) > 1 {
				rawFound := strings.TrimSpace(m[1])
				id.DetailType = rawFound
				id.Model = handleFinalModel(rawFound)
				id.Evidence = append(id.Evidence, fmt.Sprintf("命中 handle_else: %s", rawFound))
				break
			}
		}
	}

	// 4. 版本号提取 (DEV_VERSION)
	id.Version = extractHuaweiVersion(verText)

	// 5. 补丁提取 (PATCH_VERSION)
	id.Patch = extractHuaweiPatch(patchText, verText)

	// 6. VRBD 提取
	id.VRBD = extractVRBD(id.Version, verText)

	// 7. 主机名提取
	if m := sysnameRegex.FindStringSubmatch(verText); len(m) > 1 {
		id.SysName = strings.TrimSpace(m[1])
	}

	// 8. 系列归一化 (SERIES)
	id.Series = ConvertSeries(id.Model)

	return id, nil
}

// handleFinalModel 华为款型最终归一化（对齐 handle_final 规范）
func handleFinalModel(raw string) string {
	m := strings.TrimSpace(raw)

	// 1. NE5000E 多框统一
	if strings.HasPrefix(m, "NE5000") {
		return "NE5000E"
	}

	// 2. AC6005-8 -> AC6005
	if strings.HasPrefix(m, "AC6005-") {
		return "AC6005"
	}

	// 3. AirEngine 去空格
	if strings.HasPrefix(m, "AirEngine ") {
		m = strings.Replace(m, "AirEngine ", "AirEngine", 1)
	}

	// 4. 去除多余括号包裹与后缀
	m = strings.Trim(m, "()")
	if idx := strings.Index(m, "("); idx > 0 {
		m = strings.TrimSpace(m[:idx])
	}

	// 5. 剥离 WLAN AP 的 -(FIT|CLOUD) 形态后缀
	fitCloudRe := regexp.MustCompile(`(?i)-(FIT|CLOUD)$`)
	m = fitCloudRe.ReplaceAllString(m, "")

	// 6. 去除机框后缀（例如 NE40E-X8 -> NE40E）
	chassisRe := regexp.MustCompile(`(?i)-X\d+.*$`)
	m = chassisRe.ReplaceAllString(m, "")

	// 7. CE16804/08/16 -> CE16800 款型收敛（规划方案 §6.2 P2-1 要求）
	if strings.HasPrefix(m, "CE168") {
		return "CE16800"
	}

	// 8. 提取基础型号（例如 S5735-L24P4S-A2 -> S5735）
	// 注意：\d{5} 必须置于 \d{4} 之前，避免 5 位数字型号被错误截断
	subRe := regexp.MustCompile(`^([A-Za-z]+(?:\d{5}|\d{4}|\d{3}))`)
	if sm := subRe.FindStringSubmatch(m); len(sm) > 1 {
		return sm[1]
	}

	return m
}

func extractHuaweiVersion(verText string) string {
	// VRP (R) software, Version 5.160 (S5700 V200R019C00SPC500)
	re1 := regexp.MustCompile(`(?i)Version\s+\d+\.\d+\s*\((?:[^\)]*\s+)?(V\d{3}R\d{3}[A-Za-z0-9]*)\)`)
	if m := re1.FindStringSubmatch(verText); len(m) > 1 {
		return m[1]
	}

	// Version 5.170 (V200R019C00SPC500)
	re2 := regexp.MustCompile(`(?i)\((V\d{3}R\d{3}[A-Za-z0-9]*)\)`)
	if m := re2.FindStringSubmatch(verText); len(m) > 1 {
		return m[1]
	}

	// Software, Version 8.180 (CE6800 V200R005C10SPC600)
	re3 := regexp.MustCompile(`(?i)(V\d{3}R\d{3}[A-Za-z0-9]+)`)
	if m := re3.FindStringSubmatch(verText); len(m) > 1 {
		return m[1]
	}

	return ""
}

func extractHuaweiPatch(patchText, verText string) string {
	if patchText != "" {
		if m := patchVersionRegex.FindStringSubmatch(patchText); len(m) > 1 {
			return strings.TrimSpace(m[1])
		}
	}
	if m := patchInlineRegex.FindStringSubmatch(verText); len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	return ""
}

func extractVRBD(version, verText string) string {
	if version != "" {
		if m := vrbdRegex.FindStringSubmatch(version); len(m) > 1 {
			return m[1]
		}
	}
	if m := vrbdRegex.FindStringSubmatch(verText); len(m) > 1 {
		return m[1]
	}
	return ""
}

func identifyH3C(verText, patchText, devText string) (*Identity, error) {
	id := &Identity{
		Vendor:   "h3c",
		Evidence: []string{"识别为 H3C Comware 平台"},
	}

	// H3C Comware Software, Version 7.1.070, Release 2416P02
	verRe := regexp.MustCompile(`(?i)Version\s+([0-9\.]+),\s*Release\s+([A-Za-z0-9\-]+)`)
	if m := verRe.FindStringSubmatch(verText); len(m) > 2 {
		id.Version = fmt.Sprintf("V%s-R%s", m[1], m[2])
	}

	// 优先从 uptime 行提取硬件型号，如: H3C S5500-28C-EI uptime is ...
	modelUptimeRe := regexp.MustCompile(`(?mi)H3C\s+([A-Za-z0-9\-]+)\s+uptime`)
	if m := modelUptimeRe.FindStringSubmatch(verText); len(m) > 1 {
		id.DetailType = m[1]
	} else {
		// 备选提取硬件型号，排除 Comware 等操作系统行
		modelRe := regexp.MustCompile(`(?mi)H3C\s+((?:S|WX|MSR|CR|SR|SecPath|ER|WA)\d+[A-Za-z0-9\-]*)`)
		if m := modelRe.FindStringSubmatch(verText); len(m) > 1 {
			id.DetailType = m[1]
		}
	}

	if id.DetailType != "" {
		// Model 取基础型号（如 S5500）
		subRe := regexp.MustCompile(`^([A-Za-z]+\d+)`)
		if sm := subRe.FindStringSubmatch(id.DetailType); len(sm) > 1 {
			id.Model = sm[1]
		} else {
			id.Model = id.DetailType
		}
	}

	id.Series = ConvertSeries(id.Model)
	return id, nil
}

func identifyCisco(verText, patchText, devText string) (*Identity, error) {
	id := &Identity{
		Vendor:   "cisco",
		Evidence: []string{"识别为 Cisco IOS/Nexus 平台"},
	}

	// Cisco IOS Software, ... Version 16.9.4
	verRe := regexp.MustCompile(`(?i)Version\s+([0-9\.\(\)A-Za-z]+)`)
	if m := verRe.FindStringSubmatch(verText); len(m) > 1 {
		id.Version = m[1]
	}

	// 优先匹配硬件处理器行: cisco WS-C3850-24T (MIPS) processor ...
	procRe := regexp.MustCompile(`(?mi)cisco\s+([A-Za-z0-9\-\/]+)\s+\([^\)]+\)\s+processor`)
	if m := procRe.FindStringSubmatch(verText); len(m) > 1 {
		id.DetailType = m[1]
		id.Model = m[1]
	} else {
		// 备选 Model number : WS-C3850-24T
		modelNumRe := regexp.MustCompile(`(?mi)Model\s+number\s*:\s*([A-Za-z0-9\-]+)`)
		if m := modelNumRe.FindStringSubmatch(verText); len(m) > 1 {
			id.DetailType = m[1]
			id.Model = m[1]
		} else {
			// 通用匹配 cisco 硬件款型，排除 IOS、Nexus 等纯软件名
			ciscoRe := regexp.MustCompile(`(?mi)cisco\s+((?:WS-C|C\d|ASR|ISR|N\d|Nexus|Catalyst)[A-Za-z0-9\-]*)`)
			if m := ciscoRe.FindStringSubmatch(verText); len(m) > 1 {
				id.DetailType = m[1]
				id.Model = m[1]
			}
		}
	}

	id.Series = ConvertSeries(id.Model)
	return id, nil
}

func identifyGeneric(vendor, verText string) (*Identity, error) {
	id := &Identity{
		Vendor:   vendor,
		Evidence: []string{fmt.Sprintf("通用识别兜底: vendor=%s", vendor)},
	}
	verRe := regexp.MustCompile(`(?i)(?:version|release)\s*[:= ]*\s*([0-9A-Za-z\.\-_]+)`)
	if m := verRe.FindStringSubmatch(verText); len(m) > 1 {
		id.Version = m[1]
	}
	id.Series = ConvertSeries(id.Model)
	return id, nil
}
