# NetWeaverGo 代码质量问题修复详细实施方案

## 📋 项目概述

针对 bugfix.md 中"二、代码质量问题"章节提到的三个核心问题：

1. 重复代码
2. 魔法值过多
3. 错误处理不一致

本方案提供详细的修改实施计划。

---

## ✅ 已完成工作

### 1. 魔法值配置体系

#### 1.1 创建常量定义文件

**文件**: `internal/config/constants.go`

包含以下配置类别：

- **日志配置**: MaxLogsPerDevice(500), MaxLogLength(2000), LogTruncateThreshold(95)
- **超时配置**: DefaultCommandTimeout(30s), ConnectionTimeout(10s) 等
- **引擎配置**: DefaultWorkerCount(10), EventBufferSize(1000) 等
- **SSH配置**: DefaultSSHPort(22), MaxSSHSessions(5) 等
- **分页检测配置**: PaginationLineThreshold(50) 等
- **缓冲区配置**: DefaultBufferSize(4096) 等

#### 1.2 创建运行时配置管理系统

**文件**: `internal/config/runtime_config.go`

核心组件：

```go
// RuntimeSetting - 数据库模型
type RuntimeSetting struct {
    ID        uint      `gorm:"primaryKey"`
    Category  string    `gorm:"index"`  // timeout, limit, engine, buffer, pagination
    Key       string    `gorm:"index"`  // 配置键名
    Value     string    // 配置值
    UpdatedAt time.Time
}

// RuntimeConfig - 配置结构体（用于前后端交互）
type RuntimeConfig struct {
    Timeouts   struct { Command, Connection, Handshake, ShortCmd, LongCmd int }
    Limits     struct { MaxLogsPerDevice, MaxLogLength, LogTruncateThreshold, MaxConcurrentDevices int }
    Engine     struct { WorkerCount, EventBufferSize, FallbackEventCapacity int }
    Buffers    struct { DefaultSize, SmallSize, LargeSize int }
    Pagination struct { LineThreshold, CheckInterval int }
}

// RuntimeConfigManager - 热更新管理器（线程安全）
type RuntimeConfigManager struct {
    config RuntimeConfig
    mu     sync.RWMutex
}
```

功能特性：

- ✅ 支持热更新（内存缓存 + 数据库持久化）
- ✅ 线程安全（RWMutex 保护）
- ✅ 默认值自动初始化
- ✅ 辅助方法：GetCommandTimeout(), GetMaxLogsPerDevice() 等

#### 1.3 更新 SettingsService

**文件**: `internal/ui/settings_service.go`

新增 Wails 接口方法：

```go
// 获取运行时配置
func (s *SettingsService) GetRuntimeConfig() (config.RuntimeConfig, error)

// 更新运行时配置（热更新）
func (s *SettingsService) UpdateRuntimeConfig(cfg config.RuntimeConfig) error

// 重置为默认值
func (s *SettingsService) ResetRuntimeConfigToDefault() error
```

#### 1.4 初始化运行时配置管理器

**文件**: `cmd/netweaver/main.go`

在数据库初始化后添加：

```go
// 初始化运行时配置管理器
if err := config.InitRuntimeManager(config.DB); err != nil {
    logger.Error("System", "-", "运行时配置管理器初始化失败: %v", err)
    os.Exit(1)
}
```

---

### 2. 统一错误处理体系

#### 2.1 创建错误类型定义

**文件**: `internal/executor/errors.go`

核心组件：

```go
// ErrorType - 错误类型枚举
type ErrorType int
const (
    ErrorTypeNone     // 无错误
    ErrorTypeWarning  // 警告（可继续执行）
    ErrorTypeCritical // 严重（需要中断）
    ErrorTypeFatal    // 致命（系统级）
)

// ExecutionError - 统一错误类型
type ExecutionError struct {
    Type    ErrorType              // 错误类型
    IP      string                 // 设备IP
    Command string                 // 执行的命令
    Stage   string                 // 错误阶段: connect, execute, read, etc.
    Message string                 // 错误消息
    Err     error                  // 原始错误
    Context map[string]interface{} // 上下文信息
}

// 特性
- IsWarning() / IsCritical() / ShouldContinue() 判断方法
- Unwrap() 实现错误链
- ToMap() 转换为前端可用格式
- MarshalJSON() 支持 JSON 序列化
```

#### 2.2 错误构建器

```go
type ErrorBuilder struct { ... }
func NewError(ip string) *ErrorBuilder
func (b *ErrorBuilder) WithCommand(cmd string) *ErrorBuilder
func (b *ErrorBuilder) WithStage(stage string) *ErrorBuilder
func (b *ErrorBuilder) WithType(t ErrorType) *ErrorBuilder
func (b *ErrorBuilder) WithError(err error) *ErrorBuilder
func (b *ErrorBuilder) WithContext(key string, value interface{}) *ErrorBuilder
func (b *ErrorBuilder) Build() *ExecutionError
```

#### 2.3 自动错误分类

```go
func ClassifyError(err error) ErrorType
```

分类规则：

- **Fatal**: 内存不足、panic
- **Critical**: 连接拒绝、无路由、认证失败
- **Warning**: 超时、EOF、连接重置、命令不存在
- **默认**: Warning

#### 2.4 错误阶段常量

```go
const (
    StageConnect      = "connect"
    StageAuthenticate = "authenticate"
    StageExecute      = "execute"
    StageRead         = "read"
    StageParse        = "parse"
    StageClose        = "close"
    StageCleanup      = "cleanup"
)
```

#### 2.5 便捷函数

```go
func NewWarningError(ip, stage string, err error) *ExecutionError
func NewCriticalError(ip, stage string, err error) *ExecutionError
func NewFatalError(ip, stage string, err error) *ExecutionError
func IsExecutionError(err error) (*ExecutionError, bool)
```

---

## 📋 待完成工作

### 阶段1：配置系统集成（中等优先级）

#### 1.1 替换魔法值引用

需要修改的文件及内容：

**`internal/report/collector.go`**

```go
// 修改前
const MaxLogsPerDevice = 500

// 修改后
import "github.com/NetWeaverGo/core/internal/config"

func getMaxLogs() int {
    return config.GetRuntimeManager().GetMaxLogsPerDevice()
}
```

**`internal/engine/engine.go`**

```go
// 修改前
timeout := 30 * time.Second
bufferSize := 4096

// 修改后
manager := config.GetRuntimeManager()
timeout := manager.GetCommandTimeout()
bufferSize := manager.GetBufferSize()
```

**`internal/executor/executor.go`**

```go
// 修改前
reader := make([]byte, 4096)
timeout := 10 * time.Second

// 修改后
manager := config.GetRuntimeManager()
reader := make([]byte, manager.GetBufferSize())
timeout := manager.GetConnectionTimeout()
```

**`internal/sshutil/client.go`**

```go
// 修改前
Port: 22
timeout := 10 * time.Second

// 修改后
Port: config.DefaultSSHPort
timeout := config.GetRuntimeManager().GetConnectionTimeout()
```

---

### 阶段2：错误处理重构（高优先级）

#### 2.1 创建错误处理器

**新建文件**: `internal/executor/error_handler.go`

```go
package executor

import (
    "context"

    "github.com/NetWeaverGo/core/internal/events"
    "github.com/NetWeaverGo/core/internal/logger"
)

// ErrorHandler 统一错误处理器
type ErrorHandler struct {
    eventBus events.EventBus
    logger   logger.Logger
}

// NewErrorHandler 创建错误处理器
func NewErrorHandler(eventBus events.EventBus, log logger.Logger) *ErrorHandler {
    return &ErrorHandler{
        eventBus: eventBus,
        logger:   log,
    }
}

// Handle 统一处理错误
func (h *ErrorHandler) Handle(ctx context.Context, err *ExecutionError) error {
    // 1. 记录日志
    h.logError(err)

    // 2. 发送事件
    if h.eventBus != nil {
        h.eventBus.Publish("execution:error", err.ToMap())
    }

    // 3. 根据策略决定处理方式
    if err.ShouldContinue() {
        return nil // 警告级别，继续执行
    }
    return err // 严重级别，返回错误中断
}

// logError 记录错误日志
func (h *ErrorHandler) logError(err *ExecutionError) {
    fields := map[string]interface{}{
        "device":  err.IP,
        "stage":   err.Stage,
        "command": err.Command,
        "type":    err.Type.String(),
    }

    switch err.Type {
    case ErrorTypeWarning:
        h.logger.Warn(err.Message, fields)
    case ErrorTypeCritical, ErrorTypeFatal:
        h.logger.Error(err.Message, fields)
    }
}
```

#### 2.2 重构 Executor 错误处理

**文件**: `internal/executor/executor.go`

修改 `ExecuteCommandSync` 方法：

```go
func (e *Executor) ExecuteCommandSync(ctx context.Context, ip string, cmd string) error {
    errorHandler := NewErrorHandler(e.eventBus, e.logger)

    // 执行命令
    output, err := e.executeCommand(ctx, ip, cmd)
    if err != nil {
        execErr := NewError(ip).
            WithCommand(cmd).
            WithStage(StageExecute).
            WithType(ClassifyError(err)).
            WithError(err).
            Build()

        return errorHandler.Handle(ctx, execErr)
    }

    // ... 后续处理
}
```

#### 2.3 重构 Engine 错误处理

**文件**: `internal/engine/engine.go`

修改错误处理逻辑：

```go
func (e *Engine) handleDeviceError(device string, cmd string, err error) {
    // 转换为 ExecutionError
    execErr, ok := IsExecutionError(err)
    if !ok {
        execErr = NewCriticalError(device, StageExecute, err).
            WithCommand(cmd).
            Build()
    }

    // 统一发布错误事件
    e.emitEvent(events.Event{
        Type:      events.ErrorOccurred,
        Device:    device,
        Error:     execErr.Error(),
        Timestamp: time.Now(),
    })

    // 根据错误类型决定是否继续
    if !execErr.ShouldContinue() {
        e.cancel()
    }
}
```

---

### 阶段3：前端适配（中等优先级）

#### 3.1 生成新的 Wails Bindings

运行命令：

```bash
wails3 generate bindings
```

这将自动生成：

- `RuntimeConfig` 类型定义
- `GetRuntimeConfig()`, `UpdateRuntimeConfig()`, `ResetRuntimeConfigToDefault()` 方法

#### 3.2 创建运行时配置 UI 组件

**新建文件**: `frontend/src/components/settings/RuntimeConfigPanel.vue`

```vue
<template>
  <div class="runtime-config-panel">
    <h3>运行时配置</h3>

    <!-- 超时配置 -->
    <section>
      <h4>超时设置（毫秒）</h4>
      <div class="config-group">
        <label>命令执行超时</label>
        <input
          v-model.number="config.timeouts.command"
          type="number"
          min="1000"
          step="1000"
        />
      </div>
      <div class="config-group">
        <label>连接超时</label>
        <input
          v-model.number="config.timeouts.connection"
          type="number"
          min="1000"
          step="1000"
        />
      </div>
      <!-- ... 其他超时配置 -->
    </section>

    <!-- 限制配置 -->
    <section>
      <h4>限制设置</h4>
      <div class="config-group">
        <label>每设备最大日志数</label>
        <input
          v-model.number="config.limits.maxLogsPerDevice"
          type="number"
          min="100"
        />
      </div>
      <!-- ... 其他限制配置 -->
    </section>

    <!-- 引擎配置 -->
    <section>
      <h4>引擎设置</h4>
      <div class="config-group">
        <label>工作协程数</label>
        <input
          v-model.number="config.engine.workerCount"
          type="number"
          min="1"
          max="100"
        />
      </div>
      <!-- ... 其他引擎配置 -->
    </section>

    <div class="actions">
      <button @click="saveConfig" :disabled="saving">保存配置</button>
      <button @click="resetToDefault" :disabled="saving">重置为默认值</button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { GetRuntimeConfig, UpdateRuntimeConfig, ResetRuntimeConfigToDefault } from '../../bindings/...'

const config = ref({
  timeouts: { command: 30000, connection: 10000, ... },
  limits: { maxLogsPerDevice: 500, ... },
  engine: { workerCount: 10, ... },
  // ...
})

const saving = ref(false)

onMounted(async () => {
  config.value = await GetRuntimeConfig()
})

async function saveConfig() {
  saving.value = true
  try {
    await UpdateRuntimeConfig(config.value)
    // 显示成功提示
  } catch (err) {
    // 显示错误提示
  } finally {
    saving.value = false
  }
}

async function resetToDefault() {
  if (confirm('确定要重置为默认值吗？')) {
    await ResetRuntimeConfigToDefault()
    config.value = await GetRuntimeConfig()
  }
}
</script>
```

#### 3.3 更新错误处理

**文件**: `frontend/src/views/TaskExecution.vue`

修改错误处理逻辑：

```typescript
// 修改前
Events.On("execution:error", (data: string) => {
  showError(data);
});

// 修改后
Events.On("execution:error", (data: any) => {
  const error = JSON.parse(data);
  if (error.type === "CRITICAL" || error.type === "FATAL") {
    showCriticalError(error);
  } else {
    showWarning(error);
  }
});
```

---

## 📊 实施优先级

### 🔴 高优先级（立即处理）

1. 错误处理重构 - 影响系统稳定性
   - 创建 error_handler.go
   - 重构 executor.go
   - 重构 engine.go

### 🟡 中优先级（近期处理）

2. 配置系统集成 - 影响可维护性
   - 替换所有魔法值引用
   - 前端 UI 开发

### 🟢 低优先级（长期规划）

3. 代码结构优化
   - 提取重复代码（StreamReader）
   - 单元测试补充

---

## 🔧 技术细节

### 热更新机制

```go
// RuntimeConfigManager 使用双重存储
type RuntimeConfigManager struct {
    db     *gorm.DB      // 数据库持久化
    config RuntimeConfig // 内存缓存
    mu     sync.RWMutex  // 读写锁
}

// UpdateConfig 实现热更新
func (m *RuntimeConfigManager) UpdateConfig(config RuntimeConfig) error {
    // 1. 保存到数据库
    if err := SaveRuntimeConfig(m.db, config); err != nil {
        return err
    }
    // 2. 更新内存（无需重启）
    m.mu.Lock()
    m.config = config
    m.mu.Unlock()
    return nil
}
```

### 错误分类策略

- **Warning**: 超时、临时网络问题、命令语法错误 → 记录日志，继续执行
- **Critical**: 连接失败、认证失败 → 记录日志，中断当前设备，继续下一个
- **Fatal**: 内存不足、系统错误 → 记录日志，终止整个任务

---

## 📁 文件变更清单

### 新建文件

1. `internal/config/constants.go` - 常量定义
2. `internal/config/runtime_config.go` - 运行时配置管理
3. `internal/executor/errors.go` - 错误类型定义
4. `internal/executor/error_handler.go` - 错误处理器（待创建）

### 修改文件

1. `internal/ui/settings_service.go` - 添加配置接口方法
2. `cmd/netweaver/main.go` - 初始化运行时配置管理器
3. `internal/report/collector.go` - 使用配置管理器（待修改）
4. `internal/engine/engine.go` - 使用配置管理器和错误处理（待修改）
5. `internal/executor/executor.go` - 使用配置管理器和错误处理（待修改）
6. `frontend/src/views/Settings.vue` - 添加运行时配置UI（待修改）
7. `frontend/src/views/TaskExecution.vue` - 更新错误处理（待修改）

---

## ✅ 验证清单

- [x] Go 代码编译通过
- [x] Wails bindings 生成成功
- [ ] 配置热更新功能测试
- [ ] 错误分类功能测试
- [ ] 前端 UI 功能测试
- [ ] 集成测试（完整任务流程）

---

## 📝 实施建议

1. **分阶段实施**：先完成错误处理重构（高优先级），再进行配置系统集成
2. **保持向后兼容**：新错误格式需要前端同步更新
3. **充分测试**：特别是热更新和错误分类逻辑
4. **文档更新**：更新 README 中的配置说明

---

**当前状态**: 核心框架已完成，等待实施具体替换和重构工作
