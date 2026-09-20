package parser

import (
	"embed"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"sync"

	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/parser/xmlcfg"
)

//go:embed templates/parsecfg/*
var parsecfgFS embed.FS

// XmlConfigEngine 第四解析引擎，基于 XML 规则树驱动的命令回显解析引擎
type XmlConfigEngine struct {
	mu           sync.RWMutex
	configs      map[string]*xmlcfg.CommandParseConfig // key: "vendor/cmd"
	orderedKeys  map[string][]string                   // key: "vendor" -> 按"最长优先"排序的命令键列表（确定性匹配）
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
		orderedKeys:  make(map[string][]string),
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
	walkErr := fs.WalkDir(parsecfgFS, "templates/parsecfg/xmlconfig", func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(path), ".xml") {
			return nil
		}

		data, readErr := parsecfgFS.ReadFile(path)
		if readErr != nil {
			logger.Warn("XmlConfigEngine", "-", "读取 XML 解析规则失败(已跳过): path=%s err=%v", path, readErr)
			return nil
		}

		cfg, parseErr := xmlcfg.LoadConfigFromBytes(data)
		if parseErr != nil {
			logger.Warn("XmlConfigEngine", "-", "加载 XML 解析规则失败(已跳过): path=%s err=%v", path, parseErr)
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

	// 3. 重建同厂商"最长优先"有序命令键索引（P0-7：确定性匹配，消除 map 随机性）
	e.rebuildOrderedKeys()
	// 4. 输出 parsecfg 不兼容规则清单启动告警（P2-2）
	e.logBrokenParseConfigs()
	// 5. 输出正则语法改写留痕启动告警（P2-1：改写可追溯，便于人工复核）
	e.logRegexRewriteRecords()
	return walkErr
}

// logRegexRewriteRecords 输出 XML 规则加载期间的正则语法改写留痕（P2-1）
func (e *XmlConfigEngine) logRegexRewriteRecords() {
	records := xmlcfg.RewriteRecords()
	if len(records) == 0 {
		return
	}
	logger.Warn("XmlConfigEngine", "-", "XML 解析规则加载期间发生 %d 处正则语法改写（已自动适配 RE2，请人工复核）", len(records))
	const maxDetail = 5
	for i, rec := range records {
		if i >= maxDetail {
			logger.Warn("XmlConfigEngine", "-", "其余 %d 处改写留痕可通过 xmlcfg.RewriteRecords() 查询", len(records)-maxDetail)
			break
		}
		logger.Warn("XmlConfigEngine", "-", "改写留痕[%s]: %s -> %s", rec.Reason, rec.Original, rec.Rewritten)
	}
}

// rebuildOrderedKeys 重建 vendor -> 有序命令键列表索引。
// 排序规则：命令键长度倒序（最长匹配优先），长度相同按字典序，保证同输入同结果。
func (e *XmlConfigEngine) rebuildOrderedKeys() {
	index := make(map[string][]string)
	for key := range e.configs {
		parts := strings.SplitN(key, "/", 2)
		if len(parts) != 2 {
			continue
		}
		index[parts[0]] = append(index[parts[0]], parts[1])
	}
	for vendor, keys := range index {
		sort.Slice(keys, func(i, j int) bool {
			if len(keys[i]) != len(keys[j]) {
				return len(keys[i]) > len(keys[j])
			}
			return keys[i] < keys[j]
		})
		index[vendor] = keys
	}
	e.orderedKeys = index
}

// logBrokenParseConfigs 读取并输出 parsecfg 不兼容规则清单（P2-2：启动 WARN），
// 使"编译失败规则清单"从静态文件变为可观测的启动告警。
func (e *XmlConfigEngine) logBrokenParseConfigs() {
	data, err := parsecfgFS.ReadFile("templates/parsecfg/parsecfg_broken.json")
	if err != nil {
		return
	}
	var broken []struct {
		FilePath   string `json:"filePath"`
		Node       string `json:"node"`
		RawPattern string `json:"rawPattern"`
		Error      string `json:"error"`
	}
	if json.Unmarshal(data, &broken) != nil {
		return
	}
	if len(broken) == 0 {
		logger.Info("XmlConfigEngine", "-", "parsecfg 不兼容规则清单为空，全部规则编译通过")
		return
	}
	for _, b := range broken {
		logger.Warn("XmlConfigEngine", "-", "parsecfg 不兼容规则: file=%s node=%s pattern=%s err=%s", b.FilePath, b.Node, b.RawPattern, b.Error)
	}
}

// ResolveConfig 查找最匹配的 CommandParseConfig（P0-7：确定性 + 不跨厂商兜底）。
// 匹配顺序：
//  1. vendor/cmd 精确匹配；
//  2. 同厂商内按"最长命令键优先"的有序索引做前缀匹配；
//  3. 不做跨厂商兜底 —— 厂商不匹配直接未命中，避免 A 厂商规则套用到 B 厂商设备。
func (e *XmlConfigEngine) ResolveConfig(vendor, commandKey string) (*xmlcfg.CommandParseConfig, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	v := strings.ToLower(strings.TrimSpace(vendor))
	cmd := strings.ToLower(strings.TrimSpace(commandKey))

	// 1. 精确匹配 vendor/cmd
	if cfg, ok := e.configs[v+"/"+cmd]; ok {
		return cfg, true
	}

	// 2. 同厂商内最长命令键优先匹配（有序索引，结果确定）
	for _, cfgCmd := range e.orderedKeys[v] {
		if cfgCmd == cmd || strings.HasPrefix(cmd, cfgCmd) || strings.HasPrefix(cfgCmd, cmd) {
			if cfg, ok := e.configs[v+"/"+cfgCmd]; ok {
				return cfg, true
			}
		}
	}

	// 3. 无跨厂商兜底
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
