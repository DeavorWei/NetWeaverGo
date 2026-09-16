package models

import "time"

// RiskTrustEntry 信任清单条目
type RiskTrustEntry struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID    string    `gorm:"index;size:128" json:"userId"`
	Pattern   string    `gorm:"type:text;not null" json:"pattern"` // 命令正则
	ExpiresAt time.Time `json:"expiresAt"`                         // 过期时间
	Reason    string    `gorm:"type:text" json:"reason"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"createdAt"`
}

// TableName 指定表名
func (RiskTrustEntry) TableName() string {
	return "risk_trust_entries"
}

// IsExpired 判断是否已过期
func (e *RiskTrustEntry) IsExpired() bool {
	if e.ExpiresAt.IsZero() {
		return false
	}
	return time.Now().After(e.ExpiresAt)
}
