package parser

import (
	"embed"
	"fmt"
	"strings"
	"sync"

	"github.com/sirikothe/gotextfsm"
)

//go:embed templates/**/*.textfsm
var templateFS embed.FS

var vendorTemplateRegistry = map[string]map[string]string{
	"huawei": {
		"version":           "templates/huawei/version.textfsm",
		"sysname":           "templates/huawei/sysname.textfsm",
		"esn":               "templates/huawei/esn.textfsm",
		"device_info":       "templates/huawei/device_info.textfsm",
		"interface_brief":   "templates/huawei/interface_brief.textfsm",
		"interface_detail":  "templates/huawei/interface_detail.textfsm",
		"lldp_neighbor":     "templates/huawei/lldp_neighbor.textfsm",
		"mac_address":       "templates/huawei/mac_address.textfsm",
		"eth_trunk":         "templates/huawei/eth_trunk.textfsm",
		"eth_trunk_verbose": "templates/huawei/eth_trunk_verbose.textfsm",
		"arp_all":           "templates/huawei/arp_all.textfsm",
	},
	"h3c": {
		"version":         "templates/h3c/version.textfsm",
		"interface_brief": "templates/h3c/interface_brief.textfsm",
		"lldp_neighbor":   "templates/h3c/lldp_neighbor.textfsm",
		"mac_address":     "templates/h3c/mac_address.textfsm",
		"eth_trunk":       "templates/h3c/eth_trunk.textfsm",
		"arp_all":         "templates/h3c/arp_all.textfsm",
	},
	"cisco": {
		"version":          "templates/cisco/version.textfsm",
		"interface_brief":  "templates/cisco/interface_brief.textfsm",
		"interface_detail": "templates/cisco/interface_detail.textfsm",
		"lldp_neighbor":    "templates/cisco/lldp_neighbor.textfsm",
		"mac_address":      "templates/cisco/mac_address.textfsm",
		"eth_trunk":        "templates/cisco/eth_trunk.textfsm",
		"arp_all":          "templates/cisco/arp_all.textfsm",
	},
}

// TextFSMParser 基于 gotextfsm 的解析器。
type TextFSMParser struct {
	registry      map[string]string
	templateCache map[string]string
	mu            sync.RWMutex
}

func NewTextFSMParser() *TextFSMParser {
	return &TextFSMParser{
		registry:      make(map[string]string),
		templateCache: make(map[string]string),
	}
}

func (p *TextFSMParser) LoadBuiltinTemplates(vendor string) error {
	if vendor == "" {
		vendor = "huawei"
	}
	registry, ok := vendorTemplateRegistry[vendor]
	if !ok {
		registry = vendorTemplateRegistry["huawei"]
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	p.registry = make(map[string]string, len(registry))
	for k, v := range registry {
		p.registry[k] = v
	}
	return nil
}

func (p *TextFSMParser) Parse(commandKey string, rawText string) ([]map[string]string, error) {
	p.mu.RLock()
	path, ok := p.registry[commandKey]
	p.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("template not found for command key: %s", commandKey)
	}

	tpl, err := p.loadTemplate(path)
	if err != nil {
		return nil, err
	}

	fsm := gotextfsm.TextFSM{}
	if err := fsm.ParseString(tpl); err != nil {
		return nil, fmt.Errorf("parse template %s failed: %w", path, err)
	}

	output := gotextfsm.ParserOutput{}
	if err := output.ParseTextString(rawText, fsm, true); err != nil {
		return nil, fmt.Errorf("parse raw text with %s failed: %w", commandKey, err)
	}

	rows := make([]map[string]string, 0, len(output.Dict))
	for _, rec := range output.Dict {
		row := make(map[string]string, len(rec))
		for k, v := range rec {
			switch tv := v.(type) {
			case string:
				row[k] = strings.TrimSpace(tv)
			case []string:
				row[k] = strings.Join(tv, ",")
			default:
				row[k] = strings.TrimSpace(fmt.Sprintf("%v", tv))
			}
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func (p *TextFSMParser) loadTemplate(path string) (string, error) {
	p.mu.RLock()
	if tpl, ok := p.templateCache[path]; ok {
		p.mu.RUnlock()
		return tpl, nil
	}
	p.mu.RUnlock()

	data, err := templateFS.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read template %s failed: %w", path, err)
	}
	tpl := string(data)

	p.mu.Lock()
	p.templateCache[path] = tpl
	p.mu.Unlock()

	return tpl, nil
}
