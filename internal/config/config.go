package config

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/NetWeaverGo/core/internal/logger"
)

// DeviceAsset 表示单台交换机的连接凭证信息
type DeviceAsset struct {
	IP       string `json:"ip"`
	Port     int    `json:"port"`
	Protocol string `json:"protocol"` // 连接协议：SSH/SNMP/TELNET
	Username string `json:"username"`
	Password string `json:"password"`
	Group    string `json:"group"` // 设备分组
	Tag      string `json:"tag"`   // 设备标签
}

// 协议默认端口映射
var ProtocolDefaultPorts = map[string]int{
	"SSH":     22,
	"SNMP":    161,
	"TELNET":  23,
}

// ValidProtocols 有效协议列表
var ValidProtocols = []string{"SSH", "SNMP", "TELNET"}

const (
	inventoryFile = "inventory.csv"
	configFile    = "config.txt"
)

// ParseOrGenerate 尝试读取当前目录下的配置文件，若不存在则生成模板
func ParseOrGenerate(isBackup bool) ([]DeviceAsset, []string, []string, []string, error) {
	logger.Debug("Config", "-", "开始解析或生成配置文件 (isBackup=%v)", isBackup)
	var devices []DeviceAsset
	var commands []string
	var missingFiles []string

	if _, err := os.Stat(inventoryFile); os.IsNotExist(err) {
		generateInventoryTemplate()
		missingFiles = append(missingFiles, inventoryFile)
	} else {
		devs, err := readInventory()
		if err != nil {
			logger.Debug("Config", "-", "读取资产文件 %s 失败: %v", inventoryFile, err)
			return nil, nil, nil, nil, fmt.Errorf("读取资产文件 %s 失败: %v", inventoryFile, err)
		}
		logger.Debug("Config", "-", "成功读取资产文件，共 %d 台设备", len(devs))
		devices = devs
	}

	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		generateConfigTemplate()
		missingFiles = append(missingFiles, configFile)
	} else {
		cmds, err := readCommands()
		if err != nil {
			logger.Debug("Config", "-", "读取命令文件 %s 失败: %v", configFile, err)
			// 在备份模式下，即使没有命令也不报错
			if !isBackup {
				return nil, nil, nil, nil, fmt.Errorf("读取命令文件 %s 失败: %v", configFile, err)
			}
		} else {
			logger.Debug("Config", "-", "成功读取命令文件，共 %d 条命令", len(cmds))
		}
		commands = cmds
	}

	// if len(missingFiles) > 0 {
	// 	return nil, nil, fmt.Errorf("已在当前目录生成模板文件: %s，请填写内容后重新运行程序", strings.Join(missingFiles, ", "))
	// }

	return devices, commands, nil, missingFiles, nil
}

// readInventory 读取并解析资产清单文件
func readInventory() ([]DeviceAsset, error) {
	logger.DebugAll("Config", "-", "尝试打开文件: %s", inventoryFile)
	file, err := os.Open(inventoryFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	if len(records) <= 1 {
		return nil, fmt.Errorf("资产文件内容为空或只有表头")
	}

	var devices []DeviceAsset

	for i, row := range records[1:] {
		// 兼容新旧格式：
		// 新格式7列(IP,Port,Protocol,Username,Password,Group,Tag)
		// 旧格式5列(IP,Port,Protocol,Username,Password)
		// 最旧格式4列(IP,Port,Username,Password)
		if len(row) < 4 {
			return nil, fmt.Errorf("资产文件第 %d 行格式不匹配", i+2)
		}

		ip := strings.TrimSpace(row[0])
		portStr := strings.TrimSpace(row[1])
		var protocol, username, password, group, tag string

		// 判断是哪种格式
		if len(row) >= 7 {
			// 最新格式：IP,Port,Protocol,Username,Password,Group,Tag
			protocol = strings.ToUpper(strings.TrimSpace(row[2]))
			username = strings.TrimSpace(row[3])
			password = strings.TrimSpace(row[4])
			group = strings.TrimSpace(row[5])
			tag = strings.TrimSpace(row[6])
		} else if len(row) >= 5 {
			// 旧格式：IP,Port,Protocol,Username,Password
			protocol = strings.ToUpper(strings.TrimSpace(row[2]))
			username = strings.TrimSpace(row[3])
			password = strings.TrimSpace(row[4])
			group = ""
			tag = ""
		} else {
			// 最旧格式：IP,Port,Username,Password (兼容)
			protocol = "SSH"
			username = strings.TrimSpace(row[2])
			password = strings.TrimSpace(row[3])
			group = ""
			tag = ""
		}

		if ip == "" {
			continue
		}

		// 校验协议有效性
		if !isValidProtocol(protocol) {
			protocol = "SSH"
		}

		port, err := strconv.Atoi(portStr)
		if err != nil || port <= 0 {
			port = GetDefaultPort(protocol)
		}

		devices = append(devices, DeviceAsset{
			IP:       ip,
			Port:     port,
			Protocol: protocol,
			Username: username,
			Password: password,
			Group:    group,
			Tag:      tag,
		})
	}
	return devices, nil
}

// readCommands 读取并解析命令列表文件
func readCommands() ([]string, error) {
	logger.DebugAll("Config", "-", "尝试打开文件: %s", configFile)
	content, err := os.ReadFile(configFile)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(content), "\n")
	var commands []string
	for _, line := range lines {
		cmd := strings.TrimRight(line, "\r\n")
		// 忽略空行和注释
		if strings.TrimSpace(cmd) == "" || strings.HasPrefix(strings.TrimSpace(cmd), "#") {
			continue
		}
		commands = append(commands, cmd)
	}

	if len(commands) == 0 {
		return nil, fmt.Errorf("命令文件为空")
	}
	return commands, nil
}

// generateInventoryTemplate 生成默认的资产清单模板
func generateInventoryTemplate() {
	cwd, _ := os.Getwd()
	path := filepath.Join(cwd, inventoryFile)
	file, err := os.Create(path)
	if err != nil {
		fmt.Printf("无法创建资产模板文件: %v\n", err)
		return
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	writer.Write([]string{"IP", "Port", "Protocol", "Username", "Password", "Group", "Tag"})
	writer.Write([]string{"192.168.1.10", "22", "SSH", "admin", "Admin@123", "核心交换机", "生产环境"})
	writer.Write([]string{"192.168.1.11", "161", "SNMP", "public", "public", "接入交换机", "测试环境"})
}

// generateConfigTemplate 生成默认的命令列表模板
func generateConfigTemplate() {
	cwd, _ := os.Getwd()
	path := filepath.Join(cwd, configFile)
	content := []byte(`# 在此输入需要批量下发的交换机命令，每行一条
# 空行和以 # 开头的行将被忽略

system-view
display interface brief
quit
`)
	err := os.WriteFile(path, content, 0666)
	if err != nil {
		fmt.Printf("无法创建命令模板文件: %v\n", err)
	}
}

// isValidProtocol 检查协议是否有效
func isValidProtocol(protocol string) bool {
	for _, p := range ValidProtocols {
		if p == protocol {
			return true
		}
	}
	return false
}

// GetDefaultPort 根据协议获取默认端口
func GetDefaultPort(protocol string) int {
	if port, ok := ProtocolDefaultPorts[strings.ToUpper(protocol)]; ok {
		return port
	}
	return 22 // 默认 SSH 端口
}

// ValidateDevice 校验设备信息合法性
func ValidateDevice(device DeviceAsset) error {
	if device.IP == "" {
		return fmt.Errorf("IP 地址不能为空")
	}

	// 简单的 IP 格式校验
	parts := strings.Split(device.IP, ".")
	if len(parts) != 4 {
		return fmt.Errorf("IP 地址格式不正确")
	}
	for _, part := range parts {
		num, err := strconv.Atoi(part)
		if err != nil || num < 0 || num > 255 {
			return fmt.Errorf("IP 地址格式不正确")
		}
	}

	if device.Port <= 0 || device.Port > 65535 {
		return fmt.Errorf("端口号必须在 1-65535 之间")
	}

	if !isValidProtocol(device.Protocol) {
		return fmt.Errorf("无效的协议类型: %s", device.Protocol)
	}

	// 用户名和密码可选，不再强制要求

	return nil
}

// SaveCommands 保存命令列表到文件
func SaveCommands(commands []string) error {
	cwd, _ := os.Getwd()
	path := filepath.Join(cwd, configFile)

	var content strings.Builder
	for _, cmd := range commands {
		content.WriteString(cmd + "\n")
	}

	err := os.WriteFile(path, []byte(content.String()), 0666)
	if err != nil {
		return fmt.Errorf("无法保存命令文件: %v", err)
	}

	logger.Info("Config", "-", "成功保存 %d 条命令到 %s", len(commands), configFile)
	return nil
}

// SaveInventory 保存设备列表到 CSV 文件
func SaveInventory(devices []DeviceAsset) error {
	cwd, _ := os.Getwd()
	path := filepath.Join(cwd, inventoryFile)

	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("无法创建文件: %v", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// 写入表头
	if err := writer.Write([]string{"IP", "Port", "Protocol", "Username", "Password", "Group", "Tag"}); err != nil {
		return fmt.Errorf("写入表头失败: %v", err)
	}

	// 写入数据行
	for _, device := range devices {
		row := []string{
			device.IP,
			strconv.Itoa(device.Port),
			device.Protocol,
			device.Username,
			device.Password,
			device.Group,
			device.Tag,
		}
		if err := writer.Write(row); err != nil {
			return fmt.Errorf("写入数据失败: %v", err)
		}
	}

	logger.Info("Config", "-", "成功保存 %d 台设备到 %s", len(devices), inventoryFile)
	return nil
}
