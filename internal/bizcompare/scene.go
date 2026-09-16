package bizcompare

import (
	"strings"
	"sync"
)

// SceneCommand 场景中的采集命令定义
type SceneCommand struct {
	CommandKey string `json:"commandKey"`
	Command    string `json:"command"`
	Category   string `json:"category"` // interface | route | arp | mac | stp | vxlan | lldp
	IsHeavy    bool   `json:"isHeavy"`  // 是否属于重型命令（大路由表等）
}

// SceneDefinition 业务比对场景定义
type SceneDefinition struct {
	ID          string         `json:"id"`
	Domain      string         `json:"domain"` // S | NE-SR | CE | *
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Commands    []SceneCommand `json:"commands"`
}

// SceneManager 场景管理器
type SceneManager struct {
	mu     sync.RWMutex
	scenes map[string]*SceneDefinition // key: id
}

var (
	globalSceneManager *SceneManager
	sceneOnce          sync.Once
)

// GetGlobalSceneManager 获取全局场景管理器
func GetGlobalSceneManager() *SceneManager {
	sceneOnce.Do(func() {
		globalSceneManager = NewSceneManager()
		globalSceneManager.RegisterBuiltinScenes()
	})
	return globalSceneManager
}

// NewSceneManager 创建场景管理器
func NewSceneManager() *SceneManager {
	return &SceneManager{
		scenes: make(map[string]*SceneDefinition),
	}
}

// Register 注册场景
func (m *SceneManager) Register(scene *SceneDefinition) {
	if scene == nil || scene.ID == "" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.scenes[scene.ID] = scene
}

// Get 根据 ID 获取场景
func (m *SceneManager) Get(id string) (*SceneDefinition, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.scenes[id]
	return s, ok
}

// ListByDomain 按产品域获取可用场景列表
func (m *SceneManager) ListByDomain(domain string) []*SceneDefinition {
	m.mu.RLock()
	defer m.mu.RUnlock()

	normDomain := strings.ToUpper(strings.TrimSpace(domain))
	var results []*SceneDefinition

	for _, s := range m.scenes {
		if normDomain == "" || strings.ToUpper(s.Domain) == normDomain || s.Domain == "*" {
			results = append(results, s)
		}
	}
	return results
}
