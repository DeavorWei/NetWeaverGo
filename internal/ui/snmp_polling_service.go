// Package ui 提供 Wails 暴露层服务
// snmp_polling_service.go 实现轮询管理 Wails 绑定服务
package ui

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/models"
	"github.com/NetWeaverGo/core/internal/repository"
	"github.com/NetWeaverGo/core/internal/snmp"
)

// ============================================================================
// View Models - 凭据
// ============================================================================

// CredentialVM 凭据 View Model
type CredentialVM struct {
	ID              uint   `json:"id"`
	Name            string `json:"name"`
	Version         string `json:"version"`         // v1/v2c/v3
	Community       string `json:"community"`       // 已脱敏（v1/v2c）
	SecurityLevel   string `json:"securityLevel"`   // noAuthNoPriv/authNoPriv/authPriv
	Username        string `json:"username"`        // v3 用户名
	AuthProtocol    string `json:"authProtocol"`    // MD5/SHA/SHA224/SHA256/SHA384/SHA512
	HasAuthKey      bool   `json:"hasAuthKey"`      // 是否已设置认证密钥
	PrivProtocol    string `json:"privProtocol"`    // DES/AES/AES192/AES256/AES192C/AES256C
	HasPrivKey      bool   `json:"hasPrivKey"`      // 是否已设置加密密钥
	ContextName     string `json:"contextName"`     // v3 上下文名
	ContextEngineID string `json:"contextEngineId"` // v3 上下文引擎 ID
	CreatedAt       string `json:"createdAt"`
	UpdatedAt       string `json:"updatedAt"`
}

// CreateCredentialRequest 创建凭据请求
type CreateCredentialRequest struct {
	Name            string `json:"name"`
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

// UpdateCredentialRequest 更新凭据请求
type UpdateCredentialRequest struct {
	Name            string `json:"name"`
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

// ============================================================================
// View Models - 轮询模板
// ============================================================================

// PollingTemplateVM 轮询模板 View Model
type PollingTemplateVM struct {
	ID          uint              `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Category    string            `json:"category"`
	OIDItems    []OIDItemVM       `json:"oidItems"`
	CreatedAt   string            `json:"createdAt"`
	UpdatedAt   string            `json:"updatedAt"`
}

// OIDItemVM OID 项 View Model
type OIDItemVM struct {
	OID         string `json:"oid"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	Operation   string `json:"operation"`
	Description string `json:"description"`
}

// CreatePollingTemplateRequest 创建轮询模板请求
type CreatePollingTemplateRequest struct {
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Category    string        `json:"category"`
	OIDItems    []OIDItemVM   `json:"oidItems"`
}

// UpdatePollingTemplateRequest 更新轮询模板请求
type UpdatePollingTemplateRequest struct {
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Category    string        `json:"category"`
	OIDItems    []OIDItemVM   `json:"oidItems"`
}

// ============================================================================
// View Models - 轮询目标
// ============================================================================

// PollingTargetVM 轮询目标 View Model
type PollingTargetVM struct {
	ID             uint   `json:"id"`
	TargetIP       string `json:"targetIP"`
	TargetPort     int    `json:"targetPort"`
	DisplayName    string `json:"displayName"`
	CredentialID   *uint  `json:"credentialId"`
	CredentialName string `json:"credentialName"`
	TemplateID     *uint  `json:"templateId"`
	TemplateName   string `json:"templateName"`
	PollInterval   int    `json:"pollInterval"`
	Enabled        bool   `json:"enabled"`
	LastPollAt     string `json:"lastPollAt"`
	LastPollStatus string `json:"lastPollStatus"`
	LastPollError  string `json:"lastPollError"`
	CreatedAt      string `json:"createdAt"`
	UpdatedAt      string `json:"updatedAt"`
}

// PollingTargetFilterVM 轮询目标过滤条件 View Model
type PollingTargetFilterVM struct {
	TemplateID *uint  `json:"templateId"`
	Enabled   *bool   `json:"enabled"`
	SearchIP  string  `json:"searchIP"`
}

// CreatePollingTargetRequest 创建轮询目标请求
type CreatePollingTargetRequest struct {
	TargetIP     string `json:"targetIP"`
	TargetPort   int    `json:"targetPort"`
	DisplayName  string `json:"displayName"`
	CredentialID *uint  `json:"credentialId"`
	TemplateID   *uint  `json:"templateId"`
	PollInterval int    `json:"pollInterval"`
	Enabled      bool   `json:"enabled"`
}

// UpdatePollingTargetRequest 更新轮询目标请求
type UpdatePollingTargetRequest struct {
	TargetIP     string `json:"targetIP"`
	TargetPort   int    `json:"targetPort"`
	DisplayName  string `json:"displayName"`
	CredentialID *uint  `json:"credentialId"`
	TemplateID   *uint  `json:"templateId"`
	PollInterval int    `json:"pollInterval"`
	Enabled      bool   `json:"enabled"`
}

// ============================================================================
// View Models - 轮询结果
// ============================================================================

// PollingResultVM 轮询结果 View Model
type PollingResultVM struct {
	ID        uint   `json:"id"`
	TargetID  uint   `json:"targetId"`
	TargetIP  string `json:"targetIP"`
	BatchID   string `json:"batchId"`
	OID       string `json:"oid"`
	OIDName   string `json:"oidName"`
	Value     string `json:"value"`
	ValueType string `json:"valueType"`
	PollTime  string `json:"pollTime"`
	CreatedAt string `json:"createdAt"`
}

// PollingResultListVM 轮询结果列表 View Model
type PollingResultListVM struct {
	Data       []PollingResultVM `json:"data"`
	Total      int64              `json:"total"`
	Page       int                `json:"page"`
	PageSize   int                `json:"pageSize"`
	TotalPages int                `json:"totalPages"`
}

// PollingResultFilterVM 轮询结果过滤条件 View Model
type PollingResultFilterVM struct {
	TargetID  *uint  `json:"targetId"`
	OID       string `json:"oid"`
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime"`
	BatchID   string `json:"batchId"`
}

// ============================================================================
// View Models - 调度器状态
// ============================================================================

// SchedulerStatusVM 调度器状态 View Model
type SchedulerStatusVM struct {
	IsRunning        bool                `json:"isRunning"`
	TargetCount      int                 `json:"targetCount"`
	TotalPolls       int64               `json:"totalPolls"`
	LastPollTime     string              `json:"lastPollTime"`
	StartTime        string              `json:"startTime"`
	DispatcherStatus *DispatcherStatusVM `json:"dispatcherStatus,omitempty"` // 分发器状态
}

// DispatcherStatusVM 分发器运行状态 View Model
type DispatcherStatusVM struct {
	ActiveDevices int32 `json:"activeDevices"` // 当前活跃设备数
	MaxDevices    int   `json:"maxDevices"`    // 最大并发设备数
	WaitingTasks  int32 `json:"waitingTasks"`  // 排队等待的任务数
	SkippedTasks  int64 `json:"skippedTasks"`  // 累计跳过的任务数
}

// ConcurrencyConfigVM 并发配置 View Model
type ConcurrencyConfigVM struct {
	MaxDevices       int  `json:"maxDevices"`       // 最大并发设备数
	MaxOpsPerDevice  int  `json:"maxOpsPerDevice"`  // 单设备最大并发操作数
	SkipIfBusy       bool `json:"skipIfBusy"`       // 设备繁忙时是否跳过
	QueueTimeoutSecs int  `json:"queueTimeoutSecs"` // 排队超时（秒）
}

// PollingStatsVM 轮询统计 View Model
type PollingStatsVM struct {
	TotalPolls   int64   `json:"totalPolls"`
	SuccessCount int64   `json:"successCount"`
	FailCount    int64   `json:"failCount"`
	AvgLatencyMs float64 `json:"avgLatencyMs"`
	LastPollTime string  `json:"lastPollTime"`
}

// ============================================================================
// SNMPPollingService - 轮询管理服务 (Wails 绑定)
// ============================================================================

// SNMPPollingService 轮询管理服务
// 暴露凭据管理、模板管理、目标管理、结果查询、调度器管理给前端
type SNMPPollingService struct {
	poller    *snmp.Poller
	scheduler *snmp.PollerScheduler
	repo      repository.PollingRepository
	notifier  *SNMPEventNotifier
	crypto    *snmp.CredentialCrypto
}

// NewSNMPPollingService 创建服务实例
func NewSNMPPollingService(
	poller *snmp.Poller,
	scheduler *snmp.PollerScheduler,
	repo repository.PollingRepository,
	notifier *SNMPEventNotifier,
	crypto *snmp.CredentialCrypto,
) *SNMPPollingService {
	service := &SNMPPollingService{
		poller:    poller,
		scheduler: scheduler,
		repo:      repo,
		notifier:  notifier,
		crypto:    crypto,
	}

	logger.Info("SNMP-PollingService", "-", "轮询管理服务已初始化")
	return service
}

// ============================================================================
// 调度器管理
// ============================================================================

// StartScheduler 启动轮询调度器
func (s *SNMPPollingService) StartScheduler(ctx context.Context) error {
	logger.Info("SNMP-PollingService", "-", "启动轮询调度器请求")

	if s.scheduler == nil {
		return fmt.Errorf("调度器未初始化")
	}

	return s.scheduler.Start()
}

// StopScheduler 停止轮询调度器
func (s *SNMPPollingService) StopScheduler(ctx context.Context) error {
	logger.Info("SNMP-PollingService", "-", "停止轮询调度器请求")

	if s.scheduler != nil {
		return s.scheduler.Stop()
	}

	return nil
}

// GetSchedulerStatus 获取调度器状态（含分发器状态）
func (s *SNMPPollingService) GetSchedulerStatus(ctx context.Context) (*SchedulerStatusVM, error) {
	if s.scheduler == nil {
		return &SchedulerStatusVM{IsRunning: false}, nil
	}

	status := s.scheduler.GetSchedulerStatus()

	vm := &SchedulerStatusVM{
		IsRunning:   status.IsRunning,
		TargetCount: status.TargetCount,
		TotalPolls:  status.TotalPolls,
	}

	if !status.LastPollTime.IsZero() {
		vm.LastPollTime = status.LastPollTime.Format(time.RFC3339)
	}
	if !status.StartTime.IsZero() {
		vm.StartTime = status.StartTime.Format(time.RFC3339)
	}

	// 附加分发器状态
	if dispatcher := s.scheduler.GetDispatcher(); dispatcher != nil {
		ds := dispatcher.GetStatus()
		vm.DispatcherStatus = &DispatcherStatusVM{
			ActiveDevices: int32(ds.ActiveDevices),
			MaxDevices:    ds.MaxDevices,
			WaitingTasks:  int32(ds.WaitingTasks),
			SkippedTasks:  ds.SkippedTasks,
		}
	}

	return vm, nil
}

// ============================================================================
// 并发配置管理
// ============================================================================

// UpdateConcurrencyConfig 更新并发控制配置
// 更新运行时分发器配置（SkipIfBusy 和 QueueTimeout 保留现有值）
func (s *SNMPPollingService) UpdateConcurrencyConfig(ctx context.Context, maxDevices, maxOpsPerDevice int) error {
	logger.Info("SNMP-PollingService", "-", "更新并发配置: maxDevices=%d, maxOpsPerDevice=%d", maxDevices, maxOpsPerDevice)

	if s.scheduler == nil {
		return fmt.Errorf("调度器未初始化")
	}

	dispatcher := s.scheduler.GetDispatcher()
	if dispatcher == nil {
		return fmt.Errorf("分发器未初始化")
	}

	// 参数校验
	if maxDevices <= 0 {
		maxDevices = snmp.DefaultDispatcherConfig.MaxConcurrentDevices
	}
	if maxOpsPerDevice <= 0 {
		maxOpsPerDevice = snmp.DefaultDispatcherConfig.MaxOpsPerDevice
	}

	// 获取现有配置，保留 SkipIfBusy 和 QueueTimeout
	existing := dispatcher.GetConfig()
	newConfig := snmp.DispatcherConfig{
		MaxConcurrentDevices: maxDevices,
		MaxOpsPerDevice:      maxOpsPerDevice,
		SkipIfBusy:           existing.SkipIfBusy,
		QueueTimeout:         existing.QueueTimeout,
	}

	// 更新运行时分发器配置
	if err := dispatcher.UpdateConfig(newConfig); err != nil {
		logger.Error("SNMP-PollingService", "-", "更新分发器配置失败: %v", err)
		return fmt.Errorf("更新分发器配置失败: %w", err)
	}

	logger.Info("SNMP-PollingService", "-", "并发配置已更新: maxDevices=%d, maxOpsPerDevice=%d", maxDevices, maxOpsPerDevice)
	return nil
}

// GetConcurrencyConfig 获取当前并发配置
func (s *SNMPPollingService) GetConcurrencyConfig(ctx context.Context) (*ConcurrencyConfigVM, error) {
	if s.scheduler == nil {
		return &ConcurrencyConfigVM{
			MaxDevices:       snmp.DefaultDispatcherConfig.MaxConcurrentDevices,
			MaxOpsPerDevice:  snmp.DefaultDispatcherConfig.MaxOpsPerDevice,
			SkipIfBusy:       snmp.DefaultDispatcherConfig.SkipIfBusy,
			QueueTimeoutSecs: int(snmp.DefaultDispatcherConfig.QueueTimeout.Seconds()),
		}, nil
	}

	dispatcher := s.scheduler.GetDispatcher()
	if dispatcher == nil {
		return &ConcurrencyConfigVM{
			MaxDevices:       snmp.DefaultDispatcherConfig.MaxConcurrentDevices,
			MaxOpsPerDevice:  snmp.DefaultDispatcherConfig.MaxOpsPerDevice,
			SkipIfBusy:       snmp.DefaultDispatcherConfig.SkipIfBusy,
			QueueTimeoutSecs: int(snmp.DefaultDispatcherConfig.QueueTimeout.Seconds()),
		}, nil
	}

	// 从分发器获取运行时配置
	cfg := dispatcher.GetConfig()
	return &ConcurrencyConfigVM{
		MaxDevices:       cfg.MaxConcurrentDevices,
		MaxOpsPerDevice:  cfg.MaxOpsPerDevice,
		SkipIfBusy:       cfg.SkipIfBusy,
		QueueTimeoutSecs: int(cfg.QueueTimeout.Seconds()),
	}, nil
}

// ============================================================================
// 凭据管理
// ============================================================================

// GetCredentials 获取所有凭据
func (s *SNMPPollingService) GetCredentials(ctx context.Context) ([]CredentialVM, error) {
	creds, err := s.repo.ListCredentials(ctx)
	if err != nil {
		logger.Error("SNMP-PollingService", "-", "获取凭据列表失败: %v", err)
		return nil, err
	}

	vms := make([]CredentialVM, len(creds))
	for i, cred := range creds {
		vms[i] = s.credentialToVM(cred)
	}

	return vms, nil
}

// GetCredential 获取单个凭据
func (s *SNMPPollingService) GetCredential(ctx context.Context, id uint) (*CredentialVM, error) {
	cred, err := s.repo.GetCredential(ctx, id)
	if err != nil {
		logger.Error("SNMP-PollingService", "-", "获取凭据失败: ID=%d, %v", id, err)
		return nil, err
	}
	if cred == nil {
		return nil, fmt.Errorf("凭据不存在: ID=%d", id)
	}

	vm := s.credentialToVM(cred)
	return &vm, nil
}

// CreateCredential 创建凭据
func (s *SNMPPollingService) CreateCredential(ctx context.Context, req CreateCredentialRequest) error {
	cred := &models.SNMPCredential{
		Name:            req.Name,
		Version:         req.Version,
		SecurityLevel:   req.SecurityLevel,
		Username:        req.Username,
		AuthProtocol:    req.AuthProtocol,
		PrivProtocol:    req.PrivProtocol,
		ContextName:     req.ContextName,
		ContextEngineID: req.ContextEngineID,
	}

	// 加密敏感字段
	if s.crypto != nil {
		if req.Community != "" {
			encrypted, err := s.crypto.EncryptCredential(req.Community)
			if err != nil {
				return fmt.Errorf("加密 community 失败: %w", err)
			}
			cred.Community = encrypted
		}
		if req.AuthPassword != "" {
			encrypted, err := s.crypto.EncryptCredential(req.AuthPassword)
			if err != nil {
				return fmt.Errorf("加密 auth password 失败: %w", err)
			}
			cred.AuthPassword = encrypted
		}
		if req.PrivPassword != "" {
			encrypted, err := s.crypto.EncryptCredential(req.PrivPassword)
			if err != nil {
				return fmt.Errorf("加密 priv password 失败: %w", err)
			}
			cred.PrivPassword = encrypted
		}
	} else {
		// 未配置加密器，明文存储（不推荐）
		cred.Community = req.Community
		cred.AuthPassword = req.AuthPassword
		cred.PrivPassword = req.PrivPassword
		logger.Warn("SNMP-PollingService", "-", "⚠️ 凭据未加密存储，建议配置加密器")
	}

	if err := s.repo.CreateCredential(ctx, cred); err != nil {
		logger.Error("SNMP-PollingService", "-", "创建凭据失败: %v", err)
		return err
	}

	logger.Info("SNMP-PollingService", "-", "凭据已创建: ID=%d, Name=%s", cred.ID, cred.Name)
	return nil
}

// UpdateCredential 更新凭据
func (s *SNMPPollingService) UpdateCredential(ctx context.Context, id uint, req UpdateCredentialRequest) error {
	cred, err := s.repo.GetCredential(ctx, id)
	if err != nil {
		return err
	}
	if cred == nil {
		return fmt.Errorf("凭据不存在: ID=%d", id)
	}

	cred.Name = req.Name
	cred.Version = req.Version
	cred.SecurityLevel = req.SecurityLevel
	cred.Username = req.Username
	cred.AuthProtocol = req.AuthProtocol
	cred.PrivProtocol = req.PrivProtocol
	cred.ContextName = req.ContextName
	cred.ContextEngineID = req.ContextEngineID

	// 加密敏感字段
	if s.crypto != nil {
		if req.Community != "" {
			// 处理前端回传的脱敏 community 串
			// 如果前端传入的 community 为固定的脱敏占位符 "******"，说明并未被用户修改过，应保持原加密数据不变
			if req.Community == "******" {
				// 未修改，不做更新，保留原 cred.Community 的加密值
			} else {
				// 用户更改了 community，重新加密新值
				encrypted, err := s.crypto.EncryptCredential(req.Community)
				if err != nil {
					return fmt.Errorf("加密 community 失败: %w", err)
				}
				cred.Community = encrypted
			}
		}
		if req.AuthPassword != "" {
			encrypted, err := s.crypto.EncryptCredential(req.AuthPassword)
			if err != nil {
				return fmt.Errorf("加密 auth password 失败: %w", err)
			}
			cred.AuthPassword = encrypted
		}
		if req.PrivPassword != "" {
			encrypted, err := s.crypto.EncryptCredential(req.PrivPassword)
			if err != nil {
				return fmt.Errorf("加密 priv password 失败: %w", err)
			}
			cred.PrivPassword = encrypted
		}
	} else {
		cred.Community = req.Community
		cred.AuthPassword = req.AuthPassword
		cred.PrivPassword = req.PrivPassword
	}

	if err := s.repo.UpdateCredential(ctx, cred); err != nil {
		logger.Error("SNMP-PollingService", "-", "更新凭据失败: %v", err)
		return err
	}

	logger.Info("SNMP-PollingService", "-", "凭据已更新: ID=%d, Name=%s", cred.ID, cred.Name)
	return nil
}

// DeleteCredential 删除凭据
// 删除前检查是否有轮询目标关联，如果有则拒绝删除
func (s *SNMPPollingService) DeleteCredential(ctx context.Context, id uint) error {
	// 检查是否有轮询目标关联此凭据
	filter := repository.PollingTargetFilter{}
	targets, _, err := s.repo.ListPollingTargets(ctx, filter)
	if err != nil {
		logger.Error("SNMP-PollingService", "-", "查询轮询目标失败: %v", err)
		return fmt.Errorf("查询轮询目标失败: %w", err)
	}

	// 查找关联此凭据的目标
	for _, target := range targets {
		if target.CredentialID != nil && *target.CredentialID == id {
			return fmt.Errorf("凭据 ID=%d 正被轮询目标 '%s' (ID=%d) 关联，无法删除", id, target.DisplayName, target.ID)
		}
	}

	if err := s.repo.DeleteCredential(ctx, id); err != nil {
		logger.Error("SNMP-PollingService", "-", "删除凭据失败: %v", err)
		return err
	}

	logger.Info("SNMP-PollingService", "-", "凭据已删除: ID=%d", id)
	return nil
}

// ============================================================================
// 轮询模板管理
// ============================================================================

// GetPollingTemplates 获取所有轮询模板
func (s *SNMPPollingService) GetPollingTemplates(ctx context.Context) ([]PollingTemplateVM, error) {
	templates, err := s.repo.ListPollingTemplates(ctx)
	if err != nil {
		logger.Error("SNMP-PollingService", "-", "获取模板列表失败: %v", err)
		return nil, err
	}

	vms := make([]PollingTemplateVM, len(templates))
	for i, template := range templates {
		vms[i] = s.templateToVM(template)
	}

	return vms, nil
}

// GetPollingTemplate 获取单个轮询模板
func (s *SNMPPollingService) GetPollingTemplate(ctx context.Context, id uint) (*PollingTemplateVM, error) {
	template, err := s.repo.GetPollingTemplate(ctx, id)
	if err != nil {
		return nil, err
	}
	if template == nil {
		return nil, fmt.Errorf("模板不存在: ID=%d", id)
	}

	vm := s.templateToVM(template)
	return &vm, nil
}

// CreatePollingTemplate 创建轮询模板
func (s *SNMPPollingService) CreatePollingTemplate(ctx context.Context, req CreatePollingTemplateRequest) error {
	template := &models.SNMPPollingTemplate{
		Name:        req.Name,
		Description: req.Description,
		Category:    req.Category,
	}

	// 转换 OID 项
	for _, item := range req.OIDItems {
		template.OIDItems = append(template.OIDItems, models.SNMPOIDItem{
			OID:         item.OID,
			Name:        item.Name,
			Type:        item.Type,
			Operation:   item.Operation,
			Description: item.Description,
		})
	}

	if err := s.repo.CreatePollingTemplate(ctx, template); err != nil {
		logger.Error("SNMP-PollingService", "-", "创建模板失败: %v", err)
		return err
	}

	logger.Info("SNMP-PollingService", "-", "模板已创建: ID=%d, Name=%s", template.ID, template.Name)
	return nil
}

// UpdatePollingTemplate 更新轮询模板
func (s *SNMPPollingService) UpdatePollingTemplate(ctx context.Context, id uint, req UpdatePollingTemplateRequest) error {
	template, err := s.repo.GetPollingTemplate(ctx, id)
	if err != nil {
		return err
	}
	if template == nil {
		return fmt.Errorf("模板不存在: ID=%d", id)
	}

	template.Name = req.Name
	template.Description = req.Description
	template.Category = req.Category
	template.OIDItems = make([]models.SNMPOIDItem, 0)

	for _, item := range req.OIDItems {
		template.OIDItems = append(template.OIDItems, models.SNMPOIDItem{
			OID:         item.OID,
			Name:        item.Name,
			Type:        item.Type,
			Operation:   item.Operation,
			Description: item.Description,
		})
	}

	if err := s.repo.UpdatePollingTemplate(ctx, template); err != nil {
		logger.Error("SNMP-PollingService", "-", "更新模板失败: %v", err)
		return err
	}

	logger.Info("SNMP-PollingService", "-", "模板已更新: ID=%d, Name=%s", template.ID, template.Name)
	return nil
}

// DeletePollingTemplate 删除轮询模板
// 删除前检查是否有轮询目标关联，如果有则拒绝删除
func (s *SNMPPollingService) DeletePollingTemplate(ctx context.Context, id uint) error {
	// 检查是否有轮询目标关联此模板
	templateID := id
	filter := repository.PollingTargetFilter{
		TemplateID: &templateID,
	}
	targets, _, err := s.repo.ListPollingTargets(ctx, filter)
	if err != nil {
		logger.Error("SNMP-PollingService", "-", "查询轮询目标失败: %v", err)
		return fmt.Errorf("查询轮询目标失败: %w", err)
	}

	if len(targets) > 0 {
		return fmt.Errorf("模板 ID=%d 正被 %d 个轮询目标关联，无法删除", id, len(targets))
	}

	if err := s.repo.DeletePollingTemplate(ctx, id); err != nil {
		logger.Error("SNMP-PollingService", "-", "删除模板失败: %v", err)
		return err
	}

	logger.Info("SNMP-PollingService", "-", "模板已删除: ID=%d", id)
	return nil
}

// ============================================================================
// 轮询目标管理
// ============================================================================

// GetPollingTargets 获取轮询目标列表
func (s *SNMPPollingService) GetPollingTargets(ctx context.Context, filter PollingTargetFilterVM) ([]PollingTargetVM, error) {
	repoFilter := repository.PollingTargetFilter{
		TemplateID: filter.TemplateID,
		Enabled:    filter.Enabled,
		SearchIP:   filter.SearchIP,
	}

	targets, _, err := s.repo.ListPollingTargets(ctx, repoFilter)
	if err != nil {
		logger.Error("SNMP-PollingService", "-", "获取目标列表失败: %v", err)
		return nil, err
	}

	vms := make([]PollingTargetVM, len(targets))
	for i, target := range targets {
		vms[i] = s.targetToVM(target)

		// 加载关联名称
		if target.CredentialID != nil {
			if cred, _ := s.repo.GetCredential(ctx, *target.CredentialID); cred != nil {
				vms[i].CredentialName = cred.Name
			}
		}
		if target.TemplateID != nil {
			if template, _ := s.repo.GetPollingTemplate(ctx, *target.TemplateID); template != nil {
				vms[i].TemplateName = template.Name
			}
		}
	}

	return vms, nil
}

// GetPollingTarget 获取单个轮询目标
func (s *SNMPPollingService) GetPollingTarget(ctx context.Context, id uint) (*PollingTargetVM, error) {
	target, err := s.repo.GetPollingTarget(ctx, id)
	if err != nil {
		return nil, err
	}
	if target == nil {
		return nil, fmt.Errorf("目标不存在: ID=%d", id)
	}

	vm := s.targetToVM(target)

	// 加载关联名称
	if target.CredentialID != nil {
		if cred, _ := s.repo.GetCredential(ctx, *target.CredentialID); cred != nil {
			vm.CredentialName = cred.Name
		}
	}
	if target.TemplateID != nil {
		if template, _ := s.repo.GetPollingTemplate(ctx, *target.TemplateID); template != nil {
			vm.TemplateName = template.Name
		}
	}

	return &vm, nil
}

// CreatePollingTarget 创建轮询目标
func (s *SNMPPollingService) CreatePollingTarget(ctx context.Context, req CreatePollingTargetRequest) error {
	target := &models.SNMPPollingTarget{
		TargetIP:     req.TargetIP,
		TargetPort:   req.TargetPort,
		DisplayName:  req.DisplayName,
		CredentialID: req.CredentialID,
		TemplateID:   req.TemplateID,
		PollInterval: req.PollInterval,
		Enabled:      req.Enabled,
	}

	if target.TargetPort == 0 {
		target.TargetPort = 161
	}
	if target.PollInterval == 0 {
		target.PollInterval = 300 // 默认 5 分钟
	}

	if err := s.repo.CreatePollingTarget(ctx, target); err != nil {
		logger.Error("SNMP-PollingService", "-", "创建目标失败: %v", err)
		return err
	}

	// 如果调度器正在运行且目标已启用，添加到调度
	if s.scheduler != nil && s.scheduler.IsRunning() && target.Enabled {
		var template *models.SNMPPollingTemplate
		var cred *models.SNMPCredential

		if target.TemplateID != nil {
			template, _ = s.repo.GetPollingTemplate(ctx, *target.TemplateID)
		}
		if target.CredentialID != nil {
			cred, _ = s.repo.GetCredential(ctx, *target.CredentialID)
		}

		if err := s.scheduler.AddTarget(target, template, cred); err != nil {
			logger.Warn("SNMP-PollingService", "-", "添加目标到调度失败: %v", err)
		}
	}

	logger.Info("SNMP-PollingService", "-", "目标已创建: ID=%d, IP=%s", target.ID, target.TargetIP)
	return nil
}

// UpdatePollingTarget 更新轮询目标
func (s *SNMPPollingService) UpdatePollingTarget(ctx context.Context, id uint, req UpdatePollingTargetRequest) error {
	target, err := s.repo.GetPollingTarget(ctx, id)
	if err != nil {
		return err
	}
	if target == nil {
		return fmt.Errorf("目标不存在: ID=%d", id)
	}

	target.TargetIP = req.TargetIP
	target.TargetPort = req.TargetPort
	target.DisplayName = req.DisplayName
	target.CredentialID = req.CredentialID
	target.TemplateID = req.TemplateID
	target.PollInterval = req.PollInterval
	target.Enabled = req.Enabled

	if target.TargetPort == 0 {
		target.TargetPort = 161
	}
	if target.PollInterval == 0 {
		target.PollInterval = 300
	}

	if err := s.repo.UpdatePollingTarget(ctx, target); err != nil {
		logger.Error("SNMP-PollingService", "-", "更新目标失败: %v", err)
		return err
	}

	// 更新调度
	if s.scheduler != nil {
		var template *models.SNMPPollingTemplate
		var cred *models.SNMPCredential

		if target.TemplateID != nil {
			template, _ = s.repo.GetPollingTemplate(ctx, *target.TemplateID)
		}
		if target.CredentialID != nil {
			cred, _ = s.repo.GetCredential(ctx, *target.CredentialID)
		}

		if target.Enabled {
			if err := s.scheduler.UpdateTarget(target, template, cred); err != nil {
				logger.Warn("SNMP-PollingService", "-", "更新调度目标失败: %v", err)
			}
		} else {
			if err := s.scheduler.RemoveTarget(target.ID); err != nil {
				logger.Warn("SNMP-PollingService", "-", "移除调度目标失败: %v", err)
			}
		}
	}

	logger.Info("SNMP-PollingService", "-", "目标已更新: ID=%d, IP=%s", target.ID, target.TargetIP)
	return nil
}

// DeletePollingTarget 删除轮询目标
func (s *SNMPPollingService) DeletePollingTarget(ctx context.Context, id uint) error {
	// 从调度移除
	if s.scheduler != nil {
		if err := s.scheduler.RemoveTarget(id); err != nil {
			logger.Warn("SNMP-PollingService", "-", "从调度移除目标失败: %v", err)
		}
	}

	if err := s.repo.DeletePollingTarget(ctx, id); err != nil {
		logger.Error("SNMP-PollingService", "-", "删除目标失败: %v", err)
		return err
	}

	logger.Info("SNMP-PollingService", "-", "目标已删除: ID=%d", id)
	return nil
}

// EnablePollingTarget 启用轮询目标
func (s *SNMPPollingService) EnablePollingTarget(ctx context.Context, id uint) error {
	target, err := s.repo.GetPollingTarget(ctx, id)
	if err != nil {
		return err
	}
	if target == nil {
		return fmt.Errorf("目标不存在: ID=%d", id)
	}

	target.Enabled = true

	if err := s.repo.UpdatePollingTarget(ctx, target); err != nil {
		return err
	}

	// 添加到调度
	if s.scheduler != nil && s.scheduler.IsRunning() {
		var template *models.SNMPPollingTemplate
		var cred *models.SNMPCredential

		if target.TemplateID != nil {
			template, _ = s.repo.GetPollingTemplate(ctx, *target.TemplateID)
		}
		if target.CredentialID != nil {
			cred, _ = s.repo.GetCredential(ctx, *target.CredentialID)
		}

		if err := s.scheduler.AddTarget(target, template, cred); err != nil {
			logger.Warn("SNMP-PollingService", "-", "添加目标到调度失败: %v", err)
		}
	}

	logger.Info("SNMP-PollingService", "-", "目标已启用: ID=%d", id)
	return nil
}

// DisablePollingTarget 禁用轮询目标
func (s *SNMPPollingService) DisablePollingTarget(ctx context.Context, id uint) error {
	target, err := s.repo.GetPollingTarget(ctx, id)
	if err != nil {
		return err
	}
	if target == nil {
		return fmt.Errorf("目标不存在: ID=%d", id)
	}

	target.Enabled = false

	if err := s.repo.UpdatePollingTarget(ctx, target); err != nil {
		return err
	}

	// 从调度移除
	if s.scheduler != nil {
		if err := s.scheduler.RemoveTarget(id); err != nil {
			logger.Warn("SNMP-PollingService", "-", "从调度移除目标失败: %v", err)
		}
	}

	logger.Info("SNMP-PollingService", "-", "目标已禁用: ID=%d", id)
	return nil
}

// ============================================================================
// 轮询操作
// ============================================================================

// PollNow 立即执行一次轮询
func (s *SNMPPollingService) PollNow(ctx context.Context, targetID uint) ([]PollingResultVM, error) {
	if s.scheduler == nil {
		return nil, fmt.Errorf("调度器未初始化")
	}

	results, err := s.scheduler.RunNow(ctx, targetID)
	if err != nil {
		logger.Error("SNMP-PollingService", "-", "立即轮询失败: ID=%d, %v", targetID, err)
		return nil, err
	}

	vms := make([]PollingResultVM, len(results))
	for i, result := range results {
		vms[i] = s.resultToVM(result)
	}

	return vms, nil
}

// PollAllNow 立即执行所有目标轮询
func (s *SNMPPollingService) PollAllNow(ctx context.Context) (int, error) {
	if s.scheduler == nil {
		return 0, fmt.Errorf("调度器未初始化")
	}

	allResults := s.scheduler.RunAllNow(ctx)
	successCount := 0

	for _, results := range allResults {
		if results != nil && len(results) > 0 {
			successCount++
		}
	}

	logger.Info("SNMP-PollingService", "-", "全部轮询完成: 成功=%d", successCount)
	return successCount, nil
}

// ============================================================================
// 轮询结果管理
// ============================================================================

// GetPollingResults 获取轮询结果列表
func (s *SNMPPollingService) GetPollingResults(ctx context.Context, filter PollingResultFilterVM, page, pageSize int) (*PollingResultListVM, error) {
	repoFilter := repository.PollingResultFilter{
		TargetID: filter.TargetID,
		OID:      filter.OID,
		BatchID:  filter.BatchID,
	}

	if filter.StartTime != "" {
		if t, err := time.Parse(time.RFC3339, filter.StartTime); err == nil {
			repoFilter.StartTime = &t
		}
	}
	if filter.EndTime != "" {
		if t, err := time.Parse(time.RFC3339, filter.EndTime); err == nil {
			repoFilter.EndTime = &t
		}
	}

	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	results, total, err := s.repo.ListPollingResults(ctx, repoFilter, page, pageSize)
	if err != nil {
		logger.Error("SNMP-PollingService", "-", "获取结果列表失败: %v", err)
		return nil, err
	}

	vms := make([]PollingResultVM, len(results))
	for i, result := range results {
		vms[i] = s.resultToVM(result)
	}

	totalPages := int(total) / pageSize
	if int(total)%pageSize > 0 {
		totalPages++
	}

	return &PollingResultListVM{
		Data:       vms,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}, nil
}

// ClearPollingResults 清理过期轮询结果
func (s *SNMPPollingService) ClearPollingResults(ctx context.Context, before string) (int64, error) {
	beforeTime, err := time.Parse(time.RFC3339, before)
	if err != nil {
		return 0, fmt.Errorf("时间格式无效: %w", err)
	}

	count, err := s.repo.DeletePollingResultsBefore(ctx, beforeTime)
	if err != nil {
		logger.Error("SNMP-PollingService", "-", "清理结果失败: %v", err)
		return 0, err
	}

	logger.Info("SNMP-PollingService", "-", "已清理 %d 条过期结果", count)
	return count, nil
}

// GetPollingStats 获取轮询统计
func (s *SNMPPollingService) GetPollingStats(ctx context.Context, targetID uint) (*PollingStatsVM, error) {
	stats, err := s.repo.GetPollingStats(ctx, targetID)
	if err != nil {
		logger.Error("SNMP-PollingService", "-", "获取统计失败: %v", err)
		return nil, err
	}

	vm := &PollingStatsVM{
		TotalPolls:   stats.TotalPolls,
		SuccessCount: stats.SuccessCount,
		FailCount:    stats.FailCount,
		AvgLatencyMs: stats.AvgLatencyMs,
	}

	if stats.LastPollTime != nil {
		vm.LastPollTime = stats.LastPollTime.Format(time.RFC3339)
	}

	return vm, nil
}

// ============================================================================
// 转换方法
// ============================================================================

// credentialToVM 凭据模型转 View Model
// P2-15: 先解密 community 再进行脱敏处理
func (s *SNMPPollingService) credentialToVM(cred *models.SNMPCredential) CredentialVM {
	vm := CredentialVM{
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
		UpdatedAt:       cred.UpdatedAt.Format(time.RFC3339),
	}

	// P2-15: 脱敏处理 community（v1/v2c）- 先解密再脱敏
	if cred.Community != "" {
		// 尝试解密（如果是加密存储的）
		community := cred.Community
		if s.crypto != nil && snmp.IsEncrypted(cred.Community) {
			decrypted, err := s.crypto.DecryptCredential(cred.Community)
			if err == nil {
				community = decrypted
			} else {
				// 解密失败，使用占位符
				vm.Community = "******"
				// v3 密钥状态（不返回实际密钥，只返回是否已设置）
				vm.HasAuthKey = cred.AuthPassword != ""
				vm.HasPrivKey = cred.PrivPassword != ""
				return vm
			}
		}

		// 对解密后的值进行脱敏
		vm.Community = maskString(community)
	}

	// v3 密钥状态（不返回实际密钥，只返回是否已设置）
	vm.HasAuthKey = cred.AuthPassword != ""
	vm.HasPrivKey = cred.PrivPassword != ""

	return vm
}

// maskString 通用脱敏函数
// 统一返回 6 个 * 占位符，不暴露密码的实际长度
func maskString(s string) string {
	if s == "" {
		return ""
	}
	return "******"
}

// templateToVM 模板模型转 View Model
func (s *SNMPPollingService) templateToVM(template *models.SNMPPollingTemplate) PollingTemplateVM {
	vm := PollingTemplateVM{
		ID:          template.ID,
		Name:        template.Name,
		Description: template.Description,
		Category:    template.Category,
		CreatedAt:   template.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   template.UpdatedAt.Format(time.RFC3339),
	}

	vm.OIDItems = make([]OIDItemVM, len(template.OIDItems))
	for i, item := range template.OIDItems {
		vm.OIDItems[i] = OIDItemVM{
			OID:         item.OID,
			Name:        item.Name,
			Type:        item.Type,
			Operation:   item.Operation,
			Description: item.Description,
		}
	}

	return vm
}

// targetToVM 目标模型转 View Model
func (s *SNMPPollingService) targetToVM(target *models.SNMPPollingTarget) PollingTargetVM {
	vm := PollingTargetVM{
		ID:             target.ID,
		TargetIP:       target.TargetIP,
		TargetPort:     target.TargetPort,
		DisplayName:    target.DisplayName,
		CredentialID:   target.CredentialID,
		TemplateID:     target.TemplateID,
		PollInterval:   target.PollInterval,
		Enabled:        target.Enabled,
		LastPollStatus: target.LastPollStatus,
		LastPollError:  target.LastPollError,
		CreatedAt:      target.CreatedAt.Format(time.RFC3339),
		UpdatedAt:      target.UpdatedAt.Format(time.RFC3339),
	}

	if target.LastPollAt != nil {
		vm.LastPollAt = target.LastPollAt.Format(time.RFC3339)
	}

	return vm
}

// resultToVM 结果模型转 View Model
func (s *SNMPPollingService) resultToVM(result *models.SNMPPollingResult) PollingResultVM {
	return PollingResultVM{
		ID:        result.ID,
		TargetID:  result.TargetID,
		TargetIP:  result.TargetIP,
		BatchID:   result.BatchID,
		OID:       result.OID,
		OIDName:   result.OIDName,
		Value:     result.Value,
		ValueType: result.ValueType,
		PollTime:  result.PollTime.Format(time.RFC3339),
		CreatedAt: result.CreatedAt.Format(time.RFC3339),
	}
}

// ============================================================================
// 历史趋势图相关 View Models
// ============================================================================

// PollingHistoryVM 轮询历史数据（用于趋势图）
type PollingHistoryVM struct {
	TargetID   uint               `json:"targetId"`
	TargetName string             `json:"targetName"`
	DataPoints []PollingDataPoint `json:"dataPoints"`
}

// PollingDataPoint 轮询数据点
type PollingDataPoint struct {
	Timestamp string            `json:"timestamp"`
	Success   bool              `json:"success"`
	LatencyMs float64           `json:"latencyMs"`
	Values    map[string]string `json:"values"` // OID -> Value
}

// PollingTrendVM 轮询趋势数据（聚合）
type PollingTrendVM struct {
	OID        string           `json:"oid"`
	OIDName    string           `json:"oidName"`
	DataPoints []TrendDataPoint `json:"dataPoints"`
}

// TrendDataPoint 趋势数据点
type TrendDataPoint struct {
	Timestamp string `json:"timestamp"`
	Value     string `json:"value"`
	Numeric   bool   `json:"numeric"` // 是否可转换为数值
}

// ============================================================================
// 历史数据查询方法
// ============================================================================

// GetPollingHistory 获取轮询历史数据（用于趋势图）
// duration 参数: "1h", "6h", "24h", "7d"
// limit 参数: 最大返回条数，默认1000，最大5000
// offset 参数: 分页偏移量，默认0
func (s *SNMPPollingService) GetPollingHistory(ctx context.Context, targetID uint, duration string, limit int, offset int) (*PollingHistoryVM, error) {
	logger.Info("SNMP-PollingService", "-", "获取轮询历史: targetID=%d, duration=%s, limit=%d, offset=%d", targetID, duration, limit, offset)

	// 获取目标信息
	target, err := s.repo.GetPollingTarget(ctx, targetID)
	if err != nil {
		logger.Error("SNMP-PollingService", "-", "获取目标失败: %v", err)
		return nil, fmt.Errorf("获取目标失败: %w", err)
	}

	// 计算时间范围
	startTime := parseDuration(duration)
	if startTime.IsZero() {
		startTime = time.Now().Add(-24 * time.Hour) // 默认 24 小时
	}

	// 设置默认分页参数
	if limit <= 0 {
		limit = 1000
	}
	if limit > 5000 {
		limit = 5000 // 最大限制5000条
	}
	if offset < 0 {
		offset = 0
	}

	// 构建查询过滤器
	filter := repository.PollingResultFilter{
		TargetID:  &targetID,
		StartTime: &startTime,
	}

	// 查询结果（带分页）
	results, _, err := s.repo.ListPollingResults(ctx, filter, 1, limit)
	if err != nil {
		logger.Error("SNMP-PollingService", "-", "查询轮询结果失败: %v", err)
		return nil, fmt.Errorf("查询轮询结果失败: %w", err)
	}

	// 按批次聚合数据点
	batchMap := make(map[string][]*models.SNMPPollingResult)
	for _, result := range results {
		batchMap[result.BatchID] = append(batchMap[result.BatchID], result)
	}

	// 构建数据点列表
	dataPoints := make([]PollingDataPoint, 0, len(batchMap))
	for _, batchResults := range batchMap {
		if len(batchResults) == 0 {
			continue
		}

		// 使用第一个结果的时间作为数据点时间
		firstResult := batchResults[0]
		values := make(map[string]string)
		for _, r := range batchResults {
			values[r.OID] = r.Value
		}

		// 计算延迟（从 OID 名称推断或使用默认值）
		// TODO: 后续可扩展存储实际延迟数据
		dataPoint := PollingDataPoint{
			Timestamp: firstResult.PollTime.Format(time.RFC3339),
			Success:   true, // 存储的结果默认为成功
			LatencyMs: 0,    // 暂无延迟数据
			Values:    values,
		}
		dataPoints = append(dataPoints, dataPoint)
	}

	// 按时间排序（降序）
	sortDataPointsByTime(dataPoints)

	vm := &PollingHistoryVM{
		TargetID:   targetID,
		TargetName: target.DisplayName,
		DataPoints: dataPoints,
	}

	return vm, nil
}

// GetPollingTrend 获取轮询趋势数据（单个 OID 聚合）
func (s *SNMPPollingService) GetPollingTrend(ctx context.Context, targetID uint, oid string, duration string) (*PollingTrendVM, error) {
	logger.Info("SNMP-PollingService", "-", "获取轮询趋势: targetID=%d, oid=%s, duration=%s", targetID, oid, duration)

	// 计算时间范围
	startTime := parseDuration(duration)
	if startTime.IsZero() {
		startTime = time.Now().Add(-24 * time.Hour)
	}

	// 构建查询过滤器
	filter := repository.PollingResultFilter{
		TargetID:  &targetID,
		OID:       oid,
		StartTime: &startTime,
	}

	// 查询结果
	results, _, err := s.repo.ListPollingResults(ctx, filter, 1, 10000)
	if err != nil {
		logger.Error("SNMP-PollingService", "-", "查询轮询结果失败: %v", err)
		return nil, fmt.Errorf("查询轮询结果失败: %w", err)
	}

	// 构建趋势数据点
	dataPoints := make([]TrendDataPoint, 0, len(results))
	var oidName string
	for _, result := range results {
		if oidName == "" && result.OIDName != "" {
			oidName = result.OIDName
		}

		dataPoint := TrendDataPoint{
			Timestamp: result.PollTime.Format(time.RFC3339),
			Value:     result.Value,
			Numeric:   isNumericValue(result.Value),
		}
		dataPoints = append(dataPoints, dataPoint)
	}

	// 按时间排序（升序，用于趋势图）
	sortTrendDataPointsByTime(dataPoints)

	vm := &PollingTrendVM{
		OID:        oid,
		OIDName:    oidName,
		DataPoints: dataPoints,
	}

	return vm, nil
}

// ============================================================================
// 辅助函数
// ============================================================================

// parseDuration 解析时间范围字符串
func parseDuration(duration string) time.Time {
	now := time.Now()
	switch duration {
	case "1h":
		return now.Add(-1 * time.Hour)
	case "6h":
		return now.Add(-6 * time.Hour)
	case "24h":
		return now.Add(-24 * time.Hour)
	case "7d":
		return now.AddDate(0, 0, -7)
	case "30d":
		return now.AddDate(0, 0, -30)
	default:
		return now.Add(-24 * time.Hour) // 默认 24 小时
	}
}

// sortDataPointsByTime 按时间排序数据点（降序）
func sortDataPointsByTime(points []PollingDataPoint) {
	sort.Slice(points, func(i, j int) bool {
		return points[i].Timestamp > points[j].Timestamp // 降序
	})
}

// sortTrendDataPointsByTime 按时间排序趋势数据点（升序）
func sortTrendDataPointsByTime(points []TrendDataPoint) {
	sort.Slice(points, func(i, j int) bool {
		return points[i].Timestamp < points[j].Timestamp // 升序
	})
}

// isNumericValue 判断值是否为数值类型
func isNumericValue(value string) bool {
	if value == "" {
		return false
	}
	// 尝试解析为浮点数
	for _, c := range value {
		if (c >= '0' && c <= '9') || c == '.' || c == '-' || c == '+' {
			continue
		}
		return false
	}
	return true
}
