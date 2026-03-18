// Package models 包含所有数据库模型定义
package models

import "time"

// ============================================================================
// 发现任务相关模型
// ============================================================================

// DiscoveryTask 发现任务主表
type DiscoveryTask struct {
	ID           string     `json:"id" gorm:"primaryKey"`
	Name         string     `json:"name"`
	Status       string     `json:"status"` // pending / running / completed / failed / cancelled
	TotalCount   int        `json:"totalCount"`
	SuccessCount int        `json:"successCount"`
	FailedCount  int        `json:"failedCount"`
	StartedAt    *time.Time `json:"startedAt"`
	FinishedAt   *time.Time `json:"finishedAt"`
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
	MaxWorkers   int        `json:"maxWorkers"`
	TimeoutSec   int        `json:"timeoutSec"`
	Vendor       string     `json:"vendor"`
}

// TableName 指定表名
func (DiscoveryTask) TableName() string {
	return "discovery_tasks"
}

// DiscoveryDevice 发现设备结果表
type DiscoveryDevice struct {
	ID           uint       `json:"id" gorm:"primaryKey;autoIncrement"`
	TaskID       string     `json:"taskId" gorm:"index;not null"`
	DeviceIP     string     `json:"deviceIp" gorm:"index;not null"`
	DeviceID     uint       `json:"deviceId"`
	Status       string     `json:"status"` // pending / running / success / failed
	ErrorMessage string     `json:"errorMessage"`
	StartedAt    *time.Time `json:"startedAt"`
	FinishedAt   *time.Time `json:"finishedAt"`
	Vendor       string     `json:"vendor"`
	Model        string     `json:"model"`
	SerialNumber string     `json:"serialNumber"`
	Version      string     `json:"version"`
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
}

// TableName 指定表名
func (DiscoveryDevice) TableName() string {
	return "discovery_devices"
}

// RawCommandOutput 原始命令输出索引表
type RawCommandOutput struct {
	ID           uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	TaskID       string    `json:"taskId" gorm:"index;not null"`
	DeviceIP     string    `json:"deviceIp" gorm:"index;not null"`
	CommandKey   string    `json:"commandKey" gorm:"index;not null"` // version, lldp_neighbor, interface 等
	Command      string    `json:"command"`
	FilePath     string    `json:"filePath"`
	Status       string    `json:"status"` // pending / success / failed
	ErrorMessage string    `json:"errorMessage"`
	OutputSize   int64     `json:"outputSize"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// TableName 指定表名
func (RawCommandOutput) TableName() string {
	return "raw_command_outputs"
}

// ============================================================================
// 发现任务视图模型
// ============================================================================

// StartDiscoveryRequest 启动发现任务请求
type StartDiscoveryRequest struct {
	DeviceIDs  []string `json:"deviceIds"`
	GroupNames []string `json:"groupNames"`
	Vendor     string   `json:"vendor"`
	MaxWorkers int      `json:"maxWorkers"`
	TimeoutSec int      `json:"timeoutSec"`
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

// DeviceInfo 设备信息（用于任务执行）
type DeviceInfo struct {
	ID       uint
	IP       string
	Port     int
	Username string
	Password string
	Vendor   string
}
