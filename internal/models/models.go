// Package models 包含所有数据库模型定义
// 该包独立于业务逻辑，避免循环导入问题
package models

import "time"

// ============================================================================
// 设备资产相关模型
// ============================================================================

// DeviceAsset 设备资产表
type DeviceAsset struct {
	ID          uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	IP          string    `json:"ip" gorm:"uniqueIndex;not null"`
	Port        int       `json:"port"`
	Username    string    `json:"username"`
	Password    string    `json:"-" gorm:"column:password"` // JSON 完全不输出，仅用于数据库存储
	Protocol    string    `json:"protocol"`
	Group       string    `json:"group" gorm:"column:group_name"` // 映射到数据库的 group_name 列
	DisplayName string    `json:"displayName"`
	Vendor      string    `json:"vendor"`
	Role        string    `json:"role"`
	Site        string    `json:"site"`
	Description string    `json:"description"`
	Tags        []string  `json:"tags" gorm:"serializer:json"` // 标签列表
	LastSeen    time.Time `json:"lastSeen"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// DeviceAssetResponse 设备响应结构（包含密码，用于编辑场景）
type DeviceAssetResponse struct {
	DeviceAsset
	Password string `json:"password,omitempty"`
}

// ToResponse 将 DeviceAsset 转换为包含密码的响应结构
func (d *DeviceAsset) ToResponse() *DeviceAssetResponse {
	return &DeviceAssetResponse{
		DeviceAsset: *d,
		Password:    d.Password,
	}
}

// TableName 指定表名
func (DeviceAsset) TableName() string {
	return "device_assets"
}

// ============================================================================
// 全局设置相关模型
// ============================================================================

// SSHAlgorithmSettings SSH算法配置
type SSHAlgorithmSettings struct {
	// 加密算法 (Ciphers)
	Ciphers []string `json:"ciphers"`
	// 密钥交换算法
	KeyExchanges []string `json:"keyExchanges"`
	// 消息认证码
	MACs []string `json:"macs"`
	// 主机密钥算法
	HostKeyAlgorithms []string `json:"hostKeyAlgorithms"`

	// 预设模式: "secure" | "compatible" | "custom"
	PresetMode string `json:"presetMode"`
}

// GlobalSettings 全局运行参数
type GlobalSettings struct {
	ID             uint   `json:"id" gorm:"primaryKey"`
	MaxWorkers     int    `json:"maxWorkers"`     // 并发数 (当前硬编码为 32)
	ConnectTimeout string `json:"connectTimeout"` // SSH/SFTP 连接超时 (如 "10s")
	CommandTimeout string `json:"commandTimeout"` // 单条命令默认超时 (如 "30s")
	StorageRoot    string `json:"storageRoot"`    // 统一数据根目录
	ErrorMode      string `json:"errorMode"`      // "pause" | "skip" | "abort"

	// 调试日志开关
	Debug   bool `json:"debug"`   // 启用 DEBUG 级别日志
	Verbose bool `json:"verbose"` // 启用 VERBOSE 级别日志（包含详细调试信息）

	// SSH算法配置
	SSHAlgorithms SSHAlgorithmSettings `json:"sshAlgorithms" gorm:"type:text;serializer:json"`
	// SSH主机密钥校验策略: strict / accept_new / insecure
	SSHHostKeyPolicy string `json:"sshHostKeyPolicy"`
	// known_hosts 文件路径（为空时使用默认路径）
	SSHKnownHostsPath string `json:"sshKnownHostsPath"`
}

// TableName 指定表名
func (GlobalSettings) TableName() string {
	return "global_settings"
}

// ============================================================================
// 命令组相关模型
// ============================================================================

// CommandGroup 命令组
type CommandGroup struct {
	ID          uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	Name        string    `json:"name" gorm:"uniqueIndex;not null"`
	Description string    `json:"description"`
	Commands    []string  `json:"commands" gorm:"serializer:json"` // 命令列表
	Tags        []string  `json:"tags" gorm:"serializer:json"`     // 标签列表
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// TableName 指定表名
func (CommandGroup) TableName() string {
	return "command_groups"
}

// ============================================================================
// 任务组相关模型
// ============================================================================

// TaskGroup 任务组
type TaskGroup struct {
	ID           uint       `json:"id" gorm:"primaryKey;autoIncrement"`
	Name         string     `json:"name" gorm:"uniqueIndex;not null"`
	Description  string     `json:"description"`
	DeviceGroup  string     `json:"deviceGroup"`
	CommandGroup string     `json:"commandGroup"`
	MaxWorkers   int        `json:"maxWorkers"`
	Timeout      int        `json:"timeout"`
	Mode         string     `json:"mode"`                         // "group" 模式A | "binding" 模式B
	Items        []TaskItem `json:"items" gorm:"serializer:json"` // 任务项列表
	Status       string     `json:"status"`                       // "pending" | "running" | "completed" | "failed"
	Tags         []string   `json:"tags" gorm:"serializer:json"`
	EnableRawLog bool       `json:"enableRawLog"`
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
}

// TableName 指定表名
func (TaskGroup) TableName() string {
	return "task_groups"
}

// ============================================================================
// 运行时配置相关模型
// ============================================================================

// RuntimeSetting 运行时配置表
type RuntimeSetting struct {
	ID        uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	Category  string    `json:"category" gorm:"index"`
	Key       string    `json:"key" gorm:"uniqueIndex;not null"`
	Value     string    `json:"value"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// TableName 指定表名
func (RuntimeSetting) TableName() string {
	return "runtime_settings"
}

// ============================================================================
// 执行记录相关模型
// ============================================================================

// ExecutionRecord 历史执行记录表
type ExecutionRecord struct {
	ID           uint       `json:"id" gorm:"primaryKey;autoIncrement"`
	TaskGroupID  uint       `json:"taskGroupId" gorm:"index"`
	RunnerSource string     `json:"runnerSource" gorm:"index"`
	Status       string     `json:"status" gorm:"index"`
	TotalCount   int        `json:"totalCount"`
	SuccessCount int        `json:"successCount"`
	FailedCount  int        `json:"failedCount"`
	StartedAt    *time.Time `json:"startedAt" gorm:"index"`
	FinishedAt   *time.Time `json:"finishedAt"`
	CreatedAt    time.Time  `json:"createdAt" gorm:"index"`
	UpdatedAt    time.Time  `json:"updatedAt"`
}

// TableName 指定表名
func (ExecutionRecord) TableName() string {
	return "execution_records"
}

// ============================================================================
// 任务组相关辅助类型
// ============================================================================

// TaskItem 单个任务项（一组命令绑定一组设备）
type TaskItem struct {
	CommandGroupID string   `json:"commandGroupId"` // 命令组ID（模式A使用）
	Commands       []string `json:"commands"`       // 直接命令列表（模式B使用）
	DeviceIDs      []uint   `json:"deviceIDs"`      // 绑定的设备ID列表
}
