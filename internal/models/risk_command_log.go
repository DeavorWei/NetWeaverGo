package models

import "time"

// RiskCommandLog 高危命令操作留痕
type RiskCommandLog struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	RunID     string    `gorm:"index;size:64" json:"runId"`
	DeviceIP  string    `gorm:"size:64" json:"deviceIp"`
	Command   string    `gorm:"type:text" json:"command"`
	RuleID    uint      `gorm:"index" json:"ruleId"`
	Action    string    `gorm:"size:32" json:"action"` // blocked | confirmed | warned | bypassed
	Operator  string    `gorm:"size:128" json:"operator"`
	Reason    string    `gorm:"type:text" json:"reason"` // bypass 时必填理由
	CreatedAt time.Time `gorm:"autoCreateTime" json:"createdAt"`
}

// TableName 指定表名
func (RiskCommandLog) TableName() string {
	return "risk_command_logs"
}
