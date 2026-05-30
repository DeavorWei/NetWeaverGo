# SNMP轮询模块审计报告

## 审计概述

| 项目 | 内容 |
|------|------|
| 审计日期 | 2026-05-30 |
| 审计范围 | SNMP轮询模块（前端组件、后端服务、核心逻辑） |
| 审计人员 | AI审计助手 |
| 审计结论 | **中风险** - 存在凭据安全处理和并发控制优化空间 |

## 审计范围清单

| 文件路径 | 审计状态 |
|----------|----------|
| [`frontend/src/views/SNMP/SNMPPolling.vue`](frontend/src/views/SNMP/SNMPPolling.vue:1) | ✅ 已审计 |
| [`internal/ui/snmp_polling_service.go`](internal/ui/snmp_polling_service.go:1) | ✅ 已审计 |
| [`internal/snmp/poller.go`](internal/snmp/poller.go:1) | ✅ 已审计 |
| [`internal/snmp/poller_scheduler.go`](internal/snmp/poller_scheduler.go:1) | ✅ 已审计 |

---

## 1. 安全性审计

### 1.1 凭据加密审计

#### ✅ 已通过的验证

| 检查项 | 位置 | 状态 | 说明 |
|--------|------|------|------|
| 凭据加密存储 | [`CreateCredential`](internal/ui/snmp_polling_service.go:339) | ✅ 安全 | 使用CredentialCrypto加密community和密码 |
| 凭据更新加密 | [`UpdateCredential`](internal/ui/snmp_polling_service.go:392) | ✅ 安全 | 更新时同样加密敏感字段 |
| 脱敏显示 | [`credentialToVM`](internal/ui/snmp_polling_service.go:990) | ✅ 安全 | 前端显示脱敏的community和hasAuthKey/hasPrivKey标志 |

#### ⚠️ 需关注的问题

| 问题编号 | 位置 | 风险级别 | 问题描述 |
|----------|------|----------|----------|
| **S1** | [`CreateCredential`](internal/ui/snmp_polling_service.go:339) | ⚠️ 中风险 | 当crypto为nil时明文存储凭据，仅打印警告日志 |

**S1 问题详解**：

```go
// CreateCredential 凭据创建
if s.crypto != nil {
    // 加密敏感字段
    encrypted, err := s.crypto.EncryptCredential(req.Community)
    // ...
} else {
    // ⚠️ 未配置加密器，明文存储（不推荐）
    cred.Community = req.Community
    cred.AuthPassword = req.AuthPassword
    cred.PrivPassword = req.PrivPassword
    logger.Warn("SNMP-PollingService", "-", "⚠️ 凭据未加密存储，建议配置加密器")
}
```

**影响**：在加密器未正确初始化时，凭据将以明文存储在数据库中，存在安全风险。

### 1.2 输入验证审计

#### ✅ 已通过的验证

| 检查项 | 位置 | 状态 | 说明 |
|--------|------|------|------|
| 目标IP非空 | [`PollSingle`](internal/snmp/poller.go:249) | ✅ 安全 | 检查target是否为nil |
| 凭据存在检查 | [`DeleteCredential`](internal/ui/snmp_polling_service.go:450) | ✅ 安全 | 删除前检查是否被目标关联 |
| 端口默认值 | [`PollSingle`](internal/snmp/poller.go:249) | ✅ 安全 | 端口为0时默认使用161 |

#### ⚠️ 需关注的验证

| 检查项 | 位置 | 风险级别 | 说明 |
|--------|------|----------|------|
| IP格式验证 | [`CreatePollingTargetRequest`](internal/ui/snmp_polling_service.go:140) | ⚠️ 低风险 | 未验证IP地址格式有效性 |
| OID格式验证 | [`CreatePollingTemplateRequest`](internal/ui/snmp_polling_service.go:94) | ⚠️ 低风险 | 未验证OID格式有效性 |
| 轮询间隔范围 | [`CreatePollingTargetRequest`](internal/ui/snmp_polling_service.go:140) | ⚠️ 低风险 | 未限制轮询间隔最小值，可能导致过度轮询 |

### 1.3 资源访问控制

| 检查项 | 位置 | 状态 | 说明 |
|--------|------|------|------|
| 凭据删除保护 | [`DeleteCredential`](internal/ui/snmp_polling_service.go:450) | ✅ 安全 | 检查关联目标，拒绝删除被引用的凭据 |
| 并发度限制 | [`NewPoller`](internal/snmp/poller.go:78) | ✅ 安全 | 使用信号量限制最大并发数 |

---

## 2. 并发安全审计

### 2.1 锁机制分析

[`Poller`](internal/snmp/poller.go:61) 采用**信号量+读写锁**策略：

```go
type Poller struct {
    workerSem chan struct{}  // 并发控制信号量
    mu        sync.RWMutex   // 保护配置读取
}

type PollerScheduler struct {
    jobs map[uint]*scheduledJob  // 目标ID -> 任务
    mu   sync.RWMutex            // 保护jobs映射
}
```

#### ✅ 正确的锁使用

| 方法 | 锁类型 | 分析 |
|------|--------|------|
| [`Start`](internal/snmp/poller_scheduler.go:105) | `mu.Lock()` | 保护running状态和jobs初始化 ✅ |
| [`Stop`](internal/snmp/poller_scheduler.go:160) | `mu.Lock()` | 保护running状态和jobs清空 ✅ |
| [`AddTarget`](internal/snmp/poller_scheduler.go:206) | `mu.Lock()` | 保护jobs映射修改 ✅ |
| [`GetScheduledTargets`](internal/snmp/poller_scheduler.go:442) | `mu.RLock()` | 读操作使用读锁 ✅ |
| [`RunNow`](internal/snmp/poller_scheduler.go:302) | `mu.RLock()` | 读操作使用读锁 ✅ |

#### ✅ 并发控制设计亮点

| 设计 | 位置 | 分析 |
|------|------|------|
| 信号量控制 | [`PollBatch`](internal/snmp/poller.go:329) | 使用workerSem限制并发数，防止资源耗尽 ✅ |
| 原子计数器 | [`PollBatch`](internal/snmp/poller.go:329) | 使用atomic统计活跃goroutine数量 ✅ |
| 统计原子操作 | [`PollSingle`](internal/snmp/poller.go:249) | 使用atomic更新统计，无锁高效 ✅ |

### 2.2 潜在问题

| 问题编号 | 位置 | 风险级别 | 问题描述 |
|----------|------|----------|----------|
| **C1** | [`RunAllNow`](internal/snmp/poller_scheduler.go:389) | 🟡 中风险 | **统计更新竞态**：atomic.AddInt64在循环中调用，但结果保存和状态更新不在同一原子操作中 |

**C1 问题详解**：

```go
// RunAllNow 统计更新存在竞态
for i, results := range allResults {
    if results == nil || len(results) == 0 {
        continue
    }
    
    // 保存结果
    if err := s.repo.CreatePollingResults(ctx, results); err != nil {
        // ...
    }
    
    // 更新目标状态（非原子）
    s.updateTargetPollStatus(ctx, jobs[i].Target, results)
    
    // ⚠️ 原子增加计数，但与上面操作不是原子的
    atomic.AddInt64(&s.totalPolls, 1)
}
```

**影响**：在高并发场景下，统计数字可能与实际操作结果不一致。

---

## 3. 性能审计

### 3.1 批量操作效率

| 操作 | 位置 | 效率分析 |
|------|------|----------|
| 批量轮询 | [`PollBatch`](internal/snmp/poller.go:329) | ✅ 高效，使用errgroup并发控制 |
| 批量结果保存 | [`RunAllNow`](internal/snmp/poller_scheduler.go:389) | ⚠️ 中效，逐个保存而非批量保存 |
| 应用层重试 | [`pollWithRetry`](internal/snmp/poller.go:126) | ✅ 高效，指数退避+抖动避免惊群效应 |

### 3.2 重试机制分析

[`pollWithRetry`](internal/snmp/poller.go:126) 实现了**应用层重试**：

```go
// 重试配置
type PollerConfig struct {
    MaxAppRetries    int           // 应用层重试次数（默认3）
    BaseRetryDelay   time.Duration // 基础延迟（默认1s）
}

// 指数退避+抖动
func (p *Poller) calculateRetryDelay(attempt int) time.Duration {
    baseDelay := p.config.BaseRetryDelay * time.Duration(1<<uint(attempt))
    jitter := time.Duration(rand.Intn(500)) * time.Millisecond
    return baseDelay + jitter
}
```

✅ **设计优点**：
- 区分可重试错误（超时、网络错误）和不可重试错误（认证错误）
- 指数退避避免重试风暴
- 抖动避免惊群效应

### 3.3 性能问题

| 问题编号 | 位置 | 风险级别 | 问题描述 |
|----------|------|----------|----------|
| **P1** | [`RunAllNow`](internal/snmp/poller_scheduler.go:389) | 🟡 中风险 | 结果逐个保存，大量目标时效率低 |
| **P2** | [`addJob`](internal/snmp/poller_scheduler.go:476) | ⚠️ 低风险 | 每个目标都单独查询模板和凭据，N+1问题 |

---

## 4. 逻辑漏洞审计

### 4.1 状态转换正确性

| 状态转换 | 位置 | 状态 | 分析 |
|----------|------|------|------|
| 调度器启动 | [`Start`](internal/snmp/poller_scheduler.go:105) | ✅ 正确 | 加载所有已启用目标，注册cron任务 |
| 调度器停止 | [`Stop`](internal/snmp/poller_scheduler.go:160) | ✅ 正确 | 等待正在执行的任务完成，清空jobs |
| 目标添加 | [`AddTarget`](internal/snmp/poller_scheduler.go:206) | ✅ 正确 | 已存在时先移除再添加，未启用时不添加 |
| 目标更新 | [`UpdateTarget`](internal/snmp/poller_scheduler.go:257) | ✅ 正确 | 先移除旧任务，再添加新任务 |

### 4.2 错误处理完整性

| 错误场景 | 位置 | 状态 | 分析 |
|----------|------|------|------|
| 轮询失败 | [`pollWithRetry`](internal/snmp/poller.go:126) | ✅ 正确 | 重试耗尽后返回错误，记录日志 |
| 目标不存在 | [`RunNow`](internal/snmp/poller_scheduler.go:302) | ✅ 正确 | 返回明确错误信息 |
| 模板加载失败 | [`addJob`](internal/snmp/poller_scheduler.go:476) | ✅ 正确 | 记录警告但继续执行，允许无模板轮询 |
| 凭据加载失败 | [`addJob`](internal/snmp/poller_scheduler.go:476) | ✅ 正确 | 记录警告但继续执行 |

### 4.3 Cron任务管理

| 检查项 | 位置 | 状态 | 说明 |
|--------|------|------|------|
| Cron表达式生成 | [`intervalToCronSpec`](internal/snmp/poller_scheduler.go:612) | ✅ 正确 | 将秒级间隔转换为cron表达式 |
| 任务移除 | [`removeJob`](internal/snmp/poller_scheduler.go:521) | ✅ 正确 | 从scheduler移除并从jobs映射删除 |

---

## 5. 前端组件审计

### 5.1 输入验证

| 检查项 | 位置 | 状态 | 说明 |
|--------|------|------|------|
| 删除确认 | [`handleDeleteTarget`](frontend/src/views/SNMP/SNMPPolling.vue:311) | ✅ 安全 | 弹出确认对话框 |
| 表单默认值 | [`credentialForm`](frontend/src/views/SNMP/SNMPPolling.vue:136) | ✅ 安全 | 提供合理的默认值 |

### 5.2 实时事件处理

| 检查项 | 位置 | 状态 | 说明 |
|--------|------|------|------|
| 轮询结果事件 | [`handlePollingResultEvent`](frontend/src/views/SNMP/SNMPPolling.vue:48) | ✅ 正确 | 500ms防抖刷新结果列表 |
| 新结果高亮 | [`isNewResult`](frontend/src/views/SNMP/SNMPPolling.vue:75) | ✅ 正确 | 5秒内的新结果高亮显示 |

### 5.3 敏感信息处理

| 检查项 | 位置 | 状态 | 说明 |
|--------|------|------|------|
| Community显示切换 | [`showCommunityMap`](frontend/src/views/SNMP/SNMPPolling.vue:170) | ✅ 安全 | 默认隐藏，点击切换显示 |
| 密码不回显 | [`openCredentialModal`](frontend/src/views/SNMP/SNMPPolling.vue:337) | ✅ 安全 | 编辑时密码字段为空 |

---

## 6. 问题汇总

### 中风险问题

| 编号 | 问题 | 影响 | 建议修复 |
|------|------|------|----------|
| **S1** | crypto为nil时明文存储凭据 | 安全漏洞 | 强制要求加密器，否则拒绝创建凭据 |
| **C1** | RunAllNow统计更新竞态 | 统计数据不准确 | 使用事务或合并操作 |
| **P1** | 结果逐个保存 | 大量目标时性能下降 | 改为批量保存 |

### 低风险问题

| 编号 | 问题 | 影响 | 建议修复 |
|------|------|------|----------|
| **P2** | addJob的N+1查询 | 目标多时加载慢 | 批量预加载模板和凭据 |
| - | IP格式验证缺失 | 可能使用无效IP | 添加IP格式验证 |
| - | 轮询间隔无下限 | 可能过度轮询 | 设置最小轮询间隔（如10秒） |

---

## 7. 修复建议

### S1 修复方案

```go
// CreateCredential 强制要求加密器
func (s *SNMPPollingService) CreateCredential(ctx context.Context, req CreateCredentialRequest) error {
    if s.crypto == nil {
        return fmt.Errorf("加密器未初始化，无法安全存储凭据")
    }
    
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
    if req.Community != "" {
        encrypted, err := s.crypto.EncryptCredential(req.Community)
        if err != nil {
            return fmt.Errorf("加密 community 失败: %w", err)
        }
        cred.Community = encrypted
    }
    // ... 其他字段加密
    
    return s.repo.CreateCredential(ctx, cred)
}
```

### P1 修复方案

```go
// RunAllNow 批量保存结果
func (s *PollerScheduler) RunAllNow(ctx context.Context) [][]*models.SNMPPollingResult {
    // ... 轮询逻辑 ...
    
    // 收集所有需要保存的结果
    var allResultsToSave []*models.SNMPPollingResult
    for i, results := range allResults {
        if results != nil && len(results) > 0 {
            allResultsToSave = append(allResultsToSave, results...)
        }
    }
    
    // 批量保存
    if len(allResultsToSave) > 0 {
        if err := s.repo.CreatePollingResults(ctx, allResultsToSave); err != nil {
            logger.Error("SNMP-Scheduler", "-", "批量保存轮询结果失败: %v", err)
        }
    }
    
    // 更新目标状态
    for i, results := range allResults {
        if results != nil && len(results) > 0 {
            s.updateTargetPollStatus(ctx, jobs[i].Target, results)
            atomic.AddInt64(&s.totalPolls, 1)
        }
    }
    
    return allResults
}
```

---

## 8. 结论

SNMP轮询模块整体架构设计优秀，具有以下亮点：

1. **并发控制**：使用信号量限制并发数，atomic操作更新统计
2. **重试机制**：应用层重试+指数退避+抖动，设计完善
3. **凭据安全**：支持加密存储，前端脱敏显示

但存在以下需要关注的问题：

1. **安全性**：加密器未初始化时明文存储凭据，需要强制检查
2. **性能优化**：结果保存可改为批量操作
3. **输入验证**：缺少IP格式和轮询间隔范围验证

建议按优先级修复：S1 → P1 → C1 → P2 → 其他低风险问题。