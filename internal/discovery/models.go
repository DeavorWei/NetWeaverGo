package discovery

import (
	"time"
)

// DiscoveryTask 发现任务主表
type DiscoveryTask struct {
	ID           string     `json:"id" gorm:"primaryKey"`
	Name         string     `json:"name"`         // 任务名称
	Status       string     `json:"status"`       // pending / running / completed / failed / cancelled
	TotalCount   int        `json:"totalCount"`   // 设备总数
	SuccessCount int        `json:"successCount"` // 成功数量
	FailedCount  int        `json:"failedCount"`  // 失败数量
	StartedAt    *time.Time `json:"startedAt"`    // 开始时间
	FinishedAt   *time.Time `json:"finishedAt"`   // 结束时间
	CreatedAt    time.Time  `json:"createdAt"`    // 创建时间
	UpdatedAt    time.Time  `json:"updatedAt"`    // 更新时间
	MaxWorkers   int        `json:"maxWorkers"`   // 并发数
	TimeoutSec   int        `json:"timeoutSec"`   // 超时秒数
	Vendor       string     `json:"vendor"`       // 目标厂商（空表示全部）
}

// DiscoveryDevice 发现设备结果表
type DiscoveryDevice struct {
	ID           uint       `json:"id" gorm:"primaryKey;autoIncrement"`
	TaskID       string     `json:"taskId" gorm:"index;not null"`
	DeviceIP     string     `json:"deviceIp" gorm:"index;not null"`
	DeviceID     uint       `json:"deviceId"`     // 关联的设备资产ID
	Status       string     `json:"status"`       // pending / running / success / failed
	ErrorMessage string     `json:"errorMessage"` // 错误信息
	StartedAt    *time.Time `json:"startedAt"`    // 开始时间
	FinishedAt   *time.Time `json:"finishedAt"`   // 结束时间
	Vendor       string     `json:"vendor"`       // 识别到的厂商
	Model        string     `json:"model"`        // 设备型号
	SerialNumber string     `json:"serialNumber"` // 序列号
	Version      string     `json:"version"`      // 版本信息
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
}

// RawCommandOutput 原始命令输出索引表
type RawCommandOutput struct {
	ID           uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	TaskID       string    `json:"taskId" gorm:"index;not null"`
	DeviceIP     string    `json:"deviceIp" gorm:"index;not null"`
	CommandKey   string    `json:"commandKey" gorm:"index;not null"` // version, lldp_neighbor, interface 等
	Command      string    `json:"command"`                          // 实际执行的命令
	FilePath     string    `json:"filePath"`                         // 文件存储路径
	Status       string    `json:"status"`                           // pending / success / failed
	ErrorMessage string    `json:"errorMessage"`                     // 错误信息
	OutputSize   int64     `json:"outputSize"`                       // 输出大小（字节）
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// StartDiscoveryRequest 启动发现任务请求
type StartDiscoveryRequest struct {
	DeviceIDs  []string `json:"deviceIds"`  // 设备ID列表
	GroupNames []string `json:"groupNames"` // 设备组名列表
	Vendor     string   `json:"vendor"`     // 目标厂商过滤
	MaxWorkers int      `json:"maxWorkers"` // 并发数
	TimeoutSec int      `json:"timeoutSec"` // 超时秒数
}

// TaskStartResponse 任务启动响应
type TaskStartResponse struct {
	TaskID string `json:"taskId"`
}

// DiscoveryTaskView 发现任务视图
type DiscoveryTaskView struct {
	ID           string     `json:"id"`
	Name         string     `json:"name"`
	Status       string     `json:"status"`
	TotalCount   int        `json:"totalCount"`
	SuccessCount int        `json:"successCount"`
	FailedCount  int        `json:"failedCount"`
	StartedAt    *time.Time `json:"startedAt"`
	FinishedAt   *time.Time `json:"finishedAt"`
	CreatedAt    time.Time  `json:"createdAt"`
	MaxWorkers   int        `json:"maxWorkers"`
	Vendor       string     `json:"vendor"`
}

// DiscoveryDeviceView 发现设备视图
type DiscoveryDeviceView struct {
	ID           uint       `json:"id"`
	TaskID       string     `json:"taskId"`
	DeviceIP     string     `json:"deviceIp"`
	Status       string     `json:"status"`
	ErrorMessage string     `json:"errorMessage"`
	StartedAt    *time.Time `json:"startedAt"`
	FinishedAt   *time.Time `json:"finishedAt"`
	Vendor       string     `json:"vendor"`
	Model        string     `json:"model"`
	SerialNumber string     `json:"serialNumber"`
	Version      string     `json:"version"`
}
