package models

import (
	"time"

	"gorm.io/gorm"
)

// RiskCommandAction 风险命令拦截动作类型
type RiskCommandAction string

const (
	// RiskActionBlock 阻断执行，记录高危拦截日志
	RiskActionBlock RiskCommandAction = "block"

	// RiskActionConfirm 触发挂起，请求人工审批确认
	RiskActionConfirm RiskCommandAction = "confirm"

	// RiskActionWarn 仅记录高危警告日志，放行执行
	RiskActionWarn RiskCommandAction = "warn"
)

// RiskCommand 风险命令规则模型
type RiskCommand struct {
	ID        uint              `gorm:"primaryKey;autoIncrement" json:"id"`
	Vendor    string            `gorm:"index;size:64;not null" json:"vendor"`    // huawei / h3c / cisco / *
	Category  string            `gorm:"index;size:64" json:"category"`           // switch / router / firewall / *
	Pattern   string            `gorm:"type:text;not null" json:"pattern"`       // 匹配命令的正则表达式
	Action    RiskCommandAction `gorm:"type:varchar(32);not null" json:"action"` // block / confirm / warn
	Reason    string            `gorm:"type:text" json:"reason"`                 // 风险描述或拦截理由
	Enabled   bool              `gorm:"default:true" json:"enabled"`             // 是否启用
	Builtin   bool              `gorm:"default:false" json:"builtin"`            // 是否内置规则
	CreatedAt time.Time         `json:"createdAt"`
	UpdatedAt time.Time         `json:"updatedAt"`
}

// TableName 指定表名
func (RiskCommand) TableName() string {
	return "risk_commands"
}

// DefaultRiskCommandSeeds 返回内置的风险命令规则种子数据
func DefaultRiskCommandSeeds() []RiskCommand {
	return []RiskCommand{
		// 1. 全局/通用阻断 (Block)
		{
			Vendor:   "*",
			Category: "*",
			Pattern:  `(?i)^\s*format\b`,
			Action:   RiskActionBlock,
			Reason:   "格式化文件系统属于高危破坏性操作，默认系统全局阻断",
			Enabled:  true,
			Builtin:  true,
		},
		{
			Vendor:   "*",
			Category: "*",
			Pattern:  `(?i)^\s*delete\s+/unreserved\b`,
			Action:   RiskActionBlock,
			Reason:   "永久删除文件（不可恢复回收站），默认系统全局阻断",
			Enabled:  true,
			Builtin:  true,
		},

		// 2. 厂商专用阻断 (Block)
		{
			Vendor:   "huawei",
			Category: "*",
			Pattern:  `(?i)^\s*reset\s+saved-configuration\b`,
			Action:   RiskActionBlock,
			Reason:   "清空设备已保存配置文件将导致下次启动配置丢失",
			Enabled:  true,
			Builtin:  true,
		},
		{
			Vendor:   "h3c",
			Category: "*",
			Pattern:  `(?i)^\s*reset\s+saved-configuration\b`,
			Action:   RiskActionBlock,
			Reason:   "清空设备已保存配置文件将导致下次启动配置丢失",
			Enabled:  true,
			Builtin:  true,
		},
		{
			Vendor:   "cisco",
			Category: "*",
			Pattern:  `(?i)^\s*(write\s+erase|erase\s+startup-config)\b`,
			Action:   RiskActionBlock,
			Reason:   "擦除思科设备开机配置文件将导致设备失配",
			Enabled:  true,
			Builtin:  true,
		},

		// 3. 通用人工确认 (Confirm)
		{
			Vendor:   "*",
			Category: "*",
			Pattern:  `(?i)^\s*(reboot|reload)\b`,
			Action:   RiskActionConfirm,
			Reason:   "重启设备会导致业务完全中断，需人工审批确认",
			Enabled:  true,
			Builtin:  true,
		},
		{
			Vendor:   "*",
			Category: "*",
			Pattern:  `(?i)^\s*shutdown\b`,
			Action:   RiskActionConfirm,
			Reason:   "关闭端口可能导致网络拓扑中断或业务丢包，需人工审批确认",
			Enabled:  true,
			Builtin:  true,
		},
		{
			Vendor:   "huawei",
			Category: "*",
			Pattern:  `(?i)^\s*undo\s+(ospf|bgp|isis|vlan\s+batch|interface)\b`,
			Action:   RiskActionConfirm,
			Reason:   "删除核心动态路由协议、批量VLAN或物理接口配置可能导致网络瘫痪",
			Enabled:  true,
			Builtin:  true,
		},
		{
			Vendor:   "h3c",
			Category: "*",
			Pattern:  `(?i)^\s*undo\s+(ospf|bgp|isis|vlan\s+batch|interface)\b`,
			Action:   RiskActionConfirm,
			Reason:   "删除核心动态路由协议、批量VLAN或物理接口配置可能导致网络瘫痪",
			Enabled:  true,
			Builtin:  true,
		},
		{
			Vendor:   "cisco",
			Category: "*",
			Pattern:  `(?i)^\s*no\s+(router\s+ospf|router\s+bgp|router\s+isis|interface)\b`,
			Action:   RiskActionConfirm,
			Reason:   "删除思科核心路由进程或物理接口可能导致网络瘫痪",
			Enabled:  true,
			Builtin:  true,
		},
		{
			Vendor:   "*",
			Category: "*",
			Pattern:  `(?i)^\s*reset\s+ike\s+sa\b`,
			Action:   RiskActionConfirm,
			Reason:   "重置 IKE SA 安全联盟会导致现有 IPsec VPN 隧道断开重建",
			Enabled:  true,
			Builtin:  true,
		},
		{
			Vendor:   "huawei",
			Category: "*",
			Pattern:  `(?i)^\s*undo\s+ipsec\s+policy\b`,
			Action:   RiskActionConfirm,
			Reason:   "删除 IPsec 安全策略会导致加密通道中断",
			Enabled:  true,
			Builtin:  true,
		},
		{
			Vendor:   "h3c",
			Category: "*",
			Pattern:  `(?i)^\s*undo\s+ipsec\s+policy\b`,
			Action:   RiskActionConfirm,
			Reason:   "删除 IPsec 安全策略会导致加密通道中断",
			Enabled:  true,
			Builtin:  true,
		},
		{
			Vendor:   "cisco",
			Category: "*",
			Pattern:  `(?i)^\s*no\s+crypto\s+isakmp\b`,
			Action:   RiskActionConfirm,
			Reason:   "关闭思科 ISAKMP 会导致所有 VPN 隧道中断",
			Enabled:  true,
			Builtin:  true,
		},
		{
			Vendor:   "linux",
			Category: "*",
			Pattern:  `(?i)^\s*rm\s+-(rf|fr)\s+/\s*$`,
			Action:   RiskActionBlock,
			Reason:   "根目录递归强制删除将彻底摧毁操作系统",
			Enabled:  true,
			Builtin:  true,
		},

		{
			Vendor:   "huawei",
			Category: "firewall",
			Pattern:  `(?i)^\s*packet-capture\s+all-packet\b`,
			Action:   RiskActionConfirm,
			Reason:   "全包捕获命令会消耗大量 CPU 与存储资源，需人工审批确认",
			Enabled:  true,
			Builtin:  true,
		},

		// 4. 敏感告警 (Warn)
		{
			Vendor:   "*",
			Category: "*",
			Pattern:  `(?i)^\s*(debugging|terminal\s+monitor)\b`,
			Action:   RiskActionWarn,
			Reason:   "开启全局调试打印可能消耗设备 CPU 并冲刷终端日志",
			Enabled:  true,
			Builtin:  true,
		},
		{
			Vendor:   "huawei",
			Category: "wlan",
			Pattern:  `(?i)^\s*display\s+diagnostic-information\b`,
			Action:   RiskActionWarn,
			Reason:   "全量诊断信息回显极长且耗时，建议改用定向查询命令",
			Enabled:  true,
			Builtin:  true,
		},

		// 5. 补充高危破坏性命令（P1-9 种子扩充：文件系统/引导/存储/权限类）
		{
			Vendor:   "*",
			Category: "*",
			Pattern:  `(?i)^\s*(mkfs(\.[a-z0-9]+)?|dd\s+if=)\b`,
			Action:   RiskActionBlock,
			Reason:   "文件系统格式化/裸设备写入会不可逆地破坏数据",
			Enabled:  true,
			Builtin:  true,
		},
		{
			Vendor:   "*",
			Category: "*",
			Pattern:  `(?i)^\s*(rm|del)\s+(-[a-z]*[rf][a-z]*\s+)*(/|/etc|/boot|/var|flash:|cfa0:|flash:)\b`,
			Action:   RiskActionBlock,
			Reason:   "针对系统根目录或设备存储介质的删除操作属高危破坏行为",
			Enabled:  true,
			Builtin:  true,
		},
		{
			Vendor:   "*",
			Category: "*",
			Pattern:  `(?i)^\s*chmod\s+(-R\s+)?(777|000)\s+/(\s|$)`,
			Action:   RiskActionBlock,
			Reason:   "对根目录递归开放全部权限/去除全部权限将破坏系统安全基线",
			Enabled:  true,
			Builtin:  true,
		},
		{
			Vendor:   "*",
			Category: "*",
			Pattern:  `(?i)^\s*(factory-reset|reset\s+factory|restore\s+factory-default)\b`,
			Action:   RiskActionBlock,
			Reason:   "恢复出厂设置将清空全部业务配置，属不可逆高危操作",
			Enabled:  true,
			Builtin:  true,
		},
		{
			Vendor:   "*",
			Category: "*",
			Pattern:  `(?i)^\s*(delete|rmdir)\s+/force\b`,
			Action:   RiskActionConfirm,
			Reason:   "强制删除目录/文件不可恢复，需人工审批确认",
			Enabled:  true,
			Builtin:  true,
		},
		{
			Vendor:   "huawei",
			Category: "*",
			Pattern:  `(?i)^\s*(undo\s+)?(startup\s+saved-configuration|startup\s+system-software)\b`,
			Action:   RiskActionConfirm,
			Reason:   "变更下次启动加载的配置/系统软件可能造成设备无法正常引导",
			Enabled:  true,
			Builtin:  true,
		},
		{
			Vendor:   "h3c",
			Category: "*",
			Pattern:  `(?i)^\s*(boot-loader\s+file|startup\s+boot-loader)\b`,
			Action:   RiskActionConfirm,
			Reason:   "变更引导程序文件可能造成设备无法正常启动，需人工审批确认",
			Enabled:  true,
			Builtin:  true,
		},
		{
			Vendor:   "cisco",
			Category: "*",
			Pattern:  `(?i)^\s*(reload\s+in|scheduler\s+reload)\b`,
			Action:   RiskActionConfirm,
			Reason:   "定时重启会在指定时刻中断业务，需人工审批确认",
			Enabled:  true,
			Builtin:  true,
		},
		{
			Vendor:   "linux",
			Category: "*",
			Pattern:  `(?i)^\s*(shutdown|poweroff|halt|init\s+0|systemctl\s+(poweroff|reboot))\b`,
			Action:   RiskActionConfirm,
			Reason:   "关停/重启服务器将中断其上全部业务，需人工审批确认",
			Enabled:  true,
			Builtin:  true,
		},
		{
			Vendor:   "linux",
			Category: "*",
			Pattern:  `(?i)^\s*(iptables\s+-F|ufw\s+disable)\b`,
			Action:   RiskActionWarn,
			Reason:   "清空防火墙规则或在关闭防火墙会削弱主机安全防护",
			Enabled:  true,
			Builtin:  true,
		},
	}
}

// EnsureRiskCommandSeeds 确保数据库中存在内置种子规则
func EnsureRiskCommandSeeds(db *gorm.DB) error {
	if db == nil {
		return nil
	}

	seeds := DefaultRiskCommandSeeds()
	for _, seed := range seeds {
		var count int64
		err := db.Model(&RiskCommand{}).
			Where("vendor = ? AND pattern = ?", seed.Vendor, seed.Pattern).
			Count(&count).Error
		if err != nil {
			return err
		}
		if count == 0 {
			if err := db.Create(&seed).Error; err != nil {
				return err
			}
		}
	}
	return nil
}
