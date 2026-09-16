package xmlcfg

import "encoding/xml"

// CommandParseConfig 对应 XML 根节点 <CommandParseConfig>
type CommandParseConfig struct {
	XMLName  xml.Name       `xml:"CommandParseConfig"`
	Segments []SegmentParse `xml:"SegmentParse"`
}

// SegmentParse 对应 <SegmentParse> 分片解析节点
type SegmentParse struct {
	XMLName          xml.Name     `xml:"SegmentParse"`
	ChildMutex       string       `xml:"childMutex,attr"`
	ExtractGroupName string       `xml:"extractGroupName,attr"`
	HasFirst         string       `xml:"hasFirst,attr"`
	Regex            string       `xml:"Regex"`
	ParseNodes       []ParseNode  `xml:"ParseNode"`
	DefaultNode      *DefaultNode `xml:"DefaultNode"`
}

// ParseNode 对应 <ParseNode>
type ParseNode struct {
	XMLName          xml.Name                  `xml:"ParseNode"`
	Name             string                    `xml:"name,attr"`
	ProductVersions  string                    `xml:"productVersions,attr"`
	XmlConfigPath    string                    `xml:"xmlConfigPath,attr"`
	SupportDefault   string                    `xml:"supportDefault,attr"`
	Regex            string                    `xml:"Regex"`
	ConfigPolicy     *ConfigParsePolicy        `xml:"ConfigParsePolicy"`
	TableLinePolicy  *TableLineParsePolicy     `xml:"TableLineParsePolicy"`
	TableColPolicy   *TableColumnParsePolicy   `xml:"TableColumnParsePolicy"`
	DynamicColPolicy *DynamicColumnParsePolicy `xml:"DynamicColumnParsePolicy"`
	Filter           *FilterNode               `xml:"Filter"`
	DefaultValue     *DefaultValueNode         `xml:"DefaultValue"`
	MergeField       *MergeFieldNode           `xml:"MergeField"`
	RenameField      *RenameFieldNode          `xml:"RenameField"`
	SegmentParse     *SegmentParse             `xml:"SegmentParse"`
}

// ConfigParsePolicy 非表格配置解析策略
type ConfigParsePolicy struct {
	Fields []FieldNode `xml:"Field"`
}

// TableLineParsePolicy 逐行表格解析策略
type TableLineParsePolicy struct {
	Regex  string      `xml:"Regex"`
	Fields []FieldNode `xml:"Field"`
}

// TableColumnParsePolicy 逐列表格解析策略
type TableColumnParsePolicy struct {
	NewlineSupport string      `xml:"newlineSupport,attr"`
	Regex          string      `xml:"Regex"`
	Fields         []FieldNode `xml:"Field"`
}

// DynamicColumnParsePolicy 动态列表格解析策略
type DynamicColumnParsePolicy struct {
	Delimiter string      `xml:"delimiter,attr"`
	Fields    []FieldNode `xml:"Field"`
}

// FieldNode 字段规约
type FieldNode struct {
	Name           string            `xml:"name,attr"`
	Primary        string            `xml:"primary,attr"`
	Merge          string            `xml:"merge,attr"`
	MergeDelimiter string            `xml:"mergeDelimiter,attr"`
	IsEndOfLine    string            `xml:"isEndOfLine,attr"`
	Regex          string            `xml:"Regex"`
	ToUpper        *ToUpperNode      `xml:"ToUpper"`
	ToLower        *ToLowerNode      `xml:"ToLower"`
	MatchAndSet    *MatchAndSetNode  `xml:"MatchAndSet"`
	ValueMapping   *ValueMappingNode `xml:"ValueMapping"`
	StrExtract     *StrExtractNode   `xml:"StrExtract"`
	SplitField     *SplitFieldNode   `xml:"SplitField"`
	MergeField     *MergeFieldNode   `xml:"MergeField"`
	ReplaceAll     *ReplaceAllNode   `xml:"ReplaceAll"`
	Assigns        []AssignNode      `xml:"Assign"`
	StrConcat      *StrConcatNode    `xml:"StrConcat"`
	DefaultValue   *DefaultValueNode `xml:"DefaultValue"`
}

// --- 算子节点映射 ---

type ToUpperNode struct {
	Name  string `xml:"name,attr"`
	Order string `xml:"order,attr"`
}

type ToLowerNode struct {
	Name  string `xml:"name,attr"`
	Order string `xml:"order,attr"`
}

type MatchAndSetNode struct {
	Name  string `xml:"name,attr"`
	Value string `xml:"value,attr"`
	Order string `xml:"order,attr"`
}

type ValueMappingNode struct {
	Name     string `xml:"name,attr"`
	SrcValue string `xml:"srcValue,attr"`
	DstValue string `xml:"dstValue,attr"`
	Order    string `xml:"order,attr"`
}

type StrExtractNode struct {
	Name    string `xml:"name,attr"`
	Regex   string `xml:"regex,attr"`
	GroupId string `xml:"groupId,attr"`
	Order   string `xml:"order,attr"`
}

type DefaultValueNode struct {
	Name  string `xml:"name,attr"`
	Value string `xml:"value,attr"`
	Order string `xml:"order,attr"`
}

type FilterNode struct {
	FilterFieldName string `xml:"filterFieldName,attr"`
	FilterType      string `xml:"filterType,attr"`
	FilterValue     string `xml:"filterValue,attr"`
	Order           string `xml:"order,attr"`
}

type SplitFieldNode struct {
	SrcField  string `xml:"srcField,attr"`
	Delimiter string `xml:"delimiter,attr"`
	DstFields string `xml:"dstFields,attr"`
	Order     string `xml:"order,attr"`
}

type MergeFieldNode struct {
	SrcFields string `xml:"srcFields,attr"`
	Delimiter string `xml:"delimiter,attr"`
	DstField  string `xml:"dstField,attr"`
	Order     string `xml:"order,attr"`
}

type ReplaceAllNode struct {
	Name        string `xml:"name,attr"`
	Regex       string `xml:"regex,attr"`
	Replacement string `xml:"replacement,attr"`
	Order       string `xml:"order,attr"`
}

type AssignNode struct {
	AssignMap       string `xml:"assignMap,attr"`
	FilterFieldName string `xml:"filterFieldName,attr"`
	FilterType      string `xml:"filterType,attr"`
	FilterValue     string `xml:"filterValue,attr"`
	Order           string `xml:"order,attr"`
}

type RenameFieldNode struct {
	FieldMap string `xml:"fieldMap,attr"`
	Order    string `xml:"order,attr"`
}

type StrConcatNode struct {
	SrcField  string `xml:"srcField,attr"`
	PrefixStr string `xml:"prefixStr,attr"`
	SuffixStr string `xml:"suffixStr,attr"`
	DstField  string `xml:"dstField,attr"`
	Order     string `xml:"order,attr"`
}

type DefaultNode struct {
	Name         string        `xml:"name,attr"`
	Attributes   string        `xml:"attributes,attr"`
	SegmentParse *SegmentParse `xml:"SegmentParse"`
}

// --- parseitem 目录映射 ---

type CommonParseItem struct {
	XMLName       xml.Name       `xml:"CommonParseItem"`
	VendorCommons []VendorCommon `xml:"VendorCommon"`
}

type VendorCommon struct {
	Vendor        string         `xml:"vendor,attr"`
	AfterClass    string         `xml:"afterClass,attr"`
	CommandParses []CommandParse `xml:"CommandParse"`
}

type CommandParse struct {
	Cmd            string `xml:"cmd,attr"`
	IsRegexCmd     string `xml:"isRegexCmd,attr"`
	Path           string `xml:"path,attr"`
	IsCommandMutex string `xml:"isCommandMutex,attr"`
	IsResultMutex  string `xml:"isResultMutex,attr"`
	SplitRegex     string `xml:"splitRegex,attr"`
}
