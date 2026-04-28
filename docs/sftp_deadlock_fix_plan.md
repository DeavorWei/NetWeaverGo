# SFTP 服务器启动死锁问题修复方案

## 问题概述

**问题类型**: 并发死锁导致服务器启动卡住  
**影响范围**: SFTP 文件服务器启动功能  
**严重程度**: 高 - 功能完全不可用  
**根因**: 写锁内调用耗时操作，且该操作又尝试获取读锁

---

## 死锁场景详细分析

### 死锁触发条件

```
时间线：
┌─────────────────────────────────────────────────────────────┐
│ 17:31:20  第一次 ToggleServer(sftp, true)                    │
│           ├─ Start() 获取 s.mu.Lock() 写锁 ✓                │
│           ├─ 验证配置...                                    │
│           ├─ generateHostKey() 被调用                        │
│           │   └─ 尝试获取 s.mu.RLock() 读锁 ──→ 阻塞！       │
│           │       (写锁持有者无法降级，新读锁无法进入)         │
│           │                                                 │
│ 17:31:21  第二次 ToggleServer(sftp, true)                    │
│           └─ 等待 Start() 的写锁 ──→ 阻塞！                  │
│                                                             │
│ 17:31:21  第三次 ToggleServer(sftp, true)                    │
│           └─ 等待 Start() 的写锁 ──→ 阻塞！                  │
│                                                             │
│ 17:31:36  第四次 ToggleServer(sftp, true)                    │
│           └─ 等待 Start() 的写锁 ──→ 阻塞！                  │
└─────────────────────────────────────────────────────────────┘
```

### 代码层面的死锁证据

**位置**: `internal/fileserver/sftp_server.go:42`

```go
func (s *SFTPServer) Start(config *models.FileServerConfig) error {
    s.mu.Lock()              // ← 获取写锁
    defer s.mu.Unlock()      // ← 方法结束时才释放
    
    // ... 配置验证代码 ...
    
    hostKey, err := s.generateHostKey()  // ← 在持有锁期间调用
    // ...
}
```

**位置**: `internal/fileserver/sftp_server.go:418`

```go
func (s *SFTPServer) generateHostKey() (ssh.Signer, error) {
    s.mu.RLock()             // ← 尝试获取读锁！死锁发生点
    homeDir := s.config.HomeDir
    s.mu.RUnlock()
    // ...
}
```

### 为什么这是一个死锁

Go 的 `sync.RWMutex` 特性：
1. **写锁排斥一切**: 当写锁被持有时，任何新的读锁或写锁请求都会阻塞
2. **不允许锁降级**: Go 不支持将写锁降级为读锁
3. **不可重入**: 同一个 goroutine 不能重复获取同一把锁

因此：
- 线程 A 持有 `mu.Lock()`
- 线程 A 调用 `generateHostKey()` 尝试获取 `mu.RLock()`
- 由于锁不可重入且不允许降级，线程 A 被自己阻塞
- 永远不会有线程释放写锁，死锁形成

---

## 修复方案

### 方案一：移除 generateHostKey 的锁依赖（结构性修复）

**原理**: 将 `homeDir` 作为参数传递，避免在函数内部访问共享状态

**优点**:
- 彻底消除死锁可能
- 函数更纯，可测试性更好
- 锁的范围最小化

**缺点**:
- 需要修改函数签名
- 涉及多处调用点更新

**实施步骤**:

1. **修改 `generateHostKey` 函数签名**:
   ```go
   // 原代码
   func (s *SFTPServer) generateHostKey() (ssh.Signer, error)
   
   // 修改为
   func (s *SFTPServer) generateHostKey(homeDir string) (ssh.Signer, error)
   ```

2. **移除函数内部的锁操作**:
   ```go
   func (s *SFTPServer) generateHostKey(homeDir string) (ssh.Signer, error) {
       // 删除以下代码：
       // s.mu.RLock()
       // homeDir := s.config.HomeDir
       // s.mu.RUnlock()
       
       keyPath := filepath.Join(homeDir, ".sftp_host_key")
       // ... 其余逻辑不变
   }
   ```

3. **更新调用点**:
   ```go
   // 在 Start() 方法中
   func (s *SFTPServer) Start(config *models.FileServerConfig) error {
       s.mu.Lock()
       defer s.mu.Unlock()
       
       s.config = config
       
       // 修改前
       // hostKey, err := s.generateHostKey()
       
       // 修改后
       hostKey, err := s.generateHostKey(config.HomeDir)
       // ...
   }
   ```

---

### 方案二：优化锁范围（并发控制修复）

**原理**: 将耗时操作移到锁外执行，锁只保护状态变更

**优点**:
- 提高并发性能
- 减少锁持有时间
- 更符合 Go 并发最佳实践

**缺点**:
- 代码结构变化较大
- 需要仔细处理错误恢复

**实施步骤**:

1. **重构 `Start()` 方法的锁策略**:
   ```go
   func (s *SFTPServer) Start(config *models.FileServerConfig) error {
       // 第一阶段：快速检查（使用读锁或短暂写锁）
       s.mu.Lock()
       if s.running {
           s.mu.Unlock()
           return fmt.Errorf("SFTP 服务器已在运行中")
       }
       s.mu.Unlock()
       
       // 第二阶段：耗时操作（无锁）
       logger.Verbose("FileServer:SFTP", "-", "开始验证配置...")
       if err := s.validateConfig(config); err != nil {
           return err
       }
       
       logger.Verbose("FileServer:SFTP", "-", "正在生成 SSH 主机密钥...")
       hostKey, err := s.generateHostKey(config.HomeDir)  // 不需要锁
       if err != nil {
           return err
       }
       
       sshConfig := &ssh.ServerConfig{
           PasswordCallback: s.passwordCallback,
       }
       sshConfig.AddHostKey(hostKey)
       
       addr := fmt.Sprintf(":%d", config.Port)
       listener, err := net.Listen("tcp", addr)  // 系统调用，可能耗时
       if err != nil {
           return fmt.Errorf("监听端口失败: %v", err)
       }
       
       // 第三阶段：状态更新（使用写锁）
       s.mu.Lock()
       defer s.mu.Unlock()
       
       // 双重检查：可能在耗时操作期间状态已改变
       if s.running {
           listener.Close()
           return fmt.Errorf("SFTP 服务器已在运行中")
       }
       
       s.config = config
       s.sshConfig = sshConfig
       s.sshListener = listener
       s.running = true
       
       // 启动后台 goroutine（在锁外启动，但需要确保状态已设置）
       go s.acceptConnections()
       
       return nil
   }
   ```

---

### 方案三：添加启动状态保护（防御性修复）

**原理**: 在 UI 服务层添加启动状态标记，防止重复点击导致的并发启动请求

**优点**:
- 提升用户体验（即时反馈）
- 防止不必要的系统调用
- 可作为最后一道防线

**缺点**:
- 仅缓解症状，不治根
- 需要额外维护状态

**实施步骤**:

1. **在 FileServerService 中添加启动状态跟踪**:
   ```go
   type FileServerService struct {
       wailsApp *application.App
       manager  *fileserver.ServerManager
       db       *gorm.DB
       
       // 新增：启动状态保护
       startingMu sync.Mutex
       starting   map[string]bool  // protocol -> 是否正在启动
   }
   ```

2. **初始化时创建 map**:
   ```go
   func NewFileServerService() *FileServerService {
       logger.Debug("FileServerService", "-", "创建文件服务器服务实例")
       return &FileServerService{
           manager:  fileserver.NewServerManager(),
           db:       config.GetDB(),
           starting: make(map[string]bool),  // 新增
       }
   }
   ```

3. **修改 ToggleServer 方法**:
   ```go
   func (s *FileServerService) ToggleServer(protocol string, start bool) error {
       logger.Debug("FileServerService", "-", "ToggleServer 被调用: protocol=%s, start=%v", protocol, start)
       
       // 验证协议类型
       if !isValidProtocol(protocol) {
           return fmt.Errorf("无效的协议类型: %s", protocol)
       }
       
       // 新增：启动防重入保护
       if start {
           s.startingMu.Lock()
           if s.starting[protocol] {
               s.startingMu.Unlock()
               logger.Warn("FileServerService", "-", "%s 服务器正在启动中，忽略重复请求", protocol)
               return fmt.Errorf("%s 服务器正在启动中，请稍候", protocol)
           }
           s.starting[protocol] = true
           s.startingMu.Unlock()
           
           // 确保清理状态
           defer func() {
               s.startingMu.Lock()
               delete(s.starting, protocol)
               s.startingMu.Unlock()
           }()
       }
       
       // 原有逻辑...
       cfg, err := s.GetServerConfig(protocol)
       if err != nil {
           return fmt.Errorf("获取配置失败: %v", err)
       }
       
       if start {
           logger.Info("FileServerService", "-", "正在启动 %s 服务器...", protocol)
           err = s.manager.StartServer(fileserver.Protocol(protocol), cfg)
           if err != nil {
               logger.Error("FileServerService", "-", "启动 %s 服务器失败: %v", protocol, err)
               return err
           }
           logger.Info("FileServerService", "-", "%s 服务器已成功启动", protocol)
       } else {
           // ...
       }
       
       return nil
   }
   ```

---

## 代码变更清单

### 文件一：internal/fileserver/sftp_server.go

#### 变更 1.1: 修改 generateHostKey 函数签名
```diff
// generateHostKey 生成 SSH 主机密钥
-func (s *SFTPServer) generateHostKey() (ssh.Signer, error) {
+func (s *SFTPServer) generateHostKey(homeDir string) (ssh.Signer, error) {
-   s.mu.RLock()
-   homeDir := s.config.HomeDir
-   s.mu.RUnlock()
-
    keyPath := filepath.Join(homeDir, ".sftp_host_key")
    // ... 其余代码不变
}
```

#### 变更 1.2: 更新 Start 方法中的调用
```diff
    s.config = config

    logger.Verbose("FileServer:SFTP", "-", "正在生成 SSH 主机密钥...")
-   hostKey, err := s.generateHostKey()
+   hostKey, err := s.generateHostKey(config.HomeDir)
    if err != nil {
        logger.Error("FileServer:SFTP", "-", "生成主机密钥失败: %v", err)
        return fmt.Errorf("生成主机密钥失败: %v", err)
    }
    logger.Verbose("FileServer:SFTP", "-", "SSH 主机密钥生成成功")
```

### 文件二：internal/ui/fileserver_service.go

#### 变更 2.1: 添加启动状态字段
```diff
 // FileServerService 文件服务器管理服务
 type FileServerService struct {
     wailsApp *application.App
     manager  *fileserver.ServerManager
     db       *gorm.DB
+    
+    // 启动状态保护，防止重复点击
+    startingMu sync.Mutex
+    starting   map[string]bool
 }
```

#### 变更 2.2: 初始化 starting map
```diff
 // NewFileServerService 创建文件服务器服务实例
 func NewFileServerService() *FileServerService {
     logger.Debug("FileServerService", "-", "创建文件服务器服务实例")
     return &FileServerService{
         manager: fileserver.NewServerManager(),
         db:      config.GetDB(),
+        starting: make(map[string]bool),
     }
 }
```

#### 变更 2.3: 在 ToggleServer 中添加防重入保护
```diff
 // ToggleServer 启动/停止服务器
 func (s *FileServerService) ToggleServer(protocol string, start bool) error {
     logger.Debug("FileServerService", "-", "ToggleServer 被调用: protocol=%s, start=%v", protocol, start)
 
     // 验证协议类型
     if !isValidProtocol(protocol) {
         logger.Error("FileServerService", "-", "无效的协议类型: %s", protocol)
         return fmt.Errorf("无效的协议类型: %s", protocol)
     }
+    
+    // 启动防重入保护
+    if start {
+        s.startingMu.Lock()
+        if s.starting[protocol] {
+            s.startingMu.Unlock()
+            logger.Warn("FileServerService", "-", "%s 服务器正在启动中，忽略重复请求", protocol)
+            return fmt.Errorf("%s 服务器正在启动中，请稍候", protocol)
+        }
+        s.starting[protocol] = true
+        s.startingMu.Unlock()
+        
+        defer func() {
+            s.startingMu.Lock()
+            delete(s.starting, protocol)
+            s.startingMu.Unlock()
+        }()
+    }
 
     // 获取配置
     logger.Verbose("FileServerService", "-", "正在获取 %s 配置...", protocol)
```

---

## 测试验证计划

### 测试场景 1: 基本启动功能
```
步骤：
1. 启动应用
2. 进入文件服务器页面
3. 点击 SFTP 启动按钮
4. 验证：服务器成功启动，状态变为"运行中"
```

### 测试场景 2: 重复点击保护
```
步骤：
1. 停止 SFTP 服务器（如果正在运行）
2. 快速连续点击启动按钮 5 次
3. 验证：
   - 只有第一次请求被执行
   - 后续请求返回"正在启动中"提示
   - 服务器最终正常启动
```

### 测试场景 3: 并发启动测试
```
步骤：
1. 使用脚本同时发送 10 个启动请求
2. 验证：只有一个请求成功，其他被正确拒绝
3. 检查日志确认没有死锁或 panic
```

### 测试场景 4: 停止后重新启动
```
步骤：
1. 启动 SFTP 服务器
2. 停止 SFTP 服务器
3. 再次启动 SFTP 服务器
4. 验证：可以正常重启，无残留问题
```

---

## 实施建议

### 执行顺序

1. **立即执行方案一**（结构性修复）
   - 这是死锁的根本原因，必须修复
   - 风险：低，修改明确且可预测
   - 预计时间：30分钟

2. **并行执行方案三**（防御性修复）
   - 提升用户体验，防止误操作
   - 风险：低，纯新增功能
   - 预计时间：20分钟

3. **后续优化方案二**（可选）
   - 进一步优化并发性能
   - 风险：中，涉及较大重构
   - 预计时间：1-2小时

### 回滚计划

如修复引入新问题：
1. 回滚 `internal/fileserver/sftp_server.go` 到上一版本
2. 回滚 `internal/ui/fileserver_service.go` 到上一版本
3. 重新编译并部署

---

## 附录

### 相关代码文件

| 文件路径 | 修改内容 | 优先级 |
|---------|---------|-------|
| `internal/fileserver/sftp_server.go` | 修改 generateHostKey 签名，移除锁依赖 | P0 |
| `internal/ui/fileserver_service.go` | 添加 starting map 和防重入保护 | P1 |

### 相关日志位置

```
Dist/netWeaverGoData/logs/app/app.log
```

### 相关配置

SFTP 默认配置：
- 端口：2222
- 根目录：`<storage_root>`
- 用户名：admin
- 密码：admin
