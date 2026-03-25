//go:build legacy
// +build legacy

package ui

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/NetWeaverGo/core/internal/config"
	"github.com/NetWeaverGo/core/internal/models"
	"github.com/NetWeaverGo/core/internal/parser"
	"github.com/NetWeaverGo/core/internal/topology"
	"gorm.io/gorm"
)

// TopologyService 拓扑服务
type TopologyService struct {
	db      *gorm.DB
	builder *topology.Builder
}

// NewTopologyService 创建拓扑服务
func NewTopologyService(db *gorm.DB) *TopologyService {
	builder := topology.NewBuilder(db)
	if runtimeManager := config.GetRuntimeManagerIfInitialized(); runtimeManager != nil {
		builder.SetRuntimeProvider(runtimeManager)
	}
	return &TopologyService{
		db:      db,
		builder: builder,
	}
}

// ParseDiscoveryTask 解析发现任务的原始输出
// 增强版：返回解析错误列表，供前端展示
func (s *TopologyService) ParseDiscoveryTask(ctx context.Context, taskID string) ([]parser.ParseErrorDetail, error) {
	// 创建解析服务
	parseService := parser.NewService(s.db)

	// 先执行标准解析
	if err := parseService.ParseTask(taskID); err != nil {
		// 解析任务有错误时，继续获取详细错误列表
		// 不直接返回错误，让前端能看到错误详情
	}

	// 获取详细解析错误列表
	parseErrors := parseService.ParseTaskWithErrors(taskID)

	if len(parseErrors) > 0 {
		// 保存解析错误到任务元数据（可选）
		s.saveParseErrorsToTask(taskID, parseErrors)
		return parseErrors, fmt.Errorf("解析完成，但存在 %d 个错误", len(parseErrors))
	}

	return nil, nil
}

// saveParseErrorsToTask 将解析错误保存到任务元数据
func (s *TopologyService) saveParseErrorsToTask(taskID string, errors []parser.ParseErrorDetail) {
	// 可选：将解析错误序列化后保存到任务记录
	errorJSON, _ := json.Marshal(errors)
	s.db.Model(&models.DiscoveryTask{}).
		Where("id = ?", taskID).
		Update("parse_errors", string(errorJSON))
}

// BuildTopology 构建拓扑图
func (s *TopologyService) BuildTopology(ctx context.Context, taskID string) (*models.TopologyBuildResult, error) {
	// 先解析任务（如果还没解析）
	var errorStrs []string
	parseErrorDetails, err := s.ParseDiscoveryTask(ctx, taskID)
	if err != nil {
		// 解析失败收集错误，继续构建
		errorStrs = append(errorStrs, fmt.Sprintf("解析任务失败: %v", err))
	}

	// 收集详细解析错误
	for _, detail := range parseErrorDetails {
		errorStrs = append(errorStrs, fmt.Sprintf("[%s] %s: %s", detail.DeviceIP, detail.CommandKey, detail.Error))
	}

	// 构建拓扑
	result, err := s.builder.Build(taskID)
	if err != nil {
		return nil, err
	}

	// 将解析错误添加到结果中
	if len(errorStrs) > 0 {
		result.Errors = append(errorStrs, result.Errors...)
	}

	return result, nil
}

// GetTopologyGraph 获取拓扑图视图
func (s *TopologyService) GetTopologyGraph(ctx context.Context, taskID string) (*models.TopologyGraphView, error) {
	return s.builder.BuildGraphView(taskID)
}

// GetEdgeDetail 获取边详情
func (s *TopologyService) GetEdgeDetail(ctx context.Context, taskID string, edgeID string) (*models.TopologyEdgeDetailView, error) {
	return s.builder.GetEdgeDetail(taskID, edgeID)
}

// GetDeviceTopologyDetail 获取设备的拓扑详情
func (s *TopologyService) GetDeviceTopologyDetail(ctx context.Context, taskID string, deviceIP string) (*parser.ParsedResult, error) {
	parseService := parser.NewService(s.db)
	return parseService.GetParsedDeviceDetail(taskID, deviceIP)
}

// ListTopologyEdges 列出拓扑边
func (s *TopologyService) ListTopologyEdges(ctx context.Context, taskID string) ([]models.TopologyEdge, error) {
	var edges []models.TopologyEdge
	if err := s.db.Where("task_id = ?", taskID).Find(&edges).Error; err != nil {
		return nil, err
	}
	return edges, nil
}

// ListTopologyNodes 列出拓扑节点
func (s *TopologyService) ListTopologyNodes(ctx context.Context, taskID string) ([]models.GraphNode, error) {
	graph, err := s.builder.BuildGraphView(taskID)
	if err != nil {
		return nil, err
	}
	return graph.Nodes, nil
}

// GetParseErrors 获取任务解析错误列表
func (s *TopologyService) GetParseErrors(ctx context.Context, taskID string) ([]parser.ParseErrorDetail, error) {
	parseService := parser.NewService(s.db)
	return parseService.GetParseErrorsByTask(taskID)
}
