package bizcompare

import (
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/NetWeaverGo/core/internal/config"
	"github.com/NetWeaverGo/core/internal/models"
)

// SnapshotItem 单项快照数据
type SnapshotItem struct {
	Key      string `json:"key"`
	Category string `json:"category"`
	Value    string `json:"value"`
}

// 快照采集状态（P1-3：设备级失败标记，diff 时跳过，避免"失败 vs 成功"制造假差异）
const (
	SnapshotStatusOK     = "ok"
	SnapshotStatusFailed = "failed"
)

// DeviceSnapshot 设备业务快照
type DeviceSnapshot struct {
	RunID     string                  `json:"runId"`
	DeviceIP  string                  `json:"deviceIp"`
	Domain    string                  `json:"domain"`
	SceneID   string                  `json:"sceneId"`
	Phase     string                  `json:"phase"`  // before | after
	Status    string                  `json:"status"` // ok | failed
	Error     string                  `json:"error,omitempty"`
	Items     map[string]SnapshotItem `json:"items"` // key -> Item
	CreatedAt time.Time               `json:"createdAt"`
}

// NewDeviceSnapshot 创建新设备快照
func NewDeviceSnapshot(runID, deviceIP, domain, sceneID, phase string) *DeviceSnapshot {
	return &DeviceSnapshot{
		RunID:     runID,
		DeviceIP:  deviceIP,
		Domain:    domain,
		SceneID:   sceneID,
		Phase:     phase,
		Status:    SnapshotStatusOK,
		Items:     make(map[string]SnapshotItem),
		CreatedAt: time.Now(),
	}
}

// MarkFailed 标记该设备快照采集失败（携带可读原因）
func (s *DeviceSnapshot) MarkFailed(err error) {
	if s == nil {
		return
	}
	s.Status = SnapshotStatusFailed
	if err != nil {
		s.Error = err.Error()
	}
}

// Failed 返回该快照是否为采集失败状态
func (s *DeviceSnapshot) Failed() bool {
	return s != nil && s.Status == SnapshotStatusFailed
}

// AddItem 添加快照项
func (s *DeviceSnapshot) AddItem(key, category, value string) {
	if s.Items == nil {
		s.Items = make(map[string]SnapshotItem)
	}
	s.Items[key] = SnapshotItem{
		Key:      key,
		Category: category,
		Value:    value,
	}
}

// SnapshotStore 快照持久化与检索接口
type SnapshotStore interface {
	SaveSnapshot(snap *DeviceSnapshot) error
	GetSnapshot(runID, deviceIP string) (*DeviceSnapshot, error)
	ListSnapshots(runID string) ([]*DeviceSnapshot, error)
}

// DefaultSnapshotStore 默认快照存储实现（支持内存与 DB）
type DefaultSnapshotStore struct {
	mu     sync.RWMutex
	memory map[string]*DeviceSnapshot // key: runID + "|" + deviceIP
}

var (
	globalSnapshotStore *DefaultSnapshotStore
	storeOnce           sync.Once
)

// GetGlobalSnapshotStore 获取全局快照存储单例
func GetGlobalSnapshotStore() *DefaultSnapshotStore {
	storeOnce.Do(func() {
		globalSnapshotStore = &DefaultSnapshotStore{
			memory: make(map[string]*DeviceSnapshot),
		}
	})
	return globalSnapshotStore
}

func snapshotKey(runID, deviceIP string) string {
	return runID + "|" + deviceIP
}

// SaveSnapshot 保存快照
func (s *DefaultSnapshotStore) SaveSnapshot(snap *DeviceSnapshot) error {
	if snap == nil {
		return errors.New("快照不能为空")
	}

	s.mu.Lock()
	s.memory[snapshotKey(snap.RunID, snap.DeviceIP)] = snap
	s.mu.Unlock()

	db := config.GetDB()
	if db != nil {
		dataBytes, err := json.Marshal(snap.Items)
		if err == nil {
			record := models.BizSnapshot{
				RunID:     snap.RunID,
				DeviceIP:  snap.DeviceIP,
				Domain:    snap.Domain,
				SceneID:   snap.SceneID,
				Phase:     snap.Phase,
				Status:    snap.Status,
				Error:     snap.Error,
				DataJSON:  string(dataBytes),
				CreatedAt: snap.CreatedAt,
			}
			_ = db.Create(&record).Error
		}
	}

	return nil
}

// GetSnapshot 获取指定设备和运行的快照
func (s *DefaultSnapshotStore) GetSnapshot(runID, deviceIP string) (*DeviceSnapshot, error) {
	s.mu.RLock()
	snap, ok := s.memory[snapshotKey(runID, deviceIP)]
	s.mu.RUnlock()
	if ok {
		return snap, nil
	}

	db := config.GetDB()
	if db != nil {
		var record models.BizSnapshot
		if err := db.Where("run_id = ? AND device_ip = ?", runID, deviceIP).First(&record).Error; err == nil {
			snap := NewDeviceSnapshot(record.RunID, record.DeviceIP, record.Domain, record.SceneID, record.Phase)
			snap.CreatedAt = record.CreatedAt
			if record.Status != "" {
				snap.Status = record.Status
			}
			snap.Error = record.Error
			var items map[string]SnapshotItem
			if err := json.Unmarshal([]byte(record.DataJSON), &items); err == nil {
				snap.Items = items
			}
			s.mu.Lock()
			s.memory[snapshotKey(runID, deviceIP)] = snap
			s.mu.Unlock()
			return snap, nil
		}
	}

	return nil, errors.New("快照不存在")
}

// ListSnapshots 获取指定 runID 的全部设备快照
func (s *DefaultSnapshotStore) ListSnapshots(runID string) ([]*DeviceSnapshot, error) {
	var results []*DeviceSnapshot
	s.mu.RLock()
	for _, snap := range s.memory {
		if snap.RunID == runID {
			results = append(results, snap)
		}
	}
	s.mu.RUnlock()

	if len(results) > 0 {
		return results, nil
	}

	db := config.GetDB()
	if db != nil {
		var records []models.BizSnapshot
		if err := db.Where("run_id = ?", runID).Find(&records).Error; err == nil {
			for _, record := range records {
				snap := NewDeviceSnapshot(record.RunID, record.DeviceIP, record.Domain, record.SceneID, record.Phase)
				snap.CreatedAt = record.CreatedAt
				if record.Status != "" {
					snap.Status = record.Status
				}
				snap.Error = record.Error
				var items map[string]SnapshotItem
				if err := json.Unmarshal([]byte(record.DataJSON), &items); err == nil {
					snap.Items = items
				}
				results = append(results, snap)
			}
		}
	}

	return results, nil
}
