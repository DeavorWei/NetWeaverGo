# NetWeaverGo × eDeskPro 能力升级适配 — 未实施项实施方案

> **版本**: v1.2
> **依据**: 规划方案基线（`docs/eDeskPro能力升级适配规划方案.md` v1.1 与 `docs/审计报告.md` 需求基线）
> **定位**: 对规划方案 P0–P4 中尚未落地的 6 组能力给出可实施工程方案（现状核实 / 设计 / 落点 / 测试 / 风险）
> **编写约束**: 所有"现状"结论均以当前代码为准并标注 `文件:行号`；凡未经代码确认的推断显式标注「待核实」
> **范围**: 只描述实施，不替代 P0–P4 原始需求文档
> **v1.2 修订说明**: 吸收二轮架构复核意见，完成 3 项实施落地缺陷（首次安装 pre-open 判定时机、MirrorDB 显式解耦、ListTemplates 内置并集）与 4 项设计缺环（Identity.Handler 来源、分布指标分桶、巡检失败中止策略依赖澄清、细粒度命令级回退与 vendor 来源）的彻底闭环。

---

## 目录

- [0. 总览与实施顺序](#0-总览与实施顺序)
- [1. 升级前自动备份钩子（§10.4）](#1-升级前自动备份钩子104)
- [2. 命令缓存视图维度与默认策略（§5.2 P1-3 / §10.3）](#2-命令缓存视图维度与默认策略)
- [3. CEAS / 设备 golden 与样本资产（§7.4 / §6.3）](#3-ceas--设备-golden-与样本资产)
- [4. 可观测性 metrics（§10.2）](#4-可观测性-metrics)
- [5. P4 巡检三阶段编排 + 分组 + 多语言（§8.2）](#5-p4-巡检三阶段编排--分组--多语言)
- [6. 解析模板工作台组件化（§4.3.6）](#6-解析模板工作台组件化)
- [7. 数据模型与迁移汇总](#7-数据模型与迁移汇总)
- [8. 验收标准（DoD）与测试矩阵](#8-验收标准与测试矩阵)
- [9. 风险与红线](#9-风险与红线)
- [10. 审核记录（自审）](#10-审核记录自审)
- [附录 A：文件落点索引](#附录-a文件落点索引)
- [附录 B：变更记录](#附录-b变更记录)

---

## 0. 总览与实施顺序

### 0.1 前置已完成项（不在本方案范围）

| 项 | 状态 | 证据 |
|---|---|---|
| 解析模板"适用范围清空契约"Bug | **已修复** | `internal/ui/parse_template_service.go` `UpdateTemplate` 改为"nil 或空条件即清空"；新增测试 `TestParseTemplateService_UpdateTemplate_ClearAppliesTo` |
| 用户模板 `AppliesTo` 仓储回填 | 已修复 | `internal/repository/parse_template_repository.go` |
| `parser.engine_mode` 灰度接线 | 已修复 | `internal/config/settings.go` / `cmd/netweaver/main.go` / `internal/ui/settings_service.go` |
| CEAS 白名单过滤接线 | 已修复 | `internal/ui/hardware_inventory_service.go` `GetHardwareTreeFiltered` + 前端开关 |
| 导出前脱敏阻断 | 已修复 | `hardware_inventory_service.go` / `inspection_service.go` 调用 `report.ValidateExportContent` |

### 0.2 实施顺序（按风险从低到高）

| 序 | 工作项 | 依赖 | 预估 | 风险 | 建议批次 |
|---|---|---|---|---|---|
| 1 | 升级前自动备份钩子 | 新增版本号来源 | ~2 人日 | 低 | 批次一 |
| 2 | 命令缓存视图维度 + 默认 off | 提示符文本暴露 | ~2 人日 | 低 | 批次一 |
| 3 | CEAS / 设备 golden 与样本 | 无 | ~2.5 人日 | 低 | 批次一 |
| 4 | 可观测性 metrics | RunID 注入 + parser 结果扩展 | ~4 人日 | 中 | 批次二 |
| 5 | P4 三阶段编排 + 分组 + 多语言 | 巡检前一版兼容 | ~7.5 人日 | 中 | 批次二 |
| 6 | 解析模板工作台组件化 | 无（独立重构） | ~4 人日 | 中 | 批次三 |
| | **合计** | | **~22 人日** | | |

> **批次原则**：批次一（1–3）低风险、可独立上线、无跨模块契约变更；批次二涉及执行编排与解析契约扩展，需联调；批次三为纯前端重构，独立发布。

### 0.3 通用约束（承袭规划方案红线）

1. 不引入 Python/Jython；不搬运 `.py`；保持"零部署单 exe"。
2. 不建 XML 注册表矩阵，不做"款型 × 版本 × 产品"笛卡尔配置。
3. 不做常驻采集/轮询/Trap。
4. 自动行为默认最保守档；新增自动能力必须先灰度。
5. 任何输出到报告的路径必须脱敏。
6. 数据库迁移**只增不减**，新增列可空/带默认值。

---

## 1. 升级前自动备份钩子（§10.4）

### 1.1 目标与验收

**目标**：程序启动、执行数据库结构迁移**之前**，若检测到"待升级的既有主库"，自动将其备份到独立目录作为回滚底牌；备份失败不得阻断启动。

**验收标准**
1. 用旧版本主库启动：`<storageRoot>/backup/db/` 出现带版本戳的备份文件，且含 WAL/SHM 一致性快照。
2. 同版本二次启动不重复备份。
3. 首次安装（无库文件）不产生备份。
4. 从"无 `runtime_settings` 表的极早期库"升级，不抛异常。
5. 磁盘不可写等失败场景：仅告警，程序正常启动并完成迁移。
6. 数据库迁移（含 `config.InitDB` 与 `taskexec.AutoMigrate`）全部成功前**不会**被标记为"已备份"，任何一处迁移失败下次启动仍会备份。

### 1.2 现状核实

| 事实 | 证据 |
|---|---|
| 启动直接迁移、无备份 | `internal/config/db.go:28` `InitDB()` → `:67` `autoMigrateAll(db)` |
| 主迁移分步执行（跨包） | `internal/config/db.go:96-121`（基础表）+ `cmd/netweaver/main.go:42` `taskexec.AutoMigrate(config.DB)`（运行时表，含 `TaskRun`） |
| 已有镜像能力依赖全局 DB | `internal/config/db.go:168-176` `MirrorDatabaseToPath` 中直接读取 `DB` 全局变量执行 `wal_checkpoint(FULL)`，需显式解耦 |
| 主库路径 | `internal/config/paths.go:225` `GetDBPath()` |
| 现有备份目录语义 | `internal/config/paths.go:243` `GetBackupDir()` 返回 **`backup/config`**（配置备份，非 DB 备份） |
| 元数据表 | `internal/models/models.go:234-246` `RuntimeSetting` / 表名 `runtime_settings`（`key` 唯一索引） |
| **版本号当前不可用** | `build/windows/Taskfile.yml:57,90` 注入 `-X main.version={{.VERSION}}`，但 `cmd/netweaver` **不存在 `version` 变量**（搜索 0 命中）→ 该 ldflags 目前是空注入 |

### 1.3 设计

#### 1.3.1 版本号来源（前置）
- `cmd/netweaver/main.go` 增加：`var version = "dev"`（由构建期 `-X main.version=<ver>` 注入）。
- `main()` 中于 `config.InitDB()` **之前**调用 `config.SetAppVersion(version)`。
- `config` 包新增（建议 `internal/config/version.go`）：
  ```go
  var appVersion = "dev"
  func SetAppVersion(v string) { if strings.TrimSpace(v) != "" { appVersion = strings.TrimSpace(v) } }
  func GetAppVersion() string { return appVersion }
  ```
- 兜底：`version == "dev"` 时文件名用 `dev` + 时间戳（不退化为无备份）。开发模式若需多次验证迁移，可通过启动参数或删除 `runtime_settings` 中对应标记重置。

#### 1.3.2 严格顺序契约与探针（A1/A2/C3 修正）
针对 SQLite 在 `gorm.Open` 后即会创建空库文件导致 `Size()==0` 误判的问题，确立以下**不可颠倒的执行顺序契约**：

```
1. stat(dbPath)           # 必须在 gorm.Open 之前判定首次安装：isExistingDB := stat.err == nil && size > 0
  ↓
2. gorm.Open(dsn)         # 打开 SQLite 连接
  ↓
3. DB = db                # 立即赋值全局变量（确保后续 checkpoint 命中有效连接）
  ↓
4. EnsurePreUpgradeBackup(db, dbPath, isExistingDB)  # 探针 + 备份（纯只读，绝不写库、绝不自动建表）
  ↓
5. autoMigrateAll(db)     # 基础表迁移
  ↓
6. taskexec.AutoMigrate   # 运行时表迁移（TaskRun 等）
  ↓
7. CommitPreUpgradeBackupVersion(db)  # 全部迁移成功后，终态统一回写版本标记
```

**详细步骤实现**：
1. **Pre-Open 首次安装判断**：在 `gorm.Open` 之前执行 `fi, err := os.Stat(dbPath); isExistingDB := (err == nil && fi.Size() > 0)`。若 `!isExistingDB`，则后续直接跳过备份，彻底杜绝首次启动产生 0 字节无效快照。
2. **底层直连探针（不写库、不建表、无事务 Panic）**：
   ```go
   sqlDB, err := db.DB()
   var lastVersion string
   err = sqlDB.QueryRow("SELECT value FROM runtime_settings WHERE key = 'last_backup_version' LIMIT 1").Scan(&lastVersion)
   // 任何报错（含 no such table / no rows / query err）一律视为"旧版本/未标记，必须备份"
   ```
3. 探针值 == `appVersion` → 跳过备份。
4. **显式镜像备份**：
   - 改造 `MirrorDatabaseToPath(db *gorm.DB, sourceDBPath, targetDBPath string)`，直接传入连接实例 `db`，执行 `PRAGMA wal_checkpoint(FULL)`，彻底解除对未赋值全局变量的隐式时序耦合。
   - `target = <storageRoot>/backup/db/netweaver_<version>_<ts>.db`。
5. **全量迁移成功后才回写标记**：
   - 在 `cmd/netweaver/main.go` 中，只有当 `config.InitDB()` 与 `taskexec.AutoMigrate(config.DB)` **两阶段迁移均返回 nil** 后，才调用 `config.CommitPreUpgradeBackupVersion(config.DB)`。
   - 回写执行简单的 Upsert：`runtime_settings(key='last_backup_version', value=appVersion)`；回写失败仅记录 `Warn`（最多下次重启多备份一次，绝不在迁移失败时漏备）。
6. 备份失败（磁盘满/无权限）：`logger.Warn`，**继续执行后续迁移，不阻断启动**。

#### 1.3.3 目录与保留策略
- `PathManager` 新增字段 `DBBackupDir = <StorageRoot>/backup/db`，加入 `ensureDirectoriesLocked()`（`paths.go:152`）目录创建列表；
- 新增 `func (pm *PathManager) GetDBBackupDir() string`；
- 保留最近 **5** 份：备份后扫描 `netweaver_*.db`，连同关联的 `-wal/-shm` 文件按主文件修改时间倒序删除多余备份；
- 备份同步执行但**记录耗时**，超阈值（如 3s）打印告警。

### 1.4 实施步骤（文件级）

| 步骤 | 文件 | 说明 |
|---|---|---|
| S1 | `cmd/netweaver/main.go` | 新增 `var version = "dev"`；`main()` 前置调用 `config.SetAppVersion(version)`；并在 `taskexec.AutoMigrate` 成功后调用 `config.CommitPreUpgradeBackupVersion(config.DB)` |
| S2 | `internal/config/version.go`（新建） | `appVersion` + `SetAppVersion/GetAppVersion` |
| S3 | `internal/config/paths.go` | 新增 `DBBackupDir` 字段、`GetDBBackupDir()`、加入目录创建列表 |
| S4 | `internal/config/backup.go`（新建） | `EnsurePreUpgradeBackup(db *gorm.DB, dbPath string, isExistingDB bool) (bool, error)` + `CommitPreUpgradeBackupVersion(db *gorm.DB) error` + 备份保留轮转 |
| S5 | `internal/config/db.go` | `InitDB`：在 `gorm.Open` 前检查 `stat`，打开并设置 `DB = db` 后执行 `EnsurePreUpgradeBackup`；改造 `MirrorDatabaseToPath` 支持显式透传 `*gorm.DB` |
| S6 | `README.md` | 新增"升级与回滚"章节 |

### 1.5 测试

- `internal/config/backup_test.go`（新建，临时文件 sqlite）：
  1. 首次安装（Pre-Open 无文件）→ 无论是否 gorm.Open，均不备份；
  2. 旧库 + 无 `runtime_settings` 表 → 备份成功（验证"无表不崩、探针不报错建表"）；
  3. 旧库 + 标记为当前版本 → 跳过；
  4. 旧库 + 标记为旧版本 → 备份并触发 wal checkpoint；
  5. 任一迁移步骤失败未调 Commit → 不写标记，下次启动仍触发备份。
- 手工验收：真实旧库启动一次，检查 `backup/db/` 产物与 `netweaver.db-wal` 一致性。

### 1.6 风险与回退

| 风险 | 缓解 |
|---|---|
| 大库备份拖慢启动 | 记录耗时并告警；后续可异步化 |
| 备份失败阻断启动 | 明确"仅 Warn、不阻断" |
| 标记写错时机 | 严格遵循顺序契约，在 `InitDB` 和 `taskexec.AutoMigrate` 全部成功后收口提交 |
| 版本号缺失 | `dev` 兜底 + 时间戳 |

---

## 2. 命令缓存视图维度与默认策略

### 2.1 目标与验收

**目标**：命令缓存按"**同设备 + 同视图 + 同命令**"去重，避免跨视图误复用；默认值对齐安全红线。

**验收标准**
1. 同一设备、同一视图下重复命令命中缓存；跨视图（如全局配置 vs 接口配置、系统视图 vs OSPF 视图）严格不命中。
2. 视图标识来自**设备真实提示符**，不依赖易脆弱的命令跳转推断。
3. 视图上下文状态在会话生命周期内跨命令安全继承与更新。
4. 提示符无法判定视图时退化为 `unknown`（仍可复用同命令，但日志标注）。
5. 默认 `command_cache_enabled = off`（对齐红线 6），并同步修正规划方案 §10.3 中 `on` 的表述。

### 2.2 现状核实

| 事实 | 证据 |
|---|---|
| 缓存键仅命令字符串 | `internal/executor/command_cache.go:65` `Get(cmd)`、`:84` `Put(cmd,...)` |
| 调用处仅传命令 | `internal/executor/stream_engine.go:486`（读）、`:801`（写） |
| 无视图字段 | `SessionContext`/`CommandContext` 无 view 状态（`session_types.go` 搜索 0 命中） |
| `CommandContext` 生命周期单命令即焚 | `session_types.go:336` `Current *CommandContext` 每执行一条命令均被全新实例替换，不可作为跨命令视图载体 |
| 提示符匹配只返回 bool | `internal/matcher/matcher.go:126` `IsPrompt(chunk) bool`、`:211` `IsPromptStrict(line) bool` |
| 提示符行可取得但未导出 | `internal/matcher/matcher.go:292` `extractLastNonEmptyLine`（包内私有） |
| 默认值不一致 | `internal/config/settings.go:37` `CommandCacheEnabled: false`，而规划方案 §10.3 标 `on` |

### 2.3 设计

#### 2.3.1 视图来源与归一化（防止子视图碰撞）
命令跳转表在"权限不足导致 `system-view` 失败""`return` 跳回多级"等情况下会状态崩塌。**必须从真实提示符反解视图**。

- `matcher` 增加导出方法（不破坏现有 API）：
  ```go
  // MatchPrompt 返回是否命中提示符及命中的提示符整行文本
  func (m *StreamMatcher) MatchPrompt(chunk string) (bool, string)
  ```
  实现：把 `IsPrompt` 主体抽为私有 `isPromptLine(clean, line) bool`；`IsPrompt` 保留为 `ok, _ := MatchPrompt(chunk)` 的薄包装，**保证零语义变化**。
- 细粒度视图定义与归一化（新文件 `internal/matcher/view.go`，纯函数、可单测）：
  ```go
  type View string
  const (
      ViewUser       View = "user"        // 用户视图 (<host> / host>)
      ViewSystem     View = "system"      // 系统/特权视图 ([host] / host#)
      ViewInterface  View = "interface"   // 接口视图 ([host-GE0/0/1] / host(config-if)#)
      ViewRouting    View = "routing"     // 路由协议视图 ([host-ospf-1] / host(config-router)#)
      ViewConfig     View = "config"      // 全局配置视图 (host(config)#)
      ViewDiagnose   View = "diagnose"    // 诊断视图 ([host-diagnose])
      ViewSubview    View = "subview"     // 通用未知子视图 ([host-xxx])
      ViewUnknown    View = "unknown"     // 无法识别
  )
  func ResolveView(vendor, promptLine string) View
  ```
  映射规则（从 prompt 提取并区分厂商）：
  - **华为 / H3C (Comware)**：
    - `<...>` → `ViewUser`；
    - `[...]` 且不含 `-` → `ViewSystem`；
    - `[...-diagnose]` → `ViewDiagnose`；
    - `[...-GigabitEthernet...]` / `[...-GE...]` / `[...-XGE...]` / `[...-Eth-Trunk...]` / `[...-Vlanif...]` → `ViewInterface`；
    - `[...-ospf...]` / `[...-bgp...]` / `[...-isis...]` / `[...-rip...]` → `ViewRouting`；
    - 其他包含 `-` 的括号结构 → 提取具体子视图标记为 `subview:<token>` 或归为 `ViewSubview`（**严禁将所有 `-` 统归为 interface，杜绝 `display this` 在不同子视图下的缓存碰撞**）；
  - **Cisco / Ruijie**：
    - `host>` → `ViewUser`；
    - `host#` → `ViewSystem`；
    - `host(config)#` → `ViewConfig`；
    - `host(config-if)#` / `host(config-subif)#` → `ViewInterface`；
    - `host(config-router)#` → `ViewRouting`；
  - 其他/无法判定 → `ViewUnknown`。

#### 2.3.2 视图状态生命周期与缓存键
- **关键修复：视图状态保存在 `SessionContext`**：
  - `SessionContext`（跨命令会话级）增加字段 `CurrentView View`（初始 `ViewUnknown`）与 `Vendor string`，并增加方法 `SetCurrentView(v View)` 与 `GetCurrentView() View`。
  - `SessionAdapter` 暴露 `CurrentView() View` 与 `SetVendor(vendor string)`。
  - `CommandContext`（单命令级）增 `View View` 仅作为该次命令下发时的审计快照。
- **厂商识别与来源闭环（B5 修正）**：
  - 视图解析必须区分厂商规则（如华为 `<...>` 为用户视图，Cisco `host#` 为特权视图）。
  - `StreamEngine` 启动会话时，从 `e.executor` 提取标准厂商标识：优先取 `e.executor.deviceProfile.Vendor`，若未匹配画像则平滑回退至 `e.executor.opts.Vendor`（对应 `internal/executor/stream_engine.go:373-375` 与 `executor.go:82` 的成熟实践）。
  - 该 `vendor` 在构造 `SessionAdapter` 时显式注入（或通过 `sessionCtx.Vendor` 承载），避免每次解析重复回溯画像。
- **状态流转**：
  - 会话握手检测到首个提示符（`EvWarmupPromptSeen` / `EvActivePromptSeen`）时，根据提示符调用 `ResolveView(vendor, promptLine)` 初始化 `SessionContext.CurrentView`。
  - 命令执行过程中，当检测到命令结束提示符（或视图变更提示符）时，再次调用 `ResolveView(vendor, promptLine)` 更新 `SessionContext.CurrentView`。
- **缓存键**：
  - 下发命令前（`ActSendCommand` / `stream_engine.go:486`），从 `e.adapter.CurrentView()` 获取当前真实所处视图。
  - 键格式：`cacheKey := string(view) + "|" + act.Command`。
  - 写入缓存（`stream_engine.go:801`）时同样使用下发时所记录的 `view + "|" + act.Command`。
  - 兼容：`unknown` 视图仍可使用同命令复用（不降低容错体验），并在日志中标记 `[cache:unknown_view]`。

#### 2.3.3 默认值与文档
- `settings.go:37` 保持 `false`（符合红线 6）。
- 规划方案 §10.3 中 `command_cache_enabled` 初始态 `on` 修正为 `off`（以红线为准）。

### 2.4 实施步骤

| 步骤 | 文件 | 说明 |
|---|---|---|
| S1 | `internal/matcher/matcher.go` | 抽出 `isPromptLine`；新增 `MatchPrompt(chunk string) (bool, string)` |
| S2 | `internal/matcher/view.go`（新建） | `View` 常量（含 User/System/Interface/Routing/Config/Diagnose/Subview）+ `ResolveView` |
| S3 | `internal/executor/session_types.go` | `SessionContext` 增 `CurrentView View` 与 `Vendor string`；`CommandContext` 增 `View View` 审计快照 |
| S4 | `internal/executor/session_adapter.go` | 增加 `CurrentView()` 访问器；构造时接收/设置 `vendor`；提示符事件触发 `ResolveView(vendor, prompt)` 并 `SetCurrentView` |
| S5 | `internal/executor/stream_engine.go` | 会话初始化时提取 `deviceProfile.Vendor`（回退 `opts.Vendor`）注入 Adapter；`:486/:801` 读写缓存统一使用 `e.adapter.CurrentView() + "|" + cmd` 组装键 |
| S6 | `internal/config/settings.go` | 保持 `false`（不加改动，仅确认） |
| S7 | `docs/eDeskPro能力升级适配规划方案.md` | §10.3 表 `on` → `off` |

### 2.5 测试

- `internal/matcher/view_test.go`（新建）：华为/H3C/Cisco/Ruijie 各类提示符 → View 映射单测（重点覆盖接口、OSPF 路由、全局配置区分）。
- `internal/matcher/matcher_test.go`：`MatchPrompt` 行为与 `IsPrompt` 完全一致（回归）。
- `internal/executor/command_cache_test.go`：
  1. 同设备同视图同命令命中；
  2. 同设备不同视图（如 `interface|display this` 与 `routing|display this`）严格隔离不命中；
  3. 跨命令流转后 SessionContext 视图继承验证。

### 2.6 风险与回退

| 风险 | 缓解 |
|---|---|
| 提示符识别不准导致视图误判 | 无法判定即 `unknown`；只影响命中率，不影响命令下发正确性 |
| `MatchPrompt` 改动引入提示符识别回归 | `IsPrompt` 保留为 `MatchPrompt` 薄包装 + 现有单测全面回归 |
| 跨命令视图状态遗漏 | 状态置于 `SessionContext` 持久贯穿会话整个生命周期 |

---

## 3. CEAS / 设备 golden 与样本资产

### 3.1 目标与验收

**目标**：把 CEAS 硬件树与设备形态识别的"内联断言"升级为**可维护的 golden 基准 + 样本目录**，支撑格式漂移回归。

**验收标准**
1. `testdata/ceas/` 每个样本对应一份 `*_expected.json`；测试比对通过。
2. `testdata/device/` 提供各产品线 `version/patch/device` 回显样本与期望 `Identity`，四级回退用例独立成文件。
3. 提供 `-update` 能力一键刷新 golden（**当前不存在，需新增**）。
4. 规范化后比对在多次运行/跨平台稳定（无 map 乱序抖动）。

### 3.2 现状核实

| 事实 | 证据 |
|---|---|
| CEAS 仅 6 个 `.txt` 样本 | `testdata/ceas/{ce,sw,ar,wlan,fw,route}_elabel.txt` |
| CEAS 用内联硬编码断言 | `internal/ceas/elabel_test.go:19` 起（`tree.ChassisESN == "..."` 等） |
| 无 `testdata/device/` | 目录不存在（`list_dir testdata` 无） |
| device 测试为内联 + 复用 regression | `internal/device/identity_test.go` 内联；`internal/device/golden_test.go:11-70` 复用 `testdata/regression/vendor_golden` |
| **parser golden 无 `-update`** | `internal/parser/golden_test.go:12-182` 仅 `loadJSON` + 比对，无 update 逻辑 → "沿用既有 `-update`"的说法不成立，需新建 |

### 3.3 设计

#### 3.3.1 规范化器（防抖动，关键）
- 新增 `internal/ceas/golden.go`（仅测试构建或普通包均可，建议普通包便于复用）：
  ```go
  // NormalizeTreeForGolden 生成稳定可比的树快照：节点按 (Path, ID) 排序，Attrs 按 key 排序
  func NormalizeTreeForGolden(t *HardwareTree) *HardwareTree
  ```
- 同样为设备身份新增 `internal/device/golden.go` 的 `NormalizeIdentityForGolden(*Identity)`（`Evidence` 排序、去除易变字段）。
- 比对采用 `reflect.DeepEqual` 或序列化后字符串比较；`Attrs` 建议统一转为 `map[string]string` 并按 key 排序输出。

#### 3.3.2 `-update` 开关与环境变量支持（C1 修正：严禁污染生产 CLI）
- **红线约束**：Go 语言中非 `*_test.go` 的源文件若调用 `flag.Bool`，该 flag 会被编译进主程序 `cmd/netweaver`，导致生产环境命令行参数被污染（出现未预期的 `-update` 选项）。因此：
  - `golden.go` 仅作为公共纯函数库，包含结构规范化函数（如 `NormalizeTreeForGolden`）以及纯环境变量判定辅助函数；
  - `flag.Bool("update", ...)` **必须严格只在 `*_test.go`**（如 `ceas/golden_test.go`、`device/golden_test.go`）中声明！
- **双通道刷新判定实现**：
  ```go
  // 位于 internal/ceas/golden.go 或 internal/device/golden.go
  func IsUpdateGoldenEnv() bool {
      return os.Getenv("UPDATE_GOLDEN") == "1"
  }

  // 严格位于 internal/ceas/golden_test.go 与 internal/device/golden_test.go 中
  var updateGolden = flag.Bool("update", false, "update golden files")

  func shouldUpdateGolden() bool {
      return (updateGolden != nil && *updateGolden) || IsUpdateGoldenEnv()
  }
  ```
- 测试执行逻辑：
  ```go
  if shouldUpdateGolden() {
      _ = os.WriteFile(goldenPath, prettyJSON(got), 0644)
      return
  }
  ```
- **适用场景**：
  - 单包精准刷新：`go test ./internal/ceas -args -update`（此时仅当前包注册了 flag，正常消费）；
  - 跨包/CI 全量刷新：`UPDATE_GOLDEN=1 go test ./...`（环境变量全局生效，彻底杜绝 `flag provided but not defined: -update` 报错与生产 flag 泄漏）。

#### 3.3.3 目录规划
```
testdata/
├── ceas/
│   ├── ce_elabel.txt              # 现有
│   ├── ce_elabel_expected.json    # 新增（golden）
│   ├── ... （6 族各一份）
└── device/
    ├── versions/
    │   ├── huawei_s5735.txt        # display version / patch / device 回显样本
    │   └── ... （覆盖 _REG2HANDLER 20+ 类）
    ├── identities.json             # 样本文件 → 期望 Identity
    └── fallback_cases.json         # vendor/model/version → 期望 matchPath 与档位
```

### 3.4 实施步骤

| 步骤 | 文件 | 说明 |
|---|---|---|
| S1 | `internal/ceas/golden.go` 与 `golden_test.go`（新建） | `golden.go` 提供规范化器与 `IsUpdateGoldenEnv`；`golden_test.go` 注册测试专用 `-update` flag 与 6 族 golden 判定 |
| S2 | `testdata/ceas/*_expected.json`（新建 6 份） | 首次用 `-update` 生成并人工审阅 |
| S3 | `internal/device/golden.go` 与 `golden_test.go`（新建/扩展） | `golden.go` 提供 `NormalizeIdentityForGolden`；`golden_test.go` 注册测试专用 `-update` flag |
| S4 | `testdata/device/versions/*.txt`（新建） | 各产品线回显样本 |
| S5 | `testdata/device/identities.json`、`fallback_cases.json`（新建） | 期望身份与四级回退用例 |

### 3.5 测试

- `go test ./internal/ceas/... ./internal/device/...` 全绿；
- `go test ./internal/ceas/ -run TestParseELabel_Golden -args -update` 或 `UPDATE_GOLDEN=1 go test ./internal/ceas/...` 可刷新且刷新后无 diff；
- 连续运行两次结果一致（验证规范化无抖动）。

### 3.6 风险与回退

| 风险 | 缓解 |
|---|---|
| 首次 golden 由当前实现生成，掩盖既有错误 | 首次生成后**人工审阅**关键字段（ESN/Slot/Item/Path） |
| map 乱序导致 CI flaky | 规范化器强制排序 |
| `-update` 误在 CI 生效 | 仅显式传参才写盘，默认只读比对 |

---

## 4. 可观测性 metrics

### 4.1 目标与验收

**目标**：轻量、无外部依赖的运行指标；Run 结束时聚合入 journal 与 `task_runs.metrics_json`；执行详情页展示。

**验收标准**
1. 新增 `internal/metrics/`，`RunBucket` 按 RunID 分桶，`Release` 后无内存残留。
2. `task_runs` 增 `metrics_json`（可空），迁移只增不减。
3. 跑一次任意任务后 `metrics_json` 非空且字段齐全；journal 有对应记录。
4. 并发两个任务时，各自指标**互不串扰**（禁止全局区间差值）。
5. label 维度有界（handler 名、结果码、画像档位等有限集合），禁止 IP/命令行做 label。

### 4.2 现状核实

| 事实 | 证据 |
|---|---|
| 无 `internal/metrics/` 包 | `go build ./...` 包列表无该包 |
| `TaskRun` 无 `MetricsJSON` | `internal/taskexec/models.go:76-99` |
| Run 表迁移位置 | `internal/taskexec/persistence.go:516-543` `AutoMigrate` |
| 唯一现有指标为 parser 内部计数 | `internal/parser/models.go:160-167` `ParserMetrics`；`manager.go:94-126` 原子计数；无消费方 |
| Run 终态收口点 | `internal/taskexec/runtime.go:537` `finishRunWithStatus` → `:543` 报告 → `:548` `EventTypeRunFinished` → `:551` `finalizeRunResources` |
| StreamEngine/DeviceExecutor **非单例**（按 Unit 构造） | `internal/executor/executor.go:357` `NewStreamEngine(...)` 在 `ExecutePlan` 内；`NewDeviceExecutor` 由 taskexec 每 Unit 构造 |
| 全局单例仅 `ParserManager` | `cmd/netweaver/main.go:72` 单例创建 |
| ExecutorOptions 可扩展 | `internal/executor/executor.go:34-43` |
| RuntimeContext 已有 RunID | `internal/taskexec/runtime.go:21-42` `RunID() string` |

### 4.3 设计

#### 4.3.1 包结构与契约（无外部依赖，`sync/atomic`）
- 每个 `RunBucket` 维护独立的互斥锁与计数器，杜绝全局争用：
  ```go
  // internal/metrics/registry.go
  type RunBucket struct {
      mu       sync.RWMutex
      counters map[string]int64
      labels   map[string]map[string]int64
  }
  type Registry struct {
      mu   sync.RWMutex
      runs map[string]*RunBucket
  }
  func (r *Registry) Inc(runID, key string, delta int64)
  func (r *Registry) LabelInc(runID, key, label string)      // 有界 label（handler/结果码/匹配档位/分布区间）
  func (r *Registry) Snapshot(runID string) MetricSnapshot
  func (r *Registry) Release(runID string)
  var Default = NewRegistry()
  ```
- **序列化 Schema 契约（B2 修正：支持离散分布区间）**：
  ```json
  {
    "counters": {
      "parse.template_hit": 15,
      "cache.hit": 4,
      "echo.truncated": 0,
      "ceas.total_nodes": 48
    },
    "labels": {
      "inspection.result_code": { "TEST_PASS": 12, "TEST_WARNING": 2 },
      "device.handler": { "HuaweiARHandler": 3, "HuaweiCEHandler": 1 },
      "profile.match_path": { "exact": 2, "vendor": 1 },
      "ceas.node_count_range": { "<10": 1, "10-50": 2, "51-100": 0, ">100": 0 }
    }
  }
  ```

#### 4.3.2 RunID 传递（关键：不做全局差值）
- `ExecutorOptions` 增加 `RunID string`（`executor.go:34-43`）；`taskexec` 构造执行器时传 `ctx.RunID()`。
- `StreamEngine`/`SessionAdapter` 通过 `executor.opts.RunID` 感知，用于缓存命中、确认触发、风险命中、截断等打点。
- **parser 指标**：不改造 `ParserManager` 的内部接口；由 `taskexec` 在调用点打点。为让 taskexec 知道是否 fallback，推荐**新增可选接口**（向后兼容）：
  ```go
  // internal/parser/models.go 或新文件
  type ParseOutcome struct{ Engine string; Fallback bool; DurationMs int64 }
  type DetailedParser interface {
      ParseDetail(commandKey, rawText string) ([]map[string]string, ParseOutcome, error)
  }
  ```
  `CompositeParser` 实现 `ParseDetail`（复用现有 `isFallback`/`mode` 逻辑），`Parse` 保留为 `ParseDetail` 的包装。taskexec 调用处判断 `DetailedParser`，命中则 `metrics.Default.Inc(runID, "parse.template_hit"/"parse.fallback", 1)`。

#### 4.3.3 打点清单与闭环

| 指标 key | 打点位置 | 说明 |
|---|---|---|
| `parse.template_hit` / `parse.miss` / `parse.fallback` | `taskexec` 中 `cliParser.Parse` 调用点（配合 `DetailedParser`） | 解析模板与引擎分支命中统计 |
| `cache.hit` / `cache.miss` | `internal/executor/stream_engine.go:486` / `:801` | 命令缓存命中情况 |
| `confirm.triggered`（按 policy label） | `internal/executor/session_reducer.go` `handleConfirmSeen` 各分支 | 交互确认策略触发计数 |
| `risk.hit`（label=block/confirm/warn） | `internal/executor/stream_engine.go:377-467` | 高危命令阻断与告警触发 |
| `echo.truncated` | `internal/executor/command_context.go` `AppendRawData` 置 `Truncated` 处 | 超长回显截断统计 |
| `device.handler`（label=handler 名） | `internal/taskexec/executor_impl.go:1136` `device.Identify(...)` 后 | **B1 闭环**：`Identity` 增 `Handler` 字段，命中 `reg2HandlerList` 时由 `handlers.go` 回填 |
| `profile.match_path`（label=exact/series/vendor/global） | `config.ResolveProfile` 返回值消费点 | 画像降级档位分布 |
| `ceas.parse_fail` / `ceas.total_nodes` / `ceas.node_count_range` | `internal/taskexec/ceas_executor.go:249-260` | **B2 闭环**：总节点数入 counters，节点规模按 `<10`/`10-50`/`51-100`/`>100` 分桶入 labels |
| `inspection.result_code`（label=TEST_*） | `internal/taskexec/inspection_executor.go:302-310` | 巡检判定结果码分布 |

- **B1 闭环实现细节（Identity.Handler 来源）**：
  1. `internal/device/identity.go` 的 `Identity` 结构体增补字段：`Handler string `json:"handler,omitempty"` // 命中的处理器标识（如 HuaweiARHandler, HuaweiCEHandler 等）`；
  2. `internal/device/handlers.go:235-241` 在遍历 `reg2HandlerList` 命中正则提取成功时，显式赋值 `id.Handler = handler.name`；对 Cisco/H3C/Generic 识别路径同样回填对应规范化处理器名；
  3. `internal/taskexec/executor_impl.go:1136` 在 `device.Identify` 成功后直接读取 `id.Handler` 进行打点：`metrics.Default.LabelInc(runID, "device.handler", id.Handler)`。
- **B2 闭环实现细节（CEAS 节点分布分桶）**：
  - CEAS 树解析完毕后，`metrics.Default.Inc(runID, "ceas.total_nodes", int64(len(tree.Nodes)))`；
  - 同时调用辅助函数按区间离散化：`rangeLabel := resolveNodeRange(len(tree.Nodes))`（`<10`、`10-50`、`51-100`、`>100`），调用 `metrics.Default.LabelInc(runID, "ceas.node_count_range", rangeLabel)`。

#### 4.3.4 聚合与落库闭环（防字段静默丢弃）
- 在 `runtime.go:537` 之后、`emitProjectedRunEvent` 之前：
  ```go
  snapshot := metrics.Default.Snapshot(run.ID)
  if b, err := json.Marshal(snapshot); err == nil {
      s := string(b)
      handler.UpdateRunBestEffort(runtimeCtx, &RunPatch{MetricsJSON: &s}, "写入运行指标")
  }
  ```
- **关键闭环点**：
  - `TaskRun` 增 `MetricsJSON string`（`models.go`）；`RunPatch` 增 `MetricsJSON *string`。
  - **`internal/taskexec/persistence.go:89-115` (`GormRepository.UpdateRun`)** 必须补充字典映射：`if patch.MetricsJSON != nil { updates["metrics_json"] = *patch.MetricsJSON }`，否则 GORM 会将其静默忽略！
  - **`internal/taskexec/eventbus.go:547-585` (`SnapshotHub.ApplyRunPatch`)** 同步更新快照字段，确保前端通过 WebSocket/SSE 实时接收指标。
- journal：经 `ExecutionLogStore.WriteJournalRecord` 写入摘要。
- `finalizeRunResources`（`runtime.go:551`）末尾调用 `metrics.Default.Release(runID)` 彻底释放内存分桶。

### 4.4 实施步骤

| 步骤 | 文件 | 说明 |
|---|---|---|
| S1 | `internal/metrics/*`（新建） | Registry/RunBucket + 独立锁 + 单测 |
| S2 | `internal/executor/executor.go` | `ExecutorOptions.RunID`；构造时透传 |
| S3 | `internal/device/identity.go`、`handlers.go` | `Identity` 新增 `Handler` 字段；`reg2HandlerList` 匹配及厂商处理器回填 `id.Handler` |
| S4 | `internal/taskexec/*` | 各 executor 构造处传 `ctx.RunID()`；各打点（含 `device.handler` 与 `ceas.node_count_range`） |
| S5 | `internal/parser/*` | `ParseOutcome` + `DetailedParser`（兼容包装） |
| S6 | `internal/taskexec/models.go`、`persistence.go`、`eventbus.go` | `TaskRun.MetricsJSON`、`RunPatch.MetricsJSON`、`UpdateRun` 字典映射、`ApplyRunPatch` 快照同步 |
| S7 | `internal/taskexec/runtime.go` | 终态聚合写入 + `Release` |
| S8 | 前端执行详情 | 新增"本次运行指标"卡片（读 `metricsJson`，展示计数器、直方分桶与分布） |

### 4.5 测试

- `internal/metrics/registry_test.go`：并发 `Inc` / `LabelInc` 计数正确；`Snapshot` 格式符合 Schema；`Release` 后不泄漏。
- `internal/parser`：`ParseDetail` 对 tree/regex/aggregate 及 fallback 分支的 outcome 正确。
- `internal/taskexec`：Run 终态写入 `metrics_json` 并入库验证；并发两 Run 互不串扰。

### 4.6 风险与回退

| 风险 | 缓解 |
|---|---|
| 多 Run 并发串指标 | RunID 分桶 + ExecutorOptions 注入；**禁止全局差值** |
| label 基数爆炸 | 仅使用有限集合做 label；命名白名单校验 |
| 指标写入拖慢终态 | 内存快照 + `BestEffort` 异步落库 |

---

## 5. P4 巡检三阶段编排 + 分组 + 多语言

### 5.1 目标与验收

**目标**：把巡检从"单阶段内联采集+解析+判定"拆为 `inspection_collect → inspection_parse → inspection_check` 三阶段，实现"采集完成后尽早释放设备会话"；保持批量巡检的**单设备故障隔离容错能力**；补齐模板分组树与报表级多语言文案表。

**验收标准**
1. 一次巡检 Run 产生 3 个 Stage，事件流可见；`inspection_collect` 成功后所有物理设备连接即刻全部释放。
2. **批次容错隔离**：若某台设备连接/采集失败，**仅该设备**的后续阶段被跳过/标记失败，**绝不阻断其他健康设备**的解析与判定。
3. 判定结果与现有单阶段实现**逐条一致**（对照回归通过）。
4. **不引入中间解析实体表**，避免 SQLite 写入放大与 WAL 膨胀。
5. `InspectionTemplate.Groups` 可持久化并在前端按分组展示。
6. `inspection_item_texts` 表可建、可种子、可按 locale 提取处置建议文案（主要支撑海外英文巡检报告导出）。

### 5.2 现状核实

| 事实 | 证据 |
|---|---|
| 编译器只产出单 Stage | `internal/taskexec/inspection_compiler.go:119-132`（`Kind = StageKindInspectionCheck`） |
| 执行器内联全流程 | `internal/taskexec/inspection_executor.go:131-364`：连接 `:158-197`、采集 `:234-283`、解析 `:262-282`、判定 `:286-310`、落库 `:312-329` |
| 采集/解析执行器已存在但绑定拓扑 | `internal/taskexec/executor_impl.go:397` `DeviceCollectExecutor`（device_collect，命令解析用 `TopologyCommandResolver`）；`:769` `ParseExecutor`（parse，调用 `:870` `parseAndSaveRunDevice`） |
| 注册表按 StageKind 唯一 | executor 通过 `Kind()` 注册（`internal/taskexec/service.go`），**同一 kind 不能挂两个执行器** |
| 现有拓扑编排策略具有传导阻断性 | `internal/taskexec/executor_impl.go:479` 采用 `return firstErr`；`orchestration_policy.go:11-14` 只要前置有 err 即全局跳过后续，该模型**不适用于多设备并发巡检** |
| RuntimeContext 接口 | `internal/taskexec/runtime.go:21-42`（`RunID/Context/Update*/Emit/Logger/IsCancelled/GetDeviceLogPaths`） |
| 巡检原始回显已落盘 | `internal/taskexec/inspection_executor.go:256-260` + `internal/config/paths.go:386` `GetInspectionRawFilePath` |
| 模板无分组树 | `internal/models/inspection.go` `InspectionTemplate` 无 `Groups` |
| 多语言表不存在 | 全仓 grep `inspection_item_texts` 仅命中规划文档；前端当前零 i18n 依赖 |

### 5.3 设计

#### 5.3.1 编排模式与灰度
- 新增设置项 `GlobalSettings.InspectionPipelineMode`（`single` | `three_stage`），**默认 `single`**（对齐红线 6，验证通过后再切默认）。
- `InspectionTaskCompiler.Compile` 依据该设置产出 1 或 3 个 Stage。

#### 5.3.2 新增 StageKind（避免与拓扑执行器冲突）
| 常量 | 值 | 执行器 |
|---|---|---|
| `StageKindInspectionCollect` | `inspection_collect` | 新建 `InspectionCollectExecutor` |
| `StageKindInspectionParse` | `inspection_parse` | 新建 `InspectionParseExecutor` |
| `StageKindInspectionCheck` | `inspection_check` | 改造 `InspectionCheckExecutor`（仅判定+落库） |

> 不复用 `device_collect`/`parse`：注册表 kind 唯一，且拓扑命令解析器（`TopologyCommandResolver`）与巡检命令集语义不同。

#### 5.3.3 阶段契约与细粒度命令级容错（B4 修正：零中间表 + 命令级回退）
- **细粒度内存快照接口**（由 `defaultRuntimeContext` 实现，`finalizeRunResources` 中释放）：
  ```go
  // internal/taskexec/run_data_holder.go
  type RunDataHolder interface {
      SetCommandEcho(deviceIP, commandKey, echo string)
      GetCommandEcho(deviceIP, commandKey string) (string, bool)
      HasCommandEcho(deviceIP, commandKey string) bool
      SetParsedRows(deviceIP, commandKey string, rows []map[string]string)
      GetParsedRows(deviceIP, commandKey string) ([]map[string]string, bool)
  }
  ```
  实现为带 `sync.RWMutex` 的 `map[string]map[string]...`；**设总字节上限**（64MB/Run）。
- **阶段状态由 `progress_projector.go` 自动推导（禁止 Executor 手动覆盖终态）**：
  - 各阶段执行器（`InspectionCollectExecutor`, `InspectionParseExecutor`, `InspectionCheckExecutor`）的 `.Run()` 方法**严禁手动将 Stage 状态硬写为 `completed` / `partial` / `failed`**；
  - Stage 的终态和进度完全由 `internal/taskexec/progress_projector.go` 的 `projectStageCompletion` 根据下属各 Unit 状态自动投影推导；
  - 执行器仅负责通过 `UpdateUnitBestEffort` 维护各 Unit 状态，并在 Stage 层方法返回：唯有全量 Unit 失败（`SuccessUnits == 0`）或上下文被取消时返回 error，只要有 ≥1 个 Unit 成功即返回 `nil`。
- **Stage1 `inspection_collect`（命令级独立采集与释放）**：
  - 每设备一个 Unit，Steps 为去重命令列表（`IsPreCollect` 项优先）；
  - 逐条命令下发，单条命令超时或失败记录错误后继续执行该设备其他命令，不轻易中断整机采集；
  - 成功命令写原始回显文件（复用 `GetInspectionRawFilePath`）并调用 `SetCommandEcho(deviceIP, cmdKey, echo)`；
  - **设备命令下发完毕后立即 `exec.Close()`**，彻底释放物理连接与设备会话。
- **Stage2 `inspection_parse`（命令级细粒度回退与容错）**：
  - 每设备一个 Unit。遍历该设备巡检项所需的各 `commandKey`，**按具体命令维度而非整机粗粒度**执行容错回退：
    1. 优先从内存获取：`GetCommandEcho(deviceIP, commandKey)`；
    2. 若内存未命中（如超限淘汰或直接未载入），平滑回退读取磁盘原始文件（从 `GetInspectionRawFilePath` 切分对应命令块）；
    3. 取得回显则调用解析引擎，成功后写入 `SetParsedRows(deviceIP, commandKey, rows)`；
    4. 若某条特定命令在 Stage1 采集失败，仅标记该命令缺失，该设备其他成功采集的命令继续解析。
  - 若该设备全量命令均无回显，该设备 Unit 标记为 `failed`；若部分命令解析成功，Unit 标记为 `partial` 或 `completed`。
- **Stage3 `inspection_check`（命令级精准判定与落库）**：
  - 每设备一个 Unit。针对每个巡检项（`InspectionItem`）：
    1. 获取其依赖的 `(deviceIP, commandKey)`；
    2. 若内存中存在结构化数据 `GetParsedRows`，执行字段表达式匹配判定；
    3. 若无结构化数据，回退读取该命令原始回显走正则/脚本判定；
    4. 仅当该项依赖的特定命令在 Stage1/Stage2 全无数据时，该项判定结果记为 `error: 前置命令采集失败`，同设备其他依赖正常命令的巡检项**完全正常判定**。
  - 批量插入 `inspection_results`。

#### 5.3.4 依赖策略与失败策略解耦（B3 修正：区分拓扑与巡检）
- 在 `internal/taskexec/orchestration_policy.go` 中，明确两套正交策略：
  1. **依赖跳过策略 (`evaluateStageDependencyPolicy`)**：
     ```go
     if runKind == string(RunKindInspection) {
         // 巡检阶段仅在前置关键阶段全量失败（error != nil）时才跳过后续阶段
         if stage.Kind == string(StageKindInspectionParse) && results[string(StageKindInspectionCollect)] != nil {
             return true, "巡检采集阶段全量失败，跳过解析"
         }
         if stage.Kind == string(StageKindInspectionCheck) && results[string(StageKindInspectionParse)] != nil {
             return true, "巡检解析阶段全量失败，跳过判定"
         }
         return false, ""
     }
     ```
  2. **失败中止策略 (`evaluateStageFailurePolicy`)**：
     - 拓扑任务在采集或解析阶段失败时返回 `abort=true`，触发 `applyCompensationCancellation`；
     - **巡检任务针对 `runKind == RunKindInspection` 始终返回 `(false, "")`（非中止）**。
     - 即使某个阶段全败返回了 error，巡检也绝不触发级联补偿取消，而是让后续受影响阶段经依赖策略标记为 `skipped`，保持执行日志完整性并正常收口产出阶段审计报告。

#### 5.3.5 模板分组树 `InspectionGroup`
```go
type InspectionGroup struct {
    Code      string            `json:"code"`
    Name      string            `json:"name"`
    Children  []InspectionGroup `json:"children,omitempty"`
    ItemCodes []string          `json:"itemCodes,omitempty"`
}
// InspectionTemplate 增：
Groups []InspectionGroup `gorm:"serializer:json" json:"groups"`
```
- 迁移：仅加列（`config/db.go:117-119` AutoMigrate 自动处理），旧记录 `NULL` 安全。
- 前端 `Inspection.vue`：模板编辑增分组树；结果矩阵支持按分组折叠。

#### 5.3.6 多语言文案 `inspection_item_texts`（定位：报表级多语言）
```go
type InspectionItemText struct {
    ID          uint   `gorm:"primaryKey"`
    Key         string `gorm:"index:idx_item_text_key_locale,unique"`
    Locale      string `gorm:"index:idx_item_text_key_locale,unique"` // zh-CN / en-US
    Name        string
    Description string
    Advice      string
}
func (InspectionItemText) TableName() string { return "inspection_item_texts" }
```
- **定位澄清**：当前前端界面无全局 i18n 框架（纯中文）；本表重点服务于**海外工程/涉外局点的英文巡检报告导出**（`ExportInspectionResultsCSV/PDF` 携带 `locale` 参数时匹配英文建议），避免过早铺开复杂的全站 UI 国际化。
- 加入 `config/db.go` AutoMigrate；`inspection.EnsureInspectionItemTextSeeds()`（`sync.Once` 幂等），按需从内置精简 CSV 导入 `_DESCRIPTION` 作为 `Advice`。

### 5.4 实施步骤

| 步骤 | 文件 | 说明 |
|---|---|---|
| S1 | `internal/taskexec/status.go` | 新增两个 StageKind 常量 |
| S2 | `internal/taskexec/run_data_holder.go`（新建） | 内存快照接口 + 容量上限 + 设备维度判断 |
| S3 | `internal/taskexec/runtime.go` | `defaultRuntimeContext` 实现 holder；`finalizeRunResources` 释放 |
| S4 | `internal/taskexec/inspection_compiler.go` | 依 `InspectionPipelineMode` 产出 1/3 Stage |
| S5 | `internal/taskexec/inspection_collect_executor.go`（新建） | 采集阶段（设备独立容错，非全败不抛 stage 错误） |
| S6 | `internal/taskexec/inspection_parse_executor.go`（新建） | 解析阶段（逐设备消费 echo，独立错误隔离） |
| S7 | `internal/taskexec/inspection_executor.go` | 改造为纯判定+落库（保留单阶段兼容路径） |
| S8 | `internal/taskexec/orchestration_policy.go` | 依赖策略解耦，巡检仅在全败时跳过整阶段 |
| S9 | `internal/taskexec/service.go` | 注册新执行器 |
| S10 | `internal/models/inspection.go` | `Groups` + `InspectionItemText` |
| S11 | `internal/config/db.go` | AutoMigrate 加 `InspectionItemText` |
| S12 | `internal/inspection/seeds.go` | `EnsureInspectionItemTextSeeds` |
| S13 | 前端 `Inspection.vue` | 分组树编辑/展示；导出报表支持选择语言 |

### 5.5 测试

- `inspection_compiler_test.go`：`single` 产 1 Stage、`three_stage` 产 3 Stage 且顺序正确。
- **批次容错单测**：模拟 5 台设备执行巡检，其中 1 台连接失败：验证另外 4 台设备正常完成三阶段并写入判定结果，Run 最终状态为 `partial`。
- 对照回归：同一模板/全正常设备集，`single` 与 `three_stage` 的 `inspection_results` 逐条一致。
- 内存上限：超限时 Stage2 走磁盘回退且结果不变。
- `EnsureInspectionItemTextSeeds` 幂等；报表导出按 locale 正确填充。

### 5.6 风险与回退

| 风险 | 缓解 |
|---|---|
| 三阶段与单阶段结果不一致 | 默认 `single`；对照回归通过后再切换 |
| 单设备故障导致全阶段阻断 | 明确阶段级容错汇聚策略，仅全败才返回 stage error |
| 内存快照导致 OOM | 64MB 容量上限 + 磁盘持久化兜底回退 |
| 执行器注册冲突 | 使用独立 StageKind，不复用拓扑 kind |
| 分组树 JSON 列破坏旧数据 | 可空列，旧记录读取不受影响 |

---

## 6. 解析模板工作台组件化

### 6.1 目标与验收

**目标**：把 `ParseTemplates.vue`（35KB 单文件）按规划方案 §4.3.6 拆为容器 + 4 个子组件，补齐"匹配高亮"三视图；**不引入重型第三方依赖**。

**验收标准**
1. 新增 `components/parsetpl/`：`TemplateList.vue`、`RuleTableEditor.vue`、`EchoTestPane.vue`、`RegexPlayground.vue`；`ParseTemplates.vue` 退化为容器，路由 `/parse-templates` 不变。
2. 规则排序用"上移/下移"（不引入 `vuedraggable`）。
3. `EchoTestPane` 提供**树视图 / 平铺表 / 匹配高亮**三视图；匹配高亮由后端返回命中区间驱动。
4. 功能与拆分前**行为一致**（零回归），`vue-tsc -b` 与 `vite build` 通过。

### 6.2 现状核实

| 事实 | 证据 |
|---|---|
| 单文件 35KB | `frontend/src/views/ParseTemplates.vue` |
| 无 `components/parsetpl/` | 全仓搜索 `RuleTableEditor/EchoTestPane/RegexPlayground/TemplateList` 0 命中 |
| 适用范围编辑**已有**且清空 Bug 已修 | `ParseTemplates.vue:250-267, 505-506, 620-621, 731-744`；后端 `parse_template_service.go` `UpdateTemplate` |
| `TestTemplate` 返回体不含命中区间 | `internal/models/parse_template.go:67-72` `TestParseTemplateResult{Success,Results,Count,Error}` |
| 无拖拽依赖 | `frontend/package.json` 无 `vuedraggable`（仅 element-plus 等） |

### 6.3 设计

#### 6.3.1 组件拆分与模板并集汇聚（A3 修正：内置 + 用户覆盖全量透出）
- `TemplateList.vue`：vendor/commandKey 过滤、列表、"内置/用户覆盖"徽标与一键克隆覆盖。
  - **现状缺陷与 A3 闭环**：
    - `internal/ui/parse_template_service.go:28-47` 的 `ListTemplates` 目前仅执行 `repo.FindAll(vendor)`（只查数据库表 `net_user_parse_templates`）；在无用户自建覆盖时，列表直接返回空数组，导致前端既无法查看系统预置的几十种命令内置解析规则，也无法直观呈现 `Source` 徽标。
    - **并集汇聚改造**：
      1. 通过 `ParserManager.GetSnapshot(vendor).ListCommandKeys()` 获取引擎预置的全部内置命令模板键；
      2. 通过 `repo.FindAll(vendor)` 获取用户自定义/覆盖持久化记录；
      3. 合并汇聚为统一 VO 列表：
         - 若 `commandKey` 存在用户数据库记录，以数据库数据组装，显式标记 `Source: "user"`；
         - 若该 `commandKey` 仅存在于内置快照，组装只读 VO，显式标记 `Source: "builtin"`（支持提取内置规则内容展示）；
      4. 在 `UserParseTemplateVO` 中显式增加 `Source string `json:"source"``（`"builtin" | "user"`），前端据此渲染高亮徽章并控制"保存为新覆盖/删除自定义覆盖"。
- `RuleTableEditor.vue`：规则表格 + 上移/下移；父字段下拉来源为当前已定义规则；字段校验（`parentItem` 存在、无环）依赖后端保存期预编译（`CompileTreeRules` 已含校验）。
- `EchoTestPane.vue`：粘贴回显 → 调 `TestTemplate` → 三视图（树视图、平铺表、匹配高亮）。
- `RegexPlayground.vue`：正则实时 try/catch；保存前后端已预编译（双保险）。
- 拆分采用"先抽组件、后删旧代码"，每一步保持可运行。

#### 6.3.2 匹配高亮与全场景绝对偏移累加（C2 修正：覆盖 A/B/C 三类解析场景）
- `TestParseTemplateResult` 增可空字段：
  ```go
  type ParseMatch struct {
      Rule  string `json:"rule"`  // 命中的规则名/字段名
      Start int    `json:"start"` // 全文绝对起始字符索引
      End   int    `json:"end"`   // 全文绝对结束字符索引
      Text  string `json:"text"`  // 命中的切片文本
  }
  // TestParseTemplateResult 增：Matches []ParseMatch `json:"matches,omitempty"`
  ```
- **三类场景下的绝对偏移累加算法（彻底消除分块切片相对偏移漂移）**：
  由于 `TreeEngine.fillSubResult` 是递归抽取算法，子节点匹配的正则索引是基于切片后的局部子串 `blk.Body`。必须在递归链路中透传 `baseOffset int`（根文本调用时初始为 `0`），分场景严格累加：
  - **场景 A（分块规则 Block Split，如 `block: "^interface"`）**：
    - 切分分块时，`blk.Start` 为该分块在外层上下文中的相对索引；
    - 递归调用子规则提取时，透传新的绝对基准偏移：`childBaseOffset = baseOffset + blk.Start`；
    - 若需要标记分块界定符自身：`Start: baseOffset + blk.Start, End: baseOffset + blk.Start + len(delimiter)`；
  - **场景 B（表格正则提取 Table Regex，如 `regex: "(?P<ip>\\d+\\.\\d+\\.\\d+\\.\\d+)"`）**：
    - 对当前分块文本 `blk.Body` 执行正则匹配：`idxs := re.FindStringSubmatchIndex(blk.Body)`；
    - 提取字段对应的绝对区间为：
      `absStart := baseOffset + idxs[2*i]`，`absEnd := baseOffset + idxs[2*i+1]`；
    - 记录 `ParseMatch{Rule: fieldName, Start: absStart, End: absEnd, Text: rawText[absStart:absEnd]}`；
  - **场景 C（标量字段提取 Scalar Regex，如首行匹配或全局单值）**：
    - 在当前作用域直接匹配：`idxs := re.FindStringSubmatchIndex(scopeText)`；
    - 绝对区间映射：`absStart := baseOffset + idxs[2*i]`，`absEnd := baseOffset + idxs[2*i+1]`；
    - 同样记录入 `ParseMatch`。
- **开销与隔离**：仅在前端点击"测试解析"（`TestTemplate` 调试模式）时透传开启收集；生产解析 `ParseWithTemplate` 路径不开启收集，保持零额外开销。
- 前端 `EchoTestPane.vue` 按 `matches` 在原文 `<pre>` 中按区间高亮渲染（支持多颜色层叠标识）；无 `matches` 时降级为"平铺表"。

### 6.4 实施步骤

| 步骤 | 文件 | 说明 |
|---|---|---|
| S1 | `frontend/src/components/parsetpl/*.vue`（新建） | 4 个组件（`TemplateList`、`RuleTableEditor`、`EchoTestPane`、`RegexPlayground`） |
| S2 | `frontend/src/views/ParseTemplates.vue` | 改为容器，编排组件与状态 |
| S3 | `internal/models/parse_template.go` | `ParseMatch` + `Matches` + `UserParseTemplateVO.Source` |
| S4 | `internal/ui/parse_template_service.go` | `ListTemplates` 实现内置快照与 DB 用户记录并集汇聚；`TestTemplate` 结合 A/B/C 三场景 `baseOffset` 收集并返回绝对 `Matches` |
| S5 | 前端 `EchoTestPane.vue` | 三视图 + 绝对偏移高亮渲染 |

### 6.5 测试

- `vue-tsc -b`、`npx vite build` 通过；页面手工回归（增删改查、测试三视图、适用范围清空/设置）。
- 后端 `TestTemplate`：tree 模板返回 `matches` 且各区间能精确在原文切片出对应子串（无相对偏移漂移）；regex/aggregate 返回空切片。

### 6.6 风险与回退

| 风险 | 缓解 |
|---|---|
| 一次性重构导致 Vue 响应式失效 | 增量抽取，每步构建验证 |
| 引入拖拽依赖 | 用上移/下移替代，零新增依赖 |
| 高亮后端契约影响生产 | 仅 `TestTemplate` 返回，可选字段，生产路径不采集 |

---

## 7. 数据模型与迁移汇总

| 表/对象 | 变更 | 阶段 | 迁移位置与闭环落点 |
|---|---|---|---|
| `task_runs` | 新增 `metrics_json TEXT`（可空） | §4 | `internal/taskexec/persistence.go:516` `AutoMigrate`；**并在 `UpdateRun` 增加字段映射字典**；`eventbus.go` `ApplyRunPatch` 同步更新 |
| `runtime_settings` | 新增数据行 `key='last_backup_version'`（非结构变更） | §1 | `cmd/netweaver/main.go` 在所有迁移通过后显式提交写入 |
| `global_settings` | 新增 `inspection_pipeline_mode`（可空/默认 `single`） | §5 | `internal/config/db.go:99` AutoMigrate |
| `inspection_templates` | 新增 `groups JSON`（可空） | §5 | `internal/config/db.go:118` AutoMigrate |
| `inspection_item_texts`（新表） | `key+locale` 唯一索引（英文建议与国际化报表） | §5 | `internal/config/db.go` AutoMigrate 追加 |
| `Identity`（领域模型） | 新增 `Handler string`（可空） | §4 | `internal/device/identity.go`，由 `handlers.go` 识别时回填 |
| `UserParseTemplateVO`（API 模型） | 新增 `Source string`（`"builtin" \| "user"`） | §6 | `internal/models/parse_template.go`，由 `parse_template_service.go` 并集汇聚回填 |
| `SessionContext`（运行时上下文） | 新增 `CurrentView View` 与 `Vendor string` | §2 | `internal/executor/session_types.go` |
| `RunDataHolder`（快照接口） | 支持命令级回退 `GetCommandEcho(deviceIP, commandKey)` | §5 | `internal/taskexec/run_data_holder.go` |
| 执行器 StageKind（代码常量） | 新增 `inspection_collect` / `inspection_parse` | §5 | `internal/taskexec/status.go` |

> **兼容性**：全部为"只增"变更；新增列可空；旧记录读取不受影响。降级不承诺（旧程序忽略新列/新表）。

---

## 8. 验收标准（DoD）与测试矩阵

### 8.1 每项 DoD（通用）
1. `go build ./...` 通过；`go test ./...` 全绿；新增模块覆盖率 ≥ 70%。
2. `vue-tsc -b` + `vite build` 通过（涉及前端项）。
3. 新功能/既有变更同步更新对应文档与 README。
4. 不破坏既有 golden 与回归样本（`testdata/regression/`）。
5. 新增"自动行为"接入灰度开关，默认最保守档。
6. 新表/新列均为可空或带默认值，升级方向兼容。

### 8.2 测试矩阵

| 工作项 | 单元测试 | 集成/回归 | 手工验收 |
|---|---|---|---|
| 升级备份 | `config/backup_test.go` | 1. 首次安装（Pre-Open 探测文件不存在）不产生空备份；<br>2. 用旧库启动触发 WAL checkpoint 镜像备份；<br>3. 模拟 `taskexec.AutoMigrate` 失败，验证下次启动重试备份 | 检查 `backup/db/` 产物与 WAL 一致性；验证版本号注入回填 |
| 缓存视图 | `matcher/view_test.go`、`command_cache_test.go` | 1. 华为/H3C/Cisco 各视图反解单测；<br>2. 同命令跨视图（OSPF vs Interface）隔离不命中；<br>3. `SessionContext` 跨命令视图状态继承；<br>4. 从 `deviceProfile.Vendor` → `opts.Vendor` 提取厂商 | 日志包含 `[cache:unknown_view]` 或视图 tag |
| CEAS/设备 golden | `ceas/golden_test.go`、`device/golden_test.go` | 1. `flag.Bool("update")` 仅限测试文件，生产 CLI 无 `-update` 参数；<br>2. `UPDATE_GOLDEN=1 go test ./...` 稳定刷新、无跨包参数报错 | 人工审阅关键字段（ESN/Slot/Identity） |
| metrics | `metrics/registry_test.go`、parser outcome 测试 | 1. 并发两 Run 互不串扰；<br>2. `device.handler` 从 `Identity.Handler` 正确提取并打点；<br>3. `ceas.node_count_range` 正确落入离散分桶；<br>4. `UpdateRun` 与 `SnapshotHub` 字段映射不丢弃 | 执行详情页指标卡片展示真实指标与分布 |
| P4 三阶段 | 编译器/依赖策略/批次容错单测 | 1. 50 台设备中 1 台失败，其余 49 台正常出报告；<br>2. 单机多命令中部分命令采集失败时，细粒度命令级容错回退；<br>3. Stage 状态由 `progress_projector` 自动投影；<br>4. `evaluateStageFailurePolicy` 巡检非中止验证；<br>5. `single` vs `three_stage` 判定结果完全一致 | 事件流可见 3 Stage，采集完物理连接即刻释放 |
| 组件化 | — | 1. `ListTemplates` 内置与用户覆盖并集汇聚展示；<br>2. A/B/C 三类场景 `baseOffset` 绝对偏移切片高亮校验；<br>3. `vue-tsc`/`vite build` 构建通过；<br>4. 适用范围清空契约回归 | 三视图高亮精确不漂移，一键克隆覆盖正常 |

---

## 9. 风险与红线

### 9.1 关键风险

| 风险 | 概率 | 影响 | 缓解 |
|---|---|---|---|
| 备份拖慢/占满磁盘 | 中 | 中 | 保留 5 份 + 耗时告警 + 失败不阻断 |
| 首次安装误判产生空快照 | 低 | 低 | **v1.2 规避**：在 `gorm.Open` 前执行 `os.Stat` 判定，杜绝 0 字节库伪升级 |
| 备份标记过早回写导致漏备 | 中 | 高 | **v1.1 规避**：标记提交滞后至 `main.go` 中 `taskexec.AutoMigrate` 成功之后 |
| 测试 flag 污染生产 CLI | 低 | 中 | **v1.2 规避**：`flag.Bool` 严格限制在 `*_test.go`，全仓与 CI 走环境变量 |
| 视图误判导致缓存误复用 | 中 | 中 | 仅提示符驱动；细分子视图避免碰撞；默认 off |
| 跨命令视图状态丢失 | 中 | 高 | **v1.1 规避**：将视图主状态收敛在会话级 `SessionContext` |
| 提示符改动引入识别回归 | 低 | 高 | `IsPrompt` 保留薄包装 + 回归测试 |
| 全局取指标差值导致串扰 | 中 | 高 | **禁止**；RunID 分桶 + Options 注入 |
| 巡检三阶段错误阻断扩散 | 中 | 致命 | **v1.2 规避**：阶段状态自动投影推导，细化为命令级回退，失败策略非中止 |
| 巡检三阶段结果漂移 | 中 | 高 | 默认 single；对照回归通过后切默认 |
| 递归分块高亮偏移错位 | 中 | 中 | **v1.2 规避**：覆盖 A/B/C 三类场景显式累加绝对字符基准偏移 |
| 内存快照 OOM | 中 | 中 | 容量上限 64MB + 磁盘回退 |
| SQLite 写入放大 | 中 | 中 | 三阶段**零中间表**，仅原始回显落盘 |
| 组件化重构 UI 故障 | 中 | 中 | 独立批次、增量抽取、每步构建验证 |
| golden 掩盖既有 bug | 中 | 中 | 首次生成后人工审阅关键字段 |

### 9.2 绝对红线（不可违反）
1. 不引入 Python/Jython/Py 运行时。
2. 不建 XML 注册表矩阵。
3. 不做常驻采集/轮询/Trap。
4. 自动应答绝不作用于风险命令。
5. 任何输出到报告的路径必须脱敏。
6. 自动行为默认最保守档；新增自动阻断/应答/缓存必须先灰度。
7. 数据库迁移只增不减。

---

## 10. 审核记录（自审）

### 10.1 已核实事实（本方案设计依据）

| 结论 | 证据 |
|---|---|
| `InitDB` 无备份、迁移前无钩子 | `internal/config/db.go:28-93` |
| `MirrorDatabaseToPath` 可复用且含 WAL checkpoint | `internal/config/db.go:162-190` |
| `GetBackupDir()` 返回的是 `backup/config` | `internal/config/paths.go:243-247` |
| `runtime_settings` 表存在且 `key` 唯一 | `internal/models/models.go:234-246` |
| 构建注入 `main.version` 但该变量不存在 | `build/windows/Taskfile.yml:57,90`；`cmd/netweaver` 搜索 0 命中 |
| 缓存键仅命令字符串 | `internal/executor/command_cache.go:65,84`；`stream_engine.go:486,801` |
| `IsPrompt` 仅返回 bool | `internal/matcher/matcher.go:126,211` |
| `StreamEngine`/`DeviceExecutor` 非单例 | `internal/executor/executor.go:357`；`NewDeviceExecutor` 每 Unit 构造 |
| `TaskRun` 无 metrics 列 | `internal/taskexec/models.go:76-99` |
| Run 终态收口点 | `internal/taskexec/runtime.go:537,543,548,551` |
| `Identity` 模型缺 `Handler` 字段 | `internal/device/identity.go:12-23`（`handlers.go:235-241` 命中未存） |
| `ListTemplates` 仅查 DB 缺内置并集 | `internal/ui/parse_template_service.go:28-47`（`repo.FindAll`） |
| 巡检单阶段内联 | `internal/taskexec/inspection_compiler.go:119-132`、`inspection_executor.go:131-364` |
| 依赖策略与失败策略解耦需求 | `internal/taskexec/orchestration_policy.go:11-14,36-49` |
| 阶段终态由 Unit 状态投影推导 | `internal/taskexec/progress_projector.go:96-140`（`projectStageCompletion`） |
| 执行器 kind 唯一注册 | `internal/taskexec/service.go`；`DeviceCollectExecutor`/`ParseExecutor` 的 `Kind()` |
| `TestTemplate` 无命中区间 | `internal/models/parse_template.go:67-72` |
| 前端无 `components/parsetpl/`、无拖拽依赖、无全局 i18n | 全仓组件名 0 命中；`frontend/package.json` |
| `ParseTemplates.vue` 已有适用范围编辑 | `ParseTemplates.vue:250-267,620-621,731-744` |

### 10.2 已纠正的前期误判（避免方案重复踩坑）
1. **`StreamEngine` 非单例**：原"全局区间差值"方案作废，改为 `ExecutorOptions` 注入 RunID + 阶段层打点。
2. **`ParseTemplates.vue` 已有适用范围入口**：原"前端无入口"判断错误；真实缺陷是"清空契约失效"，已单独修复。
3. **`parser/golden_test.go` 没有 `-update`**：原"沿用既有 `-update`"说法不成立，本方案将其列为**新增**能力。
4. **巡检三阶段错误级联放大（v1.1 纠正）**：原设计套用拓扑依赖策略，会导致单台设备采集失败直接跳过整批后续阶段；v1.1 明确采用"设备级容错流转，非全败不跳阶段"。
5. **视图状态单命令即焚（v1.1 纠正）**：原设计将 `CurrentView` 置于 `CommandContext` 会导致跨命令视图丢失；v1.1 改为收敛于会话级 `SessionContext`。
6. **备份版本标记提早写入（v1.1 纠正）**：原设计在 `InitDB` 内部写入标记，会导致后续 `taskexec.AutoMigrate` 失败时漏备；v1.1 改为在 `main.go` 统一提交。
7. **首次安装 SQLite Pre-Open 判定时机（v1.2 纠正）**：`gorm.Open` 会在初始化前创建空文件导致误判，必须在 Open 之前执行 `os.Stat` 检查。
8. **MirrorDatabase 全局 DB 时序解耦（v1.2 纠正）**：显式透传 `*gorm.DB` 实例，彻底消除并发与未初始化风险。
9. **模板列表缺失内置规则（v1.2 纠正）**：`ListTemplates` 改造为内置快照与 DB 用户记录并集汇聚，透出 `Source` 徽标。
10. **测试 Flag 泄漏污染生产 CLI（v1.2 纠正）**：`flag.Bool("update")` 严格限制在 `*_test.go`，全仓与 CI 走环境变量。
11. **巡检容错细化为命令级（v1.2 纠正）**：由整机级粗粒度回退细化为 `(deviceIP, commandKey)` 命令级回退。
12. **Identity.Handler 来源闭环（v1.2 纠正）**：模型增加 `Handler` 字段，并在识别器有序规则表命中时回填。
13. **高亮偏移全场景闭环（v1.2 纠正）**：覆盖 A（分块）、B（表格）、C（标量）三类场景显式累加绝对字符基准偏移。

### 10.3 已确认决策
| # | 决策 | 结论与采纳方案 |
|---|---|---|
| D1 | 命令缓存默认值 | **保持 `off`**（红线 6），同步修正规划方案 §10.3 的 `on` |
| D2 | 版本号来源 | 新增 `main.version` 变量（接通 ldflags 注入） |
| D3 | 巡检三阶段默认开关 | `single` 起步，对照回归通过后切 `three_stage` |
| D4 | 内存快照容量 | 设定 64MB/Run 上限，超出平滑走磁盘回退 |
| D5 | `TemplateList` 并集展示与徽标 | 后端 `ListTemplates` 并集汇聚，VO 显式增加 `Source` 字段精准标记 |
| D6 | 多语言文案定位 | 收敛于巡检报告导出（如涉外英文报表），暂不扩大至前端 UI 国际化 |
| D7 | 测试参数隔离 | `flag.Bool("update")` 严禁入生产代码，全仓与 CI 刷新统一使用 `UPDATE_GOLDEN=1` |
| D8 | 巡检阶段终态收敛 | 阶段完成状态由 `progress_projector.go` 自动从 Unit 状态投影推导，执行器不手动覆盖 |

### 10.4 核心执行契约与实测指引

#### 10.4.1 启动期严格执行顺序契约
```
1. stat(dbPath)           # 必须在 gorm.Open 之前判定首次安装：isExistingDB := stat.err == nil && size > 0
  ↓
2. gorm.Open(dsn)         # 打开 SQLite 连接
  ↓
3. DB = db                # 立即赋值全局变量
  ↓
4. EnsurePreUpgradeBackup(db, dbPath, isExistingDB)  # 探针 + 备份（纯只读，绝不写库、绝不自动建表）
  ↓
5. autoMigrateAll(db)     # 基础表迁移
  ↓
6. taskexec.AutoMigrate   # 运行时表迁移（TaskRun 等）
  ↓
7. CommitPreUpgradeBackupVersion(db)  # 全部迁移成功后，终态统一回写版本标记
```

#### 10.4.2 实施前实测指引
1. **底层探针异常测试**：使用 `sqlDB.QueryRow` 确保无表时优雅返回 error，不 panic，不污染控制台日志。
2. **`defaultRuntimeContext` 生命周期**：确认无论正常结束、用户取消还是 panic recover，`finalizeRunResources` 均被执行，确保 holder 与 metrics bucket 及时释放。
3. **`MatchPrompt` 抽取等价性**：运行既有 `matcher_test.go` 保证提取出的整行与原匹配逻辑完全吻合。
4. **`inspection_results` 对照回归口径**：按 `(run_id, device_ip, item_code)` 对比 `Status`、`Severity`、`Problem`、`Advice`，忽略数据库自增 ID 与更新时间微小差异。

---

## 附录 A：文件落点索引

| 工作项 | 新增 | 修改 |
|---|---|---|
| 升级备份 | `internal/config/version.go`、`internal/config/backup.go`、`internal/config/backup_test.go` | `cmd/netweaver/main.go`、`internal/config/db.go`、`internal/config/paths.go`、`README.md` |
| 缓存视图 | `internal/matcher/view.go`、`internal/matcher/view_test.go` | `internal/matcher/matcher.go`、`internal/executor/session_types.go`、`internal/executor/session_adapter.go`、`internal/executor/stream_engine.go`、`docs/eDeskPro能力升级适配规划方案.md` |
| CEAS/设备 golden | `internal/ceas/golden.go`（纯函数库）、`internal/ceas/golden_test.go`（含 flag）、`internal/device/golden.go`、`internal/device/golden_test.go`、`testdata/ceas/*_expected.json`、`testdata/device/**` | `internal/ceas/elabel_test.go`、`internal/device/identity.go`（增 `Handler` 字段）、`internal/device/handlers.go` |
| metrics | `internal/metrics/*` | `internal/executor/executor.go`、`internal/taskexec/*`、`internal/parser/*`、`internal/taskexec/persistence.go`、`internal/taskexec/eventbus.go`、前端执行详情 |
| P4 三阶段/分组/多语言 | `internal/taskexec/run_data_holder.go`、`inspection_collect_executor.go`、`inspection_parse_executor.go` | `internal/taskexec/{status,service,runtime,inspection_compiler,inspection_executor,orchestration_policy,progress_projector}.go`、`internal/models/inspection.go`、`internal/config/db.go`、`internal/inspection/seeds.go`、前端 `Inspection.vue` |
| 组件化 | `frontend/src/components/parsetpl/*.vue` | `frontend/src/views/ParseTemplates.vue`、`internal/models/parse_template.go`、`internal/ui/parse_template_service.go` |

## 附录 B：变更记录

| 版本 | 日期 | 说明 |
|---|---|---|
| v1.0 | 2026-09-11 | 初稿：覆盖 6 组未实施项；纳入前期评审纠正（StreamEngine 非单例、适用范围入口已存在、parser golden 无 `-update`），给出分级批次、阶段契约与自审清单 |
| v1.1 | 2026-09-11 | 架构评审升级版：<br>1. 修复巡检三阶段因单台设备失败阻断整批任务的 P0 架构缺陷，重构为设备级容错；<br>2. 修复命令缓存视图状态在 `CommandContext` 跨命令丢失的 P0 缺陷，上移至 `SessionContext` 并细分子视图；<br>3. 修复升级备份标记在 `InitDB` 内部过早提交导致后续 `taskexec` 迁移失败时漏备的 P0 缺陷；<br>4. 修复规则树调试高亮在递归分块下的相对偏移漂移问题（透传 `baseOffset`）；<br>5. 补齐 `RunPatch.MetricsJSON` 在 `persistence.UpdateRun` 与 `SnapshotHub.ApplyRunPatch` 中的映射闭环；<br>6. 增加 `UPDATE_GOLDEN=1` 环境变量支持；<br>7. 澄清多语言文案收敛于报表导出，VO 显式增加 `Source` 徽标字段。 |
| v1.2 | 2026-09-11 | 二轮架构闭环升级版：<br>1. **首次安装备份时序规避（A1/C3）**：在 `gorm.Open` 之前通过 `os.Stat` 判定既有库，探针使用底层 `sql.DB.QueryRow`，杜绝空库误备份与建表 Panic；<br>2. **显式透传解耦（A2）**：`MirrorDatabaseToPath` 解除对未初始化全局 `DB` 的隐式依赖，显式接收连接实例；<br>3. **测试参数隔离（C1）**：`flag.Bool("update")` 严格限制在 `*_test.go` 中，杜绝生产 CLI 参数污染，全仓与 CI 统一走环境变量；<br>4. **形态处理器指标来源闭环（B1）**：`Identity` 增加 `Handler` 字段，并在 `handlers.go` 规则命中时回填，打通 `device.handler` 指标链路；<br>5. **指标分桶离散契约（B2）**：`ceas.node_count` 规范为计数器求和 + labels 离散分桶区间；<br>6. **巡检编排策略与容错深化（B3/B4）**：澄清 `evaluateStageFailurePolicy` 巡检非中止行为；明确阶段终态由 `progress_projector.go` 自动推导；容错回退由整机粗粒度细化为 `(deviceIP, commandKey)` 命令级回退；<br>7. **会话视图厂商来源规范（B5）**：规范视图反解时的 `vendor` 提取顺序（`deviceProfile.Vendor` → `opts.Vendor`）；<br>8. **模板工作台并集汇聚（A3）**：`ListTemplates` 汇聚内置快照与 DB 用户记录，前端完整透出 `Source` 徽标与一键克隆；<br>9. **全场景绝对高亮偏移算法（C2）**：完整定义 Block Split、Table Regex、Scalar Regex 三类场景下的全局字符偏移累加规范。 |


