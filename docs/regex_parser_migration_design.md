# TextFSM 到正则表达式解析器迁移详细设计

## 1. 概述

本文档详细描述从 TextFSM 迁移到原生正则表达式解析器的完整方案，包括代码结构、数据模型、API 接口、前端界面等。

### 1.1 设计目标

- 保持 `CliParser` 接口不变，确保对上层透明
- 支持用户自定义正则模板
- 支持多行聚合场景（Filldown 语义）
- 提供模板测试功能
- **彻底清除 TextFSM 相关代码和依赖**

### 1.2 影响范围

| 模块                                            | 变更类型 | 说明                     |
| ----------------------------------------------- | -------- | ------------------------ |
| `internal/parser/textfsm.go`                    | 删除     | 移除 TextFSM 解析器      |
| `internal/parser/regex_parser.go`               | 新增     | 正则解析器实现           |
| `internal/parser/multiline.go`                  | 新增     | 多行聚合处理器           |
| `internal/parser/composite_parser.go`           | 新增     | 复合解析器               |
| `internal/parser/models.go`                     | 修改     | 扩展 CliParser 接口      |
| `internal/parser/templates/**/*.textfsm`        | 删除     | 移除 24 个 TextFSM 模板  |
| `internal/parser/templates/builtin/*.json`      | 新增     | 内置正则模板             |
| `internal/parser/golden_test.go`                | 修改     | 切换解析器实现           |
| `internal/parser/service.go.474295111933871248` | 删除     | 移除 legacy 文件         |
| `internal/taskexec/executor_impl.go`            | 修改     | 切换解析器实现，依赖接口 |
| `internal/config/parse_template.go`             | 新增     | 模板配置管理             |
| `internal/ui/parse_template_service.go`         | 新增     | 模板 CRUD 服务           |
| `go.mod`                                        | 修改     | 移除 gotextfsm 依赖      |

---

## 2. 代码结构设计

### 2.1 目录结构

```
internal/parser/
├── models.go              # 接口定义（扩展）
├── mapper.go              # 结果映射器（保持不变）
├── regex_parser.go        # 正则解析器（新增）
├── multiline.go           # 多行聚合处理器（新增）
├── composite_parser.go    # 复合解析器（新增）
├── golden_test.go         # 测试文件（修改）
└── templates/
    ├── builtin/           # 内置模板（新增）
    │   ├── huawei.json
    │   ├── h3c.json
    │   └── cisco.json
    └── *.textfsm          # 删除（24个文件）
```

### 2.2 接口定义扩展

```go
// internal/parser/models.go

// CliParser CLI解析器接口
type CliParser interface {
    // Parse 解析原始CLI输出
    Parse(commandKey string, rawText string) ([]map[string]string, error)
    // LoadBuiltinTemplates 加载内置模板
    LoadBuiltinTemplates(vendor string) error
    // AddTemplate 添加自定义模板
    AddTemplate(tpl *RegexTemplate) error
}
```

### 2.3 核心类型定义

```go
// internal/parser/regex_parser.go

package parser

import (
    "embed"
    "encoding/json"
    "fmt"
    "regexp"
    "strings"
    "sync"
)

//go:embed templates/builtin/*.json
var builtinTemplateFS embed.FS

// RegexTemplate 正则模板定义
type RegexTemplate struct {
    CommandKey   string            `json:"commandKey"`
    Pattern      string            `json:"pattern"`
    Multiline    bool              `json:"multiline"`
    FieldMapping map[string]string `json:"fieldMapping,omitempty"`
    Description  string            `json:"description,omitempty"`

    // 运行时字段
    compiled *regexp.Regexp `json:"-"`
}

// VendorTemplates 厂商模板集合
type VendorTemplates struct {
    Vendor    string                   `json:"vendor"`
    Templates map[string]RegexTemplate `json:"templates"`
}

// RegexParser 正则表达式解析器
type RegexParser struct {
    templates map[string]*RegexTemplate // commandKey -> template
    mu        sync.RWMutex
}

// NewRegexParser 创建正则解析器
func NewRegexParser() *RegexParser {
    return &RegexParser{
        templates: make(map[string]*RegexTemplate),
    }
}

// LoadBuiltinTemplates 加载内置模板
func (p *RegexParser) LoadBuiltinTemplates(vendor string) error {
    if vendor == "" {
        vendor = "huawei"
    }

    data, err := builtinTemplateFS.ReadFile("templates/builtin/" + vendor + ".json")
    if err != nil {
        // 回退到默认厂商
        data, err = builtinTemplateFS.ReadFile("templates/builtin/huawei.json")
        if err != nil {
            return fmt.Errorf("加载内置模板失败: %w", err)
        }
    }

    var vt VendorTemplates
    if err := json.Unmarshal(data, &vt); err != nil {
        return fmt.Errorf("解析模板 JSON 失败: %w", err)
    }

    p.mu.Lock()
    defer p.mu.Unlock()

    for key, tpl := range vt.Templates {
        // 编译正则
        flags := ""
        if tpl.Multiline {
            flags = "(?m)"
        }
        compiled, err := regexp.Compile(flags + tpl.Pattern)
        if err != nil {
            return fmt.Errorf("编译正则 %s 失败: %w", key, err)
        }

        tpl.compiled = compiled
        p.templates[key] = &tpl
    }

    return nil
}

// Parse 实现 CliParser 接口
func (p *RegexParser) Parse(commandKey string, rawText string) ([]map[string]string, error) {
    p.mu.RLock()
    tpl, ok := p.templates[commandKey]
    p.mu.RUnlock()

    if !ok {
        return nil, fmt.Errorf("未找到命令键对应的正则模板: %s", commandKey)
    }

    // 执行正则匹配
    matches := tpl.compiled.FindAllStringSubmatch(rawText, -1)
    if len(matches) == 0 {
        return []map[string]string{}, nil
    }

    // 提取命名组
    results := make([]map[string]string, 0, len(matches))
    names := tpl.compiled.SubexpNames()

    for _, match := range matches {
        row := make(map[string]string)
        for i, name := range names {
            if i > 0 && name != "" && i < len(match) {
                row[name] = strings.TrimSpace(match[i])
            }
        }

        // 应用字段映射
        if len(tpl.FieldMapping) > 0 {
            mapped := make(map[string]string)
            for k, v := range row {
                if newKey, ok := tpl.FieldMapping[k]; ok {
                    mapped[newKey] = v
                } else {
                    mapped[k] = v
                }
            }
            row = mapped
        }

        if len(row) > 0 {
            results = append(results, row)
        }
    }

    return results, nil
}

// AddTemplate 添加自定义模板
func (p *RegexParser) AddTemplate(tpl *RegexTemplate) error {
    flags := ""
    if tpl.Multiline {
        flags = "(?m)"
    }
    compiled, err := regexp.Compile(flags + tpl.Pattern)
    if err != nil {
        return fmt.Errorf("编译正则失败: %w", err)
    }

    p.mu.Lock()
    defer p.mu.Unlock()

    tpl.compiled = compiled
    p.templates[tpl.CommandKey] = tpl
    return nil
}
```

### 2.4 多行聚合处理器

```go
// internal/parser/multiline.go

package parser

import (
    "bufio"
    "regexp"
    "strings"
)

// MultilineParser 多行聚合解析器
// 处理 TextFSM 中 Filldown 语义的多行聚合场景
type MultilineParser struct {
    aggregators map[string]AggregatorFunc
}

type AggregatorFunc func(rawText string) ([]map[string]string, error)

func NewMultilineParser() *MultilineParser {
    return &MultilineParser{
        aggregators: map[string]AggregatorFunc{
            "lldp_neighbor":      parseLLDPNeighbor,
            "interface_detail":   parseInterfaceDetail,
            "eth_trunk":          parseEthTrunk,
            "eth_trunk_verbose":  parseEthTrunkVerbose,
        },
    }
}

// Parse 解析多行聚合场景
func (p *MultilineParser) Parse(commandKey string, rawText string) ([]map[string]string, error) {
    fn, ok := p.aggregators[commandKey]
    if !ok {
        return nil, fmt.Errorf("未找到多行聚合处理器: %s", commandKey)
    }
    return fn(rawText)
}

// CanHandle 检查是否支持该命令键
func (p *MultilineParser) CanHandle(commandKey string) bool {
    _, ok := p.aggregators[commandKey]
    return ok
}

// parseLLDPNeighbor 解析 LLDP 邻居信息
// 对应 TextFSM 模板: templates/huawei/lldp_neighbor.textfsm
// Filldown 语义: local_if 字段在后续行保持不变
func parseLLDPNeighbor(rawText string) ([]map[string]string, error) {
    var results []map[string]string
    var currentIf string
    currentRecord := make(map[string]string)

    reInterface := regexp.MustCompile(`^\s*\[(\S+)\]\s*$`)
    reNoNeighbor := regexp.MustCompile(`^\s*(\S+)\s+has\s+0\s+neighbor`)
    reHasNeighbor := regexp.MustCompile(`^\s*(\S+)\s+has\s+\d+\s+neighbor`)
    reSysName := regexp.MustCompile(`System\s+name\s*:\s*(.+)`)
    rePortID := regexp.MustCompile(`Port\s+ID\s*:\s*(\S+)`)
    reChassisID := regexp.MustCompile(`Chassis\s+ID\s*:\s*(\S+)`)
    reMgmtIP := regexp.MustCompile(`Management\s+address\s*:\s*(\S+)`)

    scanner := bufio.NewScanner(strings.NewReader(rawText))
    for scanner.Scan() {
        line := scanner.Text()

        // 匹配接口行: [GE1/0/1]
        if matches := reInterface.FindStringSubmatch(line); matches != nil {
            // 保存上一条记录
            if currentRecord["neighbor_name"] != "" {
                currentRecord["local_if"] = currentIf
                results = append(results, currentRecord)
            }
            currentIf = matches[1]
            currentRecord = make(map[string]string)
            continue
        }

        // 匹配 "has 0 neighbor" 行
        if matches := reNoNeighbor.FindStringSubmatch(line); matches != nil {
            currentIf = matches[1]
            currentRecord = make(map[string]string)
            continue
        }

        // 匹配 "has N neighbor" 行
        if matches := reHasNeighbor.FindStringSubmatch(line); matches != nil {
            currentIf = matches[1]
            currentRecord = make(map[string]string)
            continue
        }

        // 匹配邻居信息
        if currentIf == "" {
            continue
        }

        if matches := reSysName.FindStringSubmatch(line); matches != nil {
            currentRecord["neighbor_name"] = strings.TrimSpace(matches[1])
        }
        if matches := rePortID.FindStringSubmatch(line); matches != nil {
            currentRecord["neighbor_port"] = matches[1]
        }
        if matches := reChassisID.FindStringSubmatch(line); matches != nil {
            currentRecord["chassis_id"] = matches[1]
        }
        if matches := reMgmtIP.FindStringSubmatch(line); matches != nil {
            currentRecord["mgmt_ip"] = matches[1]
            currentRecord["local_if"] = currentIf
            results = append(results, currentRecord)
            currentRecord = make(map[string]string)
        }
    }

    // 保存最后一条记录
    if currentRecord["neighbor_name"] != "" {
        currentRecord["local_if"] = currentIf
        results = append(results, currentRecord)
    }

    return results, nil
}

// parseInterfaceDetail 解析接口详情
// 对应 TextFSM 模板: templates/huawei/interface_detail.textfsm
// 多行聚合: 一个接口的信息分布在多行
func parseInterfaceDetail(rawText string) ([]map[string]string, error) {
    var results []map[string]string

    reInterface := regexp.MustCompile(`^(\S+)\s+current\s+state`)
    reDescription := regexp.MustCompile(`^\s*Description:\s*(.*)`)
    reMAC := regexp.MustCompile(`^\s*Hardware\s+address\s+is\s+([0-9A-Fa-f:\.-]+)`)
    reIP := regexp.MustCompile(`^\s*Internet\s+Address\s+is\s+(\d+\.\d+\.\d+\.\d+/\d+)`)

    var currentInterface string
    currentRecord := make(map[string]string)

    scanner := bufio.NewScanner(strings.NewReader(rawText))
    for scanner.Scan() {
        line := scanner.Text()

        if matches := reInterface.FindStringSubmatch(line); matches != nil {
            // 保存上一条记录
            if currentInterface != "" && len(currentRecord) > 0 {
                currentRecord["interface"] = currentInterface
                results = append(results, currentRecord)
            }
            currentInterface = matches[1]
            currentRecord = make(map[string]string)
            continue
        }

        if matches := reDescription.FindStringSubmatch(line); matches != nil {
            currentRecord["description"] = strings.TrimSpace(matches[1])
        }
        if matches := reMAC.FindStringSubmatch(line); matches != nil {
            currentRecord["mac"] = matches[1]
        }
        if matches := reIP.FindStringSubmatch(line); matches != nil {
            currentRecord["ip"] = matches[1]
        }
    }

    // 保存最后一条记录
    if currentInterface != "" && len(currentRecord) > 0 {
        currentRecord["interface"] = currentInterface
        results = append(results, currentRecord)
    }

    return results, nil
}

// parseEthTrunk 解析聚合口信息
// 对应 TextFSM 模板: templates/huawei/eth_trunk.textfsm
// 多行聚合: 聚合口定义 + 成员端口列表
func parseEthTrunk(rawText string) ([]map[string]string, error) {
    var results []map[string]string
    var currentTrunkID string

    reTrunk := regexp.MustCompile(`^\s*(?:Eth-Trunk|Trunk)(\d+)`)
    reMember := regexp.MustCompile(`^\s*(GE|XGE|10GE|40GE|100GE|GigabitEthernet|XGigabitEthernet)(\d+/\d+/\d+)`)

    scanner := bufio.NewScanner(strings.NewReader(rawText))
    for scanner.Scan() {
        line := scanner.Text()

        if matches := reTrunk.FindStringSubmatch(line); matches != nil {
            currentTrunkID = matches[1]
            results = append(results, map[string]string{
                "trunk_id": currentTrunkID,
            })
            continue
        }

        if matches := reMember.FindStringSubmatch(line); matches != nil && currentTrunkID != "" {
            results = append(results, map[string]string{
                "trunk_id": currentTrunkID,
                "if_type":  matches[1],
                "if_num":   matches[2],
            })
        }
    }

    return results, nil
}

// parseEthTrunkVerbose 解析聚合口详细信息
// 对应 TextFSM 模板: templates/huawei/eth_trunk_verbose.textfsm
func parseEthTrunkVerbose(rawText string) ([]map[string]string, error) {
    var results []map[string]string
    var currentTrunkID string

    reTrunk := regexp.MustCompile(`^\s*(?:Eth-Trunk|Trunk)(\d+)`)
    reMemberStatus := regexp.MustCompile(`^\s*(\S+)\s+(Up|Down)`)

    scanner := bufio.NewScanner(strings.NewReader(rawText))
    for scanner.Scan() {
        line := scanner.Text()

        if matches := reTrunk.FindStringSubmatch(line); matches != nil {
            currentTrunkID = matches[1]
            results = append(results, map[string]string{
                "trunk_id": currentTrunkID,
            })
            continue
        }

        if matches := reMemberStatus.FindStringSubmatch(line); matches != nil && currentTrunkID != "" {
            results = append(results, map[string]string{
                "trunk_id": currentTrunkID,
                "member":   matches[1],
                "status":   matches[2],
            })
        }
    }

    return results, nil
}
```

### 2.5 复合解析器

```go
// internal/parser/composite_parser.go

package parser

// CompositeParser 复合解析器，整合正则解析和多行聚合
type CompositeParser struct {
    regex     *RegexParser
    multiline *MultilineParser
}

// 编译时接口约束检查
var _ CliParser = (*CompositeParser)(nil)

func NewCompositeParser() *CompositeParser {
    return &CompositeParser{
        regex:     NewRegexParser(),
        multiline: NewMultilineParser(),
    }
}

func (p *CompositeParser) LoadBuiltinTemplates(vendor string) error {
    return p.regex.LoadBuiltinTemplates(vendor)
}

func (p *CompositeParser) Parse(commandKey string, rawText string) ([]map[string]string, error) {
    // 优先使用多行聚合处理器
    if p.multiline.CanHandle(commandKey) {
        return p.multiline.Parse(commandKey, rawText)
    }

    // 使用正则解析器
    return p.regex.Parse(commandKey, rawText)
}

func (p *CompositeParser) AddTemplate(tpl *RegexTemplate) error {
    return p.regex.AddTemplate(tpl)
}
```

---

## 3. 内置模板 JSON 格式

### 3.1 Huawei 模板（完整 11 个）

```json
// internal/parser/templates/builtin/huawei.json
{
  "vendor": "huawei",
  "templates": {
    "version": {
      "commandKey": "version",
      "pattern": "^.*[Vv]ersion.*[Vv](?P<version>\\S+)|^Huawei\\s+(?P<model>[A-Za-z0-9\\-_\\/\\.]+)|^.*[Ss]erial\\s*[Nn]umber.*:\\s*(?P<serial_number>\\S+)|^<(?P<hostname>\\S+)>",
      "multiline": true,
      "description": "设备版本信息"
    },
    "sysname": {
      "commandKey": "sysname",
      "pattern": "^sysname\\s+(?P<hostname>\\S+)",
      "multiline": true,
      "description": "设备主机名"
    },
    "esn": {
      "commandKey": "esn",
      "pattern": "^ESN\\s+:\\s*(?P<serial_number>\\S+)",
      "multiline": true,
      "description": "设备序列号"
    },
    "device_info": {
      "commandKey": "device_info",
      "pattern": "^.*[Pp]roduct\\s*[Tt]ype.*:\\s*(?P<model>\\S+)",
      "multiline": true,
      "description": "设备型号信息"
    },
    "interface_brief": {
      "commandKey": "interface_brief",
      "pattern": "^\\s*(?P<interface>\\S+)\\s+(?P<phy>\\S+)\\s+(?P<protocol>\\S+)\\s+(?P<description>.*)$",
      "multiline": true,
      "description": "接口简要信息"
    },
    "mac_address": {
      "commandKey": "mac_address",
      "pattern": "^\\s*(?P<vlan>\\d+|\\-\\d+)\\s+(?P<mac>[0-9A-Fa-f:\\.\\-]+)\\s+(?P<type>\\S+)\\s+(?P<interface>\\S+)",
      "multiline": true,
      "description": "MAC 地址表"
    },
    "arp_all": {
      "commandKey": "arp_all",
      "pattern": "^\\s*(?P<ip>\\d+\\.\\d+\\.\\d+\\.\\d+)\\s+(?P<mac>[0-9A-Fa-f:\\.\\-]+)\\s+(?P<type>\\S+)\\s+(?P<interface>\\S+)",
      "multiline": true,
      "description": "ARP 表"
    },
    "eth_trunk": {
      "commandKey": "eth_trunk",
      "pattern": "^\\s*(?:Eth-Trunk|Trunk)(?P<trunk_id>\\d+).*|^\\s*(?P<if_type>GE|XGE|10GE|40GE|100GE|GigabitEthernet|XGigabitEthernet)(?P<if_num>\\d+/\\d+/\\d+)",
      "multiline": true,
      "description": "聚合口信息（多行聚合处理）"
    },
    "eth_trunk_verbose": {
      "commandKey": "eth_trunk_verbose",
      "pattern": "^\\s*(?:Eth-Trunk|Trunk)(?P<trunk_id>\\d+).*|^\\s*(?P<member>\\S+)\\s+(?P<status>\\S+)",
      "multiline": true,
      "description": "聚合口详细信息（多行聚合处理）"
    },
    "lldp_neighbor": {
      "commandKey": "lldp_neighbor",
      "pattern": "^\\s*\\[(?P<local_if>\\S+)\\].*|^System\\s+name\\s*:\\s*(?P<neighbor_name>.+)|^Port\\s+ID\\s*:\\s*(?P<neighbor_port>\\S+)|^Chassis\\s+ID\\s*:\\s*(?P<chassis_id>\\S+)|^Management\\s+address\\s*:\\s*(?P<mgmt_ip>\\S+)",
      "multiline": true,
      "description": "LLDP 邻居（多行聚合处理）"
    },
    "interface_detail": {
      "commandKey": "interface_detail",
      "pattern": "^(?P<interface>\\S+)\\s+current\\s+state|^\\s*Description:\\s*(?P<description>.*)|^\\s*Hardware\\s+address\\s+is\\s+(?P<mac>[0-9A-Fa-f:\\.\\-]+)|^\\s*Internet\\s+Address\\s+is\\s+(?P<ip>\\d+\\.\\d+\\.\\d+\\.\\d+/\\d+)",
      "multiline": true,
      "description": "接口详情（多行聚合处理）"
    }
  }
}
```

### 3.2 H3C 模板（完整 6 个）

```json
// internal/parser/templates/builtin/h3c.json
{
  "vendor": "h3c",
  "templates": {
    "version": {
      "commandKey": "version",
      "pattern": "^H3C\\s+Comware.*Version\\s+(?P<version>\\S+)|^.*[Ss]erial\\s*[Nn]umber.*:\\s*(?P<serial_number>\\S+)",
      "multiline": true,
      "description": "设备版本信息"
    },
    "interface_brief": {
      "commandKey": "interface_brief",
      "pattern": "^\\s*(?P<interface>\\S+)\\s+(?P<phy>\\S+)\\s+(?P<protocol>\\S+)\\s+(?P<description>.*)$",
      "multiline": true,
      "description": "接口简要信息"
    },
    "lldp_neighbor": {
      "commandKey": "lldp_neighbor",
      "pattern": "^\\s*(?P<local_if>\\S+)\\s+(?P<neighbor_name>\\S+)\\s+(?P<neighbor_port>\\S+)",
      "multiline": true,
      "description": "LLDP 邻居"
    },
    "mac_address": {
      "commandKey": "mac_address",
      "pattern": "^\\s*(?P<mac>[0-9A-Fa-f:\\.\\-]+)\\s+(?P<vlan>\\d+)\\s+(?P<interface>\\S+)\\s+(?P<type>\\S+)",
      "multiline": true,
      "description": "MAC 地址表"
    },
    "arp_all": {
      "commandKey": "arp_all",
      "pattern": "^\\s*(?P<ip>\\d+\\.\\d+\\.\\d+\\.\\d+)\\s+(?P<mac>[0-9A-Fa-f:\\.\\-]+)\\s+(?P<vlan>\\d+)\\s+(?P<interface>\\S+)\\s+(?P<type>\\S+)",
      "multiline": true,
      "description": "ARP 表"
    },
    "eth_trunk": {
      "commandKey": "eth_trunk",
      "pattern": "^\\s*(?:Eth-Trunk|Trunk)(?P<trunk_id>\\d+).*|^\\s*(?P<member>\\S+)\\s+(?P<status>\\S+)",
      "multiline": true,
      "description": "聚合口信息"
    }
  }
}
```

### 3.3 Cisco 模板（完整 7 个）

```json
// internal/parser/templates/builtin/cisco.json
{
  "vendor": "cisco",
  "templates": {
    "version": {
      "commandKey": "version",
      "pattern": "^Cisco.*Version\\s+(?P<version>\\S+)|^.*[Pp]rocessor.*:\\s*(?P<model>\\S+)|^.*[Ss]erial\\s*[Nn]umber.*:\\s*(?P<serial_number>\\S+)|^(?P<hostname>\\S+)#",
      "multiline": true,
      "description": "设备版本信息"
    },
    "interface_brief": {
      "commandKey": "interface_brief",
      "pattern": "^\\s*(?P<interface>\\S+)\\s+(?P<status>\\S+)\\s+(?P<protocol>\\S+)\\s*(?P<description>.*)$",
      "multiline": true,
      "description": "接口简要信息"
    },
    "interface_detail": {
      "commandKey": "interface_detail",
      "pattern": "^(?P<interface>\\S+)\\s+is\\s+.*|^\\s*Description:\\s*(?P<description>.*)|^\\s*Hardware\\s+is\\s+.*address\\s+is\\s+(?P<mac>[0-9A-Fa-f:\\.]+)|^\\s*Internet\\s+address\\s+is\\s+(?P<ip>\\d+\\.\\d+\\.\\d+\\.\\d+/\\d+)",
      "multiline": true,
      "description": "接口详情（多行聚合处理）"
    },
    "lldp_neighbor": {
      "commandKey": "lldp_neighbor",
      "pattern": "^\\s*(?P<local_if>\\S+)\\s+(?P<neighbor_name>\\S+)\\s+(?P<neighbor_port>\\S+)",
      "multiline": true,
      "description": "LLDP 邻居"
    },
    "mac_address": {
      "commandKey": "mac_address",
      "pattern": "^\\s*(?P<vlan>\\d+)\\s+(?P<mac>[0-9A-Fa-f:\\.\\-]+)\\s+(?P<type>\\S+)\\s+(?P<interface>\\S+)",
      "multiline": true,
      "description": "MAC 地址表"
    },
    "arp_all": {
      "commandKey": "arp_all",
      "pattern": "^\\s*(?P<ip>\\d+\\.\\d+\\.\\d+\\.\\d+)\\s+(?P<mac>[0-9A-Fa-f:\\.\\-]+)\\s+(?P<interface>\\S+)",
      "multiline": true,
      "description": "ARP 表"
    },
    "eth_trunk": {
      "commandKey": "eth_trunk",
      "pattern": "^\\s*(?:Port-channel|Po)(?P<trunk_id>\\d+).*|^\\s*(?P<member>\\S+)\\s+(?P<status>\\S+)",
      "multiline": true,
      "description": "聚合口信息"
    }
  }
}
```

---

## 4. 数据库模型设计

### 4.1 用户自定义模板表

```go
// internal/models/parse_template.go

package models

import "time"

// UserParseTemplate 用户自定义解析模板
type UserParseTemplate struct {
    ID           uint      `gorm:"primaryKey" json:"id"`
    Vendor       string    `gorm:"column:vendor;not null;index" json:"vendor"`
    CommandKey   string    `gorm:"column:command_key;not null;index" json:"commandKey"`
    Pattern      string    `gorm:"column:pattern;not null;type:text" json:"pattern"`
    Multiline    bool      `gorm:"column:multiline;default:true" json:"multiline"`
    FieldMapping string    `gorm:"column:field_mapping;type:text" json:"fieldMapping"` // JSON
    Description  string    `gorm:"column:description" json:"description"`
    Enabled      bool      `gorm:"column:enabled;default:true" json:"enabled"`
    CreatedAt    time.Time `gorm:"column:created_at" json:"createdAt"`
    UpdatedAt    time.Time `gorm:"column:updated_at" json:"updatedAt"`
}

func (UserParseTemplate) TableName() string {
    return "net_user_parse_templates"
}
```

### 4.2 数据库迁移

```sql
-- 创建用户解析模板表
CREATE TABLE IF NOT EXISTS net_user_parse_templates (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    vendor TEXT NOT NULL,
    command_key TEXT NOT NULL,
    pattern TEXT NOT NULL,
    multiline BOOLEAN DEFAULT 1,
    field_mapping TEXT,
    description TEXT,
    enabled BOOLEAN DEFAULT 1,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(vendor, command_key)
);

-- 创建索引
CREATE INDEX IF NOT EXISTS idx_user_parse_templates_vendor ON net_user_parse_templates(vendor);
CREATE INDEX IF NOT EXISTS idx_user_parse_templates_command_key ON net_user_parse_templates(command_key);
```

---

## 5. API 接口设计

### 5.1 模板管理服务

```go
// internal/ui/parse_template_service.go

package ui

import (
    "encoding/json"
    "fmt"
    "regexp"
    "time"

    "github.com/NetWeaverGo/core/internal/models"
    "github.com/NetWeaverGo/core/internal/parser"
    "gorm.io/gorm"
)

type ParseTemplateService struct {
    db     *gorm.DB
    parser *parser.CompositeParser
}

func NewParseTemplateService(db *gorm.DB, p *parser.CompositeParser) *ParseTemplateService {
    return &ParseTemplateService{
        db:     db,
        parser: p,
    }
}

// ListTemplates 列出模板
func (s *ParseTemplateService) ListTemplates(vendor string) ([]UserParseTemplateVO, error) {
    var templates []models.UserParseTemplate
    query := s.db.Model(&models.UserParseTemplate{})
    if vendor != "" {
        query = query.Where("vendor = ?", vendor)
    }
    if err := query.Find(&templates).Error; err != nil {
        return nil, err
    }

    result := make([]UserParseTemplateVO, len(templates))
    for i, t := range templates {
        result[i] = UserParseTemplateVO{
            ID:          t.ID,
            Vendor:      t.Vendor,
            CommandKey:  t.CommandKey,
            Pattern:     t.Pattern,
            Multiline:   t.Multiline,
            Description: t.Description,
            Enabled:     t.Enabled,
            CreatedAt:   t.CreatedAt,
            UpdatedAt:   t.UpdatedAt,
        }
    }
    return result, nil
}

// CreateTemplate 创建模板
func (s *ParseTemplateService) CreateTemplate(req CreateParseTemplateRequest) error {
    // 验证正则语法
    flags := ""
    if req.Multiline {
        flags = "(?m)"
    }
    if _, err := regexp.Compile(flags + req.Pattern); err != nil {
        return fmt.Errorf("正则语法错误: %w", err)
    }

    fieldMappingJSON := ""
    if len(req.FieldMapping) > 0 {
        data, _ := json.Marshal(req.FieldMapping)
        fieldMappingJSON = string(data)
    }

    tpl := &models.UserParseTemplate{
        Vendor:       req.Vendor,
        CommandKey:   req.CommandKey,
        Pattern:      req.Pattern,
        Multiline:    req.Multiline,
        FieldMapping: fieldMappingJSON,
        Description:  req.Description,
        Enabled:      true,
    }

    if err := s.db.Create(tpl).Error; err != nil {
        return err
    }

    // 添加到运行时解析器
    return s.parser.AddTemplate(&parser.RegexTemplate{
        CommandKey:   req.CommandKey,
        Pattern:      req.Pattern,
        Multiline:    req.Multiline,
        FieldMapping: req.FieldMapping,
    })
}

// TestTemplate 测试模板
func (s *ParseTemplateService) TestTemplate(req TestParseTemplateRequest) (*TestParseTemplateResult, error) {
    // 创建临时解析器
    tempParser := parser.NewRegexParser()

    tpl := &parser.RegexTemplate{
        CommandKey: req.CommandKey,
        Pattern:    req.Pattern,
        Multiline:  req.Multiline,
    }

    if err := tempParser.AddTemplate(tpl); err != nil {
        return nil, err
    }

    // 执行解析
    results, err := tempParser.Parse(req.CommandKey, req.SampleText)
    if err != nil {
        return &TestParseTemplateResult{
            Success: false,
            Error:   err.Error(),
        }, nil
    }

    return &TestParseTemplateResult{
        Success: true,
        Count:   len(results),
        Rows:    results,
    }, nil
}

// VO 和 Request 定义
type UserParseTemplateVO struct {
    ID           uint               `json:"id"`
    Vendor       string             `json:"vendor"`
    CommandKey   string             `json:"commandKey"`
    Pattern      string             `json:"pattern"`
    Multiline    bool               `json:"multiline"`
    FieldMapping map[string]string  `json:"fieldMapping,omitempty"`
    Description  string             `json:"description"`
    Enabled      bool               `json:"enabled"`
    CreatedAt    time.Time          `json:"createdAt"`
    UpdatedAt    time.Time          `json:"updatedAt"`
}

type CreateParseTemplateRequest struct {
    Vendor       string            `json:"vendor"`
    CommandKey   string            `json:"commandKey"`
    Pattern      string            `json:"pattern"`
    Multiline    bool              `json:"multiline"`
    FieldMapping map[string]string `json:"fieldMapping,omitempty"`
    Description  string            `json:"description"`
}

type TestParseTemplateRequest struct {
    CommandKey string `json:"commandKey"`
    Pattern    string `json:"pattern"`
    Multiline  bool   `json:"multiline"`
    SampleText string `json:"sampleText"`
}

type TestParseTemplateResult struct {
    Success bool                `json:"success"`
    Error   string              `json:"error,omitempty"`
    Count   int                 `json:"count"`
    Rows    []map[string]string `json:"rows"`
}
```

### 5.2 Wails 绑定

```go
// 在 cmd/netweaver/main.go 中注册服务

func main() {
    // ... 其他初始化

    compositeParser := parser.NewCompositeParser()
    templateService := ui.NewParseTemplateService(db, compositeParser)

    app := wails.NewApplication()
    app.Bind(templateService)
    // ...
}
```

---

## 6. 前端界面设计

### 6.1 模板管理页面

```vue
<!-- frontend/src/views/ParseTemplates.vue -->
<template>
  <div class="parse-templates-page">
    <div class="page-header">
      <h2>解析模板管理</h2>
      <a-button type="primary" @click="showCreateModal"> 新建模板 </a-button>
    </div>

    <div class="filter-bar">
      <a-select
        v-model:value="selectedVendor"
        style="width: 200px"
        placeholder="选择厂商"
      >
        <a-select-option value="">全部厂商</a-select-option>
        <a-select-option value="huawei">Huawei</a-select-option>
        <a-select-option value="h3c">H3C</a-select-option>
        <a-select-option value="cisco">Cisco</a-select-option>
      </a-select>
    </div>

    <a-table :dataSource="templates" :columns="columns" rowKey="id">
      <template #bodyCell="{ column, record }">
        <template v-if="column.key === 'action'">
          <a-space>
            <a-button size="small" @click="testTemplate(record)">测试</a-button>
            <a-button size="small" @click="editTemplate(record)">编辑</a-button>
            <a-popconfirm
              title="确定删除?"
              @confirm="deleteTemplate(record.id)"
            >
              <a-button size="small" danger>删除</a-button>
            </a-popconfirm>
          </a-space>
        </template>
      </template>
    </a-table>

    <!-- 创建/编辑模板弹窗 -->
    <ParseTemplateModal
      v-model:visible="modalVisible"
      :template="editingTemplate"
      @saved="loadTemplates"
    />

    <!-- 测试模板弹窗 -->
    <TestTemplateModal
      v-model:visible="testModalVisible"
      :template="testingTemplate"
    />
  </div>
</template>
```

### 6.2 模板编辑弹窗

```vue
<!-- frontend/src/components/parser/ParseTemplateModal.vue -->
<template>
  <a-modal
    :visible="visible"
    :title="isEdit ? '编辑模板' : '新建模板'"
    @ok="handleSave"
    @cancel="handleCancel"
    width="800px"
  >
    <a-form :model="form" :rules="rules" ref="formRef" layout="vertical">
      <a-row :gutter="16">
        <a-col :span="12">
          <a-form-item label="厂商" name="vendor">
            <a-select v-model:value="form.vendor">
              <a-select-option value="huawei">Huawei</a-select-option>
              <a-select-option value="h3c">H3C</a-select-option>
              <a-select-option value="cisco">Cisco</a-select-option>
            </a-select>
          </a-form-item>
        </a-col>
        <a-col :span="12">
          <a-form-item label="命令键" name="commandKey">
            <a-input v-model:value="form.commandKey" placeholder="如: lldp_neighbor" />
          </a-form-item>
        </a-col>
      </a-row>

      <a-form-item label="正则表达式" name="pattern">
        <a-textarea
          v-model:value="form.pattern"
          :rows="4"
          placeholder="使用 (?P<name>...) 命名组定义字段"
        />
        <div class="pattern-help">
          <a-typography-text type="secondary">
            提示：使用 (?P<字段名>模式) 定义命名组，如 (?P<ip>\d+\.\d+\.\d+\.\d+)
          </a-typography-text>
        </div>
      </a-form-item>

      <a-row :gutter="16">
        <a-col :span="12">
          <a-form-item label="多行模式">
            <a-switch v-model:checked="form.multiline" />
            <a-typography-text type="secondary" style="margin-left: 8px">
              启用后 ^$ 匹配每行开头结尾
            </a-typography-text>
          </a-form-item>
        </a-col>
      </a-row>

      <a-form-item label="描述">
        <a-input v-model:value="form.description" />
      </a-form-item>
    </a-form>
  </a-modal>
</template>
```

### 6.3 模板测试弹窗

```vue
<!-- frontend/src/components/parser/TestTemplateModal.vue -->
<template>
  <a-modal :visible="visible" title="测试解析模板" width="900px" :footer="null">
    <div class="test-container">
      <div class="input-section">
        <h4>示例输入</h4>
        <a-textarea
          v-model:value="sampleText"
          :rows="10"
          placeholder="粘贴设备命令输出..."
        />
        <a-button type="primary" @click="runTest" style="margin-top: 8px">
          执行测试
        </a-button>
      </div>

      <div class="output-section">
        <h4>解析结果</h4>
        <div v-if="result" class="result-area">
          <a-alert
            v-if="!result.success"
            :message="result.error"
            type="error"
            show-icon
          />
          <template v-else>
            <a-tag color="success">成功解析 {{ result.count }} 条记录</a-tag>
            <pre class="result-json">{{
              JSON.stringify(result.rows, null, 2)
            }}</pre>
          </template>
        </div>
      </div>
    </div>
  </a-modal>
</template>

<script setup lang="ts">
import { ref } from "vue";
import { TestParseTemplate } from "../../services/parser";

const props = defineProps<{
  visible: boolean;
  template: ParseTemplate | null;
}>();

const sampleText = ref("");
const result = ref<TestResult | null>(null);

const runTest = async () => {
  if (!props.template) return;

  result.value = await TestParseTemplate({
    commandKey: props.template.commandKey,
    pattern: props.template.pattern,
    multiline: props.template.multiline,
    sampleText: sampleText.value,
  });
};
</script>
```

---

## 7. 迁移执行计划

### 7.1 阶段一：基础设施

| 序号 | 任务                       | 文件                                       | 说明                                              |
| ---- | -------------------------- | ------------------------------------------ | ------------------------------------------------- |
| 1.1  | 扩展 CliParser 接口        | `internal/parser/models.go`                | 添加 `LoadBuiltinTemplates` 和 `AddTemplate` 方法 |
| 1.2  | 创建 `regex_parser.go`     | `internal/parser/regex_parser.go`          | 实现正则解析器                                    |
| 1.3  | 创建 `multiline.go`        | `internal/parser/multiline.go`             | 实现多行聚合处理器（含 eth_trunk）                |
| 1.4  | 创建 `composite_parser.go` | `internal/parser/composite_parser.go`      | 实现复合解析器，添加接口约束检查                  |
| 1.5  | 创建内置模板 JSON          | `internal/parser/templates/builtin/*.json` | 转换全部 24 个模板                                |
| 1.6  | 编写单元测试               | `internal/parser/*_test.go`                | 确保解析结果与 TextFSM 一致                       |

### 7.2 阶段二：集成替换

| 序号 | 任务                | 文件                                            | 说明                                                          |
| ---- | ------------------- | ----------------------------------------------- | ------------------------------------------------------------- |
| 2.1  | 修改 ParseExecutor  | `internal/taskexec/executor_impl.go:690-698`    | 改为依赖 `parser.CliParser` 接口，使用 `NewCompositeParser()` |
| 2.2  | 更新 golden_test.go | `internal/parser/golden_test.go:13`             | 改用 `NewCompositeParser()`                                   |
| 2.3  | 运行集成测试        | -                                               | 验证拓扑采集流程                                              |
| 2.4  | 删除 TextFSM 解析器 | `internal/parser/textfsm.go`                    | 移除文件                                                      |
| 2.5  | 删除 TextFSM 模板   | `internal/parser/templates/**/*.textfsm`        | 移除 24 个 .textfsm 文件                                      |
| 2.6  | 删除 legacy 文件    | `internal/parser/service.go.474295111933871248` | 移除文件                                                      |
| 2.7  | 更新 go.mod         | `go.mod`                                        | 移除 `github.com/sirikothe/gotextfsm` 依赖                    |

### 7.3 阶段三：用户自定义支持

| 序号 | 任务             | 文件                                    | 说明                               |
| ---- | ---------------- | --------------------------------------- | ---------------------------------- |
| 3.1  | 创建数据库模型   | `internal/models/parse_template.go`     | 定义 `UserParseTemplate`           |
| 3.2  | 执行数据库迁移   | -                                       | 创建 `net_user_parse_templates` 表 |
| 3.3  | 实现模板管理服务 | `internal/ui/parse_template_service.go` | CRUD + 测试接口                    |
| 3.4  | 注册 Wails 服务  | `cmd/netweaver/main.go`                 | 绑定模板服务                       |
| 3.5  | 实现前端界面     | `frontend/src/views/ParseTemplates.vue` | 模板管理 + 测试页面                |

### 7.4 阶段四：测试与文档

| 序号 | 任务         | 说明                     |
| ---- | ------------ | ------------------------ |
| 4.1  | 全量回归测试 | 确保所有拓扑采集功能正常 |
| 4.2  | 对比测试     | 新旧解析器结果对比验证   |
| 4.3  | 更新用户文档 | 解析模板使用指南         |

---

## 8. 需要删除的文件清单

```
# TextFSM 解析器
internal/parser/textfsm.go

# Legacy 文件
internal/parser/service.go.474295111933871248

# Huawei 模板 (11 个)
internal/parser/templates/huawei/version.textfsm
internal/parser/templates/huawei/sysname.textfsm
internal/parser/templates/huawei/esn.textfsm
internal/parser/templates/huawei/device_info.textfsm
internal/parser/templates/huawei/interface_brief.textfsm
internal/parser/templates/huawei/interface_detail.textfsm
internal/parser/templates/huawei/lldp_neighbor.textfsm
internal/parser/templates/huawei/mac_address.textfsm
internal/parser/templates/huawei/eth_trunk.textfsm
internal/parser/templates/huawei/eth_trunk_verbose.textfsm
internal/parser/templates/huawei/arp_all.textfsm

# H3C 模板 (6 个)
internal/parser/templates/h3c/version.textfsm
internal/parser/templates/h3c/interface_brief.textfsm
internal/parser/templates/h3c/lldp_neighbor.textfsm
internal/parser/templates/h3c/mac_address.textfsm
internal/parser/templates/h3c/eth_trunk.textfsm
internal/parser/templates/h3c/arp_all.textfsm

# Cisco 模板 (7 个)
internal/parser/templates/cisco/version.textfsm
internal/parser/templates/cisco/interface_brief.textfsm
internal/parser/templates/cisco/interface_detail.textfsm
internal/parser/templates/cisco/lldp_neighbor.textfsm
internal/parser/templates/cisco/mac_address.textfsm
internal/parser/templates/cisco/eth_trunk.textfsm
internal/parser/templates/cisco/arp_all.textfsm
```

---

## 9. 风险与缓解

| 风险                          | 概率 | 影响 | 缓解措施                                                             |
| ----------------------------- | ---- | ---- | -------------------------------------------------------------------- |
| 正则解析结果与 TextFSM 不一致 | 低   | 高   | 编写对比测试，逐一验证 24 个模板                                     |
| 多行聚合场景遗漏              | 低   | 中   | 已覆盖 lldp_neighbor、interface_detail、eth_trunk、eth_trunk_verbose |
| 用户正则语法错误              | 高   | 低   | 提供即时测试功能，友好错误提示                                       |
| 性能回退                      | 低   | 低   | 正则性能与 TextFSM 相当，可忽略                                      |

---

## 10. 验收标准

1. **功能验收**
   - 所有现有拓扑采集功能正常
   - 解析结果与 TextFSM 完全一致
   - 用户可创建、编辑、测试自定义模板

2. **性能验收**
   - 解析耗时与 TextFSM 相当（差异 < 10%）

3. **代码验收**
   - 移除 `gotextfsm` 依赖
   - 移除所有 `.textfsm` 文件
   - 移除 `textfsm.go` 和 legacy 文件
   - `ParseExecutor` 依赖 `CliParser` 接口
   - 单元测试覆盖率 > 80%
   - 代码通过 lint 检查
