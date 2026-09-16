package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// XML 模型
type ProductItemXML struct {
	XMLName  xml.Name     `xml:"xml"`
	DevTypes []DevTypeXML `xml:"devType"`
}

type DevTypeXML struct {
	Name       string    `xml:"name,attr"`
	Index      string    `xml:"index,attr"`
	Type       string    `xml:"type,attr"`
	IsSoftware string    `xml:"isSoftware,attr"`
	Langs      []LangXML `xml:"lang"`
}

type LangXML struct {
	Value string `xml:"value,attr"`
	Text  string `xml:",chardata"`
}

// 目标 JSON 结构
type ProductCategory struct {
	Index      int    `json:"index"`
	Name       string `json:"name"`
	ZhName     string `json:"zhName"`
	TypeRegex  string `json:"typeRegex"`
	IsSoftware bool   `json:"isSoftware"`
}

type DomainRule struct {
	Order   int    `json:"order"`
	Pattern string `json:"pattern"`
	Domain  string `json:"domain"`
}

type CapabilityMatrix struct {
	DeviceTypes    []string            `json:"deviceTypes"`
	DeviceVersions map[string][]string `json:"deviceVersions"`
}

func main() {
	baseDir := `D:\ICSLite_download\eDesk_Pro_V100R025C10SPC300\eDeskPro_V100R025C10SPC300-windows-x64`
	outDir := `internal/config/device_profiles`
	deviceOutDir := `internal/device/profiles`
	_ = os.MkdirAll(outDir, 0755)
	_ = os.MkdirAll(deviceOutDir, 0755)

	// 1. 转换 productItem.xml
	productItemPath := filepath.Join(baseDir, `config\devicemapping\productItem.xml`)
	convertProductItems(productItemPath, filepath.Join(outDir, "product_items.json"))
	copyFile(filepath.Join(outDir, "product_items.json"), filepath.Join(deviceOutDir, "product_items.json"))

	// 2. 转换 domain.json（保序）
	domainPath := filepath.Join(baseDir, `services\IPOnlineService\webapps\ROOT\WEB-INF\classes\template\businesscompare\domain.json`)
	convertDomains(domainPath, filepath.Join(outDir, "domains.json"))
	copyFile(filepath.Join(outDir, "domains.json"), filepath.Join(deviceOutDir, "domains.json"))

	// 3. 转换 support_devices.json
	supportPath := filepath.Join(baseDir, `services\IPOnlineService\webapps\ROOT\WEB-INF\classes\template\businesscompare\support_devices.json`)
	convertCapabilities(supportPath, filepath.Join(outDir, "capabilities.json"))
	copyFile(filepath.Join(outDir, "capabilities.json"), filepath.Join(deviceOutDir, "capabilities.json"))
}

func copyFile(src, dst string) {
	b, err := os.ReadFile(src)
	if err == nil {
		_ = os.WriteFile(dst, b, 0644)
	}
}

func convertProductItems(inPath, outPath string) {
	data, err := os.ReadFile(inPath)
	if err != nil {
		fmt.Printf("读取 %s 失败: %v\n", inPath, err)
		return
	}

	var root ProductItemXML
	if err := xml.Unmarshal(data, &root); err != nil {
		fmt.Printf("解析 XML 失败: %v\n", err)
		return
	}

	var items []ProductCategory
	seen := make(map[string]bool)

	for _, dt := range root.DevTypes {
		idx, _ := strconv.Atoi(dt.Index)
		isSoft := strings.EqualFold(dt.IsSoftware, "true")
		zh := dt.Name
		for _, l := range dt.Langs {
			if l.Value == "zh" {
				zh = strings.TrimSpace(l.Text)
			}
		}

		key := fmt.Sprintf("%d:%s:%s", idx, dt.Name, dt.Type)
		if seen[key] {
			continue
		}
		seen[key] = true

		items = append(items, ProductCategory{
			Index:      idx,
			Name:       dt.Name,
			ZhName:     zh,
			TypeRegex:  dt.Type,
			IsSoftware: isSoft,
		})
	}

	b, _ := json.MarshalIndent(items, "", "  ")
	_ = os.WriteFile(outPath, b, 0644)
	fmt.Printf("生成 product_items.json 成功: %d 个条目\n", len(items))
}

func convertDomains(inPath, outPath string) {
	data, err := os.ReadFile(inPath)
	if err != nil {
		fmt.Printf("读取 %s 失败: %v\n", inPath, err)
		return
	}

	// 保持原始 JSON 中键值对的出现顺序！
	lines := strings.Split(string(data), "\n")
	reKV := regexp.MustCompile(`"([^"]+)"\s*:\s*"([^"]+)"`)

	var rules []DomainRule
	order := 1

	for _, line := range lines {
		m := reKV.FindStringSubmatch(line)
		if len(m) > 2 {
			k, err1 := strconv.Unquote(`"` + m[1] + `"`)
			v, err2 := strconv.Unquote(`"` + m[2] + `"`)
			if err1 != nil {
				k = m[1]
			}
			if err2 != nil {
				v = m[2]
			}
			rules = append(rules, DomainRule{
				Order:   order,
				Pattern: k,
				Domain:  v,
			})
			order++
		}
	}

	b, _ := json.MarshalIndent(rules, "", "  ")
	_ = os.WriteFile(outPath, b, 0644)
	fmt.Printf("生成 domains.json 成功: %d 条保序规则\n", len(rules))
}

func convertCapabilities(inPath, outPath string) {
	data, err := os.ReadFile(inPath)
	if err != nil {
		fmt.Printf("读取 %s 失败: %v\n", inPath, err)
		return
	}

	var matrix CapabilityMatrix
	if err := json.Unmarshal(data, &matrix); err != nil {
		fmt.Printf("解析 support_devices.json 失败: %v\n", err)
		return
	}

	b, _ := json.MarshalIndent(matrix, "", "  ")
	_ = os.WriteFile(outPath, b, 0644)
	fmt.Printf("生成 capabilities.json 成功: %d 种款型\n", len(matrix.DeviceTypes))
}
