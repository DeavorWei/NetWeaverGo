// Package ui 提供 Wails UI 服务层
// snmp_query_service.go 提供 SNMP 即时查询与查询凭据管理能力
package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/models"
	"github.com/NetWeaverGo/core/internal/repository"
	"github.com/NetWeaverGo/core/internal/snmp"
)

// ============================================================================
// 常量
// ============================================================================

const (
	// snmpOpDeviceInfo 采集设备基本信息（sysDescr/sysUpTime/sysName）
	snmpOpDeviceInfo = "device_info"
	// snmpOpGet 执行 SNMP GET
	snmpOpGet = "get"
	// snmpOpWalk 执行 SNMP WALK
	snmpOpWalk = "walk"
)

// snmpQueryTimeout 单次查询超时
const snmpQueryTimeout = 30 * time.Second

// snmpBatchQueryTimeout 批量查询总超时
const snmpBatchQueryTimeout = 10 * time.Minute

// ============================================================================
// 视图模型
// ============================================================================

// SNMPResultVM SNMP 查询结果视图模型
type SNMPResultVM struct {
	OID       string `json:"oid"`
	OIDName   string `json:"oidName"`
	Value     string `json:"value"`
	ValueType string `json:"valueType"`
	Error     string `json:"error,omitempty"`
}

// SNMPCredentialVM SNMP 查询凭据视图模型
// 敏感字段仅在创建/更新时传入，列表查询不返回明文
type SNMPCredentialVM struct {
	ID              uint   `json:"id"`
	Name            string `json:"name"`
	Version         string `json:"version"`
	Community       string `json:"community,omitempty"`
	SecurityLevel   string `json:"securityLevel"`
	Username        string `json:"username"`
	AuthProtocol    string `json:"authProtocol"`
	AuthPassword    string `json:"authPassword,omitempty"`
	PrivProtocol    string `json:"privProtocol"`
	PrivPassword    string `json:"privPassword,omitempty"`
	ContextName     string `json:"contextName"`
	ContextEngineID string `json:"contextEngineId"`
	CreatedAt       string `json:"createdAt"`
}

// SNMPCredentialInputVM 临时凭据（不落库，用于一次性查询）
type SNMPCredentialInputVM struct {
	Version         string `json:"version"`
	Community       string `json:"community"`
	SecurityLevel   string `json:"securityLevel"`
	Username        string `json:"username"`
	AuthProtocol    string `json:"authProtocol"`
	AuthPassword    string `json:"authPassword"`
	PrivProtocol    string `json:"privProtocol"`
	PrivPassword    string `json:"privPassword"`
	ContextName     string `json:"contextName"`
	ContextEngineID string `json:"contextEngineId"`
}

// SNMPQueryRequest 单次查询请求
type SNMPQueryRequest struct {
	Address         string                  `json:"address"`
	Operation       string                  `json:"operation"`       // device_info / get / walk
	OID             string                  `json:"oid"`             // get: 多个 OID 以逗号或换行分隔；walk: 根 OID
	CredentialID    *uint                   `json:"credentialId"`    // 已保存凭据 ID，为空则使用临时凭据
	TempCredential  *SNMPCredentialInputVM  `json:"tempCredential"`  // 临时凭据
}

// SNMPQueryResponse 单次查询响应
type SNMPQueryResponse struct {
	Address string         `json:"address"`
	Results []SNMPResultVM `json:"results"`
	Latency int64          `json:"latencyMs"`
}

// SNMPBatchQueryRequest 批量查询请求
type SNMPBatchQueryRequest struct {
	Addresses       []string                `json:"addresses"`
	CredentialID    *uint                   `json:"credentialId"`
	TempCredential  *SNMPCredentialInputVM  `json:"tempCredential"`
}

// SNMPBatchQueryResponse 批量查询响应
type SNMPBatchQueryResponse struct {
	Results   []SNMPBatchResultVM `json:"results"`
	Total     int                 `json:"total"`
	Reachable int                 `json:"reachable"`
	Latency   int64               `json:"latencyMs"`
}

// SNMPBatchResultVM 批量查询中单台设备的结果
type SNMPBatchResultVM struct {
	Address   string         `json:"address"`
	Reachable bool           `json:"reachable"`
	Results   []SNMPResultVM `json:"results,omitempty"`
	Error     string         `json:"error,omitempty"`
	Latency   int64          `json:"latencyMs"`
}

// ============================================================================
// 服务
// ============================================================================

// SNMPQueryService SNMP 即时查询服务
// 面向交付现场的设备上线确认与资产信息采集，不提供周期性轮询与 Trap 值守
type SNMPQueryService struct {
	querier *snmp.Querier
	repo    repository.CredentialRepository
	crypto  *snmp.CredentialCrypto
}

// NewSNMPQueryService 创建 SNMP 查询服务
func NewSNMPQueryService(querier *snmp.Querier, repo repository.CredentialRepository, crypto *snmp.CredentialCrypto) *SNMPQueryService {
	return &SNMPQueryService{querier: querier, repo: repo, crypto: crypto}
}

// ============================================================================
// 查询能力
// ============================================================================

// Query 执行单次 SNMP 查询
func (s *SNMPQueryService) Query(req SNMPQueryRequest) (*SNMPQueryResponse, error) {
	addr := strings.TrimSpace(req.Address)
	if addr == "" {
		return nil, fmt.Errorf("目标地址不能为空")
	}

	cred, err := s.resolveCredential(req.CredentialID, req.TempCredential)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), snmpQueryTimeout)
	defer cancel()

	start := time.Now()
	var results []snmp.SNMPResult

	switch strings.ToLower(strings.TrimSpace(req.Operation)) {
	case snmpOpGet:
		oids := parseOIDList(req.OID)
		if len(oids) == 0 {
			return nil, fmt.Errorf("GET 操作需要至少指定一个 OID")
		}
		results, err = s.querier.Get(ctx, addr, oids, cred)

	case snmpOpWalk:
		rootOID := strings.TrimSpace(req.OID)
		if rootOID == "" {
			return nil, fmt.Errorf("WALK 操作需要指定根 OID")
		}
		results, err = s.querier.Walk(ctx, addr, rootOID, cred)

	default: // device_info
		results, err = s.querier.GetDeviceInfo(ctx, addr, cred)
	}

	latency := time.Since(start).Milliseconds()
	if err != nil {
		logger.Warn("SNMP-Query", addr, "查询失败: 操作=%s, 错误=%v", req.Operation, err)
		return nil, err
	}

	return &SNMPQueryResponse{
		Address: addr,
		Results: toResultVMs(results),
		Latency: latency,
	}, nil
}

// BatchQueryDeviceInfo 批量采集设备基本信息（设备上线确认）
func (s *SNMPQueryService) BatchQueryDeviceInfo(req SNMPBatchQueryRequest) (*SNMPBatchQueryResponse, error) {
	if len(req.Addresses) == 0 {
		return nil, fmt.Errorf("目标地址列表不能为空")
	}

	cred, err := s.resolveCredential(req.CredentialID, req.TempCredential)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), snmpBatchQueryTimeout)
	defer cancel()

	start := time.Now()
	results := s.querier.BatchGetDeviceInfo(ctx, req.Addresses, cred)

	resp := &SNMPBatchQueryResponse{
		Results: make([]SNMPBatchResultVM, 0, len(results)),
		Total:   len(results),
		Latency: time.Since(start).Milliseconds(),
	}
	for _, r := range results {
		resp.Results = append(resp.Results, SNMPBatchResultVM{
			Address:   r.Address,
			Reachable: r.Reachable,
			Results:   toResultVMs(r.Results),
			Error:     r.Error,
			Latency:   r.Latency,
		})
		if r.Reachable {
			resp.Reachable++
		}
	}

	logger.Info("SNMP-Query", "-", "批量设备信息采集完成: 总数=%d, 可达=%d", resp.Total, resp.Reachable)
	return resp, nil
}

// GetDeviceInfoOIDs 获取设备基本信息 OID 列表（供前端展示）
func (s *SNMPQueryService) GetDeviceInfoOIDs() []SNMPResultVM {
	entries := snmp.DeviceInfoOIDs
	vms := make([]SNMPResultVM, 0, len(entries))
	for _, e := range entries {
		vms = append(vms, SNMPResultVM{OID: e.OID, OIDName: e.Name})
	}
	return vms
}

// ============================================================================
// 凭据管理
// ============================================================================

// GetCredentials 获取所有查询凭据（不含敏感字段明文）
func (s *SNMPQueryService) GetCredentials() ([]SNMPCredentialVM, error) {
	creds, err := s.repo.ListCredentials(context.Background())
	if err != nil {
		return nil, err
	}

	vms := make([]SNMPCredentialVM, 0, len(creds))
	for _, c := range creds {
		vms = append(vms, toCredentialVM(c, false))
	}
	return vms, nil
}

// GetCredential 获取单个查询凭据（不含敏感字段明文）
func (s *SNMPQueryService) GetCredential(id uint) (*SNMPCredentialVM, error) {
	cred, err := s.repo.GetCredential(context.Background(), id)
	if err != nil {
		return nil, err
	}
	if cred == nil {
		return nil, fmt.Errorf("凭据不存在: ID=%d", id)
	}
	vm := toCredentialVM(cred, false)
	return &vm, nil
}

// CreateCredential 创建查询凭据（敏感字段加密存储）
func (s *SNMPQueryService) CreateCredential(vm SNMPCredentialVM) (*SNMPCredentialVM, error) {
	name := strings.TrimSpace(vm.Name)
	if name == "" {
		return nil, fmt.Errorf("凭据名称不能为空")
	}
	if strings.TrimSpace(vm.Version) == "" {
		return nil, fmt.Errorf("SNMP 版本不能为空")
	}

	existing, err := s.repo.GetCredentialByName(context.Background(), name)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, fmt.Errorf("凭据名称已存在: %s", name)
	}

	cred := &models.SNMPCredential{
		Name:            name,
		Version:         strings.ToLower(strings.TrimSpace(vm.Version)),
		Community:       vm.Community,
		SecurityLevel:   vm.SecurityLevel,
		Username:        vm.Username,
		AuthProtocol:    vm.AuthProtocol,
		AuthPassword:    vm.AuthPassword,
		PrivProtocol:    vm.PrivProtocol,
		PrivPassword:    vm.PrivPassword,
		ContextName:     vm.ContextName,
		ContextEngineID: vm.ContextEngineID,
	}
	if err := s.encryptCredential(cred); err != nil {
		return nil, err
	}

	if err := s.repo.CreateCredential(context.Background(), cred); err != nil {
		return nil, err
	}

	created := toCredentialVM(cred, false)
	return &created, nil
}

// UpdateCredential 更新查询凭据
// 敏感字段留空表示保持原值不变
func (s *SNMPQueryService) UpdateCredential(vm SNMPCredentialVM) error {
	if vm.ID == 0 {
		return fmt.Errorf("凭据 ID 不能为空")
	}

	cred, err := s.repo.GetCredential(context.Background(), vm.ID)
	if err != nil {
		return err
	}
	if cred == nil {
		return fmt.Errorf("凭据不存在: ID=%d", vm.ID)
	}

	if name := strings.TrimSpace(vm.Name); name != "" {
		cred.Name = name
	}
	if version := strings.TrimSpace(vm.Version); version != "" {
		cred.Version = strings.ToLower(version)
	}
	cred.SecurityLevel = vm.SecurityLevel
	cred.Username = vm.Username
	cred.AuthProtocol = vm.AuthProtocol
	cred.PrivProtocol = vm.PrivProtocol
	cred.ContextName = vm.ContextName
	cred.ContextEngineID = vm.ContextEngineID

	// 仅当传入新值时才更新加密字段
	if vm.Community != "" {
		cred.Community = vm.Community
	}
	if vm.AuthPassword != "" {
		cred.AuthPassword = vm.AuthPassword
	}
	if vm.PrivPassword != "" {
		cred.PrivPassword = vm.PrivPassword
	}
	if err := s.encryptCredential(cred); err != nil {
		return err
	}

	return s.repo.UpdateCredential(context.Background(), cred)
}

// DeleteCredential 删除查询凭据
func (s *SNMPQueryService) DeleteCredential(id uint) error {
	return s.repo.DeleteCredential(context.Background(), id)
}

// ============================================================================
// 内部方法
// ============================================================================

// resolveCredential 解析查询所需凭据
// 优先使用已保存凭据，其次使用临时凭据，都没有则返回 nil（使用默认 community）
func (s *SNMPQueryService) resolveCredential(credentialID *uint, temp *SNMPCredentialInputVM) (*models.SNMPCredential, error) {
	if credentialID != nil && *credentialID > 0 {
		cred, err := s.repo.GetCredential(context.Background(), *credentialID)
		if err != nil {
			return nil, fmt.Errorf("获取凭据失败: %w", err)
		}
		if cred == nil {
			return nil, fmt.Errorf("凭据不存在: ID=%d", *credentialID)
		}
		return cred, nil
	}

	if temp != nil {
		return &models.SNMPCredential{
			Version:         strings.ToLower(strings.TrimSpace(temp.Version)),
			Community:       temp.Community,
			SecurityLevel:   temp.SecurityLevel,
			Username:        temp.Username,
			AuthProtocol:    temp.AuthProtocol,
			AuthPassword:    temp.AuthPassword,
			PrivProtocol:    temp.PrivProtocol,
			PrivPassword:    temp.PrivPassword,
			ContextName:     temp.ContextName,
			ContextEngineID: temp.ContextEngineID,
		}, nil
	}

	return nil, nil
}

// encryptCredential 加密凭据中的敏感字段（幂等，已加密则跳过）
func (s *SNMPQueryService) encryptCredential(cred *models.SNMPCredential) error {
	if s.crypto == nil {
		return nil
	}

	var err error
	if cred.Community != "" && !snmp.IsEncrypted(cred.Community) {
		if cred.Community, err = s.crypto.EncryptCredential(cred.Community); err != nil {
			return fmt.Errorf("加密 community 失败: %w", err)
		}
	}
	if cred.AuthPassword != "" && !snmp.IsEncrypted(cred.AuthPassword) {
		if cred.AuthPassword, err = s.crypto.EncryptCredential(cred.AuthPassword); err != nil {
			return fmt.Errorf("加密认证密码失败: %w", err)
		}
	}
	if cred.PrivPassword != "" && !snmp.IsEncrypted(cred.PrivPassword) {
		if cred.PrivPassword, err = s.crypto.EncryptCredential(cred.PrivPassword); err != nil {
			return fmt.Errorf("加密加密密码失败: %w", err)
		}
	}
	return nil
}

// ============================================================================
// 辅助函数
// ============================================================================

// toResultVMs 转换查询结果为视图模型
func toResultVMs(results []snmp.SNMPResult) []SNMPResultVM {
	if results == nil {
		return []SNMPResultVM{}
	}
	vms := make([]SNMPResultVM, 0, len(results))
	for _, r := range results {
		vms = append(vms, SNMPResultVM{
			OID:       r.OID,
			OIDName:   r.OIDName,
			Value:     r.Value,
			ValueType: r.ValueType,
			Error:     r.Error,
		})
	}
	return vms
}

// toCredentialVM 转换凭据为视图模型
// includeSensitive 为 false 时清空敏感字段
func toCredentialVM(cred *models.SNMPCredential, includeSensitive bool) SNMPCredentialVM {
	vm := SNMPCredentialVM{
		ID:              cred.ID,
		Name:            cred.Name,
		Version:         cred.Version,
		SecurityLevel:   cred.SecurityLevel,
		Username:        cred.Username,
		AuthProtocol:    cred.AuthProtocol,
		PrivProtocol:    cred.PrivProtocol,
		ContextName:     cred.ContextName,
		ContextEngineID: cred.ContextEngineID,
		CreatedAt:       cred.CreatedAt.Format(time.RFC3339),
	}
	if includeSensitive {
		vm.Community = cred.Community
		vm.AuthPassword = cred.AuthPassword
		vm.PrivPassword = cred.PrivPassword
	}
	return vm
}

// parseOIDList 解析 OID 列表（支持逗号、分号、换行、空格分隔）
func parseOIDList(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	sep := func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\r' || r == ' ' || r == '\t'
	}

	oids := make([]string, 0, 4)
	for _, part := range strings.FieldsFunc(raw, sep) {
		oid := strings.TrimSpace(part)
		if oid != "" {
			oids = append(oids, oid)
		}
	}
	return oids
}
