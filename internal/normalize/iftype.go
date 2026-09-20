package normalize

import (
	_ "embed"
	"encoding/json"
	"sort"
	"strings"
	"sync"
)

//go:embed data/ifname_alias.json
var embeddedIfNameAliasJSON []byte

var (
	aliasMap     map[string]string
	aliasMapOnce sync.Once
)

func getAliasMap() map[string]string {
	aliasMapOnce.Do(func() {
		aliasMap = make(map[string]string)
		if len(embeddedIfNameAliasJSON) > 0 {
			_ = json.Unmarshal(embeddedIfNameAliasJSON, &aliasMap)
		}
	})
	return aliasMap
}

// ifType 数字编码常量（对齐 IANA ifType MIB 与 eDeskPro InterfaceTransition）
const (
	IFTypeOther                  = 1
	IFTypeRegular1822            = 2
	IFTypeHdh1822                = 3
	IFTypeDdnX25                 = 4
	IFTypeRfc877x25              = 5
	IFTypeEthernet               = 6
	IFTypeIso88023Csmacd         = 7
	IFTypeIso88024TokenBus       = 8
	IFTypeIso88025TokenRing      = 9
	IFTypeFddi                   = 15
	IFTypeLapb                   = 16
	IFTypeSdlc                   = 17
	IFTypeDs1                    = 18
	IFTypeE1                     = 19
	IFTypeBasicISDN              = 20
	IFTypePrimaryISDN            = 21
	IFTypePropPointToPointSerial = 22
	IFTypePpp                    = 23
	IFTypeLoopback               = 24
	IFTypeEon                    = 25
	IFTypeEthernet3Mbit          = 26
	IFTypeNsip                   = 27
	IFTypeSlip                   = 28
	IFTypeUltra                  = 29
	IFTypeDs3                    = 30
	IFTypeSip                    = 31
	IFTypeFrameRelay             = 32
	IFTypeAtm                    = 37
	IFTypeMiox25                 = 38
	IFTypePos                    = 39
	IFTypeVlanif                 = 53
	IFTypeTunnel                 = 131
	IFTypeTrunk                  = 161
	IFTypeLldp                   = 200
	IFTypeCellular               = 243
)

var (
	codeToName = map[int]string{
		IFTypeEthernet:   "Ethernet",
		IFTypePos:        "Pos",
		IFTypeVlanif:     "Vlanif",
		IFTypeTunnel:     "Tunnel",
		IFTypeTrunk:      "Trunk",
		IFTypeLoopback:   "LoopBack",
		IFTypeAtm:        "Atm",
		IFTypeCellular:   "Cellular",
		IFTypeFrameRelay: "FrameRelay",
		IFTypePpp:        "Ppp",
	}

	nameToCode = map[string]int{
		"ethernet":  IFTypeEthernet,
		"eth":       IFTypeEthernet,
		"ge":        IFTypeEthernet,
		"xge":       IFTypeEthernet,
		"10ge":      IFTypeEthernet,
		"25ge":      IFTypeEthernet,
		"40ge":      IFTypeEthernet,
		"100ge":     IFTypeEthernet,
		"400ge":     IFTypeEthernet,
		"fe":        IFTypeEthernet,
		"pos":       IFTypePos,
		"vlanif":    IFTypeVlanif,
		"vlan":      IFTypeVlanif,
		"tunnel":    IFTypeTunnel,
		"trunk":     IFTypeTrunk,
		"eth-trunk": IFTypeTrunk,
		"loop":      IFTypeLoopback,
		"loopback":  IFTypeLoopback,
		"atm":       IFTypeAtm,
		"cellular":  IFTypeCellular,
		"cell":      IFTypeCellular,
	}
)

// nameToCodeOrdered 按"最长键优先"排序的接口名前缀列表。
// 顺序固定（长度倒序 + 字典序），消除 map 迭代随机性导致的前缀命中不确定
// （例如 eth-trunk10 必须命中 eth-trunk 而非 eth）。
var nameToCodeOrdered = func() []struct {
	key  string
	code int
} {
	items := make([]struct {
		key  string
		code int
	}, 0, len(nameToCode))
	for k, v := range nameToCode {
		items = append(items, struct {
			key  string
			code int
		}{k, v})
	}
	sort.Slice(items, func(i, j int) bool {
		if len(items[i].key) != len(items[j].key) {
			return len(items[i].key) > len(items[j].key)
		}
		return items[i].key < items[j].key
	})
	return items
}()

// NameToIFType 接口名转换为标准 ifType 数字编码（最长前缀优先，结果确定）
func NameToIFType(ifName string) (int, bool) {
	norm := strings.ToLower(NormalizeInterfaceName(ifName))
	for _, item := range nameToCodeOrdered {
		if strings.HasPrefix(norm, item.key) {
			return item.code, true
		}
	}
	return 0, false
}

// IFTypeToName 数字编码转换为标准规范名称
func IFTypeToName(ifType int) (string, bool) {
	name, ok := codeToName[ifType]
	return name, ok
}

// aliasOrdered 按"最长别名优先"排序的别名替换列表（确定性）
var aliasOrdered = func() []struct {
	alias       string
	replacement string
} {
	aliases := getAliasMap()
	items := make([]struct {
		alias       string
		replacement string
	}, 0, len(aliases))
	for a, r := range aliases {
		items = append(items, struct {
			alias       string
			replacement string
		}{a, r})
	}
	sort.Slice(items, func(i, j int) bool {
		if len(items[i].alias) != len(items[j].alias) {
			return len(items[i].alias) > len(items[j].alias)
		}
		return items[i].alias < items[j].alias
	})
	return items
}()

// NormalizeWithAlias 增强版接口名归一化（包含厂商别名表替换，最长别名优先）
func NormalizeWithAlias(name string) string {
	raw := strings.TrimSpace(name)
	if raw == "" {
		return ""
	}

	for _, item := range aliasOrdered {
		if strings.HasPrefix(raw, item.alias) {
			remainder := strings.TrimPrefix(raw, item.alias)
			return item.replacement + remainder
		}
	}

	return NormalizeInterfaceName(raw)
}
