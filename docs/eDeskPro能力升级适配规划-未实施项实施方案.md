# NetWeaverGo × eDeskPro 能力升级适配 — 未实施项实施方案

> **版本**: v1.0
> **依据**: [`docs/eDeskPro能力升级适配规划方案.md`](eDeskPro能力升级适配规划方案.md) v1.1、[`docs/审计报告.md`](审计报告.md)
> **定位**: 对规划方案 P0–P4 中尚未落地的 6 组能力给出可实施工程方案（现状核实 / 设计 / 落点 / 测试 / 风险）
> **编写约束**: 所有"现状"结论均以当前代码为准并标注 `文件:行号`；凡未经代码确认的推断显式标注「待核实」
> **范围**: 只描述实施，不替代 P0–P4 原始需求文档

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
6. `autoMigrateAll` 迁移失败时**不会**被标记为"已备份"，下次启动仍会备份。

### 1.2 现状核实

| 事实 | 证据 |
|---|---|
| 启动直接迁移、无备份 | `internal/config/db.go:28` `InitDB()` → `:67` `autoMigrateAll(db)` |
| 迁移表清单 | `internal/config/db.go:96-121` `autoMigrateAll` |
| 已有镜像能力可复用 | `internal/config/db.go:162-190` `MirrorDatabaseToPath`（含 `PRAGMA wal_checkpoint(FULL)` 与 `-wal/-shm` 拷贝） |
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
- 兜底：`version == "dev"` 时文件名用 `dev` + 时间戳（不退化为无备份）。

#### 1.3.2 探针与顺序（关键：避免"无表启动"与"脏库无备"）
执行顺序（均在 `autoMigrateAll` **之前**）：

1. `dbPath := paths.GetDBPath()`；`os.Stat`：不存在或 `Size()==0` → 首次安装，**跳过**。
2. **低阶只读探针**（不经 GORM 模型，避免表缺失报错/事务 Panic）：
   ```sql
   SELECT value FROM runtime_settings WHERE key = 'last_backup_version' LIMIT 1
   ```
   - 用 `db.Raw(...).Scan(&v)`；**任何报错（含 `no such table`）一律视为"必须备份"**。
3. 探针值 == `appVersion` → 跳过。
4. 执行备份：`target = <storageRoot>/backup/db/netweaver_<version>_<ts>.db`，调用 `MirrorDatabaseToPath(dbPath, target)`。
5. **迁移成功后**才回写标记：`autoMigrateAll(db)` 返回 nil 后 Upsert `runtime_settings(key='last_backup_version', value=appVersion)`；回写失败仅 `Warn`（最多下次重复备份一次）。
6. 备份失败（磁盘满/无权限）：`logger.Warn`，**继续执行迁移**。

> **刻意滞后写入**：宁可多备份一次，也绝不在迁移失败时误判为"已备份"。

#### 1.3.3 目录与保留策略
- `PathManager` 新增字段 `DBBackupDir = <StorageRoot>/backup/db`，加入 `ensureDirectoriesLocked()`（`paths.go:152`）目录创建列表；
- 新增 `func (pm *PathManager) GetDBBackupDir() string`；
- 保留最近 **5** 份：备份后扫描 `netweaver_*.db`（含 `-wal/-shm`），按修改时间倒序删除多余；
- 备份同步执行但**记录耗时**，超阈值（如 3s）打印告警（本期不异步，避免与迁移竞态）。

### 1.4 实施步骤（文件级）

| 步骤 | 文件 | 说明 |
|---|---|---|
| S1 | `cmd/netweaver/main.go` | 新增 `var version = "dev"`；`main()` 中 `config.SetAppVersion(version)` |
| S2 | `internal/config/version.go`（新建） | `appVersion` + `SetAppVersion/GetAppVersion` |
| S3 | `internal/config/paths.go` | 新增 `DBBackupDir` 字段、`GetDBBackupDir()`、加入目录创建列表 |
| S4 | `internal/config/backup.go`（新建） | `EnsurePreUpgradeBackup(db *gorm.DB, dbPath string) error`：探针 + 备份 + 保留 |
| S5 | `internal/config/db.go` | `InitDB`：探针备份（迁移前）→ `autoMigrateAll` → 成功后回写标记 |
| S6 | `README.md` | 新增"升级与回滚"章节 |

### 1.5 测试

- `internal/config/backup_test.go`（新建，临时文件 sqlite）：
  1. 首次安装（无文件）→ 不备份；
  2. 旧库 + 无 `runtime_settings` 表 → 备份成功（验证"无表不崩"）；
  3. 旧库 + 标记为当前版本 → 跳过；
  4. 旧库 + 标记为旧版本 → 备份；
  5. 迁移失败（注入 hook）→ 不写标记。
- 手工验收：真实旧库启动一次，检查 `backup/db/` 产物与 `netweaver.db-wal` 一致性。

### 1.6 风险与回退

| 风险 | 缓解 |
|---|---|
| 大库备份拖慢启动 | 记录耗时并告警；后续可异步化 |
| 备份失败阻断启动 | 明确"仅 Warn、不阻断" |
| 标记写错时机 | 必须在 `autoMigrateAll` 成功后写 |
| 版本号缺失 | `dev` 兜底 + 时间戳 |

---

## 2. 命令缓存视图维度与默认策略

### 2.1 目标与验收

**目标**：命令缓存按"**同设备 + 同视图 + 同命令**"去重，避免跨视图误复用；默认值对齐安全红线。

**验收标准**
1. 同一设备、同一视图下重复命令命中缓存；视图不同不命中。
2. 视图标识来自**设备真实提示符**，不依赖命令跳转推断。
3. 提示符无法判定视图时退化为 `unknown`（仍可复用同命令，但日志标注）。
4. 默认 `command_cache_enabled = off`（对齐红线 6），并同步修正规划方案 §10.3 中 `on` 的表述。

### 2.2 现状核实

| 事实 | 证据 |
|---|---|
| 缓存键仅命令字符串 | `internal/executor/command_cache.go:65` `Get(cmd)`、`:84` `Put(cmd,...)` |
| 调用处仅传命令 | `internal/executor/stream_engine.go:486`（读）、`:801`（写） |
| 无视图字段 | `SessionContext`/`CommandContext` 无 view 状态（`session_types.go` 搜索 0 命中） |
| 提示符匹配只返回 bool | `internal/matcher/matcher.go:126` `IsPrompt(chunk) bool`、`:211` `IsPromptStrict(line) bool` |
| 提示符行可取得但未导出 | `internal/matcher/matcher.go:292` `extractLastNonEmptyLine`（包内私有） |
| 默认值不一致 | `internal/config/settings.go:37` `CommandCacheEnabled: false`，而规划方案 §10.3 标 `on` |

### 2.3 设计

#### 2.3.1 视图来源：基于提示符（非命令跳转表）
命令跳转表在"权限不足导致 `system-view` 失败""`return` 跳回多级"等情况下会状态崩塌。**改为从真实提示符反解视图**。

- `matcher` 增加导出方法（不破坏现有 API）：
  ```go
  // MatchPrompt 返回是否命中提示符及命中的提示符整行文本
  func (m *StreamMatcher) MatchPrompt(chunk string) (bool, string)
  ```
  实现：把 `IsPrompt` 主体抽为私有 `isPromptLine(clean, line) bool`；`IsPrompt` 保留为 `ok, _ := MatchPrompt(chunk)` 的薄包装，**保证零语义变化**。
- 视图归一化（新文件 `internal/matcher/view.go`，纯函数、可单测）：
  ```go
  type View string
  const (
      ViewUser       View = "user"
      ViewSystem     View = "system"
      ViewInterface  View = "interface"
      ViewDiagnose   View = "diagnose"
      ViewPrivileged View = "privileged"
      ViewConfig     View = "config"
      ViewUnknown    View = "unknown"
  )
  func ResolveView(vendor, promptLine string) View
  ```
  映射规则（从 prompt 行提取）：
  - 华为：`<host>` → `user`；`[host]` → `system`；`[host-diagnose]` → `diagnose`；含 `-` 且非 diagnose（如 `[host-GE0/0/1]`）→ `interface`；
  - Cisco：`host>` → `user`；`host#` → `privileged`；`host(config)#` / `host(config-if)#` → `config`；
  - 其他/无法判定 → `unknown`。
- 视图在**命令结束时**更新：`StreamEngine` 命中提示符处调用 `MatchPrompt`，得到行文本后 `ctx.SetCurrentView(ResolveView(vendor, line))`；同时依据命令回显错误行（`Matcher.MatchErrorRule`）避免"视图切换失败仍变更视图"——**仅当提示符变化才更新视图**。

#### 2.3.2 缓存键
- `CommandContext` 增加 `CurrentView View`（默认 `unknown`）。
- 键格式：`cacheKey := string(view) + "|" + cmd`。
- 改造点：`command_cache.go` 的 `Get/Put` 签名不变（仍收字符串 key），由 `stream_engine.go:486/801` 组装 key；`command_cache_test.go` 增补视图维度用例。
- 兼容：`unknown` 视图仍复用同命令（不牺牲现有收益），仅记日志。

#### 2.3.3 默认值与文档
- `settings.go:37` 保持 `false`（符合红线 6）。
- 规划方案 §10.3 中 `command_cache_enabled` 初始态 `on` 修正为 `off`（**这是文档内部矛盾**，以红线为准）。
- `loadSettingsFromDB`（`settings.go:67-114`）对 bool 无空值兜底，本次不改（默认 false 与零值一致）。

### 2.4 实施步骤

| 步骤 | 文件 | 说明 |
|---|---|---|
| S1 | `internal/matcher/matcher.go` | 抽出 `isPromptLine`；新增 `MatchPrompt` |
| S2 | `internal/matcher/view.go`（新建） | `View` 常量 + `ResolveView` |
| S3 | `internal/executor/session_types.go` | `CommandContext` 增 `CurrentView`、`SetCurrentView` |
| S4 | `internal/executor/stream_engine.go` | 提示符命中处更新视图；`:486/:801` 用 `view|cmd` 组装键 |
| S5 | `internal/config/settings.go` | 保持 `false`（不加改动，仅确认） |
| S6 | `docs/eDeskPro能力升级适配规划方案.md` | §10.3 表 `on` → `off` |

### 2.5 测试

- `internal/matcher/view_test.go`（新建）：华为/Cisco 各提示符 → View 映射；未知 → `unknown`。
- `internal/matcher/matcher_test.go`：`MatchPrompt` 行为与 `IsPrompt` 完全一致（回归）。
- `internal/executor/command_cache_test.go`：同命令不同视图不命中；同命令同视图命中。

### 2.6 风险与回退

| 风险 | 缓解 |
|---|---|
| 提示符识别不准导致视图误判 | 无法判定即 `unknown`；只影响命中率，不影响正确性 |
| `MatchPrompt` 改动引入提示符识别回归 | `IsPrompt` 保留薄包装 + 回归测试 |
| 视图切换失败误判 | 仅"提示符变化"才更新视图 |

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

#### 3.3.2 `-update` 开关（新增，非沿用）
- 在包内定义：`var updateGolden = flag.Bool("update", false, "update golden files")`（每个包一个，或集中于测试辅助文件）。
- 测试逻辑：
  ```go
  if *updateGolden { os.WriteFile(goldenPath, prettyJSON(got), 0644); return }
  ```
- 注意：`go test ./...` 与 `-run` 组合时 flag 注册是包级的，需避免重复注册（每包仅一处）。

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
| S1 | `internal/ceas/golden.go` + `golden_test.go`（新建） | 规范化器 + `-update` + 6 族 golden |
| S2 | `testdata/ceas/*_expected.json`（新建 6 份） | 首次用 `-update` 生成并人工审阅 |
| S3 | `internal/device/golden.go` + `golden_test.go`（新建/扩展） | 规范化器 + `-update` |
| S4 | `testdata/device/versions/*.txt`（新建） | 各产品线回显样本 |
| S5 | `testdata/device/identities.json`、`fallback_cases.json`（新建） | 期望身份与四级回退用例 |

### 3.5 测试

- `go test ./internal/ceas/... ./internal/device/...` 全绿；
- `go test ./internal/ceas/ -run TestParseELabel_Golden -update` 可刷新且刷新后无 diff；
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

#### 4.3.1 包结构（无外部依赖，`sync/atomic`）
```go
// internal/metrics/registry.go
type Registry struct{ mu sync.Mutex; runs map[string]*RunBucket }
func (r *Registry) Inc(runID, key string, delta int64)
func (r *Registry) Observe(runID, key string, v float64)
func (r *Registry) LabelInc(runID, key, label string)      // 有界 label（handler/结果码/匹配档位）
func (r *Registry) Snapshot(runID string) map[string]any
func (r *Registry) Release(runID string)
var Default = NewRegistry()
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

#### 4.3.3 打点清单（落点已在代码中）

| 指标 key | 打点位置 |
|---|---|
| `parse.template_hit` / `parse.miss` / `parse.fallback` | `taskexec` 中 `cliParser.Parse` 调用点（配合 `DetailedParser`） |
| `cache.hit` / `cache.miss` | `internal/executor/stream_engine.go:486` / `:801` |
| `confirm.triggered`（按 policy label） | `internal/executor/session_reducer.go` `handleConfirmSeen` 各分支 |
| `risk.hit`（label=block/confirm/warn） | `internal/executor/stream_engine.go:377-467` |
| `echo.truncated` | `internal/executor/command_context.go` `AppendRawData` 置 `Truncated` 处 |
| `device.handler`（label=handler 名） | `internal/taskexec/executor_impl.go:1136` `device.Identify(...)` 后 |
| `profile.match_path`（label=exact/series/vendor/global） | `config.ResolveProfile` 返回值消费点 |
| `ceas.parse_fail` / `ceas.node_count` | `internal/taskexec/ceas_executor.go:249-260` |
| `inspection.result_code`（label=TEST_*） | `internal/taskexec/inspection_executor.go:302-310` |

#### 4.3.4 聚合与落库
- 在 `runtime.go:537` 之后、`emitProjectedRunEvent` 之前：
  ```go
  snapshot := metrics.Default.Snapshot(run.ID)
  if b, err := json.Marshal(snapshot); err == nil {
      s := string(b)
      handler.UpdateRunBestEffort(runtimeCtx, &RunPatch{MetricsJSON: &s}, "写入运行指标")
  }
  ```
- `RunPatch` 增 `MetricsJSON *string`（`models.go:196-203`）。
- journal：经 `ExecutionLogStore.WriteJournalRecord` 写入摘要（与现有日志链路一致）。
- `finalizeRunResources`（`runtime.go:551`）末尾调用 `metrics.Default.Release(runID)`。

### 4.4 实施步骤

| 步骤 | 文件 | 说明 |
|---|---|---|
| S1 | `internal/metrics/*`（新建） | Registry/RunBucket/Histogram + 单测 |
| S2 | `internal/executor/executor.go` | `ExecutorOptions.RunID`；构造时透传 |
| S3 | `internal/taskexec/*` | 各 executor 构造处传 `ctx.RunID()`；各打点 |
| S4 | `internal/parser/*` | `ParseOutcome` + `DetailedParser`（兼容包装） |
| S5 | `internal/taskexec/models.go`、`persistence.go` | `TaskRun.MetricsJSON`、`RunPatch.MetricsJSON`；AutoMigrate 自动加列 |
| S6 | `internal/taskexec/runtime.go` | 终态聚合写入 + `Release` |
| S7 | 前端执行详情 | 新增"本次运行指标"卡片（读 `metricsJson`） |

### 4.5 测试

- `internal/metrics/registry_test.go`：并发 `Inc` 计数正确；`Snapshot` 稳定；`Release` 后不泄漏。
- `internal/parser`：`ParseDetail` 对 tree/regex/aggregate 及 fallback 分支的 outcome 正确。
- `internal/taskexec`：Run 终态写入 `metrics_json`；并发两 Run 不串扰。

### 4.6 风险与回退

| 风险 | 缓解 |
|---|---|
| 多 Run 并发串指标 | RunID 分桶 + ExecutorOptions 注入；**禁止全局差值** |
| label 基数爆炸 | 仅使用有限集合做 label；命名白名单校验 |
| 指标写入拖慢终态 | 内存快照 + `BestEffort` 写库 |

---

## 5. P4 巡检三阶段编排 + 分组 + 多语言

### 5.1 目标与验收

**目标**：把巡检从"单阶段内联采集+解析+判定"拆为 `inspection_collect → inspection_parse → inspection_check` 三阶段，实现"采集完成后尽早释放设备会话"；补齐模板分组树与多语言文案表。

**验收标准**
1. 一次巡检 Run 产生 3 个 Stage，事件流可见；`inspection_collect` 成功后所有设备连接已释放。
2. 前一阶段失败时后续阶段被 `skipped`（需泛化 stage 依赖策略）。
3. 判定结果与现有单阶段实现**逐条一致**（对照回归）。
4. **不引入中间解析实体表**，避免 SQLite 写入放大与 WAL 膨胀。
5. `InspectionTemplate.Groups` 可持久化并在前端按分组展示。
6. `inspection_item_texts` 表可建、可种子、可按 locale 取文案（缺失回退默认）。

### 5.2 现状核实

| 事实 | 证据 |
|---|---|
| 编译器只产出单 Stage | `internal/taskexec/inspection_compiler.go:119-132`（`Kind = StageKindInspectionCheck`） |
| 执行器内联全流程 | `internal/taskexec/inspection_executor.go:131-364`：连接 `:158-197`、采集 `:234-283`、解析 `:262-282`、判定 `:286-310`、落库 `:312-329` |
| 采集/解析执行器已存在但绑定拓扑 | `internal/taskexec/executor_impl.go:397` `DeviceCollectExecutor`（device_collect，命令解析用 `TopologyCommandResolver`）；`:769` `ParseExecutor`（parse，调用 `:870` `parseAndSaveRunDevice`） |
| 注册表按 StageKind 唯一 | executor 通过 `Kind()` 注册（`internal/taskexec/service.go`），**同一 kind 不能挂两个执行器** |
| 依赖策略仅对拓扑生效 | `internal/taskexec/orchestration_policy.go:11-14` `if runKind != topology → return false`；`:41-49` 失败中止同样仅拓扑 |
| RuntimeContext 接口 | `internal/taskexec/runtime.go:21-42`（`RunID/Context/Update*/Emit/Logger/IsCancelled/GetDeviceLogPaths`） |
| 巡检原始回显已落盘 | `internal/taskexec/inspection_executor.go:256-260` + `internal/config/paths.go:386` `GetInspectionRawFilePath` |
| 模板无分组树 | `internal/models/inspection.go` `InspectionTemplate` 无 `Groups` |
| 多语言表不存在 | 全仓 grep `inspection_item_texts` 仅命中规划文档 |

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

#### 5.3.3 阶段契约：内存快照 + 落盘原始（**零中间表**）
- 新增可选接口（由 `defaultRuntimeContext` 实现，`finalizeRunResources` 中释放）：
  ```go
  // internal/taskexec/run_data_holder.go
  type RunDataHolder interface {
      SetCommandEcho(deviceIP, commandKey, echo string)
      GetCommandEcho(deviceIP, commandKey string) (string, bool)
      SetParsedRows(deviceIP, commandKey string, rows []map[string]string)
      GetParsedRows(deviceIP, commandKey string) ([]map[string]string, bool)
  }
  ```
  实现为带 `sync.RWMutex` 的 `map[string]map[string]...`；**设总字节上限**（建议 64MB/ Run，与 §10.1 预算一致）。
- Stage1 `inspection_collect`：每设备一个 Unit，Steps = 去重命令（`IsPreCollect` 项优先）；写原始回显文件（复用 `GetInspectionRawFilePath`）+ `SetCommandEcho`。
- Stage2 `inspection_parse`：每设备一个 Unit；优先读内存 `GetCommandEcho`，内存超限/缺失时回退读原始文件；解析后 `SetParsedRows`。
- Stage3 `inspection_check`：`GetParsedRows` + `GetCommandEcho` → `inspection.EvaluateItem` → 批量写 `inspection_results`（保留现有事务与幂等清理）。
- 超限降级：内存预算耗尽时 Stage2 直接从磁盘读，Stage3 解析结果缺位时按现有逻辑回退原始回显正则（`internal/inspection/engine.go` 已支持）。

#### 5.3.4 依赖策略泛化
`orchestration_policy.go` 由"仅 topology"改为按 RunKind 查表：
```go
var stageDependencies = map[RunKind]map[StageKind]StageKind{
    RunKindTopology: {StageKindParse: StageKindDeviceCollect, StageKindTopologyBuild: StageKindParse},
    RunKindInspection: {StageKindInspectionParse: StageKindInspectionCollect, StageKindInspectionCheck: StageKindInspectionParse},
}
```
失败中止策略同理：巡检的 `inspection_collect` 失败则 skip 后续。

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

#### 5.3.6 多语言文案 `inspection_item_texts`
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
- 加入 `config/db.go` AutoMigrate；`inspection.EnsureInspectionItemTextSeeds()`（`sync.Once` 幂等），按需从内置精简 CSV 导入 `_DESCRIPTION` 作为 `Advice`。
- 关联：`InspectionItem.Code` ↔ `InspectionItemText.Key`；前端按当前 locale 取，缺失回退 `InspectionItem.Name`。

### 5.4 实施步骤

| 步骤 | 文件 | 说明 |
|---|---|---|
| S1 | `internal/taskexec/status.go` | 新增两个 StageKind 常量 |
| S2 | `internal/taskexec/run_data_holder.go`（新建） | 内存快照接口 + 容量上限 |
| S3 | `internal/taskexec/runtime.go` | `defaultRuntimeContext` 实现 holder；`finalizeRunResources` 释放 |
| S4 | `internal/taskexec/inspection_compiler.go` | 依 `InspectionPipelineMode` 产出 1/3 Stage |
| S5 | `internal/taskexec/inspection_collect_executor.go`（新建） | 采集阶段 |
| S6 | `internal/taskexec/inspection_parse_executor.go`（新建） | 解析阶段 |
| S7 | `internal/taskexec/inspection_executor.go` | 改造为纯判定+落库（保留单阶段兼容路径） |
| S8 | `internal/taskexec/orchestration_policy.go` | 依赖/失败策略按 RunKind 查表 |
| S9 | `internal/taskexec/service.go` | 注册新执行器 |
| S10 | `internal/models/inspection.go` | `Groups` + `InspectionItemText` |
| S11 | `internal/config/db.go` | AutoMigrate 加 `InspectionItemText` |
| S12 | `internal/inspection/seeds.go` | `EnsureInspectionItemTextSeeds` |
| S13 | 前端 `Inspection.vue` | 分组树编辑/展示；文案 locale 展示 |

### 5.5 测试

- `inspection_compiler_test.go`：`single` 产 1 Stage、`three_stage` 产 3 Stage 且顺序正确。
- 依赖策略单测：`inspection_collect` 失败 → 后续 `skipped`。
- 对照回归：同一模板/设备集，`single` 与 `three_stage` 的 `inspection_results` 逐条一致。
- 内存上限：超限时 Stage2 走磁盘回退且结果不变。
- `EnsureInspectionItemTextSeeds` 幂等；locale 缺失回退。

### 5.6 风险与回退

| 风险 | 缓解 |
|---|---|
| 三阶段与单阶段结果不一致 | 默认 `single`；对照回归通过后再切换 |
| 内存快照导致 OOM | 容量上限 + 磁盘回退 |
| 执行器注册冲突 | 使用独立 StageKind，不复用拓扑 kind |
| 分组树 JSON 列破坏旧数据 | 可空列，旧记录读取不受影响 |
| 迁移改造引入回归 | 保留 `single` 兼容路径，独立开关回退 |

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

#### 6.3.1 组件拆分（增量、可回退）
- `TemplateList.vue`：vendor/commandKey 过滤、列表、"内置/用户覆盖"徽标（覆盖标记可由后端 VO 增加 `source` 字段，**可选**；缺失时用"是否存在"近似）。
- `RuleTableEditor.vue`：规则表格 + 上移/下移；父字段下拉来源为当前已定义规则；字段校验（`parentItem` 存在、无环）依赖后端保存期预编译（`CompileTreeRules` 已含校验）。
- `EchoTestPane.vue`：粘贴回显 → 调 `TestTemplate` → 三视图。
- `RegexPlayground.vue`：正则实时 try/catch；保存前后端已预编译（双保险）。
- 拆分采用"先抽组件、后删旧代码"，每一步保持可运行。

#### 6.3.2 匹配高亮（后端契约扩展，向后兼容）
- `TestParseTemplateResult` 增可空字段：
  ```go
  type ParseMatch struct{ Rule string `json:"rule"`; Start int `json:"start"`; End int `json:"end"`; Text string `json:"text"` }
  // TestParseTemplateResult 增：Matches []ParseMatch `json:"matches,omitempty"`
  ```
- 后端在 tree 引擎 `fillSubResult` 抽取命中时顺带记录区间（`regexp.FindStringSubmatchIndex` 已可拿到偏移）；仅 `TestTemplate` 调试路径返回，**生产解析不记录**（避免开销）。
- 前端按 `matches` 在原文 `<pre>` 中高亮；无 `matches` 时降级为"仅平铺表"。

### 6.4 实施步骤

| 步骤 | 文件 | 说明 |
|---|---|---|
| S1 | `frontend/src/components/parsetpl/*.vue`（新建） | 4 个组件 |
| S2 | `frontend/src/views/ParseTemplates.vue` | 改为容器，编排组件与状态 |
| S3 | `internal/models/parse_template.go` | `ParseMatch` + `Matches` |
| S4 | `internal/ui/parse_template_service.go` | `TestTemplate` 收集并返回 `Matches`（tree 分支） |
| S5 | 前端 `EchoTestPane.vue` | 三视图 + 高亮渲染 |

### 6.5 测试

- `vue-tsc -b`、`npx vite build` 通过；页面手工回归（增删改查、测试三视图、适用范围清空/设置）。
- 后端 `TestTemplate`：tree 模板返回 `matches` 且区间能覆盖原文；regex/aggregate 不返回（或返回空）。

### 6.6 风险与回退

| 风险 | 缓解 |
|---|---|
| 一次性重构导致 Vue 响应式失效 | 增量抽取，每步构建验证 |
| 引入拖拽依赖 | 用上移/下移替代，零新增依赖 |
| 高亮后端契约影响生产 | 仅 `TestTemplate` 返回，可选字段，生产路径不采集 |

---

## 7. 数据模型与迁移汇总

| 表/对象 | 变更 | 阶段 | 迁移位置 |
|---|---|---|---|
| `task_runs` | 新增 `metrics_json TEXT`（可空） | §4 | `internal/taskexec/persistence.go:516` `AutoMigrate` |
| `runtime_settings` | 新增数据行 `key='last_backup_version'`（非结构变更） | §1 | 运行期 Upsert |
| `global_settings` | 新增 `inspection_pipeline_mode`（可空/默认 `single`） | §5 | `internal/config/db.go:99` AutoMigrate |
| `inspection_templates` | 新增 `groups JSON`（可空） | §5 | `internal/config/db.go:118` AutoMigrate |
| `inspection_item_texts`（新表） | `key+locale` 唯一索引 | §5 | `internal/config/db.go` AutoMigrate 追加 |
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
| 升级备份 | `config/backup_test.go` | 用旧库真实启动 | 检查 `backup/db/` 产物与 WAL 一致性 |
| 缓存视图 | `matcher/view_test.go`、`command_cache_test.go` | 同命令跨视图不命中 | — |
| CEAS/设备 golden | `ceas/golden_test.go`、`device/golden_test.go` | `-update` 幂等、两次运行一致 | 人工审阅关键字段 |
| metrics | `metrics/registry_test.go`、parser outcome 测试 | 并发两 Run 不串扰 | 执行详情页指标卡片 |
| P4 三阶段 | 编译器/依赖策略/对照回归 | `single` vs `three_stage` 结果一致 | 事件流可见 3 Stage |
| 组件化 | — | `vue-tsc`/`vite build` | 三视图与清空契约回归 |

---

## 9. 风险与红线

### 9.1 关键风险

| 风险 | 概率 | 影响 | 缓解 |
|---|---|---|---|
| 备份拖慢/占满磁盘 | 中 | 中 | 保留 5 份 + 耗时告警 + 失败不阻断 |
| 视图误判导致缓存误复用 | 中 | 中 | 仅提示符驱动；无法判定即 `unknown`；默认 off |
| 提示符改动引入识别回归 | 低 | 高 | `IsPrompt` 保留薄包装 + 回归测试 |
| 全局取指标差值导致串扰 | 中 | 高 | **禁止**；RunID 分桶 + Options 注入 |
| 巡检三阶段结果漂移 | 中 | 高 | 默认 single；对照回归通过后切默认 |
| 内存快照 OOM | 中 | 中 | 容量上限 + 磁盘回退 |
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
| 巡检单阶段内联 | `internal/taskexec/inspection_compiler.go:119-132`、`inspection_executor.go:131-364` |
| 依赖策略仅拓扑 | `internal/taskexec/orchestration_policy.go:11-14,41-49` |
| 执行器 kind 唯一注册 | `internal/taskexec/service.go`；`DeviceCollectExecutor`/`ParseExecutor` 的 `Kind()` |
| `TestTemplate` 无命中区间 | `internal/models/parse_template.go:67-72` |
| 前端无 `components/parsetpl/`、无拖拽依赖 | 全仓组件名 0 命中；`frontend/package.json` |
| `ParseTemplates.vue` 已有适用范围编辑 | `ParseTemplates.vue:250-267,620-621,731-744` |

### 10.2 已纠正的前期误判（避免方案重复踩坑）
1. **`StreamEngine` 非单例**：原"全局区间差值"方案作废，改为 `ExecutorOptions` 注入 RunID + 阶段层打点。
2. **`ParseTemplates.vue` 已有适用范围入口**：原"前端无入口"判断错误；真实缺陷是"清空契约失效"，已单独修复。
3. **`parser/golden_test.go` 没有 `-update`**：原"沿用既有 `-update`"说法不成立，本方案将其列为**新增**能力。

### 10.3 待确认决策（需产品/架构确认）
| # | 决策 | 建议 |
|---|---|---|
| D1 | 命令缓存默认值 | **保持 `off`**（红线 6），同步修正规划方案 §10.3 的 `on` |
| D2 | 版本号来源 | 新增 `main.version` 变量（当前 ldflags 空注入） |
| D3 | 巡检三阶段默认开关 | `single` 起步，对照回归通过后切 `three_stage` |
| D4 | 内存快照容量 | 建议 64MB/Run，超出走磁盘回退 |
| D5 | `TemplateList` "内置/用户覆盖"徽标 | 可后端 VO 增 `source` 字段；否则以"存在性"近似 |

### 10.4 待核实项（实施前需实测）
1. **GORM 探针在"表不存在"时的具体行为**：需实测 `db.Raw("SELECT ... FROM runtime_settings ...").Scan()` 返回的 error 形态，确保按"一律视为必须备份"处理且不 panic。
2. **`defaultRuntimeContext` 生命周期**：需确认每 Run 唯一、`finalizeRunResources` 一定会被调用（含取消/异常路径），以保证 holder 无泄漏。
3. **`MatchPrompt` 抽出后 `IsPrompt` 的边界等价性**：需以现有 `matcher_test.go` 用例做等价回归。
4. **`inspection_results` 对照回归口径**：需明确比较字段集合与排序，避免事务/时间戳导致假不一致。
5. **前端 locale 来源**：`inspection_item_texts` 的当前语言从何获取（设置项/浏览器），需确认后再定 API。

---

## 附录 A：文件落点索引

| 工作项 | 新增 | 修改 |
|---|---|---|
| 升级备份 | `internal/config/version.go`、`internal/config/backup.go`、`internal/config/backup_test.go` | `cmd/netweaver/main.go`、`internal/config/db.go`、`internal/config/paths.go`、`README.md` |
| 缓存视图 | `internal/matcher/view.go`、`internal/matcher/view_test.go` | `internal/matcher/matcher.go`、`internal/executor/session_types.go`、`internal/executor/stream_engine.go`、`docs/eDeskPro能力升级适配规划方案.md` |
| CEAS/设备 golden | `internal/ceas/golden.go`、`internal/ceas/golden_test.go`、`internal/device/golden.go`、`testdata/ceas/*_expected.json`、`testdata/device/**` | `internal/ceas/elabel_test.go`、`internal/device/golden_test.go` |
| metrics | `internal/metrics/*` | `internal/executor/executor.go`、`internal/taskexec/*`、`internal/parser/*`、前端执行详情 |
| P4 三阶段/分组/多语言 | `internal/taskexec/run_data_holder.go`、`inspection_collect_executor.go`、`inspection_parse_executor.go` | `internal/taskexec/{status,service,runtime,inspection_compiler,inspection_executor,orchestration_policy}.go`、`internal/models/inspection.go`、`internal/config/db.go`、`internal/inspection/seeds.go`、前端 `Inspection.vue` |
| 组件化 | `frontend/src/components/parsetpl/*.vue` | `frontend/src/views/ParseTemplates.vue`、`internal/models/parse_template.go`、`internal/ui/parse_template_service.go` |

## 附录 B：变更记录

| 版本 | 日期 | 说明 |
|---|---|---|
| v1.0 | 2026-09-11 | 初稿：覆盖 6 组未实施项；纳入前期评审纠正（StreamEngine 非单例、适用范围入口已存在、parser golden 无 `-update`），给出分级批次、阶段契约与自审清单 |

