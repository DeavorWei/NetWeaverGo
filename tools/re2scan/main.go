package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Issue 表示正则不兼容项
type Issue struct {
	Source      string
	Pattern     string
	Feature     string
	Description string
}

// IncompatiblePatterns 定义不支持的 Python/PCRE 正则特征
var IncompatiblePatterns = []struct {
	Feature     string
	Regex       *regexp.Regexp
	Description string
}{
	{
		Feature:     "Positive Lookbehind (?<=...)",
		Regex:       regexp.MustCompile(`\(\?<=`),
		Description: "Go RE2 不支持正向后行断言。需改写为普通捕获组在代码中取值，或利用前后定界分块处理。",
	},
	{
		Feature:     "Negative Lookbehind (?<!...)",
		Regex:       regexp.MustCompile(`\(\?<!`),
		Description: "Go RE2 不支持负向后行断言。需改写逻辑或在匹配后过滤。",
	},
	{
		Feature:     "Positive Lookahead (?=...)",
		Regex:       regexp.MustCompile(`\(\?=`),
		Description: "Go RE2 不支持正向先行断言。需改写为普通捕获组。",
	},
	{
		Feature:     "Negative Lookahead (?!...)",
		Regex:       regexp.MustCompile(`\(\?!`),
		Description: "Go RE2 不支持负向先行断言。需改写为字符集排除或匹配后代码过滤。",
	},
	{
		Feature:     "Backreference (\\1, \\2...)",
		Regex:       regexp.MustCompile(`\\[1-9]`),
		Description: "Go RE2 不支持反向引用。需两次匹配比对或代码内判定。",
	},
	{
		Feature:     "Named Backreference (?P=...)",
		Regex:       regexp.MustCompile(`\(\?P=[a-zA-Z0-9_]+\)`),
		Description: "Go RE2 不支持命名反向引用。",
	},
	{
		Feature:     "Conditional Pattern (?(id/name))",
		Regex:       regexp.MustCompile(`\(\?\([a-zA-Z0-9_]+\)`),
		Description: "Go RE2 不支持条件匹配分支。需拆分为两条规则。",
	},
}

// ScanPattern 检查单个正则表达式
func ScanPattern(source, pattern string) []Issue {
	var issues []Issue
	for _, check := range IncompatiblePatterns {
		if check.Regex.MatchString(pattern) {
			issues = append(issues, Issue{
				Source:      source,
				Pattern:     pattern,
				Feature:     check.Feature,
				Description: check.Description,
			})
		}
	}

	// 尝试用 regexp.Compile 验证
	if _, err := regexp.Compile(pattern); err != nil {
		if len(issues) == 0 {
			issues = append(issues, Issue{
				Source:      source,
				Pattern:     pattern,
				Feature:     "Syntax Error in RE2",
				Description: fmt.Sprintf("RE2 编译失败: %v", err),
			})
		}
	}
	return issues
}

// ScanFile 扫描单个文件中的所有正则
func ScanFile(filePath string) ([]Issue, int) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, 0
	}

	ext := strings.ToLower(filepath.Ext(filePath))
	patterns := make(map[string]string)

	if ext == ".json" {
		var obj interface{}
		if err := json.Unmarshal(data, &obj); err == nil {
			extractPatternsFromJSON(obj, filepath.Base(filePath), patterns)
		}
	} else {
		lines := strings.Split(string(data), "\n")
		rePy := regexp.MustCompile(`re\.(?:compile|search|match|findall|split)\s*\(\s*r?["']([^"']+)["']`)
		for lineNum, line := range lines {
			if m := rePy.FindStringSubmatch(line); len(m) > 1 {
				key := fmt.Sprintf("%s:L%d", filepath.Base(filePath), lineNum+1)
				patterns[key] = m[1]
			}
		}
	}

	var issues []Issue
	for src, pat := range patterns {
		iss := ScanPattern(src, pat)
		issues = append(issues, iss...)
	}

	return issues, len(patterns)
}

func extractPatternsFromJSON(v interface{}, prefix string, out map[string]string) {
	switch val := v.(type) {
	case map[string]interface{}:
		for k, item := range val {
			kLower := strings.ToLower(k)
			if kLower == "pattern" || kLower == "splitregex" || kLower == "parseregex" {
				if s, ok := item.(string); ok && s != "" {
					out[prefix+"."+k] = s
				}
			} else if kLower == "recordstart" {
				if arr, ok := item.([]interface{}); ok {
					for idx, p := range arr {
						if s, ok := p.(string); ok && s != "" {
							out[fmt.Sprintf("%s.recordStart[%d]", prefix, idx)] = s
						}
					}
				}
			} else {
				extractPatternsFromJSON(item, prefix+"."+k, out)
			}
		}
	case []interface{}:
		for idx, item := range val {
			extractPatternsFromJSON(item, fmt.Sprintf("%s[%d]", prefix, idx), out)
		}
	}
}

func main() {
	targetDir := flag.String("target", "internal/parser/templates", "目标扫描目录或文件路径")
	outFile := flag.String("out", "docs/不兼容正则迁移清单.md", "输出 Markdown 清单报告路径")
	flag.Parse()

	fmt.Println("==================================================")
	fmt.Println("  Go RE2 正则兼容性静态扫描器 (tools/re2scan)")
	fmt.Println("==================================================")

	totalFiles := 0
	totalPatterns := 0
	var targetIssues []Issue

	// 1. 扫描目标目录/文件（真实目标）
	if info, err := os.Stat(*targetDir); err == nil {
		if info.IsDir() {
			_ = filepath.WalkDir(*targetDir, func(path string, d fs.DirEntry, err error) error {
				if err != nil || d.IsDir() {
					return nil
				}
				ext := strings.ToLower(filepath.Ext(path))
				if ext == ".json" || ext == ".py" || ext == ".txt" {
					issues, count := ScanFile(path)
					totalFiles++
					totalPatterns += count
					targetIssues = append(targetIssues, issues...)
				}
				return nil
			})
		} else {
			issues, count := ScanFile(*targetDir)
			totalFiles++
			totalPatterns += count
			targetIssues = append(targetIssues, issues...)
		}
	}

	fmt.Printf("扫描完成！共扫描文件: %d 个，提取正则模式: %d 条\n", totalFiles, totalPatterns)
	fmt.Printf("目标资产中发现不兼容项: %d 处\n\n", len(targetIssues))

	for i, issue := range targetIssues {
		fmt.Printf("[%d] 来源: %s\n", i+1, issue.Source)
		fmt.Printf("    模式: %s\n", issue.Pattern)
		fmt.Printf("    特征: %s\n", issue.Feature)
		fmt.Printf("    指导: %s\n\n", issue.Description)
	}

	// 2. 独立准备典型不兼容样例基线（仅作规范参考，不混入真实扫描问题计数）
	sampleBaselines := []struct {
		Name        string
		Pattern     string
		Feature     string
		Description string
	}{
		{
			Name:        "正向后行断言样例",
			Pattern:     `(?<=Slot_)\d+`,
			Feature:     "Positive Lookbehind (?<=...)",
			Description: "Go RE2 不支持。改写为普通捕获组 `Slot_(\\d+)`并在代码中提取第 1 组，或以 `Slot_` 为分块边界。",
		},
		{
			Name:        "正向先行断言样例",
			Pattern:     `interface (?=GigabitEthernet)`,
			Feature:     "Positive Lookahead (?=...)",
			Description: "Go RE2 不支持。改写为 `interface (GigabitEthernet)` 普通捕获组。",
		},
		{
			Name:        "反向引用样例",
			Pattern:     `<(\w+)>.*?</\1>`,
			Feature:     "Backreference (\\1, \\2...)",
			Description: "Go RE2 不支持。需在匹配后于业务代码中比对首尾标签是否一致。",
		},
	}

	// 3. 输出 Markdown 报告
	if *outFile != "" {
		_ = os.MkdirAll(filepath.Dir(*outFile), 0755)
		var sb strings.Builder
		sb.WriteString("# Go RE2 正则兼容性扫描与迁移清单\n\n")
		sb.WriteString(fmt.Sprintf("> 扫描目标: `%s` | 扫描文件数: %d | 提取模式数: %d | 目标资产问题数: %d\n\n", *targetDir, totalFiles, totalPatterns, len(targetIssues)))

		sb.WriteString("## 一、语法特征兼容性对照表\n\n")
		sb.WriteString("| 语法特征 | RE2 支持状态 | 推荐迁移与适配策略 |\n")
		sb.WriteString("|---|---|---|\n")
		sb.WriteString("| 正向后行断言 `(?<=...)` | ❌ 不支持 | 改写为普通捕获组并在代码中按 GroupIndex 提取，或用前后分块定界 |\n")
		sb.WriteString("| 负向后行断言 `(?<!...)` | ❌ 不支持 | 改写为字符集排除 `[^...]` 或在匹配后通过 Go 代码过滤 |\n")
		sb.WriteString("| 正向先行断言 `(?=...)` | ❌ 不支持 | 改写为普通捕获组合并匹配 |\n")
		sb.WriteString("| 负向先行断言 `(?!...)` | ❌ 不支持 | 改写为字符类排除或后置状态机过滤 |\n")
		sb.WriteString("| 反向引用 `\\1`, `\\2` | ❌ 不支持 | 拆分为两次匹配并在业务层比对相等性 |\n\n")

		sb.WriteString("## 二、扫描目标资产检测结果\n\n")
		if len(targetIssues) == 0 {
			sb.WriteString("✅ **目标范围内的所有正则表达式均 100% 兼容 Go RE2 引擎，零不兼容语法阻碍。**\n\n")
		} else {
			sb.WriteString("| 序号 | 来源 | 正则模式 | 命中特征 | 改写指引 |\n")
			sb.WriteString("|---|---|---|---|---|\n")
			for i, iss := range targetIssues {
				escapedPat := strings.ReplaceAll(iss.Pattern, "|", "\\|")
				sb.WriteString(fmt.Sprintf("| %d | `%s` | `%s` | **%s** | %s |\n", i+1, iss.Source, escapedPat, iss.Feature, iss.Description))
			}
			sb.WriteString("\n")
		}

		sb.WriteString("## 三、常见不兼容语法基线对照与改写示范（参考基准）\n\n")
		sb.WriteString("以下为典型 Python/PCRE 正则在迁移至 Go RE2 时的标准改写示范（不计入目标资产问题数）：\n\n")
		sb.WriteString("| 样例场景 | 原始 PCRE 正则 | 不兼容特征 | Go RE2 标准改写指引 |\n")
		sb.WriteString("|---|---|---|---|\n")
		for _, b := range sampleBaselines {
			escapedPat := strings.ReplaceAll(b.Pattern, "|", "\\|")
			sb.WriteString(fmt.Sprintf("| %s | `%s` | **%s** | %s |\n", b.Name, escapedPat, b.Feature, b.Description))
		}
		sb.WriteString("\n")

		_ = os.WriteFile(*outFile, []byte(sb.String()), 0644)
		fmt.Printf("迁移清单报告已生成至: %s\n", *outFile)
	}
}
