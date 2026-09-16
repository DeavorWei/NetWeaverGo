package connutil

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
)

//go:embed connect_commands.json
var embeddedConnectCommandsJSON []byte

// JumpCommandTemplate 跳板连接命令模板
type JumpCommandTemplate struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Protocol        string `json:"protocol"`
	CommandTemplate string `json:"commandTemplate"`
	PasswordPrompt  string `json:"passwordPrompt,omitempty"`
	ConfirmPrompt   string `json:"confirmPrompt,omitempty"`
	Description     string `json:"description,omitempty"`
}

// JumpHostConfig 跳板机与目标设备连接参数配置
type JumpHostConfig struct {
	// 跳板机自身信息
	JumpHostIP   string `json:"jumpHostIp"`
	JumpHostPort int    `json:"jumpHostPort"`
	JumpUsername string `json:"jumpUsername"`
	JumpPassword string `json:"jumpPassword"`
	JumpProtocol string `json:"jumpProtocol"` // ssh | telnet

	// 目标设备信息
	TargetIP       string `json:"targetIp"`
	TargetPort     int    `json:"targetPort"`
	TargetUsername string `json:"targetUsername"`
	TargetPassword string `json:"targetPassword"`
	TargetProtocol string `json:"targetProtocol"` // ssh | telnet

	// 选用的跳转模板标识（为空时根据 TargetProtocol 自动选用标准模板）
	TemplateID string `json:"templateId,omitempty"`
	// 可选代理中间变量
	ProxyHost string `json:"proxyHost,omitempty"`
	ProxyPort int    `json:"proxyPort,omitempty"`
}

// JumpConnector 跳板连接器管理
type JumpConnector struct {
	mu        sync.RWMutex
	templates map[string]JumpCommandTemplate
}

var (
	defaultJumpConnector     *JumpConnector
	defaultJumpConnectorOnce sync.Once
)

// GetDefaultJumpConnector 获取默认单例跳板连接器
func GetDefaultJumpConnector() *JumpConnector {
	defaultJumpConnectorOnce.Do(func() {
		defaultJumpConnector = NewJumpConnector()
		_ = defaultJumpConnector.LoadEmbeddedTemplates()
	})
	return defaultJumpConnector
}

// NewJumpConnector 创建跳板连接器
func NewJumpConnector() *JumpConnector {
	return &JumpConnector{
		templates: make(map[string]JumpCommandTemplate),
	}
}

// LoadEmbeddedTemplates 加载内置的 13 条连接命令模板
func (jc *JumpConnector) LoadEmbeddedTemplates() error {
	var list []JumpCommandTemplate
	if err := json.Unmarshal(embeddedConnectCommandsJSON, &list); err != nil {
		return fmt.Errorf("解析内置跳板命令模板失败: %w", err)
	}

	jc.mu.Lock()
	defer jc.mu.Unlock()
	for _, tpl := range list {
		jc.templates[tpl.ID] = tpl
	}
	return nil
}

// ListTemplates 获取所有已加载的模板列表
func (jc *JumpConnector) ListTemplates() []JumpCommandTemplate {
	jc.mu.RLock()
	defer jc.mu.RUnlock()

	res := make([]JumpCommandTemplate, 0, len(jc.templates))
	for _, t := range jc.templates {
		res = append(res, t)
	}
	return res
}

// FindTemplate 按 ID 查询模板
func (jc *JumpConnector) FindTemplate(id string) (*JumpCommandTemplate, error) {
	jc.mu.RLock()
	defer jc.mu.RUnlock()

	if t, ok := jc.templates[id]; ok {
		return &t, nil
	}
	return nil, fmt.Errorf("未找到跳板命令模板: %s", id)
}

// BuildJumpCommand 根据配置与模板渲染跳转命令
func (jc *JumpConnector) BuildJumpCommand(cfg JumpHostConfig) (string, error) {
	tplID := cfg.TemplateID
	if tplID == "" {
		if strings.EqualFold(cfg.TargetProtocol, ProtocolTelnet) {
			tplID = "telnet_standard"
		} else {
			tplID = "ssh_standard"
		}
	}

	tpl, err := jc.FindTemplate(tplID)
	if err != nil {
		return "", err
	}

	port := cfg.TargetPort
	if port <= 0 {
		if strings.EqualFold(cfg.TargetProtocol, ProtocolTelnet) {
			port = DefaultTelnetPort
		} else {
			port = DefaultSSHPort
		}
	}

	cmd := tpl.CommandTemplate
	cmd = strings.ReplaceAll(cmd, "{ip}", cfg.TargetIP)
	cmd = strings.ReplaceAll(cmd, "{port}", strconv.Itoa(port))
	cmd = strings.ReplaceAll(cmd, "{username}", cfg.TargetUsername)
	cmd = strings.ReplaceAll(cmd, "{proxy_host}", cfg.ProxyHost)
	cmd = strings.ReplaceAll(cmd, "{proxy_port}", strconv.Itoa(cfg.ProxyPort))

	return cmd, nil
}
