package fileserver

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/models"
)

// WebServer HTTP 文件服务器实现
type WebServer struct {
	mu          sync.RWMutex
	config      *models.FileServerConfig
	httpServer  *http.Server
	running     bool
	manager     *ServerManager
	connections sync.Map
	connWg      sync.WaitGroup
}

// NewWebServer 创建 HTTP 服务器实例
func NewWebServer(manager *ServerManager) *WebServer {
	logger.Debug("FileServer:HTTP", "-", "创建 HTTP 服务器实例")
	return &WebServer{
		manager: manager,
	}
}

// Start 启动 HTTP 服务器
func (s *WebServer) Start(config *models.FileServerConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	logger.Debug("FileServer:HTTP", "-", "Start 方法被调用")

	if s.running {
		logger.Warn("FileServer:HTTP", "-", "HTTP 服务器已在运行中")
		return fmt.Errorf("HTTP 服务器已在运行中")
	}

	// 验证配置
	logger.Verbose("FileServer:HTTP", "-", "开始验证配置: Port=%d, HomeDir=%s", config.Port, config.HomeDir)
	if err := s.validateConfig(config); err != nil {
		logger.Error("FileServer:HTTP", "-", "配置验证失败: %v", err)
		return err
	}
	logger.Verbose("FileServer:HTTP", "-", "配置验证通过")

	s.config = config

	// 创建路由
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleRequest)

	// 创建 HTTP 服务器
	s.httpServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", config.Port),
		Handler: s.authMiddleware(mux),
	}

	s.running = true

	// 使用 safeGo 启动服务器
	safeGo("HTTP-ListenAndServe", func() {
		logger.Info("FileServer:HTTP", "-", "HTTP 服务器 goroutine 已启动，开始监听端口 %d...", config.Port)
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.mu.Lock()
			s.running = false
			s.mu.Unlock()
			logger.Error("FileServer:HTTP", "-", "HTTP 服务器运行错误: %v", err)
			s.manager.emitLog(LogEvent{
				Level:    LogLevelError,
				Protocol: ProtocolHTTP,
				Action:   ActionError,
				Message:  fmt.Sprintf("HTTP 服务器运行错误: %v", err),
			})
		}
		logger.Info("FileServer:HTTP", "-", "HTTP 服务器 goroutine 已退出")
	})

	// 等待一小段时间确认服务器启动
	time.Sleep(100 * time.Millisecond)

	logger.Info("FileServer:HTTP", "-", "HTTP 服务器已成功启动，监听端口 %d", config.Port)

	s.manager.emitLog(LogEvent{
		Level:    LogLevelSuccess,
		Protocol: ProtocolHTTP,
		Action:   ActionConnect,
		Message:  fmt.Sprintf("HTTP 服务器已启动，监听端口 %d", config.Port),
	})

	return nil
}

// Stop 停止 HTTP 服务器
func (s *WebServer) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	logger.Debug("FileServer:HTTP", "-", "Stop 方法被调用")

	if !s.running || s.httpServer == nil {
		logger.Verbose("FileServer:HTTP", "-", "HTTP 服务器未运行，无需停止")
		return nil
	}

	logger.Verbose("FileServer:HTTP", "-", "正在关闭 HTTP 服务器...")

	// 创建一个带超时的上下文用于优雅关闭
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.httpServer.Shutdown(ctx); err != nil {
		logger.Error("FileServer:HTTP", "-", "停止 HTTP 服务器失败: %v", err)
		return fmt.Errorf("停止 HTTP 服务器失败: %v", err)
	}

	s.running = false

	logger.Verbose("FileServer:HTTP", "-", "等待所有连接处理完成...")
	s.connWg.Wait()

	logger.Info("FileServer:HTTP", "-", "HTTP 服务器已停止")

	s.manager.emitLog(LogEvent{
		Level:    LogLevelInfo,
		Protocol: ProtocolHTTP,
		Action:   ActionDisconnect,
		Message:  "HTTP 服务器已停止",
	})

	return nil
}

// IsRunning 检查服务器是否运行中
func (s *WebServer) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	logger.Verbose("FileServer:HTTP", "-", "IsRunning: %v", s.running)
	return s.running
}

// DisconnectAll 断开所有客户端连接
func (s *WebServer) DisconnectAll() error {
	logger.Debug("FileServer:HTTP", "-", "DisconnectAll 方法被调用")

	// HTTP 是无状态协议，这里不需要断开连接
	// 但我们可以记录日志
	logger.Info("FileServer:HTTP", "-", "所有 HTTP 连接已断开（无状态）")

	s.manager.emitLog(LogEvent{
		Level:    LogLevelInfo,
		Protocol: ProtocolHTTP,
		Action:   ActionDisconnect,
		Message:  "所有 HTTP 连接已断开",
	})

	return nil
}

// GetProtocol 获取协议类型
func (s *WebServer) GetProtocol() Protocol {
	return ProtocolHTTP
}

// validateConfig 验证配置
func (s *WebServer) validateConfig(config *models.FileServerConfig) error {
	logger.Verbose("FileServer:HTTP", "-", "验证端口号: %d", config.Port)
	if config.Port <= 0 || config.Port > 65535 {
		logger.Error("FileServer:HTTP", "-", "无效的端口号: %d", config.Port)
		return fmt.Errorf("无效的端口号: %d", config.Port)
	}

	logger.Verbose("FileServer:HTTP", "-", "验证根目录: %s", config.HomeDir)
	if config.HomeDir == "" {
		logger.Error("FileServer:HTTP", "-", "根目录不能为空")
		return fmt.Errorf("根目录不能为空")
	}

	logger.Verbose("FileServer:HTTP", "-", "检查/创建根目录: %s", config.HomeDir)
	if err := os.MkdirAll(config.HomeDir, 0755); err != nil {
		logger.Error("FileServer:HTTP", "-", "无法创建根目录 %s: %v", config.HomeDir, err)
		return fmt.Errorf("无法创建根目录: %v", err)
	}

	logger.Verbose("FileServer:HTTP", "-", "配置验证通过")
	return nil
}

// authMiddleware 基本认证中间件
func (s *WebServer) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.RLock()
		config := s.config
		s.mu.RUnlock()

		// 如果配置了用户名和密码，则进行基本认证
		if config.Username != "" || config.Password != "" {
			user, pass, ok := r.BasicAuth()
			if !ok {
				s.requestAuth(w)
				return
			}

			// 使用恒定时间比较防止时序攻击
			userMatch := subtle.ConstantTimeCompare([]byte(user), []byte(config.Username)) == 1
			passMatch := subtle.ConstantTimeCompare([]byte(pass), []byte(config.Password)) == 1

			if !userMatch || !passMatch {
				logger.Warn("FileServer:HTTP", r.RemoteAddr, "认证失败: 用户名或密码错误")
				s.manager.emitLog(LogEvent{
					Level:    LogLevelWarn,
					Protocol: ProtocolHTTP,
					ClientIP: s.getClientIP(r),
					Action:   ActionError,
					Message:  "认证失败: 用户名或密码错误",
				})
				s.requestAuth(w)
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

// requestAuth 请求认证
func (s *WebServer) requestAuth(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", `Basic realm="File Server"`)
	http.Error(w, "Unauthorized", http.StatusUnauthorized)
}

// handleRequest 处理所有 HTTP 请求
func (s *WebServer) handleRequest(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	config := s.config
		running := s.running
		s.mu.RUnlock()

	if !running {
		http.Error(w, "Server not running", http.StatusServiceUnavailable)
		return
	}

	// 增加连接计数
	s.connWg.Add(1)
	defer s.connWg.Done()

	// 获取客户端 IP
	clientIP := s.getClientIP(r)

	// 清理路径 - 使用正斜杠确保 URL 格式正确
	path := filepath.Clean(r.URL.Path)
	if path == "." {
		path = "/"
	}
	// Windows 下 filepath.Clean 会使用反斜杠，需要转换为正斜杠
	path = filepath.ToSlash(path)

	logger.Verbose("FileServer:HTTP", clientIP, "%s %s", r.Method, path)

	// 路由请求
	switch r.Method {
	case http.MethodGet:
		s.handleGet(w, r, config, clientIP, path)
	case http.MethodPut, http.MethodPost:
		s.handleUpload(w, r, config, clientIP, path)
	case http.MethodDelete:
		s.handleDelete(w, r, config, clientIP, path)
	case http.MethodOptions:
		s.handleOptions(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleGet 处理 GET 请求（文件下载或目录浏览）
func (s *WebServer) handleGet(w http.ResponseWriter, r *http.Request, config *models.FileServerConfig, clientIP, path string) {
	// 检查读取权限
	if !config.AllowGet {
		logger.Warn("FileServer:HTTP", clientIP, "拒绝 GET 请求: 权限不足")
		s.manager.emitLog(LogEvent{
			Level:    LogLevelWarn,
			Protocol: ProtocolHTTP,
			ClientIP: clientIP,
			Action:   ActionError,
			Message:  fmt.Sprintf("拒绝 GET 请求 %s: 权限不足", path),
		})
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// 安全检查：防止路径穿越
	safePath, err := s.safePath(config.HomeDir, path)
	if err != nil {
		logger.Warn("FileServer:HTTP", clientIP, "路径安全检查失败: %s, err: %v", path, err)
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// 检查文件/目录是否存在
	info, err := os.Stat(safePath)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "Not found", http.StatusNotFound)
		} else {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}

	// 如果是目录，处理目录请求
	if info.IsDir() {
		s.handleDirectoryBrowse(w, r, config, clientIP, safePath, path)
		return
	}

	// 提供文件下载
	logger.Info("FileServer:HTTP", clientIP, "下载文件: %s", path)
	s.manager.emitLog(LogEvent{
		Level:    LogLevelInfo,
		Protocol: ProtocolHTTP,
		ClientIP: clientIP,
		Action:   ActionDownload,
		Message:  fmt.Sprintf("下载文件 %s", path),
		File:     path,
	})

	// 设置响应头
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filepath.Base(safePath)))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size()))

	// 发送文件
	http.ServeFile(w, r, safePath)
}

// handleDirectoryBrowse 处理目录浏览
func (s *WebServer) handleDirectoryBrowse(w http.ResponseWriter, r *http.Request, config *models.FileServerConfig, clientIP, safePath, urlPath string) {
	logger.Debug("FileServer:HTTP", clientIP, "浏览目录: %s", urlPath)

	// 检查是否存在 index.html
	indexPath := filepath.Join(safePath, "index.html")
	if indexInfo, err := os.Stat(indexPath); err == nil && !indexInfo.IsDir() {
		logger.Info("FileServer:HTTP", clientIP, "返回 index.html: %s", urlPath)
		s.manager.emitLog(LogEvent{
			Level:    LogLevelInfo,
			Protocol: ProtocolHTTP,
			ClientIP: clientIP,
			Action:   ActionBrowse,
			Message:  fmt.Sprintf("返回 index.html: %s", urlPath),
			File:     urlPath,
		})
		http.ServeFile(w, r, indexPath)
		return
	}

	// 读取目录内容
	entries, err := os.ReadDir(safePath)
	if err != nil {
		logger.Error("FileServer:HTTP", clientIP, "读取目录失败: %s, err: %v", urlPath, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// 构建文件列表
	files := make([]fileInfoItem, 0, len(entries))
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		files = append(files, fileInfoItem{
			Name:    entry.Name(),
			Size:    info.Size(),
			IsDir:   entry.IsDir(),
			ModTime: info.ModTime(),
		})
	}

	logger.Info("FileServer:HTTP", clientIP, "目录浏览: %s (%d 个项目)", urlPath, len(files))
	s.manager.emitLog(LogEvent{
		Level:    LogLevelInfo,
		Protocol: ProtocolHTTP,
		ClientIP: clientIP,
		Action:   ActionBrowse,
		Message:  fmt.Sprintf("浏览目录 %s (%d 个项目)", urlPath, len(files)),
		File:     urlPath,
	})

	// 返回 HTML 格式的目录列表，传入权限配置
	s.renderDirectoryListing(w, urlPath, files, config)
}

// fileInfoItem 文件信息项
type fileInfoItem struct {
	Name    string
	Size    int64
	IsDir   bool
	ModTime time.Time
}

// buildBreadcrumb 构建面包屑导航
func (s *WebServer) buildBreadcrumb(urlPath string) string {
	if urlPath == "/" {
		return `<a href="/">根目录</a>`
	}

	// 移除首尾斜杠并分割
	cleanPath := strings.Trim(strings.Trim(urlPath, "/"), "/")
	if cleanPath == "" {
		return `<a href="/">根目录</a>`
	}

	parts := strings.Split(cleanPath, "/")
	var result strings.Builder
	result.WriteString(`<a href="/">根目录</a>`)

	currentPath := "/"
	for _, part := range parts {
		if part == "" {
			continue
		}
		currentPath += part + "/"
		result.WriteString(`<span>/</span>`)
		result.WriteString(`<a href="` + currentPath + `">` + part + `</a>`)
	}

	return result.String()
}

// renderDirectoryListing 渲染 HTML 格式的目录列表
func (s *WebServer) renderDirectoryListing(w http.ResponseWriter, urlPath string, files []fileInfoItem, config *models.FileServerConfig) {
	// 确保路径以 / 结尾
	if !strings.HasSuffix(urlPath, "/") {
		urlPath += "/"
	}

	// 构建页面标题
	title := "目录浏览: " + urlPath
	if urlPath == "/" {
		title = "根目录"
	}

	// 权限标识
	canDownload := config.AllowGet
	canUpload := config.AllowPut
	canDelete := config.AllowDel

	// HTML 头部和样式 - 简洁风格
	html := `<!DOCTYPE html>
<html lang="zh-CN">
<head>
	   <meta charset="UTF-8">
	   <meta name="viewport" content="width=device-width, initial-scale=1.0">
	   <title>` + title + `</title>
	   <style>
	       * {
	           margin: 0;
	           padding: 0;
	           box-sizing: border-box;
	       }
	       body {
	           font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
	           background: #f5f5f5;
	           min-height: 100vh;
	           padding: 0;
	           color: #333;
	       }
	       .container {
	           max-width: 960px;
	           margin: 0 auto;
	           background: #fff;
	           min-height: 100vh;
	       }
	       .header {
	           padding: 16px 24px;
	           border-bottom: 1px solid #e0e0e0;
	           background: #fafafa;
	       }
	       .header h1 {
	           font-size: 1.1em;
	           font-weight: 500;
	           color: #333;
	           display: flex;
	           align-items: center;
	           gap: 8px;
	       }
	       .breadcrumb {
	           margin-top: 8px;
	           font-size: 0.85em;
	           color: #666;
	       }
	       .breadcrumb a {
	           color: #0066cc;
	           text-decoration: none;
	       }
	       .breadcrumb a:hover {
	           text-decoration: underline;
	       }
	       .breadcrumb span {
	           color: #999;
	           margin: 0 4px;
	       }
	       .upload-section {
	           padding: 12px 24px;
	           border-bottom: 1px solid #e0e0e0;
	           display: flex;
	           align-items: center;
	           gap: 12px;
	           background: #fafafa;
	       }
	       .upload-section.hidden {
	           display: none;
	       }
	       .upload-section input[type="file"] {
	           font-size: 0.85em;
	           color: #555;
	       }
	       .btn {
	           padding: 6px 12px;
	           border: 1px solid #ddd;
	           border-radius: 4px;
	           cursor: pointer;
	           font-size: 0.85em;
	           font-weight: 400;
	           background: #fff;
	           color: #333;
	           transition: background 0.15s, border-color 0.15s;
	           display: inline-flex;
	           align-items: center;
	           gap: 4px;
	       }
	       .btn:disabled {
	           opacity: 0.5;
	           cursor: not-allowed;
	       }
	       .btn:hover:not(:disabled) {
	           background: #f5f5f5;
	           border-color: #ccc;
	       }
	       .btn-primary {
	           background: #0066cc;
	           color: #fff;
	           border-color: #0066cc;
	       }
	       .btn-primary:hover:not(:disabled) {
	           background: #0055aa;
	           border-color: #0055aa;
	       }
	       .btn-download {
	           background: transparent;
	           color: #0066cc;
	           border-color: transparent;
	           padding: 4px 8px;
	           font-size: 0.8em;
	       }
	       .btn-download:hover {
	           background: #e8f4fc;
	           border-color: transparent;
	           text-decoration: none;
	       }
	       .btn-delete {
	           background: transparent;
	           color: #cc3333;
	           border-color: transparent;
	           padding: 4px 8px;
	           font-size: 0.8em;
	       }
	       .btn-delete:hover:not(:disabled) {
	           background: #fce8e8;
	           border-color: transparent;
	       }
	       table {
	           width: 100%;
	           border-collapse: collapse;
	       }
	       th {
	           padding: 10px 16px;
	           text-align: left;
	           font-weight: 500;
	           color: #666;
	           font-size: 0.8em;
	           text-transform: uppercase;
	           letter-spacing: 0.5px;
	           border-bottom: 1px solid #e0e0e0;
	           background: #fafafa;
	       }
	       th.size {
	           width: 100px;
	           text-align: right;
	       }
	       th.modified {
	           width: 160px;
	       }
	       th.actions {
	           width: 120px;
	           text-align: center;
	       }
	       td {
	           padding: 8px 16px;
	           border-bottom: 1px solid #f0f0f0;
	           font-size: 0.9em;
	       }
	       td.actions {
	           text-align: center;
	           white-space: nowrap;
	       }
	       tr:hover {
	           background: #f9f9f9;
	       }
	       a.file-link {
	           color: #0066cc;
	           text-decoration: none;
	           display: flex;
	           align-items: center;
	           gap: 6px;
	       }
	       a.file-link:hover {
	           text-decoration: underline;
	       }
	       .icon {
	           font-size: 1em;
	           opacity: 0.7;
	       }
	       .size {
	           text-align: right;
	           color: #888;
	           font-family: "SF Mono", Monaco, "Courier New", monospace;
	           font-size: 0.85em;
	       }
	       .modified {
	           color: #888;
	           font-size: 0.85em;
	       }
	       .empty {
	           text-align: center;
	           padding: 48px 24px;
	           color: #999;
	           font-size: 0.9em;
	       }
	       .footer {
	           text-align: center;
	           padding: 16px;
	           color: #999;
	           font-size: 0.8em;
	           border-top: 1px solid #e0e0e0;
	           background: #fafafa;
	       }
	       .toast {
	           position: fixed;
	           bottom: 24px;
	           right: 24px;
	           padding: 10px 16px;
	           border-radius: 4px;
	           color: #fff;
	           font-size: 0.85em;
	           font-weight: 400;
	           z-index: 1000;
	           box-shadow: 0 2px 8px rgba(0,0,0,0.15);
	           animation: fadeIn 0.2s ease;
	       }
	       .toast.success {
	           background: #2d8a2d;
	       }
	       .toast.error {
	           background: #cc3333;
	       }
	       @keyframes fadeIn {
	           from { opacity: 0; transform: translateY(10px); }
	           to { opacity: 1; transform: translateY(0); }
	       }
	       .loading {
	           opacity: 0.6;
	           pointer-events: none;
	       }
	       .drop-zone {
	           position: fixed;
	           top: 0;
	           left: 0;
	           right: 0;
	           bottom: 0;
	           background: rgba(0, 102, 204, 0.1);
	           border: 2px dashed #0066cc;
	           display: none;
	           align-items: center;
	           justify-content: center;
	           z-index: 100;
	           font-size: 1.2em;
	           color: #0066cc;
	       }
	       .drop-zone.active {
	           display: flex;
	       }
	   </style>
</head>
<body>
	   <div class="container">
	       <div class="header">
	           <h1><span class="icon">📁</span> ` + title + `</h1>
	           <div class="breadcrumb">` + s.buildBreadcrumb(urlPath) + `</div>
	       </div>
	       <div class="upload-section` + map[bool]string{true: "", false: " hidden"}[canUpload] + `">
	           <input type="file" id="fileInput" multiple>
	           <button class="btn btn-primary" onclick="uploadFiles()">上传</button>
	       </div>
	       <table>
	           <thead>
	               <tr>
	                   <th>名称</th>
	                   <th class="size">大小</th>
	                   <th class="modified">修改时间</th>
	                   <th class="actions">操作</th>
	               </tr>
	           </thead>
	           <tbody>
`

	// 添加上级目录链接（非根目录时）
	if urlPath != "/" {
		// 使用 path 包处理 URL 路径，确保使用正斜杠
		parentPath := path.Dir(strings.TrimSuffix(urlPath, "/"))
		if parentPath == "." {
			parentPath = "/"
		}
		if !strings.HasSuffix(parentPath, "/") {
			parentPath += "/"
		}
		html += `                <tr>
	                   <td><a href="` + parentPath + `" class="file-link"><span class="icon">📁</span> ..</a></td>
	                   <td class="size">-</td>
	                   <td class="modified">-</td>
	                   <td class="actions">-</td>
	               </tr>
`
	}

	// 格式化文件大小
	formatSize := func(size int64) string {
		if size < 1024 {
			return fmt.Sprintf("%d B", size)
		} else if size < 1024*1024 {
			return fmt.Sprintf("%.1f KB", float64(size)/1024)
		} else if size < 1024*1024*1024 {
			return fmt.Sprintf("%.1f MB", float64(size)/(1024*1024))
		}
		return fmt.Sprintf("%.1f GB", float64(size)/(1024*1024*1024))
	}

	// 格式化时间
	formatTime := func(t time.Time) string {
		return t.Format("2006-01-02 15:04")
	}

	// 先添加目录，再添加文件
	for _, f := range files {
		if f.IsDir {
			dirPath := urlPath + f.Name + "/"
			deleteBtn := ""
			if canDelete {
				deleteBtn = `<button class="btn btn-delete" onclick="deleteItem('` + dirPath + `', true)" title="删除">删除</button>`
			}
			html += `                <tr>
	                   <td><a href="` + dirPath + `" class="file-link"><span class="icon">📁</span> ` + f.Name + `/</a></td>
	                   <td class="size">-</td>
	                   <td class="modified">` + formatTime(f.ModTime) + `</td>
	                   <td class="actions">` + deleteBtn + `</td>
	               </tr>
`
		}
	}

	for _, f := range files {
		if !f.IsDir {
			filePath := urlPath + f.Name
			// 构建操作按钮
			var actionBtns string
			if canDownload {
				actionBtns += `<a href="` + filePath + `" download class="btn btn-download" title="下载">下载</a>`
			}
			if canDelete {
				actionBtns += ` <button class="btn btn-delete" onclick="deleteItem('` + filePath + `', false)" title="删除">删除</button>`
			}
			if actionBtns == "" {
				actionBtns = "-"
			}
			html += `                <tr>
	                   <td><a href="` + filePath + `" class="file-link"><span class="icon">📄</span> ` + f.Name + `</a></td>
	                   <td class="size">` + formatSize(f.Size) + `</td>
	                   <td class="modified">` + formatTime(f.ModTime) + `</td>
	                   <td class="actions">` + actionBtns + `</td>
	               </tr>
`
		}
	}

	// 如果目录为空（除了可能有的上级目录）
	if len(files) == 0 && urlPath == "/" {
		html += `                <tr><td colspan="4" class="empty">目录为空</td></tr>
`
	}

	html += `            </tbody>
	       </table>
	       <div class="footer">
	           NetWeaverGo HTTP File Server
	       </div>
	   </div>
	   <div class="drop-zone" id="dropZone">拖放文件到此处上传</div>
	   <script>
	       // 当前目录路径
	       const currentPath = '` + urlPath + `';
	       const canUpload = ` + fmt.Sprintf("%v", canUpload) + `;
	       const canDelete = ` + fmt.Sprintf("%v", canDelete) + `;

	       // 显示提示消息
	       function showToast(message, type) {
	           const toast = document.createElement('div');
	           toast.className = 'toast ' + type;
	           toast.textContent = message;
	           document.body.appendChild(toast);
	           setTimeout(() => toast.remove(), 3000);
	       }

	       // 上传文件
	       async function uploadFiles() {
	           const input = document.getElementById('fileInput');
	           if (!input.files || input.files.length === 0) {
	               showToast('请先选择文件', 'error');
	               return;
	           }

	           const uploadBtn = document.querySelector('.upload-section .btn-primary');
	           uploadBtn.disabled = true;
	           uploadBtn.textContent = '上传中...';

	           let successCount = 0;
	           let failCount = 0;

	           for (const file of input.files) {
	               const filePath = currentPath + file.name;
	               try {
	                   const response = await fetch(filePath, {
	                       method: 'PUT',
	                       body: file
	                   });
	                   if (response.ok) {
	                       successCount++;
	                   } else {
	                       failCount++;
	                   }
	               } catch (err) {
	                   failCount++;
	                   console.error('Upload error:', err);
	               }
	           }

	           uploadBtn.disabled = false;
	           uploadBtn.textContent = '上传';
	           input.value = '';

	           if (successCount > 0) {
	               showToast('成功上传 ' + successCount + ' 个文件', 'success');
	               setTimeout(() => location.reload(), 1000);
	           }
	           if (failCount > 0) {
	               showToast('上传失败 ' + failCount + ' 个文件', 'error');
	           }
	       }

	       // 删除文件或目录
	       async function deleteItem(itemPath, isDir) {
	           const itemType = isDir ? '目录' : '文件';
	           const itemName = itemPath.split('/').filter(s => s).pop() || itemPath;
	           
	           if (!confirm('确定要删除' + itemType + ' "' + itemName + '" 吗？\\n此操作不可撤销！')) {
	               return;
	           }

	           try {
	               const response = await fetch(itemPath, {
	                   method: 'DELETE'
	               });
	               
	               if (response.ok) {
	                   showToast(itemType + '删除成功', 'success');
	                   setTimeout(() => location.reload(), 1000);
	               } else if (response.status === 403) {
	                   showToast('没有删除权限', 'error');
	               } else {
	                   showToast('删除失败: ' + response.statusText, 'error');
	               }
	           } catch (err) {
	               showToast('删除失败: ' + err.message, 'error');
	               console.error('Delete error:', err);
	           }
	       }

	       // 支持拖拽上传
	       if (canUpload) {
	           const dropZone = document.getElementById('dropZone');
	           
	           document.addEventListener('dragover', (e) => {
	               e.preventDefault();
	               dropZone.classList.add('active');
	           });
	           
	           document.addEventListener('dragleave', (e) => {
	               e.preventDefault();
	               if (e.target === document || e.target === document.body) {
	                   dropZone.classList.remove('active');
	               }
	           });
	           
	           document.addEventListener('drop', async (e) => {
	               e.preventDefault();
	               dropZone.classList.remove('active');
	               
	               const files = e.dataTransfer.files;
	               if (files.length === 0) return;

	               const uploadBtn = document.querySelector('.upload-section .btn-primary');
	               uploadBtn.disabled = true;
                uploadBtn.textContent = '⏳ 上传中...';

                let successCount = 0;
                let failCount = 0;

                for (const file of files) {
                    const filePath = currentPath + file.name;
                    try {
                        const response = await fetch(filePath, {
                            method: 'PUT',
                            body: file
                        });
                        if (response.ok) {
                            successCount++;
                        } else {
                            failCount++;
                        }
                    } catch (err) {
                        failCount++;
                        console.error('Upload error:', err);
                    }
                }

                uploadBtn.disabled = false;
                uploadBtn.textContent = '上传';

                if (successCount > 0) {
                    showToast('成功上传 ' + successCount + ' 个文件', 'success');
                    setTimeout(() => location.reload(), 1000);
                }
                if (failCount > 0) {
                    showToast('上传失败 ' + failCount + ' 个文件', 'error');
                }
            });
        }
    </script>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(html))
}

// handleUpload 处理文件上传（PUT/POST）
func (s *WebServer) handleUpload(w http.ResponseWriter, r *http.Request, config *models.FileServerConfig, clientIP, path string) {
	// 检查写入权限
	if !config.AllowPut {
		logger.Warn("FileServer:HTTP", clientIP, "拒绝上传: 权限不足")
		s.manager.emitLog(LogEvent{
			Level:    LogLevelWarn,
			Protocol: ProtocolHTTP,
			ClientIP: clientIP,
			Action:   ActionError,
			Message:  fmt.Sprintf("拒绝上传 %s: 权限不足", path),
		})
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// 安全检查：防止路径穿越
	safePath, err := s.safePath(config.HomeDir, path)
	if err != nil {
		logger.Warn("FileServer:HTTP", clientIP, "路径安全检查失败: %s, err: %v", path, err)
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// 确保目录存在
	dir := filepath.Dir(safePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		logger.Error("FileServer:HTTP", clientIP, "创建目录失败: %s, err: %v", dir, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// 创建文件
	file, err := os.Create(safePath)
	if err != nil {
		logger.Error("FileServer:HTTP", clientIP, "创建文件失败: %s, err: %v", path, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	// 复制请求体到文件
	size, err := io.Copy(file, r.Body)
	if err != nil {
		logger.Error("FileServer:HTTP", clientIP, "写入文件失败: %s, err: %v", path, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	logger.Info("FileServer:HTTP", clientIP, "上传文件成功: %s (%d bytes)", path, size)
	s.manager.emitLog(LogEvent{
		Level:    LogLevelSuccess,
		Protocol: ProtocolHTTP,
		ClientIP: clientIP,
		Action:   ActionUpload,
		Message:  fmt.Sprintf("上传文件 %s (%d bytes)", path, size),
		File:     path,
	})

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "File uploaded successfully",
		"size":    size,
	})
}

// handleDelete 处理文件删除（DELETE）
func (s *WebServer) handleDelete(w http.ResponseWriter, r *http.Request, config *models.FileServerConfig, clientIP, path string) {
	// 检查删除权限
	if !config.AllowDel {
		logger.Warn("FileServer:HTTP", clientIP, "拒绝删除: 权限不足")
		s.manager.emitLog(LogEvent{
			Level:    LogLevelWarn,
			Protocol: ProtocolHTTP,
			ClientIP: clientIP,
			Action:   ActionError,
			Message:  fmt.Sprintf("拒绝删除 %s: 权限不足", path),
		})
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// 安全检查：防止路径穿越
	safePath, err := s.safePath(config.HomeDir, path)
	if err != nil {
		logger.Warn("FileServer:HTTP", clientIP, "路径安全检查失败: %s, err: %v", path, err)
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// 删除文件/目录
	if err := os.RemoveAll(safePath); err != nil {
		logger.Error("FileServer:HTTP", clientIP, "删除失败: %s, err: %v", path, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	logger.Info("FileServer:HTTP", clientIP, "删除成功: %s", path)
	s.manager.emitLog(LogEvent{
		Level:    LogLevelSuccess,
		Protocol: ProtocolHTTP,
		ClientIP: clientIP,
		Action:   ActionDelete,
		Message:  fmt.Sprintf("删除 %s", path),
		File:     path,
	})

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "File deleted successfully",
	})
}

// handleOptions 处理 OPTIONS 请求（CORS 预检）
func (s *WebServer) handleOptions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	w.WriteHeader(http.StatusOK)
}

// safePath 安全检查路径，防止路径穿越攻击
func (s *WebServer) safePath(homeDir, requestedPath string) (string, error) {
	// 清理请求路径
	cleanedPath := filepath.Clean(requestedPath)

	// 防止路径穿越
	if strings.Contains(cleanedPath, "..") {
		return "", fmt.Errorf("路径包含非法字符")
	}

	// 构建完整路径
	fullPath := filepath.Join(homeDir, cleanedPath)

	// 确保解析后的路径仍在 homeDir 内
	resolvedPath, err := filepath.EvalSymlinks(fullPath)
	if err != nil {
		// 如果文件不存在，检查其父目录
		resolvedPath = fullPath
	}

	// 添加路径分隔符以确保前缀匹配正确
	homeDirAbs, err := filepath.Abs(homeDir)
	if err != nil {
		return "", fmt.Errorf("无法解析根目录: %v", err)
	}

	resolvedAbs, err := filepath.Abs(resolvedPath)
	if err != nil {
		return "", fmt.Errorf("无法解析请求路径: %v", err)
	}

	// 确保路径以根目录开头
	if !strings.HasPrefix(resolvedAbs, homeDirAbs+string(filepath.Separator)) && resolvedAbs != homeDirAbs {
		return "", fmt.Errorf("路径超出根目录范围")
	}

	return resolvedAbs, nil
}

// getClientIP 获取客户端 IP 地址
func (s *WebServer) getClientIP(r *http.Request) string {
	// 优先从 X-Forwarded-For 获取（代理场景）
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}

	// 其次从 X-Real-Ip 获取
	xri := r.Header.Get("X-Real-Ip")
	if xri != "" {
		return xri
	}

	// 最后使用 RemoteAddr
	ip := r.RemoteAddr
	// 移除端口部分
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}
	// 移除 IPv6 的方括号
	ip = strings.Trim(ip, "[]")

	return ip
}
