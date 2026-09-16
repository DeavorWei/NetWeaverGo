package xmlcfg

import (
	"encoding/xml"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// BrokenRegexInfo 记录 RE2 编译失败的正则
type BrokenRegexInfo struct {
	FilePath   string `json:"filePath"`
	Node       string `json:"node"`
	RawPattern string `json:"rawPattern"`
	Error      string `json:"error"`
}

// LoadResult 加载与编译统计结果
type LoadResult struct {
	TotalFiles       int
	LoadedFiles      int
	TotalPatterns    int
	CompiledPatterns int
	RewrittenCount   int
	BrokenList       []BrokenRegexInfo
}

// LoadConfigFromBytes 反序列化单份 XML 解析配置
func LoadConfigFromBytes(data []byte) (*CommandParseConfig, error) {
	var cfg CommandParseConfig
	if err := xml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("XML 反序列化失败: %w", err)
	}
	return &cfg, nil
}

// LoadConfigFromFile 从文件反序列化配置
func LoadConfigFromFile(filePath string) (*CommandParseConfig, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	return LoadConfigFromBytes(data)
}

// ValidateAndCompileConfig 校验并预编译配置内的全部正则表达式
func ValidateAndCompileConfig(cfg *CommandParseConfig, filePath string, res *LoadResult) {
	if cfg == nil {
		return
	}

	for _, seg := range cfg.Segments {
		checkPattern(seg.Regex, filePath, "SegmentParse.Regex", res)
		for _, node := range seg.ParseNodes {
			checkPattern(node.Regex, filePath, "ParseNode["+node.Name+"].Regex", res)
			if node.TableLinePolicy != nil {
				checkPattern(node.TableLinePolicy.Regex, filePath, "TableLinePolicy.Regex", res)
				for _, f := range node.TableLinePolicy.Fields {
					checkFieldPatterns(f, filePath, res)
				}
			}
			if node.TableColPolicy != nil {
				checkPattern(node.TableColPolicy.Regex, filePath, "TableColPolicy.Regex", res)
				for _, f := range node.TableColPolicy.Fields {
					checkFieldPatterns(f, filePath, res)
				}
			}
			if node.ConfigPolicy != nil {
				for _, f := range node.ConfigPolicy.Fields {
					checkFieldPatterns(f, filePath, res)
				}
			}
			if node.DynamicColPolicy != nil {
				for _, f := range node.DynamicColPolicy.Fields {
					checkFieldPatterns(f, filePath, res)
				}
			}
		}
	}
}

func checkFieldPatterns(f FieldNode, filePath string, res *LoadResult) {
	checkPattern(f.Regex, filePath, "Field["+f.Name+"].Regex", res)
	if f.StrExtract != nil {
		checkPattern(f.StrExtract.Regex, filePath, "Field["+f.Name+"].StrExtract.Regex", res)
	}
	if f.ReplaceAll != nil {
		checkPattern(f.ReplaceAll.Regex, filePath, "Field["+f.Name+"].ReplaceAll.Regex", res)
	}
}

func checkPattern(pat, filePath, nodeName string, res *LoadResult) {
	trimmed := strings.TrimSpace(pat)
	if trimmed == "" {
		return
	}
	res.TotalPatterns++

	_, _, rewritten, err := CleanAndCompileRegex(trimmed)
	if err != nil {
		res.BrokenList = append(res.BrokenList, BrokenRegexInfo{
			FilePath:   filePath,
			Node:       nodeName,
			RawPattern: trimmed,
			Error:      err.Error(),
		})
	} else {
		res.CompiledPatterns++
		if rewritten {
			res.RewrittenCount++
		}
	}
}

// LoadAndScanDir 遍历指定目录并加载校验所有 XML 规则
func LoadAndScanDir(dirPath string) (*LoadResult, error) {
	res := &LoadResult{}

	err := filepath.WalkDir(dirPath, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(path), ".xml") {
			return nil
		}
		// 跳过 DTD 或非规则文件
		base := strings.ToLower(filepath.Base(path))
		if base == "commandparse.dtd" || base == "datacategoryconfig.xml" || base == "modelcmdconfig.xml" {
			return nil
		}

		res.TotalFiles++
		cfg, err := LoadConfigFromFile(path)
		if err != nil {
			return fmt.Errorf("文件 %s 反序列化失败: %w", path, err)
		}
		res.LoadedFiles++

		ValidateAndCompileConfig(cfg, path, res)
		return nil
	})

	return res, err
}
