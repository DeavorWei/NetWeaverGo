package config

import "time"

// ==================== 日志配置 ====================
const (
	// MaxLogsPerDevice 每个设备最大保留日志条数
	MaxLogsPerDevice = 500

	// MaxLogLength 单条日志最大长度（字符）
	MaxLogLength = 2000

	// LogTruncateThreshold 日志截断百分比阈值 (0-100)
	LogTruncateThreshold = 95

	// LogBufferSize 日志缓冲区大小
	LogBufferSize = 100
)

// ==================== 超时配置 ====================
const (
	// DefaultCommandTimeout 默认命令执行超时
	DefaultCommandTimeout = 30 * time.Second

	// ShortCommandTimeout 短命令超时（如简单查询）
	ShortCommandTimeout = 10 * time.Second

	// LongCommandTimeout 长命令超时（如批量操作）
	LongCommandTimeout = 5 * time.Minute

	// ConnectionTimeout SSH连接超时
	ConnectionTimeout = 10 * time.Second

	// HandshakeTimeout SSH握手超时
	HandshakeTimeout = 10 * time.Second
)

// ==================== 引擎配置 ====================
const (
	// DefaultWorkerCount 默认工作协程数
	DefaultWorkerCount = 10

	// DefaultDiscoveryPerDeviceTimeout 默认单设备发现超时
	DefaultDiscoveryPerDeviceTimeout = 3 * time.Minute

	// EventBufferSize 事件缓冲区大小
	EventBufferSize = 1000

	// FallbackEventCapacity 后备事件存储容量
	FallbackEventCapacity = 500

	// MaxConcurrentDevices 最大并发设备数
	MaxConcurrentDevices = 100
)

// ==================== 拓扑配置 ====================
const (
	// DefaultTopologyMaxInferenceCandidates FDB 推断允许的最大候选设备数
	DefaultTopologyMaxInferenceCandidates = 8
)

// ==================== 发现任务配置 ====================
const (
	// DefaultDiscoveryEventBufferSize 发现事件缓冲区大小
	DefaultDiscoveryEventBufferSize = 200

	// DefaultDiscoveryMaxWorkers 发现任务默认并发数
	DefaultDiscoveryMaxWorkers = 32

	// MaxDiscoveryWorkers 发现任务最大并发数
	MaxDiscoveryWorkers = 100
)

// ==================== SSH配置 ====================
const (
	// DefaultSSHPort 默认SSH端口
	DefaultSSHPort = 22

	// MaxSSHSessions 每个设备最大SSH会话数
	MaxSSHSessions = 5

	// SSHKeepAliveInterval 保持连接心跳间隔
	SSHKeepAliveInterval = 30 * time.Second
)

// ==================== 分页检测配置 ====================
const (
	// PaginationLineThreshold 分页检测行数阈值
	PaginationLineThreshold = 50

	// PaginationCheckInterval 分页检测间隔
	PaginationCheckInterval = 100 * time.Millisecond
)

// ==================== 缓冲区配置 ====================
const (
	// DefaultBufferSize 默认缓冲区大小
	DefaultBufferSize = 4096

	// SmallBufferSize 小缓冲区大小
	SmallBufferSize = 1024

	// LargeBufferSize 大缓冲区大小
	LargeBufferSize = 8192
)

// ==================== 脱敏配置 ====================
const (
	// SensitivePatternCount 敏感信息正则表达式数量
	SensitivePatternCount = 3
)
