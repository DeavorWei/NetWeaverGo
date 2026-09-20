package models

import "time"

// BizSnapshot 业务比对设备快照模型
type BizSnapshot struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	RunID     string    `gorm:"index;size:64;not null" json:"runId"`
	DeviceIP  string    `gorm:"index;size:64;not null" json:"deviceIp"`
	Domain    string    `gorm:"size:32;not null" json:"domain"`  // S | NE-SR | CE
	SceneID   string    `gorm:"size:64;not null" json:"sceneId"` // routing | interface | l2vpn | etc.
	Phase     string    `gorm:"size:32;not null" json:"phase"`   // before | after
	DataJSON  string    `gorm:"type:text;not null" json:"data"`  // JSON 格式键值对
	CreatedAt time.Time `gorm:"autoCreateTime" json:"createdAt"`
}

// TableName 指定表名
func (BizSnapshot) TableName() string {
	return "biz_snapshots"
}

// BizCompareTask 业务比对任务记录
type BizCompareTask struct {
	ID          uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	TaskID      string    `gorm:"uniqueIndex;size:64;not null" json:"taskId"`
	Name        string    `gorm:"size:128;not null" json:"name"`
	Domain      string    `gorm:"size:32;not null" json:"domain"`  // S | NE-SR | CE
	SceneID     string    `gorm:"size:64;not null" json:"sceneId"` // 场景ID
	BeforeRunID string    `gorm:"size:64;not null" json:"beforeRunId"`
	AfterRunID  string    `gorm:"size:64;not null" json:"afterRunId"`
	DiffCount   int       `gorm:"default:0" json:"diffCount"`
	Status      string    `gorm:"size:32;default:'completed'" json:"status"`
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"createdAt"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime" json:"updatedAt"`
}

// TableName 指定表名
func (BizCompareTask) TableName() string {
	return "biz_compare_tasks"
}

// BizCompareItem 业务比对差异项
type BizCompareItem struct {
	ID            uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	CompareTaskID uint      `gorm:"index;not null" json:"compareTaskId"`
	DeviceIP      string    `gorm:"index;size:64;not null" json:"deviceIp"`
	Domain        string    `gorm:"size:32;not null" json:"domain"`
	ItemKey       string    `gorm:"size:128;not null" json:"itemKey"`     // 如 GigabitEthernet0/0/1.status
	ItemCategory  string    `gorm:"size:64;not null" json:"itemCategory"` // interface | route | arp | mac
	DiffType      string    `gorm:"size:32;not null" json:"diffType"`     // added | deleted | modified | drift
	BeforeValue   string    `gorm:"type:text" json:"beforeValue"`
	AfterValue    string    `gorm:"type:text" json:"afterValue"`
	ImpactLevel   string    `gorm:"size:32;default:'minor'" json:"impactLevel"` // critical | major | minor | info
	ImpactScope   string    `gorm:"size:256" json:"impactScope"`
	CreatedAt     time.Time `gorm:"autoCreateTime" json:"createdAt"`
}

// TableName 指定表名
func (BizCompareItem) TableName() string {
	return "biz_compare_items"
}
