// Package snmp 提供 SNMP 即时查询能力
// types.go 定义查询相关类型与凭据加密管理器结构
package snmp

import "sync"

// ============================================================================
// 查询结果类型
// ============================================================================

// SNMPResult SNMP 查询结果
type SNMPResult struct {
	OID       string `json:"oid"`
	OIDName   string `json:"oidName"`
	Value     string `json:"value"`
	ValueType string `json:"valueType"`
	Error     string `json:"error,omitempty"`
}

// BatchQueryResult 批量查询结果（单台设备）
type BatchQueryResult struct {
	Address   string       `json:"address"`
	Reachable bool         `json:"reachable"`
	Results   []SNMPResult `json:"results,omitempty"`
	Error     string       `json:"error,omitempty"`
	Latency   int64        `json:"latencyMs"`
}

// ============================================================================
// 凭据加密
// ============================================================================

// CredentialCrypto SNMP 凭据加密管理器
// 使用 AES-256-GCM 对敏感字段加密，实现见 crypto.go
type CredentialCrypto struct {
	key []byte       // 32 bytes AES-256 key
	mu  sync.RWMutex // 并发保护
}
