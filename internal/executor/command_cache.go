package executor

import (
	"sync"
)

const (
	// DefaultMaxCacheEntries 默认单设备缓存条目上限
	DefaultMaxCacheEntries = 100

	// MaxCacheEntrySizeBytes 超过 512KB 的单条命令输出不入缓存
	MaxCacheEntrySizeBytes = 512 * 1024

	// MaxTotalCacheSizeBytes 单设备缓存总容量上限 32MB，超出触发 LRU 淘汰
	MaxTotalCacheSizeBytes = 32 * 1024 * 1024
)

// cacheNode LRU 双向链表节点
type cacheNode struct {
	key    string
	result *CommandResult
	size   int64
	prev   *cacheNode
	next   *cacheNode
}

// CommandCache 任务级单设备命令缓存
// 生命周期与单次执行任务绑定，每台设备拥有独立实例，无跨设备并发干扰。
type CommandCache struct {
	mu               sync.RWMutex
	maxEntries       int
	maxEntrySize     int64
	maxTotalSize     int64
	currentTotalSize int64
	items            map[string]*cacheNode
	head             *cacheNode // 最近最多使用 (MRU)
	tail             *cacheNode // 最久未使用 (LRU)
}

// NewCommandCache 创建自定义容量的命令缓存
func NewCommandCache(maxEntries int, maxEntrySize int64, maxTotalSize int64) *CommandCache {
	if maxEntries <= 0 {
		maxEntries = DefaultMaxCacheEntries
	}
	if maxEntrySize <= 0 {
		maxEntrySize = MaxCacheEntrySizeBytes
	}
	if maxTotalSize <= 0 {
		maxTotalSize = MaxTotalCacheSizeBytes
	}
	return &CommandCache{
		maxEntries:   maxEntries,
		maxEntrySize: maxEntrySize,
		maxTotalSize: maxTotalSize,
		items:        make(map[string]*cacheNode),
	}
}

// DefaultCommandCache 创建默认配置的命令缓存
func DefaultCommandCache() *CommandCache {
	return NewCommandCache(DefaultMaxCacheEntries, MaxCacheEntrySizeBytes, MaxTotalCacheSizeBytes)
}

// Get 从缓存获取命令执行结果
func (c *CommandCache) Get(cmd string) (*CommandResult, bool) {
	if c == nil {
		return nil, false
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	node, found := c.items[cmd]
	if !found {
		return nil, false
	}

	// 移至表头 (MRU)
	c.moveToHead(node)
	return node.result, true
}

// Put 将命令结果放入缓存
func (c *CommandCache) Put(cmd string, result *CommandResult) {
	if c == nil || result == nil || cmd == "" {
		return
	}

	// 计算当前条目大小（以 RawSize + NormalizedSize 估算）
	entrySize := result.RawSize + result.NormalizedSize
	if entrySize > c.maxEntrySize {
		// 单条回显超限，禁止入缓存保全内存
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if node, found := c.items[cmd]; found {
		// 更新已有节点
		c.currentTotalSize -= node.size
		node.result = result
		node.size = entrySize
		c.currentTotalSize += entrySize
		c.moveToHead(node)
	} else {
		// 新建节点
		node := &cacheNode{
			key:    cmd,
			result: result,
			size:   entrySize,
		}
		c.items[cmd] = node
		c.addToHead(node)
		c.currentTotalSize += entrySize
	}

	// 触发驱逐：条目超限或总内存超限
	for len(c.items) > c.maxEntries || (c.currentTotalSize > c.maxTotalSize && len(c.items) > 1) {
		c.removeTail()
	}
}

// Len 返回当前缓存条目数
func (c *CommandCache) Len() int {
	if c == nil {
		return 0
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}

// TotalSize 返回当前缓存总字节数
func (c *CommandCache) TotalSize() int64 {
	if c == nil {
		return 0
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.currentTotalSize
}

// Clear 清空缓存
func (c *CommandCache) Clear() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[string]*cacheNode)
	c.head = nil
	c.tail = nil
	c.currentTotalSize = 0
}

func (c *CommandCache) addToHead(node *cacheNode) {
	node.next = c.head
	node.prev = nil
	if c.head != nil {
		c.head.prev = node
	}
	c.head = node
	if c.tail == nil {
		c.tail = node
	}
}

func (c *CommandCache) removeNode(node *cacheNode) {
	if node.prev != nil {
		node.prev.next = node.next
	} else {
		c.head = node.next
	}
	if node.next != nil {
		node.next.prev = node.prev
	} else {
		c.tail = node.prev
	}
}

func (c *CommandCache) moveToHead(node *cacheNode) {
	if c.head == node {
		return
	}
	c.removeNode(node)
	c.addToHead(node)
}

func (c *CommandCache) removeTail() {
	if c.tail == nil {
		return
	}
	lru := c.tail
	delete(c.items, lru.key)
	c.currentTotalSize -= lru.size
	c.removeNode(lru)
}
