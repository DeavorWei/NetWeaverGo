# NetWeaverGo v1.x 功能细化与架构优化计划

基于对当前代码的深入分析以及网络自动化运维的最佳实践，以下是 v1.x 功能模块的详细实现架构设计与进阶优化方案。

---

## 一、全局运行参数外置 (`settings.yaml`)

### 1.1 目标

保留当前灵活轻量的 `inventory.csv` (设备清单) + `config.txt` (命令清单) 模式，不再引入复杂的 YAML 分组及模板体系。
引入独立的全局配置文件 `settings.yaml`，将代码中硬编码的并发数、超时时间、日志路径等提取出来，便于灵活调整。

### 1.2 新增文件

```text
internal/config/
├── settings.go        [新增] 全局参数配置结构体与加载逻辑
```

### 1.3 核心结构体设计与范例

```go
// settings.go

// GlobalSettings 全局运行参数
type GlobalSettings struct {
    MaxWorkers     int    `yaml:"max_workers"`      // 并发数 (当前硬编码为 32)
    ConnectTimeout string `yaml:"connect_timeout"`  // SSH/SFTP 连接超时 (如 "10s")
    CommandTimeout string `yaml:"command_timeout"`  // 单条命令默认超时 (如 "30s")
    OutputDir      string `yaml:"output_dir"`       // 回显输出与配置备份的根目录
    LogDir         string `yaml:"log_dir"`          // 系统运行日志存放目录
    ErrorMode      string `yaml:"error_mode"`       // "pause" | "skip" | "abort" (控制发生识别到的错误时，默认的流水线动作)
}
```

```yaml
# settings.yaml 范例
settings:
  max_workers: 32
  connect_timeout: "10s"
  command_timeout: "30s"
  output_dir: "output"
  log_dir: "logs"
  error_mode: "pause" # pause(挂起询问) / skip(跳过报错命令) / abort(终止该设备)
```

### 1.4 架构专家优化建议：非交互模式与控制流平滑

- **防爆破/防拥塞机制的平滑启动**：在并发发起对认证服务器（如 TACACS+/RADIUS）的请求时，几十甚至上百并发的突发 SSH 握手可能导致 AAA 服务器限流或老旧交换机 CPU 飙升。建议在 `Engine.Run` 中对 goroutine 的启动进行微小的抖动延迟（Jitter Delay，如 `time.Sleep(time.Duration(rand.Intn(500)) * time.Millisecond)`），使得 SSH 握手压力被平滑分摊。
- **无人值守（Non-Interactive）模式**：为满足 CI/CD 及定时自动化巡检等场景，支持通过命令行标志 `--non-interactive` 启动。在此模式下，系统将严格遵循 `settings.yaml` 中的 `error_mode` 策略自动放行或中断，发生错误时绝对不会挂起等待标准输入（`bufio.Read`），彻底避免进程永远僵死的风险。
- **预检机制（Pre-flight Check）**：在发送 `config.txt` 中的重负荷长效配置组合前，自动下发一次空回车 `\n` 进行端到端全链路探活与终端 CLI 完整性验证。只有确认终端完全可用、响应及时，才正式进入指令执行循环。

---

## 二、Error Matcher 完善与分级处理

### 2.1 目标

内置各厂商错误模式库，引入错误分级系统，避免将轻微告警（如配置不变的提示）误判为严重错误导致整条流水线被异常阻塞。

### 2.2 新增文件

```text
internal/matcher/
├── matcher.go        [修改] 引入 ErrorRule 和分级系统
├── rules.go          [新增] 内置厂商错误规则库
```

### 2.3 核心结构体设计

```go
// matcher.go

type ErrorSeverity int

const (
    SeverityWarning  ErrorSeverity = iota // 仅告警，自动跳过并在日志内黄字提示，不影响后续指令下发
    SeverityCritical                      // 严重错误，触发 settings.error_mode 策略进行处理
)

// ErrorRule 匹配规则
type ErrorRule struct {
    Name     string         // 规则名称 (如 "命令不完整")
    Pattern  *regexp.Regexp // 正则表达式
    Severity ErrorSeverity  // 严重程度
    Vendor   string         // 适用厂商 (huawei, h3c, cisco)
    Message  string         // 语义化说明
}

// StreamMatcher 调整 (示意)
func (m *StreamMatcher) MatchError(line string) (bool, *ErrorRule) {
    // 轮询内置库，返回匹配情况及对应命中的策略规则
}
```

### 2.4 Executor 集成

```go
// executor.go
matched, rule := e.Matcher.MatchError(line)
if matched {
    if rule.Severity == matcher.SeverityWarning {
        logger.Warn("[%s] [告警放行] %s: %s", e.IP, rule.Name, rule.Message)
        continue // 仅提示，不受阻塞继续下一条
    }
    // Critical 级别才交由 Engine.handleSuspend 按照交互模式或 error_mode 进行后续防线拦截
}
```

---

## 三、执行增强（进度与超时追踪）

### 3.1 目标

增强控制台输出体验，引入非阻塞刷新进度大盘机制，便于用户在一批设备进行大规模集群操作（几百上千台）时能直观了解宏观进度。改进原先全局唯一超时一刀切的问题。

### 3.2 进度大盘与事件流（Event Bus）

- **现状问题**：当几十台设备并发运行时，标准输出 (`stdout`) 的杂项日志会严重刷屏，即使通过 `promptMu` 锁住了报错输入游标，用户的屏显反馈仍是一团乱麻。
- **专家优化方案**：解耦核心执行器和输出层，采用事件总线体系。在 `executor.go` 中高频抛出微事件（`EventCmdStart`, `EventCmdComplete`, `EventDeviceError` 等），并由上层统一的 `ProgressTracker` 包进行渲染收集。采用 `gosuri/uiprogress` 或是清屏渲染机制，在终端最后几行维持一个吸底的固定视图：

```text
[ NetWeaverGo ] 进度大盘: 成功 [12/32] | 运行中 [18] | 失败/挂起 [2]
► 192.168.1.1: [██████░░░░] 3/5 cmds - 正在下发 `sysname CORE-01`
► 192.168.1.5: [..........] 💥 连接异常: read timeout, 正在回退释放连接...
```

### 3.3 命令级超时的内联声明

虽然全局有 `settings.yaml` 中的 `command_timeout` 限制，但是在实际工程中，部分重量级持久化操作（如 `save` 或 `display diagnostic-information`）自身耗时极大概率超过全局均值。
- **专家解决方案**：支持 `config.txt` 中的内联伪注释指令。解析器发现行尾类似于 `save // nw-timeout=120s` 时，会为此时刻赋予特别的指令级别生命周期延长赋权，无需全盘放飞超时间隔。

---

## 四、结构化报告与输出模块 (Report Module)

### 4.1 目标

当前架构仅把 CLI 的生肉抓取并抛入 `output/` 中。面对企业级运维需求，需要在收尾时刻呈现清晰可落地的报表。

### 4.2 架构设计与改动

```text
internal/report/
├── collector.go    [新增] 线程安全的无锁或互斥锁执行结果收集中心
├── csv_export.go   [新增] 格式化投递导出逻辑
```

```go
// collector.go
type DeviceResult struct {
    IP         string
    Status     string        // "success" | "partial_failed" | "failed"
    Duration   time.Duration // 执行任务的总时间跨流
    ConnError  string        // 连接或运行中崩溃抛出的原因摘要
    LogsPath   string        // 指向对应的 output/ip_xxxx 记录文件
}
```

在 `engine.go` 的 `Run()` 方法阻塞并完成 WaitGroup 同步后，触发生成阶段的调用，将整段的成果整合进结构化变量 `Summary`，打印一份报告到控制台并同步导出存盘至 `output/report_YYYYMMDD_HHMM.csv`，便于通过 Excel 进行对数或工单交接。

---

## 五、各功能实现路线评估

1. **第一阶段：基建分离**。引入 `settings.yaml` 体系解析；改造 `Engine` 组件接入结构化参数进行速率和超时平分。
2. **第二阶段：防刷屏与报告体系**。重构 Executor 日志打印策略转向 Event-Based；实现执行后结算小票导出。
3. **第三阶段：规则隔离细分**。针对已有框架填充各厂商常见的识别特征进入内置 Matcher 集合，实装 Warning / Critical 设计。
4. **第四阶段：平滑性治理**。基于架构优选，全量补充 Jitter 建连散列等待和非交互守护挂机能力，全面适应 CICD。
