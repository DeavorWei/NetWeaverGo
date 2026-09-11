package taskexec

import (
	"sync"
)

// maxRunDataBytes 单次运行的内存数据快照容量上限（64MB，对齐方案 §10.1 预算）
const maxRunDataBytes = 64 * 1024 * 1024

// RunDataHolder 运行期内存数据快照（阶段间传递中间产物）。
//
// 设计要点（方案 §5.3.3）：巡检三阶段**不引入中间解析实体表**，
// 避免"先写临时表 → 再查临时表 → 最后删临时表"带来的 SQLite 写入放大与 WAL 膨胀；
// 原始回显已落盘，内存仅保存解析后结果，超限可安全丢弃并回退读盘。
type RunDataHolder interface {
	SetCommandEcho(deviceIP, commandKey, echo string)
	GetCommandEcho(deviceIP, commandKey string) (string, bool)
	HasDeviceEchoes(deviceIP string) bool
	HasAnyData() bool
	SetParsedRows(deviceIP, commandKey string, rows []map[string]string)
	GetParsedRows(deviceIP, commandKey string) ([]map[string]string, bool)
}

type runDataStore struct {
	mu       sync.RWMutex
	size     int64
	full     bool
	echos    map[string]map[string]string
	parsed   map[string]map[string][]map[string]string
	anyDataSet bool
}

func newRunDataStore() *runDataStore {
	return &runDataStore{
		echos:  make(map[string]map[string]string),
		parsed: make(map[string]map[string][]map[string]string),
	}
}

func (s *runDataStore) SetCommandEcho(deviceIP, commandKey, echo string) {
	if deviceIP == "" || commandKey == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.full {
		return
	}
	if s.size+int64(len(echo)) > maxRunDataBytes {
		s.full = true
		return
	}
	byCmd, ok := s.echos[deviceIP]
	if !ok {
		byCmd = make(map[string]string)
		s.echos[deviceIP] = byCmd
	}
	if _, exists := byCmd[commandKey]; !exists {
		s.size += int64(len(echo))
	}
	byCmd[commandKey] = echo
	s.anyDataSet = true
}

func (s *runDataStore) GetCommandEcho(deviceIP, commandKey string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	byCmd, ok := s.echos[deviceIP]
	if !ok {
		return "", false
	}
	v, ok := byCmd[commandKey]
	return v, ok
}

func (s *runDataStore) HasDeviceEchoes(deviceIP string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.echos[deviceIP]) > 0
}

func (s *runDataStore) HasAnyData() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.anyDataSet
}

func (s *runDataStore) SetParsedRows(deviceIP, commandKey string, rows []map[string]string) {
	if deviceIP == "" || commandKey == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.full {
		return
	}
	byCmd, ok := s.parsed[deviceIP]
	if !ok {
		byCmd = make(map[string][]map[string]string)
		s.parsed[deviceIP] = byCmd
	}
	byCmd[commandKey] = rows
	s.anyDataSet = true
}

func (s *runDataStore) GetParsedRows(deviceIP, commandKey string) ([]map[string]string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	byCmd, ok := s.parsed[deviceIP]
	if !ok {
		return nil, false
	}
	v, ok := byCmd[commandKey]
	return v, ok
}

// runDataRegistry 按 RunID 保存内存快照，运行结束时统一释放
type runDataRegistry struct {
	mu   sync.Mutex
	runs map[string]*runDataStore
}

var runData = &runDataRegistry{runs: make(map[string]*runDataStore)}

// GetRunData 获取（或惰性创建）指定运行的数据快照；runID 为空时返回只读空实现
func GetRunData(runID string) RunDataHolder {
	if runID == "" {
		return newRunDataStore()
	}
	runData.mu.Lock()
	defer runData.mu.Unlock()
	store, ok := runData.runs[runID]
	if !ok {
		store = newRunDataStore()
		runData.runs[runID] = store
	}
	return store
}

// ReleaseRunData 释放指定运行的内存快照
func ReleaseRunData(runID string) {
	if runID == "" {
		return
	}
	runData.mu.Lock()
	defer runData.mu.Unlock()
	delete(runData.runs, runID)
}
