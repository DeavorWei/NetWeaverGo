package parser

import (
	"embed"
	"encoding/xml"
	"fmt"
	"io/fs"
	"strings"
	"sync"

	"github.com/NetWeaverGo/core/internal/parser/xmlcfg"
)

//go:embed templates/parsecfg/*
var parsecfgFS embed.FS

// XmlConfigEngine 第四解析引擎，基于 XML 规则树驱动的命令回显解析引擎
type XmlConfigEngine struct {
	mu           sync.RWMutex
	configs      map[string]*xmlcfg.CommandParseConfig // key: "vendor/cmd"
	parseItems   map[string][]xmlcfg.CommandParse      // key: "vendor" -> []CommandParse
	configPolicy *ConfigPolicyExecutor
	tablePolicy  *TableLinePolicyExecutor
}

var (
	defaultXmlConfigEngine     *XmlConfigEngine
	defaultXmlConfigEngineOnce sync.Once
)

// GetDefaultXmlConfigEngine 获取 XmlConfig 解析引擎全局单例
func GetDefaultXmlConfigEngine() *XmlConfigEngine {
	defaultXmlConfigEngineOnce.Do(func() {
		engine := NewXmlConfigEngine()
		_ = engine.LoadEmbedded()
		defaultXmlConfigEngine = engine
	})
	return defaultXmlConfigEngine
}

// NewXmlConfigEngine 创建解析引擎实例
func NewXmlConfigEngine() *XmlConfigEngine {
	return &XmlConfigEngine{
		configs:      make(map[string]*xmlcfg.CommandParseConfig),
		parseItems:   make(map[string][]xmlcfg.CommandParse),
		configPolicy: &ConfigPolicyExecutor{},
		tablePolicy:  &TableLinePolicyExecutor{},
	}
}

// LoadEmbedded 从嵌入的 parsecfgFS 加载所有 XML 规则与 parseitem 映射
func (e *XmlConfigEngine) LoadEmbedded() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// 1. 加载 parseitem
	parseItemEntries, err := fs.ReadDir(parsecfgFS, "templates/parsecfg/parseitem")
	if err == nil {
		for _, entry := range parseItemEntries {
			if strings.HasSuffix(entry.Name(), ".xml") {
				data, readErr := parsecfgFS.ReadFile("templates/parsecfg/parseitem/" + entry.Name())
				if readErr == nil {
					var item xmlcfg.CommonParseItem
					if xml.Unmarshal(data, &item) == nil {
						for _, vc := range item.VendorCommons {
							vKey := strings.ToLower(strings.TrimSpace(vc.Vendor))
							e.parseItems[vKey] = append(e.parseItems[vKey], vc.CommandParses...)
						}
					}
				}
			}
		}
	}

	// 2. 遍历 xmlconfig 目录加载具体命令解析规则
	return fs.WalkDir(parsecfgFS, "templates/parsecfg/xmlconfig", func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(path), ".xml") {
			return nil
		}

		data, readErr := parsecfgFS.ReadFile(path)
		if readErr != nil {
			return nil
		}

		cfg, parseErr := xmlcfg.LoadConfigFromBytes(data)
		if parseErr != nil {
			return nil
		}

		// path 形如 templates/parsecfg/xmlconfig/huawei/display interface.xml
		relPath := strings.TrimPrefix(path, "templates/parsecfg/xmlconfig/")
		parts := strings.Split(relPath, "/")
		if len(parts) >= 2 {
			vendor := strings.ToLower(parts[0])
			cmd := strings.TrimSuffix(parts[1], ".xml")
			key := vendor + "/" + strings.ToLower(cmd)
			e.configs[key] = cfg
		}

		return nil
	})
}

// ResolveConfig 查找最匹配的 CommandParseConfig
func (e *XmlConfigEngine) ResolveConfig(vendor, commandKey string) (*xmlcfg.CommandParseConfig, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	v := strings.ToLower(strings.TrimSpace(vendor))
	cmd := strings.ToLower(strings.TrimSpace(commandKey))

	// 1. 精确匹配 vendor/cmd
	key := v + "/" + cmd
	if cfg, ok := e.configs[key]; ok {
		return cfg, true
	}

	// 2. 针对常见华为命令去除 display 前缀或包含匹配
	for k, cfg := range e.configs {
		if strings.HasPrefix(k, v+"/") {
			cfgCmd := strings.TrimPrefix(k, v+"/")
			if cfgCmd == cmd || strings.HasPrefix(cmd, cfgCmd) || strings.HasPrefix(cfgCmd, cmd) {
				return cfg, true
			}
		}
	}

	// 3. 兜底尝试任何 vendor 下匹配同名命令
	for k, cfg := range e.configs {
		parts := strings.Split(k, "/")
		if len(parts) == 2 && parts[1] == cmd {
			return cfg, true
		}
	}

	return nil, false
}

// Parse 实现 CliParser 接口
func (e *XmlConfigEngine) Parse(commandKey string, rawText string) ([]map[string]string, error) {
	return e.ParseWithVendor("huawei", commandKey, rawText)
}

// ParseWithVendor 指定厂商和命令进行解析
func (e *XmlConfigEngine) ParseWithVendor(vendor, commandKey, rawText string) ([]map[string]string, error) {
	if strings.TrimSpace(rawText) == "" {
		return nil, nil
	}

	cfg, found := e.ResolveConfig(vendor, commandKey)
	if !found {
		return nil, fmt.Errorf("未找到适合 %s / %s 的 XML 解析规则", vendor, commandKey)
	}

	var allResults []map[string]string

	for _, seg := range cfg.Segments {
		rows := e.parseSegment(&seg, rawText)
		allResults = append(allResults, rows...)
	}

	return allResults, nil
}

// parseSegment 解析单个 SegmentParse 节点
func (e *XmlConfigEngine) parseSegment(seg *xmlcfg.SegmentParse, text string) []map[string]string {
	var chunks []string

	if strings.TrimSpace(seg.Regex) != "" {
		_, re, _, err := xmlcfg.CleanAndCompileRegex(seg.Regex)
		if err == nil && re != nil {
			// 如果配置了正则分片
			indices := re.FindAllStringIndex(text, -1)
			if len(indices) > 0 {
				start := 0
				if strings.EqualFold(seg.HasFirst, "false") {
					start = indices[0][0]
				}
				for i := 0; i < len(indices); i++ {
					end := len(text)
					if i+1 < len(indices) {
						end = indices[i+1][0]
					}
					chunks = append(chunks, text[indices[i][0]:end])
				}
				if len(chunks) == 0 && start < len(text) {
					chunks = []string{text[start:]}
				}
			}
		}
	}

	if len(chunks) == 0 {
		chunks = []string{text}
	}

	var results []map[string]string

	for _, chunk := range chunks {
		chunkMatched := false
		for _, node := range seg.ParseNodes {
			// 若 ParseNode 指定了 Regex，需先匹配
			if strings.TrimSpace(node.Regex) != "" {
				_, nodeRe, _, err := xmlcfg.CleanAndCompileRegex(node.Regex)
				if err == nil && nodeRe != nil && !nodeRe.MatchString(chunk) {
					continue
				}
			}

			var nodeRows []map[string]string
			if node.TableLinePolicy != nil {
				rows, _ := e.tablePolicy.Execute(chunk, &node)
				nodeRows = append(nodeRows, rows...)
			} else if node.ConfigPolicy != nil {
				rows, _ := e.configPolicy.Execute(chunk, &node)
				nodeRows = append(nodeRows, rows...)
			}

			if len(nodeRows) > 0 {
				results = append(results, nodeRows...)
				chunkMatched = true
				if strings.EqualFold(seg.ChildMutex, "true") {
					break
				}
			}
		}

		// 若无匹配且存在 DefaultNode
		if !chunkMatched && seg.DefaultNode != nil && seg.DefaultNode.Attributes != "" {
			defRow := make(map[string]string)
			pairs := strings.Split(seg.DefaultNode.Attributes, ",")
			for _, p := range pairs {
				kv := strings.SplitN(p, ":", 2)
				if len(kv) == 2 {
					defRow[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
				}
			}
			if len(defRow) > 0 {
				results = append(results, defRow)
			}
		}
	}

	return results
}
