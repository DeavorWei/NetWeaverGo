package config

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// DeviceAsset 表示单台交换机的连接凭证信息
type DeviceAsset struct {
	IP       string
	Port     int
	Username string
	Password string
}

const (
	inventoryFile = "inventory.csv"
	configFile    = "config.txt"
)

// ParseOrGenerate 尝试读取当前目录下的配置文件，若不存在则生成模板
func ParseOrGenerate() ([]DeviceAsset, []string, error) {
	var devices []DeviceAsset
	var commands []string
	var missingFiles []string

	if _, err := os.Stat(inventoryFile); os.IsNotExist(err) {
		generateInventoryTemplate()
		missingFiles = append(missingFiles, inventoryFile)
	} else {
		devs, err := readInventory()
		if err != nil {
			return nil, nil, fmt.Errorf("读取资产文件 %s 失败: %v", inventoryFile, err)
		}
		devices = devs
	}

	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		generateConfigTemplate()
		missingFiles = append(missingFiles, configFile)
	} else {
		cmds, err := readCommands()
		if err != nil {
			return nil, nil, fmt.Errorf("读取命令文件 %s 失败: %v", configFile, err)
		}
		commands = cmds
	}

	if len(missingFiles) > 0 {
		return nil, nil, fmt.Errorf("已在当前目录生成模板文件: %s，请填写内容后重新运行程序", strings.Join(missingFiles, ", "))
	}

	return devices, commands, nil
}

func readInventory() ([]DeviceAsset, error) {
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
		if len(row) < 4 {
			return nil, fmt.Errorf("资产文件第 %d 行格式不匹配，必须包含 IP,Port,Username,Password", i+2)
		}
		ip := strings.TrimSpace(row[0])
		portStr := strings.TrimSpace(row[1])
		username := strings.TrimSpace(row[2])
		password := strings.TrimSpace(row[3])

		if ip == "" {
			continue
		}

		port, err := strconv.Atoi(portStr)
		if err != nil || port <= 0 {
			port = 22
		}

		devices = append(devices, DeviceAsset{
			IP:       ip,
			Port:     port,
			Username: username,
			Password: password,
		})
	}
	return devices, nil
}

func readCommands() ([]string, error) {
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

	writer.Write([]string{"IP", "Port", "Username", "Password"})
	writer.Write([]string{"192.168.1.10", "22", "admin", "Admin@123"})
	writer.Write([]string{"192.168.1.11", "22", "root", "Root@456"})
}

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
