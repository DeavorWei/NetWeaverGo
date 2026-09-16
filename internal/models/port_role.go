package models

import (
	"strings"
)

// PortRole 端口及链路角色
type PortRole string

const (
	PortRoleUplink       PortRole = "Uplink"       // 上联
	PortRoleDownlink     PortRole = "Downlink"     // 下联
	PortRoleInterconnect PortRole = "Interconnect" // 横向互联 / 对等 (Core-Core, Spine-Spine)
	PortRoleStacking     PortRole = "Stacking"     // 堆叠/集群链路
	PortRoleOOB          PortRole = "OOB"          // 带外管理口
	PortRoleAccess       PortRole = "Access"       // 终端/业务接入
	PortRoleUnknown      PortRole = "Unknown"      // 未知角色
)

// InferPortRole 根据本端与对端设备角色及端口特征推断端口链路角色
func InferPortRole(localDevRole, remoteDevRole string, localIf, remoteIf string) PortRole {
	locIfLower := strings.ToLower(localIf)
	remIfLower := strings.ToLower(remoteIf)

	// 1. 堆叠与集群链路判定
	if strings.Contains(locIfLower, "stack") || strings.Contains(locIfLower, "irf") ||
		strings.Contains(locIfLower, "fabric") || strings.Contains(remIfLower, "stack") {
		return PortRoleStacking
	}

	// 2. 带外管理端口判定
	if strings.Contains(locIfLower, "mgmt") || strings.Contains(locIfLower, "meth") ||
		strings.Contains(locIfLower, "oob") || strings.Contains(locIfLower, "management") {
		return PortRoleOOB
	}

	locRole := strings.ToLower(strings.TrimSpace(localDevRole))
	remRole := strings.ToLower(strings.TrimSpace(remoteDevRole))

	// 层级权重定义 (Core/Spine > Agg/Leaf > Access > Terminal)
	tierWeight := func(role string) int {
		switch {
		case strings.Contains(role, "core") || strings.Contains(role, "spine") || strings.Contains(role, "border") || strings.Contains(role, "wan"):
			return 4
		case strings.Contains(role, "agg") || strings.Contains(role, "distribution"):
			return 3
		case strings.Contains(role, "leaf"):
			return 2
		case strings.Contains(role, "access") || strings.Contains(role, "tor"):
			return 1
		default:
			return 0
		}
	}

	wLocal := tierWeight(locRole)
	wRemote := tierWeight(remRole)

	// 3. 上下级层级推断
	if wLocal > 0 && wRemote > 0 {
		if wLocal < wRemote {
			return PortRoleUplink
		}
		if wLocal > wRemote {
			return PortRoleDownlink
		}
		return PortRoleInterconnect
	}

	// 4. 单边已知层级
	if wLocal > 0 {
		if wLocal >= 3 {
			return PortRoleDownlink
		}
		return PortRoleUplink
	}
	if wRemote > 0 {
		if wRemote >= 3 {
			return PortRoleUplink
		}
		return PortRoleDownlink
	}

	// 5. 接口命名辅助推断 (如 Uplink-Eth-Trunk)
	if strings.Contains(locIfLower, "up") {
		return PortRoleUplink
	}
	if strings.Contains(locIfLower, "down") {
		return PortRoleDownlink
	}

	return PortRoleInterconnect
}
