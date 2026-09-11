package models

import (
	"time"
)

// TaskCEASNode CEAS 硬件清单树节点
type TaskCEASNode struct {
	ID           uint      `json:"id" gorm:"primaryKey"`
	TaskRunID    string    `json:"taskRunId" gorm:"column:task_run_id;index;size:64;not null"`
	DeviceIP     string    `json:"deviceIp" gorm:"column:device_ip;index;size:64;not null"`
	NodeID       string    `json:"nodeId" gorm:"column:node_id;size:128;not null"`       // 层级ID (如 0_1_2)
	ParentID     string    `json:"parentId" gorm:"column:parent_id;size:128"`            // 父节点ID
	Level        int       `json:"level" gorm:"column:level;not null;default:1"`         // 树层级 (1: 框, 2: 槽位, 3: 子卡/端口)
	Type         string    `json:"type" gorm:"column:type;size:64;not null"`             // 节点类型 (frame, slot, card, port, power, fanframe)
	Name         string    `json:"name" gorm:"column:name;size:128;not null"`            // 节点名称 (如 Slot_1, BackPlane_1)
	Path         string    `json:"path" gorm:"column:path;size:128"`                     // 三段式端口/物理位置 (如 1/0, 1/2, 1/2/3)
	Slot         string    `json:"slot" gorm:"column:slot;size:64"`                      // 继承或所属主槽位号
	Item         string    `json:"item" gorm:"column:item;size:64;index"`                // BOM 编码 (如 02311ABC)
	BarCode      string    `json:"barCode" gorm:"column:barcode;size:128"`               // 条形码 / 电子标签序列号
	Description  string    `json:"description" gorm:"column:description;size:256"`       // 物料描述
	Manufactured string    `json:"manufactured" gorm:"column:manufactured;size:64"`      // 生产日期
	VendorName   string    `json:"vendorName" gorm:"column:vendor_name;size:64"`         // 厂商名称
	AttrsJSON    string    `json:"attrsJson" gorm:"column:attrs_json;type:text"`         // 原始提取的所有扩展属性键值对 JSON
	CreatedAt    time.Time `json:"createdAt" gorm:"column:created_at"`
}

func (TaskCEASNode) TableName() string {
	return "task_ceas_nodes"
}

// BOMWatchlistItem BOM 观察清单 / 批次预警条目
type BOMWatchlistItem struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	Item        string    `json:"item" gorm:"column:item;size:64;uniqueIndex;not null"` // BOM 编码 (如 02311ABC)
	Category    string    `json:"category" gorm:"column:category;size:64"`              // 产品线/类别 (ar_items, s_items, ce_items, ne_items, user_custom)
	Model       string    `json:"model" gorm:"column:model;size:128"`                   // 对应款型/部件名称
	Description string    `json:"description" gorm:"column:description;size:256"`       // 预警说明/批次原因
	Severity    string    `json:"severity" gorm:"column:severity;size:32;default:'warning'"` // 风险级别: warning, danger, critical
	Enabled     bool      `json:"enabled" gorm:"column:enabled;default:true"`           // 是否启用
	BatchNo     string    `json:"batchNo" gorm:"column:batch_no;size:64"`               // 批次编号/预警通知单号
	CreatedAt   time.Time `json:"createdAt" gorm:"column:created_at"`
	UpdatedAt   time.Time `json:"updatedAt" gorm:"column:updated_at"`
}

func (BOMWatchlistItem) TableName() string {
	return "bom_watchlist"
}
