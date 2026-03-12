NetWeaverGo 项目架构分析报告
经过连续三轮深入分析，我已完整审阅了项目的所有核心模块。以下是详细的问题清单和优化建议：

📋 问题清单
🔴 严重问题 (Critical)

1. main.go 中数据库初始化重复调用
   // cmd/netweaver/main.go
   func main() {
   // ...
   if err := config.InitDB(); err != nil { ... } // 第一次调用
   // ...
   if *isCLI || *isBackup || \*nonInteractive {
   runCLI(...)
   } else {
   runGUI()
   }
   }

func runGUI() {
if err := config.InitDB(); err != nil { ... } // 第二次调用（冗余）
}

影响：GUI 模式下数据库会被初始化两次，可能导致资源浪费或潜在的连接问题。

2. EngineService 中事件监听器同步机制过于复杂
   internal/ui/engine_service.go 中 StartEngine、StartEngineWithSelection、StartBackup 三个方法都使用了相同的事件监听器同步模式，代码重复率高：

type eventListenerState struct {
ready chan struct{}
active chan struct{}
listening chan struct{}
}

影响：维护困难，容易出现同步问题。

3. TaskGroupService 模式B实现存在问题
   // internal/ui/task_group_service.go
   // 模式B：每台设备独立命令 - 但实际实现将所有命令合并发送给所有设备
   // 这违背了模式B的设计意图（每台设备执行独立命令）

影响：模式B的功能不符合预期设计。

🟠 中等问题 (Medium) 4. ProgressTracker 日志存储未实际使用
internal/report/collector.go 中定义了 deviceLogs 和 logCounts 字段，但 handleEvent 方法并未向其写入日志：

func (p \*ProgressTracker) handleEvent(evt ExecutorEvent) {
// ... 处理事件但未调用 AddDeviceLog
}

影响：GetSnapshot() 返回的 logs 数组始终为空。

5. QueryService 内存中过滤效率低
   internal/ui/query_service.go 中 ListDevices 每次都加载全部设备到内存再过滤：

func (s *QueryService) ListDevices(opts QueryOptions) *QueryResult {
allDevices, _, _, \_, err := config.ParseOrGenerate(false) // 加载全部
// 内存中过滤...
}

影响：设备数量大时性能下降明显，应使用数据库查询。

6. 全局变量 globalEngine 和 globalSuspendManager 缺乏初始化保护
   // internal/engine/global_state.go
   var globalEngine = &GlobalEngineState{}

// internal/ui/suspend_manager.go
var globalSuspendManager = &SuspendManager{
sessions: make(map[string]\*SuspendSession),
sessionsByIP: make(map[string]string),
}

影响：并发访问时可能出现初始化顺序问题。

8. 前端 api.ts 存在大量向后兼容代码
   // frontend/src/services/api.ts - 大量 deprecated 导出
   export const ListDevices = DeviceAPI.listDevices // @deprecated
   // ... 50+ 行向后兼容导出

影响：增加打包体积，增加维护负担。

🟡 轻微问题 (Minor)

10. 配置文件路径分散
    // internal/config/config.go
    const (
    inventoryFile = "inventory.csv"
    configFile = "config.txt"
    )
    // settings.go 中没有统一的配置路径管理

影响：配置文件分散，不便于管理。

11. 前端事件类型与后端不完全一致
    // frontend/src/types/events.ts
    export type EventType = 'start' | 'cmd' | 'success' | 'error' | 'skip' | 'abort'

// 后端 report/event.go 可能使用不同的大小写或命名

影响：可能导致事件解析错误。

12. Engine 状态机转换不完整
    // internal/engine/engine.go
    type engineState int32
    const (
    stateIdle engineState = iota
    stateRunning
    stateClosing
    stateClosed
    )
    // 缺少 stateError、statePaused 等中间状态

🔧 可优化的地方
架构层面优化

1. 抽取事件监听器为独立组件
   // 建议：创建 internal/ui/event_bridge.go
   type EventBridge struct {
   wailsApp \*application.App
   ready chan struct{}
   active chan struct{}
   }

func (b \*EventBridge) StartListening(frontendBus chan report.ExecutorEvent) {
// 统一的事件转发逻辑
}

2. 引入依赖注入容器
   当前各服务通过 application.NewService() 注册，建议使用 Wire 或 Dig 进行依赖注入：

// 示例：使用 Wire 生成依赖
func InitializeApp() \*application.App {
wire.Build(
NewDeviceService,
NewEngineService,
NewQueryService,
// ...
)
return nil
}

3. 实现 Repository 模式
   // 建议：创建数据访问层
   type DeviceRepository interface {
   FindAll(opts QueryOptions) (*QueryResult, error)
   FindByIP(ip string) (*DeviceAsset, error)
   Save(device \*DeviceAsset) error
   Delete(id uint) error
   }

type SQLiteDeviceRepository struct {
db \*gorm.DB
}

性能优化 4. 使用数据库分页替代内存过滤
// 修改 QueryService.ListDevices
func (s *QueryService) ListDevices(opts QueryOptions) *QueryResult {
var devices []config.DeviceAsset
query := DB.Model(&config.DeviceAsset{})

    if opts.SearchQuery != "" {
        query = query.Where("ip LIKE ? OR group_name LIKE ?",
            "%"+opts.SearchQuery+"%", "%"+opts.SearchQuery+"%")
    }

    query.Count(&total)
    query.Offset((opts.Page - 1) * opts.PageSize).Limit(opts.PageSize).Find(&devices)
    // ...

}

5. 引入连接池管理
   当前每次执行都创建新的 SSH 连接，建议引入连接池：

type SSHConnectionPool struct {
mu sync.RWMutex
conns map[string]\*SSHClient
maxAge time.Duration
}

6. 优化事件总线缓冲区大小
   // 当前固定大小
   EventBus: make(chan report.ExecutorEvent, 1000),
   FrontendBus: make(chan report.ExecutorEvent, 1000),

// 建议：根据设备数量动态调整
bufferSize := max(1000, len(assets)\*10)

代码质量优化 7. 统一错误处理
// 建议：创建 internal/errors/errors.go
type AppError struct {
Code string
Message string
Cause error
}

func (e \*AppError) Error() string {
return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
}

var (
ErrEngineRunning = &AppError{Code: "ENGINE_001", Message: "引擎正在运行"}
ErrDeviceNotFound = &AppError{Code: "DEVICE_001", Message: "设备未找到"}
) 9. 完善配置热重载
// 建议：监听配置变化
type SettingsWatcher struct {
onChange func(settings \*GlobalSettings)
}

func (w \*SettingsWatcher) Watch(ctx context.Context) {
// 监听数据库变化或文件变化
}

前端优化 10. 清理向后兼容代码
移除 api.ts 中的 deprecated 导出，使用 TypeScript 的 deprecation 注解提示迁移。

11. 优化组件状态管理
    // Devices.vue 中状态过于分散，建议使用 Pinia
    export const useDeviceStore = defineStore('device', {
    state: () => ({
    devices: [] as Device[],
    selectedIds: new Set<string>(),
    loading: false,
    }),
    actions: {
    async loadDevices(opts: QueryOptions) { ... }
    }
    })

12. 改进事件处理
    // 当前每个组件独立监听事件，建议使用事件总线
    import { useEventBus } from '@/composables/useEventBus'

const bus = useEventBus()
bus.on('engine:finished', handleFinished)

📊 总结
| 类别 | 数量 | 优先级 |
|------|------|--------|
| 严重问题 | 3 | 🔴 高 |
| 中等问题 | 5 | 🟠 中 |
| 轻微问题 | 4 | 🟡 低 |
| 优化建议 | 12 | - |

建议优先处理顺序：

修复数据库重复初始化问题
重构事件监听器同步机制
修复模式B的实现逻辑
实现数据库分页查询
完善 ProgressTracker 日志功能
