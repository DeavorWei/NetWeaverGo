// Package ui 提供 Wails 暴露层服务
// snmp_trap_service.go 实现 Trap 管理 Wails 绑定服务
package ui

import (
	"context"
	"fmt"
	"time"

	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/models"
	"github.com/NetWeaverGo/core/internal/repository"
	"github.com/NetWeaverGo/core/internal/snmp"
)

// ============================================================================
// SNMPTrapService - Trap 管理服务 (Wails 绑定)
// ============================================================================

// SNMPTrapService Trap 管理服务
// 暴露 Trap 监听器管理、Trap 记录管理、过滤规则管理、服务器配置管理给前端
type SNMPTrapService struct {
	trapRepo     repository.TrapRepository
	listener     *snmp.TrapListener
	handler      *snmp.TrapHandler
	filterEngine *snmp.TrapFilterEngine
	oidResolver  *snmp.OIDResolver
	notifier     *SNMPEventNotifier
}

// NewSNMPTrapService 创建服务实例
func NewSNMPTrapService(
	trapRepo repository.TrapRepository,
	listener *snmp.TrapListener,
	handler *snmp.TrapHandler,
	filterEngine *snmp.TrapFilterEngine,
	oidResolver *snmp.OIDResolver,
	notifier *SNMPEventNotifier,
) *SNMPTrapService {
	service := &SNMPTrapService{
		trapRepo:     trapRepo,
		listener:     listener,
		handler:      handler,
		filterEngine: filterEngine,
		oidResolver:  oidResolver,
		notifier:     notifier,
	}

	logger.Info("SNMP-TrapService", "-", "Trap 管理服务已初始化")
	return service
}

// ============================================================================
// 监听器管理
// ============================================================================

// StartListener 启动 Trap 监听器
func (s *SNMPTrapService) StartListener(ctx context.Context, config ServerConfigVM) error {
	// L3: 验证端口范围
	if config.TrapPort < 1 || config.TrapPort > 65535 {
		return fmt.Errorf("trap 监听端口无效: %d, 有效范围为 1-65535", config.TrapPort)
	}

	logger.Info("SNMP-TrapService", "-", "启动 Trap 监听器请求: 端口=%d, 绑定地址=%s", config.TrapPort, config.BindAddress)

	// 转换 View Model 到模型
	bindAddress := config.BindAddress
	if bindAddress == "" {
		bindAddress = "0.0.0.0"
	}
	modelConfig := &models.SNMPServerConfig{
		TrapEnabled:      true,
		TrapPort:         config.TrapPort,
		BindAddress:      bindAddress,
		TrapCommunity:    config.TrapCommunity,
		MaxStorageDays:   config.MaxStorageDays,
	}

	if s.listener == nil {
		return fmt.Errorf("监听器未初始化")
	}

	// 如果监听器已在运行，先停止再更新配置
	if s.listener.IsRunning() {
		logger.Info("SNMP-TrapService", "-", "监听器正在运行，先停止再重启")
		if err := s.listener.Stop(); err != nil {
			logger.Error("SNMP-TrapService", "-", "停止监听器失败: %v", err)
			return err
		}
	}

	// 更新监听器配置（此时监听器已停止，UpdateConfig 不会自动启动）
	s.listener.UpdateConfig(modelConfig)

	// 启动监听器
	return s.listener.Start()
}

// StopListener 停止 Trap 监听器
func (s *SNMPTrapService) StopListener(ctx context.Context) error {
	logger.Info("SNMP-TrapService", "-", "停止 Trap 监听器请求")

	if s.listener != nil {
		return s.listener.Stop()
	}

	return nil
}

// GetListenerStatus 获取监听器状态
func (s *SNMPTrapService) GetListenerStatus(ctx context.Context) (*ListenerStatusVM, error) {
	if s.listener == nil {
		return &ListenerStatusVM{
			IsRunning: false,
		}, nil
	}

	stats := s.listener.GetStats()
	handlerStats := s.handler.GetStats()

	return &ListenerStatusVM{
		IsRunning:    stats.IsRunning,
		ListenAddr:   stats.ListenAddr,
		TotalTraps:   stats.TotalTraps,
		FilteredOut:  stats.FilteredOut,
		LastTrapTime: stats.LastTrapTime.Format("2006-01-02 15:04:05"),
		StartTime:    stats.StartTime.Format("2006-01-02 15:04:05"),
		HandlerStats: HandlerStatsVM{
			TotalReceived: handlerStats.TotalReceived,
			TotalStored:   handlerStats.TotalStored,
			TotalFiltered: handlerStats.TotalFiltered,
			TotalErrors:   handlerStats.TotalErrors,
		},
	}, nil
}

// ============================================================================
// Trap 记录管理
// ============================================================================

// GetTrapRecords 获取 Trap 记录列表
func (s *SNMPTrapService) GetTrapRecords(ctx context.Context, filter TrapFilterVM, page, pageSize int) (*TrapRecordListVM, error) {
	// 转换 View Model 到 Repository 过滤条件
	repoFilter := repository.TrapFilter{
		SourceIP:     filter.SourceIP,
		TrapOID:      filter.TrapOID,
		Severity:     filter.Severity,
		SearchQuery:  filter.SearchQuery,
		Acknowledged: filter.Acknowledged,
	}

	if filter.StartTime != "" {
		t, err := time.Parse("2006-01-02 15:04:05", filter.StartTime)
		if err == nil {
			repoFilter.StartTime = &t
		}
	}
	if filter.EndTime != "" {
		t, err := time.Parse("2006-01-02 15:04:05", filter.EndTime)
		if err == nil {
			repoFilter.EndTime = &t
		}
	}

	// 查询记录
	traps, total, err := s.trapRepo.ListTraps(ctx, repoFilter, page, pageSize)
	if err != nil {
		logger.Error("SNMP-TrapService", "-", "查询 Trap 记录失败: %v", err)
		return nil, err
	}

	// 转换为 View Model
	var trapVMs []TrapRecordVM
	for _, trap := range traps {
		trapVMs = append(trapVMs, s.convertTrapToVM(trap))
	}

	// 防止除零 panic，设置默认 pageSize
	if pageSize <= 0 {
		pageSize = 20 // 默认每页20条
	}

	totalPages := int(total) / pageSize
	if int(total)%pageSize > 0 {
		totalPages++
	}

	return &TrapRecordListVM{
		Data:       trapVMs,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}, nil
}

// GetTrapRecord 获取单个 Trap 记录详情
func (s *SNMPTrapService) GetTrapRecord(ctx context.Context, id uint) (*TrapRecordVM, error) {
	trap, err := s.trapRepo.GetTrap(ctx, id)
	if err != nil {
		return nil, err
	}
	if trap == nil {
		return nil, fmt.Errorf("Trap 记录不存在: ID=%d", id)
	}

	// 转换为 View Model
	vm := s.convertTrapToVM(trap)
	return &vm, nil
}

// DeleteTrapRecord 删除单个 Trap 记录
func (s *SNMPTrapService) DeleteTrapRecord(ctx context.Context, id uint) error {
	return s.trapRepo.DeleteTrap(ctx, id)
}

// ClearTrapRecords 清理过期 Trap 记录
func (s *SNMPTrapService) ClearTrapRecords(ctx context.Context, before string) (int64, error) {
	beforeTime, err := time.Parse("2006-01-02", before)
	if err != nil {
		return 0, fmt.Errorf("时间格式错误: %v", err)
	}

	return s.trapRepo.DeleteTrapsBefore(ctx, beforeTime)
}

// AcknowledgeTrap 确认单个 Trap 记录
func (s *SNMPTrapService) AcknowledgeTrap(ctx context.Context, id uint) error {
	return s.trapRepo.AcknowledgeTrap(ctx, id)
}

// BatchAcknowledgeTraps 批量确认 Trap 记录
func (s *SNMPTrapService) BatchAcknowledgeTraps(ctx context.Context, ids []uint) error {
	return s.trapRepo.BatchAcknowledgeTraps(ctx, ids)
}

// GetTrapStats 获取 Trap 统计信息
func (s *SNMPTrapService) GetTrapStats(ctx context.Context) (*TrapStatsVM, error) {
	stats, err := s.trapRepo.GetTrapStats(ctx)
	if err != nil {
		return nil, err
	}

	return &TrapStatsVM{
		TotalCount:     stats.TotalCount,
		Unacknowledged: stats.Unacknowledged,
		CriticalCount:  stats.CriticalCount,
		MajorCount:     stats.MajorCount,
		MinorCount:     stats.MinorCount,
		InfoCount:      stats.InfoCount,
		TodayCount:     stats.TodayCount,
		LastHourCount:  stats.LastHourCount,
	}, nil
}

// ============================================================================
// 过滤规则管理
// ============================================================================

// GetFilterRules 获取所有过滤规则
func (s *SNMPTrapService) GetFilterRules(ctx context.Context) ([]FilterRuleVM, error) {
	rules, err := s.trapRepo.ListFilterRules(ctx)
	if err != nil {
		return nil, err
	}

	var ruleVMs []FilterRuleVM
	for _, rule := range rules {
		ruleVMs = append(ruleVMs, FilterRuleVM{
			ID:               rule.ID,
			Name:             rule.Name,
			Enabled:          rule.Enabled,
			Priority:         rule.Priority,
			Action:           rule.Action,
			SourceIPPattern:  rule.SourceIPPattern,
			OIDPattern:       rule.OIDPattern,
			CommunityPattern: rule.CommunityPattern,
			OverrideSeverity: rule.OverrideSeverity,
			Description:      rule.Description,
			CreatedAt:        rule.CreatedAt.Format("2006-01-02 15:04:05"),
			UpdatedAt:        rule.UpdatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	return ruleVMs, nil
}

// CreateFilterRule 创建过滤规则
func (s *SNMPTrapService) CreateFilterRule(ctx context.Context, req CreateFilterRuleRequest) error {
	rule := &models.SNMPTrapFilterRule{
		Name:             req.Name,
		Enabled:          req.Enabled,
		Priority:         req.Priority,
		Action:           req.Action,
		SourceIPPattern:  req.SourceIPPattern,
		OIDPattern:       req.OIDPattern,
		CommunityPattern: req.CommunityPattern,
		OverrideSeverity: req.OverrideSeverity,
		Description:      req.Description,
	}

	if err := s.trapRepo.CreateFilterRule(ctx, rule); err != nil {
		return err
	}

	// 更新过滤引擎
	if s.filterEngine != nil {
		s.filterEngine.AddRule(rule)
	}

	return nil
}

// UpdateFilterRule 更新过滤规则
func (s *SNMPTrapService) UpdateFilterRule(ctx context.Context, id uint, req UpdateFilterRuleRequest) error {
	rule, err := s.trapRepo.GetFilterRule(ctx, id)
	if err != nil {
		return err
	}
	if rule == nil {
		return fmt.Errorf("过滤规则不存在: ID=%d", id)
	}

	// 更新字段
	rule.Name = req.Name
	rule.Enabled = req.Enabled
	rule.Priority = req.Priority
	rule.Action = req.Action
	rule.SourceIPPattern = req.SourceIPPattern
	rule.OIDPattern = req.OIDPattern
	rule.CommunityPattern = req.CommunityPattern
	rule.OverrideSeverity = req.OverrideSeverity
	rule.Description = req.Description

	if err := s.trapRepo.UpdateFilterRule(ctx, rule); err != nil {
		return err
	}

	// 更新过滤引擎
	if s.filterEngine != nil {
		s.filterEngine.UpdateSingleRule(rule)
	}

	return nil
}

// DeleteFilterRule 删除过滤规则
func (s *SNMPTrapService) DeleteFilterRule(ctx context.Context, id uint) error {
	if err := s.trapRepo.DeleteFilterRule(ctx, id); err != nil {
		return err
	}

	// 更新过滤引擎
	if s.filterEngine != nil {
		s.filterEngine.RemoveRule(id)
	}

	return nil
}

// ReorderFilterRules 重新排序过滤规则
func (s *SNMPTrapService) ReorderFilterRules(ctx context.Context, ids []uint) error {
	if err := s.trapRepo.ReorderFilterRules(ctx, ids); err != nil {
		return err
	}

	// 更新过滤引擎
	if s.filterEngine != nil {
		rules, err := s.trapRepo.ListFilterRules(ctx)
		if err == nil {
			s.filterEngine.UpdateRules(rules)
		}
	}

	return nil
}

// ============================================================================
// 服务器配置管理
// ============================================================================

// GetServerConfigs 获取服务器配置
func (s *SNMPTrapService) GetServerConfigs(ctx context.Context) ([]ServerConfigVM, error) {
	configs, err := s.trapRepo.ListServerConfigs(ctx)
	if err != nil {
		return nil, err
	}

	// 初始化为空数组，避免 JSON 序列化为 null
	configVMs := make([]ServerConfigVM, 0, len(configs))
	for _, config := range configs {
		configVMs = append(configVMs, ServerConfigVM{
			ID:            config.ID,
			TrapEnabled:   config.TrapEnabled,
			TrapPort:      config.TrapPort,
			BindAddress:   config.BindAddress,
			TrapCommunity: config.TrapCommunity,
			MaxStorageDays: config.MaxStorageDays,
		})
	}

	return configVMs, nil
}

// GetActiveServerConfig 获取活动服务器配置
func (s *SNMPTrapService) GetActiveServerConfig(ctx context.Context) (*ServerConfigVM, error) {
	config, err := s.trapRepo.GetActiveServerConfig(ctx)
	if err != nil {
		return nil, err
	}

	return &ServerConfigVM{
		ID:            config.ID,
		TrapEnabled:   config.TrapEnabled,
		TrapPort:      config.TrapPort,
		BindAddress:   config.BindAddress,
		TrapCommunity: config.TrapCommunity,
		MaxStorageDays: config.MaxStorageDays,
	}, nil
}

// UpdateServerConfig 更新服务器配置
func (s *SNMPTrapService) UpdateServerConfig(ctx context.Context, id uint, req UpdateServerConfigRequest) error {
	config, err := s.trapRepo.GetServerConfig(ctx, id)
	if err != nil {
		return err
	}
	if config == nil {
		return fmt.Errorf("服务器配置不存在: ID=%d", id)
	}

	// 更新字段
	config.TrapEnabled = req.TrapEnabled
	config.TrapPort = req.TrapPort
	config.BindAddress = req.BindAddress
	config.TrapCommunity = req.TrapCommunity
	config.MaxStorageDays = req.MaxStorageDays

	if err := s.trapRepo.UpdateServerConfig(ctx, config); err != nil {
		return err
	}

	// 如果监听器正在运行，更新配置
	if s.listener != nil {
		s.listener.UpdateConfig(config)
	}

	return nil
}

// CreateServerConfig 创建服务器配置
func (s *SNMPTrapService) CreateServerConfig(ctx context.Context, req CreateServerConfigRequest) error {
	config := &models.SNMPServerConfig{
		TrapEnabled:    req.TrapEnabled,
		TrapPort:       req.TrapPort,
		BindAddress:    req.BindAddress,
		TrapCommunity:  req.TrapCommunity,
		MaxStorageDays: req.MaxStorageDays,
	}

	if err := s.trapRepo.CreateServerConfig(ctx, config); err != nil {
		return err
	}

	logger.Info("SNMP-TrapService", "-", "创建服务器配置成功: ID=%d", config.ID)
	return nil
}

// ============================================================================
// 辅助方法
// ============================================================================

// convertTrapToVM 转换 TrapRecord 到 View Model
func (s *SNMPTrapService) convertTrapToVM(trap *models.SNMPTrapRecord) TrapRecordVM {
	vm := TrapRecordVM{
		ID:           trap.ID,
		SourceIP:     trap.SourceIP,
		SourcePort:   trap.SourcePort,
		Version:      trap.Version,
		Community:    trap.Community,
		TrapOID:      trap.TrapOID,
		TrapName:     trap.TrapName,
		Enterprise:   trap.Enterprise,
		GenericTrap:  trap.GenericTrap,
		SpecificTrap: trap.SpecificTrap,
		Severity:     trap.Severity,
		Variables:    trap.Variables,
		Acknowledged: trap.Acknowledged,
		ReceivedAt:   trap.ReceivedAt.Format("2006-01-02 15:04:05"),

		// 告警元数据
		TrapAlarmSeverity: trap.TrapAlarmSeverity,
		TrapCategory:      trap.TrapCategory,
		ManagedObject:     trap.ManagedObject,
		AlarmID:           trap.AlarmID,
		TrapEventTime:     trap.TrapEventTime,
		TrapSequenceNum:   trap.TrapSequenceNum,
	}

	if trap.AcknowledgedAt != nil {
		vm.AcknowledgedAt = trap.AcknowledgedAt.Format("2006-01-02 15:04:05")
	}

	return vm
}

// ============================================================================
// View Models
// ============================================================================

// ServerConfigVM 服务器配置 View Model
type ServerConfigVM struct {
	ID             uint   `json:"id"`
	TrapEnabled    bool   `json:"trapEnabled"`
	TrapPort       int    `json:"trapPort"`
	BindAddress    string `json:"bindAddress"`
	TrapCommunity  string `json:"trapCommunity"`
	MaxStorageDays int    `json:"maxStorageDays"`
}

// ListenerStatusVM 监听器状态 View Model
type ListenerStatusVM struct {
	IsRunning    bool          `json:"isRunning"`
	ListenAddr   string        `json:"listenAddr"`
	TotalTraps   int64         `json:"totalTraps"`
	FilteredOut  int64         `json:"filteredOut"`
	LastTrapTime string        `json:"lastTrapTime"`
	StartTime    string        `json:"startTime"`
	HandlerStats HandlerStatsVM `json:"handlerStats"`
}

// HandlerStatsVM 处理统计 View Model
type HandlerStatsVM struct {
	TotalReceived int64 `json:"totalReceived"`
	TotalStored   int64 `json:"totalStored"`
	TotalFiltered int64 `json:"totalFiltered"`
	TotalErrors   int64 `json:"totalErrors"`
}

// TrapRecordVM Trap 记录 View Model
type TrapRecordVM struct {
	ID           uint   `json:"id"`
	SourceIP     string `json:"sourceIP"`
	SourcePort   int    `json:"sourcePort"`
	Version      string `json:"version"`
	Community    string `json:"community"`
	TrapOID      string `json:"trapOID"`
	TrapName     string `json:"trapName"`
	Enterprise   string `json:"enterprise"`
	GenericTrap  int    `json:"genericTrap"`
	SpecificTrap int    `json:"specificTrap"`
	Severity     string `json:"severity"`
	Variables    string `json:"variables"`
	Acknowledged bool   `json:"acknowledged"`
	AcknowledgedAt string `json:"acknowledgedAt"`
	ReceivedAt   string `json:"receivedAt"`

	// 告警元数据
	TrapAlarmSeverity  int    `json:"trapAlarmSeverity"`
	TrapCategory       int    `json:"trapCategory"`
	ManagedObject      string `json:"managedObject"`
	AlarmID            string `json:"alarmId"`
	TrapEventTime      string `json:"trapEventTime"`
	TrapSequenceNum    int    `json:"trapSequenceNum"`
}

// TrapRecordListVM Trap 记录列表 View Model
type TrapRecordListVM struct {
	Data       []TrapRecordVM `json:"data"`
	Total      int64          `json:"total"`
	Page       int            `json:"page"`
	PageSize   int            `json:"pageSize"`
	TotalPages int            `json:"totalPages"`
}

// TrapFilterVM Trap 过滤条件 View Model
type TrapFilterVM struct {
	SourceIP     string `json:"sourceIP"`
	TrapOID      string `json:"trapOID"`
	Severity     string `json:"severity"`
	StartTime    string `json:"startTime"`
	EndTime      string `json:"endTime"`
	Acknowledged *bool  `json:"acknowledged"`
	SearchQuery  string `json:"searchQuery"`
}

// TrapStatsVM Trap 统计 View Model
type TrapStatsVM struct {
	TotalCount     int64 `json:"totalCount"`
	Unacknowledged int64 `json:"unacknowledged"`
	CriticalCount  int64 `json:"criticalCount"`
	MajorCount     int64 `json:"majorCount"`
	MinorCount     int64 `json:"minorCount"`
	InfoCount      int64 `json:"infoCount"`
	TodayCount     int64 `json:"todayCount"`
	LastHourCount  int64 `json:"lastHourCount"`
}

// FilterRuleVM 过滤规则 View Model
type FilterRuleVM struct {
	ID               uint   `json:"id"`
	Name             string `json:"name"`
	Enabled          bool   `json:"enabled"`
	Priority         int    `json:"priority"`
	Action           string `json:"action"` // accept/drop/severity_override
	SourceIPPattern  string `json:"sourceIPPattern"`
	OIDPattern       string `json:"oidPattern"`
	CommunityPattern string `json:"communityPattern"`
	OverrideSeverity string `json:"overrideSeverity"`
	Description      string `json:"description"`
	CreatedAt        string `json:"createdAt"`
	UpdatedAt        string `json:"updatedAt"`
}

// CreateFilterRuleRequest 创建过滤规则请求
type CreateFilterRuleRequest struct {
	Name             string `json:"name"`
	Enabled          bool   `json:"enabled"`
	Priority         int    `json:"priority"`
	Action           string `json:"action"`
	SourceIPPattern  string `json:"sourceIPPattern"`
	OIDPattern       string `json:"oidPattern"`
	CommunityPattern string `json:"communityPattern"`
	OverrideSeverity string `json:"overrideSeverity"`
	Description      string `json:"description"`
}

// UpdateFilterRuleRequest 更新过滤规则请求
type UpdateFilterRuleRequest struct {
	Name             string `json:"name"`
	Enabled          bool   `json:"enabled"`
	Priority         int    `json:"priority"`
	Action           string `json:"action"`
	SourceIPPattern  string `json:"sourceIPPattern"`
	OIDPattern       string `json:"oidPattern"`
	CommunityPattern string `json:"communityPattern"`
	OverrideSeverity string `json:"overrideSeverity"`
	Description      string `json:"description"`
}

// CreateServerConfigRequest 创建服务器配置请求
type CreateServerConfigRequest struct {
	TrapEnabled    bool   `json:"trapEnabled"`
	TrapPort       int    `json:"trapPort"`
	BindAddress    string `json:"bindAddress"`
	TrapCommunity  string `json:"trapCommunity"`
	MaxStorageDays int    `json:"maxStorageDays"`
}

// UpdateServerConfigRequest 更新服务器配置请求
type UpdateServerConfigRequest struct {
	TrapEnabled    bool   `json:"trapEnabled"`
	TrapPort       int    `json:"trapPort"`
	BindAddress    string `json:"bindAddress"`
	TrapCommunity  string `json:"trapCommunity"`
	MaxStorageDays int    `json:"maxStorageDays"`
}

// ============================================================================
// v3 用户管理 View Models
// ============================================================================

// V3UserVM v3 用户 View Model
type V3UserVM struct {
	Username      string `json:"username"`
	AuthProtocol  string `json:"authProtocol"`
	PrivProtocol  string `json:"privProtocol"`
	SecurityLevel string `json:"securityLevel"`
}

// AddV3UserRequest 添加 v3 用户请求
type AddV3UserRequest struct {
	Username      string `json:"username"`
	AuthProtocol  string `json:"authProtocol"`
	AuthKey       string `json:"authKey"`
	PrivProtocol  string `json:"privProtocol"`
	PrivKey       string `json:"privKey"`
	SecurityLevel string `json:"securityLevel"`
}

// ============================================================================
// v3 用户管理方法
// ============================================================================

// AddV3User 添加 v3 用户配置
func (s *SNMPTrapService) AddV3User(ctx context.Context, req AddV3UserRequest) error {
	if s.listener == nil {
		return fmt.Errorf("监听器未初始化")
	}

	config := &snmp.V3UserConfig{
		Username:      req.Username,
		AuthProtocol:  req.AuthProtocol,
		AuthKey:       req.AuthKey,
		PrivProtocol:  req.PrivProtocol,
		PrivKey:       req.PrivKey,
		SecurityLevel: req.SecurityLevel,
	}

	if err := s.listener.AddV3User(config); err != nil {
		logger.Error("SNMP-TrapService", "-", "添加 v3 用户失败: %v", err)
		return err
	}

	logger.Info("SNMP-TrapService", "-", "v3 用户已添加: %s", req.Username)
	return nil
}

// RemoveV3User 移除 v3 用户配置
func (s *SNMPTrapService) RemoveV3User(ctx context.Context, username string) error {
	if s.listener == nil {
		return fmt.Errorf("监听器未初始化")
	}

	if err := s.listener.RemoveV3User(username); err != nil {
		logger.Error("SNMP-TrapService", "-", "移除 v3 用户失败: %v", err)
		return err
	}

	logger.Info("SNMP-TrapService", "-", "v3 用户已移除: %s", username)
	return nil
}

// ListV3Users 获取所有 v3 用户配置
func (s *SNMPTrapService) ListV3Users(ctx context.Context) ([]V3UserVM, error) {
	if s.listener == nil {
		return nil, fmt.Errorf("监听器未初始化")
	}

	users := s.listener.GetV3Users()
	vms := make([]V3UserVM, len(users))
	for i, user := range users {
		vms[i] = V3UserVM{
			Username:      user.Username,
			AuthProtocol:  user.AuthProtocol,
			PrivProtocol:  user.PrivProtocol,
			SecurityLevel: user.SecurityLevel,
		}
	}

	return vms, nil
}

// ============================================================================
// 清理配置管理 View Models
// ============================================================================

// CleanupConfigVM 清理配置 View Model
type CleanupConfigVM struct {
	TrapRetentionDays       int  `json:"trapRetentionDays"`       // Trap 保留天数
	PollResultRetentionDays int  `json:"pollResultRetentionDays"` // 轮询结果保留天数
	CleanupIntervalHours    int  `json:"cleanupIntervalHours"`    // 清理间隔（小时）
	Enabled                 bool `json:"enabled"`                 // 是否启用自动清理
}

// CleanupResultVM 清理执行结果 View Model
type CleanupResultVM struct {
	TrapDeleted       int64  `json:"trapDeleted"`       // 删除的 Trap 记录数
	PollResultDeleted int64  `json:"pollResultDeleted"` // 删除的轮询结果数
	ExecutedAt        string `json:"executedAt"`       // 执行时间
	DurationMs        int64  `json:"durationMs"`        // 执行耗时（毫秒）
}

// ============================================================================
// 清理配置管理方法
// ============================================================================

// GetCleanupConfig 获取清理配置
func (s *SNMPTrapService) GetCleanupConfig(ctx context.Context) (*CleanupConfigVM, error) {
	// 从服务器配置中获取清理相关参数
	config, err := s.trapRepo.GetActiveServerConfig(ctx)
	if err != nil {
		logger.Error("SNMP-TrapService", "-", "获取服务器配置失败: %v", err)
		return nil, fmt.Errorf("获取服务器配置失败: %w", err)
	}

	if config == nil {
		// 返回默认配置
		return &CleanupConfigVM{
			TrapRetentionDays:       30,
			PollResultRetentionDays: 7,
			CleanupIntervalHours:     24,
			Enabled:                  true,
		}, nil
	}

	return &CleanupConfigVM{
		TrapRetentionDays:       config.MaxStorageDays,
		PollResultRetentionDays: config.PollingResultRetentionDays,
		CleanupIntervalHours:     24, // 默认 24 小时
		Enabled:                  true,
	}, nil
}

// UpdateCleanupConfig 更新清理配置
func (s *SNMPTrapService) UpdateCleanupConfig(ctx context.Context, config CleanupConfigVM) error {
	logger.Info("SNMP-TrapService", "-", "更新清理配置: Trap保留=%d天, 轮询保留=%d天",
		config.TrapRetentionDays, config.PollResultRetentionDays)

	// 获取当前活动配置
	activeConfig, err := s.trapRepo.GetActiveServerConfig(ctx)
	if err != nil {
		logger.Error("SNMP-TrapService", "-", "获取服务器配置失败: %v", err)
		return fmt.Errorf("获取服务器配置失败: %w", err)
	}

	if activeConfig == nil {
		// 创建新配置
		activeConfig = &models.SNMPServerConfig{
			TrapEnabled:                true,
			MaxStorageDays:             config.TrapRetentionDays,
			PollingResultRetentionDays: config.PollResultRetentionDays,
		}
		if err := s.trapRepo.CreateServerConfig(ctx, activeConfig); err != nil {
			logger.Error("SNMP-TrapService", "-", "创建服务器配置失败: %v", err)
			return fmt.Errorf("创建服务器配置失败: %w", err)
		}
	} else {
		// 更新现有配置
		activeConfig.MaxStorageDays = config.TrapRetentionDays
		activeConfig.PollingResultRetentionDays = config.PollResultRetentionDays
		if err := s.trapRepo.UpdateServerConfig(ctx, activeConfig); err != nil {
			logger.Error("SNMP-TrapService", "-", "更新服务器配置失败: %v", err)
			return fmt.Errorf("更新服务器配置失败: %w", err)
		}
	}

	logger.Info("SNMP-TrapService", "-", "清理配置已更新")
	return nil
}

// RunCleanupNow 立即执行清理
func (s *SNMPTrapService) RunCleanupNow(ctx context.Context) (*CleanupResultVM, error) {
	logger.Info("SNMP-TrapService", "-", "手动触发数据清理")

	// 获取清理配置
	config, err := s.GetCleanupConfig(ctx)
	if err != nil {
		return nil, err
	}

	startTime := time.Now()

	// 执行 Trap 清理
	var trapDeleted int64
	if config.TrapRetentionDays > 0 {
		trapBefore := time.Now().AddDate(0, 0, -config.TrapRetentionDays)
		trapDeleted, err = s.trapRepo.DeleteTrapsBefore(ctx, trapBefore)
		if err != nil {
			logger.Error("SNMP-TrapService", "-", "清理 Trap 记录失败: %v", err)
			return nil, fmt.Errorf("清理 Trap 记录失败: %w", err)
		}
		logger.Info("SNMP-TrapService", "-", "清理 Trap: 截止时间=%s, 删除数=%d",
			trapBefore.Format("2006-01-02"), trapDeleted)
	}

	// 执行轮询结果清理（需要 pollingRepo）
	// 注意：这里暂时只清理 Trap，轮询结果清理由 PollingService 负责
	// 如果需要统一清理，可以在 main.go 中协调

	duration := time.Since(startTime).Milliseconds()

	result := &CleanupResultVM{
		TrapDeleted:       trapDeleted,
		PollResultDeleted: 0, // 由 PollingService 处理
		ExecutedAt:        startTime.Format("2006-01-02 15:04:05"),
		DurationMs:        duration,
	}

	logger.Info("SNMP-TrapService", "-", "数据清理完成: Trap删除=%d, 耗时=%dms",
		trapDeleted, duration)

	return result, nil
}