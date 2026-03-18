// Package models 包含所有数据库模型定义
package models

import "time"

// ============================================================================
// 规划比对相关模型
// ============================================================================

// PlannedLink 规划链路
type PlannedLink struct {
	ID          uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	PlanFileID  string    `json:"planFileId" gorm:"index;not null"`
	ADeviceName string    `json:"aDeviceName" gorm:"index"`
	AIf         string    `json:"aIf"`
	BDeviceName string    `json:"bDeviceName" gorm:"index"`
	BIf         string    `json:"bIf"`
	LinkType    string    `json:"linkType"` // physical / aggregate
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// TableName 指定表名
func (PlannedLink) TableName() string {
	return "planned_links"
}

// PlanFile 规划文件
type PlanFile struct {
	ID         uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	FileName   string    `json:"fileName"`
	FilePath   string    `json:"filePath"`
	TotalLinks int       `json:"totalLinks"`
	ImportedAt time.Time `json:"importedAt"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

// TableName 指定表名
func (PlanFile) TableName() string {
	return "plan_files"
}

// DiffReport 差异报告
type DiffReport struct {
	ID                string    `json:"id" gorm:"primaryKey"`
	TaskID            string    `json:"taskId" gorm:"index;not null"`
	PlanFileID        string    `json:"planFileId" gorm:"index;not null"`
	TotalPlanned      int       `json:"totalPlanned"`
	TotalActual       int       `json:"totalActual"`
	Matched           int       `json:"matched"`
	MissingLinks      int       `json:"missingLinks"`
	UnexpectedLinks   int       `json:"unexpectedLinks"`
	InconsistentItems int       `json:"inconsistentItems"`
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
}

// TableName 指定表名
func (DiffReport) TableName() string {
	return "diff_reports"
}

// DiffItem 差异项
type DiffItem struct {
	ID          uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	ReportID    string    `json:"reportId" gorm:"index;not null"`
	DiffType    string    `json:"diffType"` // missing / unexpected / inconsistent
	ADeviceName string    `json:"aDeviceName"`
	AIf         string    `json:"aIf"`
	BDeviceName string    `json:"bDeviceName"`
	BIf         string    `json:"bIf"`
	ExpectedIf  string    `json:"expectedIf"`
	ActualIf    string    `json:"actualIf"`
	Reason      string    `json:"reason"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// TableName 指定表名
func (DiffItem) TableName() string {
	return "diff_items"
}

// ============================================================================
// 规划比对视图模型
// ============================================================================

// PlanImportResult 规划导入结果
type PlanImportResult struct {
	PlanFileID string   `json:"planFileId"`
	TotalLinks int      `json:"totalLinks"`
	Warnings   []string `json:"warnings,omitempty"`
}

// CompareResult 比对结果
type CompareResult struct {
	ReportID          string     `json:"reportId"`
	TotalPlanned      int        `json:"totalPlanned"`
	TotalActual       int        `json:"totalActual"`
	Matched           int        `json:"matched"`
	MissingLinks      []DiffItem `json:"missingLinks"`
	UnexpectedLinks   []DiffItem `json:"unexpectedLinks"`
	InconsistentItems []DiffItem `json:"inconsistentItems"`
}

// DiffReportView 差异报告视图
type DiffReportView struct {
	ID                string    `json:"id"`
	TaskID            string    `json:"taskId"`
	PlanFileID        string    `json:"planFileId"`
	PlanFileName      string    `json:"planFileName"`
	TotalPlanned      int       `json:"totalPlanned"`
	TotalActual       int       `json:"totalActual"`
	Matched           int       `json:"matched"`
	MissingLinks      int       `json:"missingLinks"`
	UnexpectedLinks   int       `json:"unexpectedLinks"`
	InconsistentItems int       `json:"inconsistentItems"`
	CreatedAt         time.Time `json:"createdAt"`
}
