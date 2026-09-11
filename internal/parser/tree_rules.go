package parser

import (
	"regexp"
)

// TreeRule 声明式解析规则（对齐 eDesk Pro ParseRule 体系）
type TreeRule struct {
	ParseItem    string `json:"parseItem"`    // 字段名
	ParentItem   string `json:"parentItem"`   // 父字段名；空字符串表示根节点
	IsList       bool   `json:"isList"`       // 是否为列表节点（分块或多匹配）
	ItemType     string `json:"itemType"`     // string | number
	ParseRegex   string `json:"parseRegex"`   // 取值正则
	ParseFlags   string `json:"parseFlags"`   // "m" / "i" / "mi" → 内联标志 (?mi)
	GroupIndex   int    `json:"groupIndex"`   // 捕获组序号（默认 1）
	SplitRegex   string `json:"splitRegex"`   // 分块正则（列表节点）
	SplitFlags   string `json:"splitFlags"`   // 分块正则标志
	IsOutput     bool   `json:"isOutput"`     // 是否输出到平铺结果
	DefaultValue string `json:"defaultValue"` // 未命中时的默认值（缺省 ""）
	Order        int    `json:"order"`        // 执行/排序优先级
}

// TreeTemplate 树形模板配置（存入模板记录）
type TreeTemplate struct {
	SchemaVersion  string     `json:"schemaVersion,omitempty"` // 模板格式版本预留
	Author         string     `json:"author,omitempty"`        // 作者预留
	Tags           []string   `json:"tags,omitempty"`          // 标签预留
	Rules          []TreeRule `json:"rules"`                   // 规则清单
	MaxOutputLevel int        `json:"maxOutputLevel"`          // 拍平深度（0 表示全部）
}

// CompiledTreeRule 预编译后的树形规则
type CompiledTreeRule struct {
	TreeRule
	CompiledParseRegex *regexp.Regexp
	CompiledSplitRegex *regexp.Regexp
	Children           []*CompiledTreeRule `json:"-"`
	IsPath             bool                `json:"-"` // 回溯标记：是否在通往 isOutput 的路径上
}

// ResultNode 树形解析生成的内部结果节点
type ResultNode struct {
	ItemName string            // 规则名 / parseItem
	Attrs    map[string]string // 当前层级提取的属性键值对
	Children []*ResultNode     // 子节点列表
	RawBlock string            // 提取到的原始文本块（调试/回溯用）
}

// NewResultNode 创建新的结果节点
func NewResultNode(itemName string, rawBlock string) *ResultNode {
	return &ResultNode{
		ItemName: itemName,
		Attrs:    make(map[string]string),
		Children: make([]*ResultNode, 0),
		RawBlock: rawBlock,
	}
}
