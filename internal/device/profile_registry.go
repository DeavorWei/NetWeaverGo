package device

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
)

//go:embed profiles/product_items.json
var embeddedProductItemsJSON []byte

//go:embed profiles/domains.json
var embeddedDomainsJSON []byte

//go:embed profiles/capabilities.json
var embeddedCapabilitiesJSON []byte

// ProductCategory 产品大类（显式保留 index 字段）
type ProductCategory struct {
	Index      int            `json:"index"`
	Name       string         `json:"name"`
	ZhName     string         `json:"zhName"`
	TypeRegex  string         `json:"typeRegex"`
	IsSoftware bool           `json:"isSoftware"`
	compiled   *regexp.Regexp `json:"-"`
}

// DomainRule 产品域规则（顺序敏感，首条命中）
type DomainRule struct {
	Order    int            `json:"order"`
	Pattern  string         `json:"pattern"`
	Domain   string         `json:"domain"`
	compiled *regexp.Regexp `json:"-"`
}

// CapabilityMatrix 设备能力准入矩阵
type CapabilityMatrix struct {
	DeviceTypes    []string            `json:"deviceTypes"`
	DeviceVersions map[string][]string `json:"deviceVersions"`
}

// ProfileRegistry 统一设备画像与能力注册表
type ProfileRegistry struct {
	mu         sync.RWMutex
	categories []ProductCategory
	domains    []DomainRule
	matrix     CapabilityMatrix
}

var (
	defaultProfileRegistry     *ProfileRegistry
	defaultProfileRegistryOnce sync.Once
)

// GetDefaultProfileRegistry 获取画像注册表全局单例
func GetDefaultProfileRegistry() *ProfileRegistry {
	defaultProfileRegistryOnce.Do(func() {
		reg, err := NewProfileRegistry()
		if err != nil {
			defaultProfileRegistry = &ProfileRegistry{}
			return
		}
		defaultProfileRegistry = reg
	})
	return defaultProfileRegistry
}

// NewProfileRegistry 初始化画像注册表
func NewProfileRegistry() (*ProfileRegistry, error) {
	reg := &ProfileRegistry{}

	// 1. 加载 product_items
	if len(embeddedProductItemsJSON) > 0 {
		var cats []ProductCategory
		if err := json.Unmarshal(embeddedProductItemsJSON, &cats); err != nil {
			return nil, fmt.Errorf("解析 product_items 失败: %w", err)
		}
		for i := range cats {
			if cats[i].TypeRegex != "" {
				// 支持以管道分隔的型号前缀
				pat := fmt.Sprintf("^(?:%s)", cats[i].TypeRegex)
				re, err := regexp.Compile(pat)
				if err == nil {
					cats[i].compiled = re
				}
			}
		}
		reg.categories = cats
	}

	// 2. 加载 domains（必须保序！）
	if len(embeddedDomainsJSON) > 0 {
		var dms []DomainRule
		if err := json.Unmarshal(embeddedDomainsJSON, &dms); err != nil {
			return nil, fmt.Errorf("解析 domains 失败: %w", err)
		}
		for i := range dms {
			re, err := regexp.Compile(dms[i].Pattern)
			if err == nil {
				dms[i].compiled = re
			}
		}
		reg.domains = dms
	}

	// 3. 加载 capabilities
	if len(embeddedCapabilitiesJSON) > 0 {
		var matrix CapabilityMatrix
		if err := json.Unmarshal(embeddedCapabilitiesJSON, &matrix); err != nil {
			return nil, fmt.Errorf("解析 capabilities 失败: %w", err)
		}
		reg.matrix = matrix
	}

	return reg, nil
}

// MatchDomain 按严格声明顺序首条命中返回产品域
func (r *ProfileRegistry) MatchDomain(model string) (string, bool) {
	m := strings.TrimSpace(model)
	if m == "" {
		return "", false
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, rule := range r.domains {
		if rule.compiled != nil && rule.compiled.MatchString(m) {
			return rule.Domain, true
		}
	}
	return "", false
}

// MatchCategory 匹配产品大类
func (r *ProfileRegistry) MatchCategory(model string) (*ProductCategory, bool) {
	m := strings.TrimSpace(model)
	if m == "" {
		return nil, false
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, cat := range r.categories {
		if cat.compiled != nil && cat.compiled.MatchString(m) {
			catCopy := cat
			return &catCopy, true
		}
	}
	return nil, false
}

// IsDeviceSupported 校验款型及其版本是否在能力准入白名单中
func (r *ProfileRegistry) IsDeviceSupported(model, version string) (bool, string) {
	m := strings.TrimSpace(model)
	if m == "" {
		return false, "款型为空"
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	// 1. 查找款型匹配（支持前缀或正则）
	matchedType := ""
	for _, dt := range r.matrix.DeviceTypes {
		if strings.EqualFold(m, dt) || strings.HasPrefix(m, dt) {
			matchedType = dt
			break
		}
	}

	if matchedType == "" {
		return false, fmt.Sprintf("款型 %s 未在支持设备清单中", model)
	}

	// 若未指定版本，则款型命中即视为支持
	v := strings.TrimSpace(version)
	if v == "" {
		return true, ""
	}

	// 2. 校验版本
	supportedVers, ok := r.matrix.DeviceVersions[matchedType]
	if !ok || len(supportedVers) == 0 {
		return true, "" // 款型无版本限制
	}

	for _, sv := range supportedVers {
		// 转换 eDesk 通配符，如 V200RXXX -> ^V200R\d+
		pat := strings.ReplaceAll(sv, "XXX", `\d+`)
		re, err := regexp.Compile("(?i)" + pat)
		if err == nil && re.MatchString(v) {
			return true, ""
		}
	}

	return false, fmt.Sprintf("款型 %s 的版本 %s 未在支持版本列表中", model, version)
}

// GetDomains 返回全部域规则（测试与调试用）
func (r *ProfileRegistry) GetDomains() []DomainRule {
	r.mu.RLock()
	defer r.mu.RUnlock()
	res := make([]DomainRule, len(r.domains))
	copy(res, r.domains)
	return res
}

// GetCategories 返回全部大类（测试与调试用）
func (r *ProfileRegistry) GetCategories() []ProductCategory {
	r.mu.RLock()
	defer r.mu.RUnlock()
	res := make([]ProductCategory, len(r.categories))
	copy(res, r.categories)
	return res
}
