package ui

import (
	"errors"

	"github.com/NetWeaverGo/core/internal/smartping"
)

// SmartPingService 智能网络质量与 Ping 诊断前端服务
type SmartPingService struct {
	engine *smartping.Engine
}

// NewSmartPingService 创建智能 Ping 服务实例
func NewSmartPingService() *SmartPingService {
	return &SmartPingService{
		engine: smartping.GetGlobalEngine(),
	}
}

// BatchAnalyzeItem 批量诊断输入项
type BatchAnalyzeItem struct {
	Metric smartping.PingHostMetric `json:"metric"`
	Scene  string                   `json:"scene"`
}

// AnalyzePing 单次网络质量与 Ping 指标智能诊断
func (s *SmartPingService) AnalyzePing(metric *smartping.PingHostMetric, scene string) (*smartping.AnalysisReport, error) {
	if s.engine == nil {
		return nil, errors.New("智能诊断引擎未初始化")
	}
	if metric == nil {
		return nil, errors.New("探测指标不能为空")
	}
	res := s.engine.Analyze(metric, scene)
	return res, nil
}

// ListRules 获取当前加载的所有智能 Ping 诊断规则
func (s *SmartPingService) ListRules() []smartping.SmartPingRule {
	if s.engine == nil {
		return nil
	}
	rules := s.engine.GetRules()
	res := make([]smartping.SmartPingRule, len(rules))
	for i, r := range rules {
		if r != nil {
			res[i] = *r
		}
	}
	return res
}

// BatchAnalyze 批量网络质量诊断
func (s *SmartPingService) BatchAnalyze(items []BatchAnalyzeItem) ([]smartping.AnalysisReport, error) {
	if s.engine == nil {
		return nil, errors.New("智能诊断引擎未初始化")
	}
	results := make([]smartping.AnalysisReport, 0, len(items))
	for _, item := range items {
		m := item.Metric
		report := s.engine.Analyze(&m, item.Scene)
		if report != nil {
			results = append(results, *report)
		}
	}
	return results, nil
}
