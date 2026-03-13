package engine

import (
	"sync"

	"github.com/NetWeaverGo/core/internal/report"
)

// RingBuffer 环形缓冲区 - O(1) 插入和淘汰
type RingBuffer struct {
	mu       sync.Mutex
	buffer   []report.ExecutorEvent
	capacity int
	head     int // 读取位置
	tail     int // 写入位置
	count    int // 当前元素数量
}

// NewRingBuffer 创建环形缓冲区
func NewRingBuffer(capacity int) *RingBuffer {
	if capacity <= 0 {
		capacity = 100 // 默认容量，防止零或负数
	}
	return &RingBuffer{
		buffer:   make([]report.ExecutorEvent, capacity),
		capacity: capacity,
	}
}

// Push 添加元素，满时自动覆盖最旧元素（O(1)）
func (r *RingBuffer) Push(ev report.ExecutorEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.buffer[r.tail] = ev
	r.tail = (r.tail + 1) % r.capacity

	if r.count < r.capacity {
		r.count++
	} else {
		// 缓冲区已满，覆盖旧数据
		r.head = (r.head + 1) % r.capacity
	}
}

// GetAll 获取所有元素（从旧到新）
func (r *RingBuffer) GetAll() []report.ExecutorEvent {
	r.mu.Lock()
	defer r.mu.Unlock()

	result := make([]report.ExecutorEvent, 0, r.count)
	for i := 0; i < r.count; i++ {
		idx := (r.head + i) % r.capacity
		result = append(result, r.buffer[idx])
	}
	return result
}

// Clear 清空缓冲区
func (r *RingBuffer) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.head = 0
	r.tail = 0
	r.count = 0
}
