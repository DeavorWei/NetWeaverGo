package ui

import (
	"context"

	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// FrontendLogService 前端日志接收服务
type FrontendLogService struct {
	wailsApp *application.App
}

// NewFrontendLogService 创建前端日志服务实例
func NewFrontendLogService() *FrontendLogService {
	return &FrontendLogService{}
}

// ServiceStartup Wails 服务启动生命周期钩子
func (s *FrontendLogService) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	s.wailsApp = application.Get()
	return nil
}

// Log 接收单条前端日志
func (s *FrontendLogService) Log(entry logger.FrontendLogEntry) {
	writer := logger.GetFrontendLogWriter()
	if writer == nil {
		return
	}

	// error 级别日志立即写入，其他级别进入缓冲队列
	if entry.Level == "error" {
		writer.WriteImmediate(entry)
		return
	}

	// 级别过滤：debug 日志受 EnableDebug 控制
	if entry.Level == "debug" && !logger.EnableDebug && !logger.EnableVerbose {
		return
	}

	writer.Write(entry)
}

// LogBatch 接收批量前端日志
func (s *FrontendLogService) LogBatch(entries []logger.FrontendLogEntry) {
	writer := logger.GetFrontendLogWriter()
	if writer == nil {
		return
	}

	// 批量写入前进行级别过滤和 error 级别分离处理
	for _, entry := range entries {
		// error 级别日志立即写入
		if entry.Level == "error" {
			writer.WriteImmediate(entry)
			continue
		}

		// debug 日志受 EnableDebug 控制
		if entry.Level == "debug" && !logger.EnableDebug && !logger.EnableVerbose {
			continue
		}

		writer.Write(entry)
	}
}

// Flush 强制刷新日志缓冲区
func (s *FrontendLogService) Flush() {
	writer := logger.GetFrontendLogWriter()
	if writer != nil {
		writer.Flush()
	}
}