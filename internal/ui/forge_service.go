package ui

import (
	"net"
	"regexp"
	"strconv"
	"strings"

	"github.com/NetWeaverGo/core/internal/forge"
)

// ForgeService 配置构建服务 (Wails Binding)
type ForgeService struct {
	builder *forge.ConfigBuilder
}

// NewForgeService 创建配置构建服务
func NewForgeService() *ForgeService {
	return &ForgeService{
		builder: forge.NewConfigBuilder(),
	}
}

// BuildConfig 构建配置 (Wails Binding)
// 前端调用此方法执行配置生成
func (s *ForgeService) BuildConfig(req *forge.BuildRequest) (*forge.BuildResult, error) {
	return s.builder.Build(req)
}

// ExpandValues 展开变量值 (Wails Binding)
// 前端调用此方法预览语法糖展开结果
func (s *ForgeService) ExpandValues(req *forge.ExpandRequest) (*forge.ExpandResult, error) {
	return s.builder.ExpandValues(req)
}

// ForgeIPValidationResult IP验证结果（Forge专用）
type ForgeIPValidationResult struct {
	IsValid bool   `json:"isValid"` // 是否有效
	Type    string `json:"type"`    // IP类型: "IPv4", "IPv6", ""
	Message string `json:"message"` // 提示信息
}

// ValidateIP 验证IP格式 (Wails Binding)
// 支持 IPv4 和 IPv6 格式验证
func (s *ForgeService) ValidateIP(ip string) *ForgeIPValidationResult {
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return &ForgeIPValidationResult{
			IsValid: false,
			Type:    "",
			Message: "IP地址为空",
		}
	}

	// 尝试解析为 IPv4
	if ip4 := net.ParseIP(ip); ip4 != nil {
		if ip4.To4() != nil {
			return &ForgeIPValidationResult{
				IsValid: true,
				Type:    "IPv4",
				Message: "有效的IPv4地址",
			}
		}
		// IPv6 地址
		return &ForgeIPValidationResult{
			IsValid: true,
			Type:    "IPv6",
			Message: "有效的IPv6地址",
		}
	}

	return &ForgeIPValidationResult{
		IsValid: false,
		Type:    "",
		Message: "无效的IP地址格式",
	}
}

// IPRangeResult IP范围解析结果
type IPRangeResult struct {
	IsValid bool     `json:"isValid"` // 是否有效
	Start   string   `json:"start"`   // 起始IP
	End     string   `json:"end"`     // 结束IP
	Count   int      `json:"count"`   // IP数量
	List    []string `json:"list"`    // IP列表
	Message string   `json:"message"` // 提示信息
}

// ParseIPRange 解析IP范围语法 (Wails Binding)
// 支持格式: "192.168.1.10-20" -> {start: "192.168.1.10", end: "192.168.1.20", count: 11}
func (s *ForgeService) ParseIPRange(ipRange string) *IPRangeResult {
	ipRange = strings.TrimSpace(ipRange)
	if ipRange == "" {
		return &IPRangeResult{
			IsValid: false,
			Message: "IP范围为空",
		}
	}

	// 正则匹配: 前缀 + 起始数字 + 分隔符 + 结束数字
	// 例如: 192.168.1.10-20
	re := regexp.MustCompile(`^(.*?)(\d+)([-~])(\d+)$`)
	matches := re.FindStringSubmatch(ipRange)

	if matches == nil {
		return &IPRangeResult{
			IsValid: false,
			Message: "无法识别IP范围格式，期望格式如: 192.168.1.10-20",
		}
	}

	prefix := matches[1]
	startStr := matches[2]
	endStr := matches[4]

	start := net.ParseIP(prefix + startStr)
	end := net.ParseIP(prefix + endStr)

	if start == nil || end == nil {
		// 尝试作为数字解析
		startNum, _ := strconv.Atoi(startStr)
		endNum, _ := strconv.Atoi(endStr)

		// 验证基础IP部分是否有效
		baseIP := net.ParseIP(prefix)
		if baseIP == nil {
			return &IPRangeResult{
				IsValid: false,
				Message: "无效的IP前缀: " + prefix,
			}
		}

		// 防呆：范围太大
		count := endNum - startNum + 1
		if count > 1000 || count < 0 {
			return &IPRangeResult{
				IsValid: false,
				Message: "IP范围太大或无效，最大支持1000个IP",
			}
		}

		// 生成IP列表
		padLen := len(startStr)
		list := make([]string, 0, count)
		for i := startNum; i <= endNum; i++ {
			numStr := strconv.Itoa(i)
			// 补零对齐
			for len(numStr) < padLen {
				numStr = "0" + numStr
			}
			list = append(list, prefix+numStr)
		}

		return &IPRangeResult{
			IsValid: true,
			Start:   prefix + startStr,
			End:     prefix + endStr,
			Count:   count,
			List:    list,
			Message: "成功解析IP范围",
		}
	}

	return &IPRangeResult{
		IsValid: false,
		Message: "无法解析IP范围",
	}
}

// IPsValidationResult 批量IP验证结果
type IPsValidationResult struct {
	ValidCount   int      `json:"validCount"`   // 有效IP数量
	InvalidCount int      `json:"invalidCount"` // 无效IP数量
	ValidIPs     []string `json:"validIPs"`     // 有效IP列表
	InvalidIPs   []string `json:"invalidIPs"`   // 无效IP列表
}

// ValidateIPs 批量验证IP (Wails Binding)
// 输入逗号或换行分隔的IP字符串，返回验证结果
func (s *ForgeService) ValidateIPs(ipString string) *IPsValidationResult {
	parts := strings.FieldsFunc(ipString, func(r rune) bool {
		return r == ',' || r == '\n'
	})

	validIPs := make([]string, 0)
	invalidIPs := make([]string, 0)

	for _, part := range parts {
		ip := strings.TrimSpace(part)
		if ip == "" {
			continue
		}

		result := s.ValidateIP(ip)
		if result.IsValid {
			validIPs = append(validIPs, ip)
		} else {
			invalidIPs = append(invalidIPs, ip)
		}
	}

	return &IPsValidationResult{
		ValidCount:   len(validIPs),
		InvalidCount: len(invalidIPs),
		ValidIPs:     validIPs,
		InvalidIPs:   invalidIPs,
	}
}

// BindingPreview 绑定预览结果
type BindingPreview struct {
	IP       string `json:"ip"`       // IP地址
	Commands string `json:"commands"` // 对应的命令
}

// GenerateBindingPreview 生成绑定模式预览 (Wails Binding)
// isIPBinding: 是否启用IP绑定模式（由前端开关控制）
func (s *ForgeService) GenerateBindingPreview(template string, variables []forge.VarInput, isIPBinding bool) ([]BindingPreview, error) {
	// 如果未启用IP绑定模式，直接返回空结果
	if !isIPBinding {
		return []BindingPreview{}, nil
	}
	// 首先构建配置
	req := &forge.BuildRequest{
		Template:  template,
		Variables: variables,
	}

	result, err := s.builder.Build(req)
	if err != nil {
		return nil, err
	}

	// 获取第一个变量的值作为IP列表
	if len(variables) == 0 {
		return []BindingPreview{}, nil
	}

	expanded, err := forge.ParseVariableValues(variables[0].ValueString)
	if err != nil {
		return nil, err
	}

	// 过滤有效IP
	previews := make([]BindingPreview, 0)
	for i, ip := range expanded {
		validation := s.ValidateIP(ip)
		if !validation.IsValid {
			continue
		}

		blockIndex := i
		if blockIndex >= len(result.Blocks) {
			blockIndex = len(result.Blocks) - 1
		}
		if blockIndex < 0 {
			blockIndex = 0
		}

		var commands string
		if len(result.Blocks) > 0 && blockIndex < len(result.Blocks) {
			commands = result.Blocks[blockIndex]
		}

		previews = append(previews, BindingPreview{
			IP:       ip,
			Commands: commands,
		})
	}

	return previews, nil
}
