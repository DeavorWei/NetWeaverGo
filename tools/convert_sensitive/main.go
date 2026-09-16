package main

import (
	"encoding/json"
	"encoding/xml"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type SensitiveCmdXML struct {
	XMLName    xml.Name      `xml:"SensitiveCmd"`
	Categories []CategoryXML `xml:"Category"`
}

type CategoryXML struct {
	Name string   `xml:"name,attr"`
	Cmds []CmdXML `xml:"Cmd"`
}

type CmdXML struct {
	Name    string      `xml:"name,attr"`
	Filters []FilterXML `xml:"Filter"`
}

type FilterXML struct {
	Name string `xml:"name,attr"`
}

// RuleExport JSON 导出结构
type RuleExport struct {
	Category    string   `json:"category"`
	Commands    []string `json:"commands"`
	RawPattern  string   `json:"pattern"`
	Replacement string   `json:"replacement"`
	IsRegex     bool     `json:"isRegex"`
}

var reUnicodeEscape = regexp.MustCompile(`\\u([0-9a-fA-F]{4})`)

func normalizePatternForRE2(pat string) string {
	// Go RE2 使用 \x{4e00} 代替 \u4e00
	return reUnicodeEscape.ReplaceAllString(pat, `\x{$1}`)
}

func main() {
	xmlPath := flag.String("xml", `D:\ICSLite_download\eDesk_Pro_V100R025C10SPC300\eDeskPro_V100R025C10SPC300-windows-x64\config\deviceversion\sensitiveCmd.xml`, "sensitiveCmd.xml 路径")
	outPath := flag.String("out", `internal/config/sanitize_rules/sensitive_cmd.json`, "输出 JSON 路径")
	flag.Parse()

	fmt.Println("==================================================")
	fmt.Println("  敏感命令脱敏规则转换器 (tools/convert_sensitive)")
	fmt.Println("==================================================")

	data, err := os.ReadFile(*xmlPath)
	if err != nil {
		fmt.Printf("读取 XML 文件失败: %v\n", err)
		os.Exit(1)
	}

	var root SensitiveCmdXML
	if err := xml.Unmarshal(data, &root); err != nil {
		fmt.Printf("XML 解析失败: %v\n", err)
		os.Exit(1)
	}

	var allRules []RuleExport
	totalCategories := len(root.Categories)
	totalRawFilters := 0
	validCompileCount := 0
	brokenCount := 0

	for _, cat := range root.Categories {
		catName := strings.TrimSpace(cat.Name)
		for _, cmd := range cat.Cmds {
			rawCmds := strings.Split(cmd.Name, ",,,")
			var cleanedCmds []string
			for _, c := range rawCmds {
				c = strings.TrimSpace(c)
				if c != "" {
					cleanedCmds = append(cleanedCmds, c)
				}
			}

			for _, filter := range cmd.Filters {
				pattern := strings.TrimSpace(filter.Name)
				if pattern == "" {
					continue
				}
				totalRawFilters++

				normalized := normalizePatternForRE2(pattern)
				_, compErr := regexp.Compile(normalized)
				isRegex := (compErr == nil)
				if isRegex {
					validCompileCount++
				} else {
					brokenCount++
					fmt.Printf("[警告] RE2 编译不兼容规则: [%s] -> %s (Err: %v)\n", catName, pattern, compErr)
				}

				allRules = append(allRules, RuleExport{
					Category:    catName,
					Commands:    cleanedCmds,
					RawPattern:  normalized,
					Replacement: "****",
					IsRegex:     isRegex,
				})
			}
		}
	}

	fmt.Printf("解析统计: 包含 %d 个 Category, %d 条脱敏规则 (编译成功: %d, 失败: %d，成功率: %.2f%%)\n",
		totalCategories, totalRawFilters, validCompileCount, brokenCount, float64(validCompileCount)*100/float64(totalRawFilters))

	_ = os.MkdirAll(filepath.Dir(*outPath), 0755)
	outBytes, err := json.MarshalIndent(allRules, "", "  ")
	if err != nil {
		fmt.Printf("JSON 序列化失败: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(*outPath, outBytes, 0644); err != nil {
		fmt.Printf("写入输出文件失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("规则已成功转换并写入: %s\n", *outPath)
}
