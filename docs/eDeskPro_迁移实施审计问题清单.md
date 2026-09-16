# eDesk Pro 迁移实施 · 代码审计问题清单

> **审计基准**：[eDeskPro_分阶段迁移方案.md](file:///d:/Document/GO/NetWeaverGo/docs/eDeskPro_分阶段迁移方案.md)（以下简称"方案"）
> **审计范围**：`9de17dc`（迁移前基线）.. `794e483`（HEAD），共 5 个提交：一期 `df06880` / 二期 `25c7fc0` / 三期 `e32f6d2` / 四期 `85ba1f2` / 收尾 `794e483`
> **变更规模**：新增 147 个文件、修改 29 个文件（其中新增 `.go` 约 60 个、数据/规则文件约 80 个）
> **审计日期**：2026-09-16
> **审计方法**：`go build` / `go vet` / `go test ./internal/...` 全量验证 + 逐文件人工审读 + 生产调用点反查（grep/LSP）+ 关键结论编写临时测试实证（已删除，不残留）
> **验证结论**：构建 ✅、vet ✅、`go test ./internal/...` 全绿 ✅（含新增单测）

---

## 0. 总体结论

| 维度 | 结论 |
|------|------|
| 工程质量（可编译、测试通过） | ✅ 良好，新增单测全部通过 |
| 数据资产落地（XML/JSON 规则导入） | ✅ 基本达成：44 份 XML、581/585 条脱敏规则、62 条 DSL 规则、13 条跳板命令模板、19 条告警规则 |
| **功能接线（是否真正进入生产链路）** | ❌ **主要问题**：约 12 个模块"实现存在但无生产调用点"，属于**死代码/断头路** |
| **合规有效性（A1 脱敏）** | ❌ 分厂商脱敏引擎未接入，且分级阻断存在边界漏洞（附实证） |
| 数据一致性（重复文件、embed 路径、基线 SQL） | ⚠️ 存在多处重复数据与路径不一致 |
| 验收标准可验证性 | ⚠️ 多数验收项缺少对应自动化测试（190 款型比对、Shadow 差异率、GBK 回显等） |
| 合规红线 P7（不复制源码/原文） | ✅ 未发现 eDesk Pro 源码或 `#title_cn/#suggest_cn` 原文搬运痕迹 |
| 原则 P3（只增不减） | ✅ 未发现对既有公开接口/结构的删除或重命名 |
| 原则 P4/P5/P6 | ⚠️ 兜底存在；Shadow 未运行；Golden 仅覆盖 XmlConfig 6 模型与脱敏，其余模块缺失 |

**一句话总结**：数据与代码"搬进来了"，但相当一部分"没接上电"。需要优先完成"接线"工作，否则合规（A1）、能力准入（A5）、策略外置（A6）、字符集（B2）、告警（B9）、弱光（B4）、跳板（B10）、接口名（B8）、智能 Ping（B6）、入参校验（C3）等功能对实际业务零效果。

**问题分级统计**：

| 级别 | 数量 | 含义 |
|------|------|------|
| P0 严重 | 9 | 直接导致方案目标失效或存在合规/安全缺口 |
| P1 高 | 10 | 数据/schema/闭环缺陷，可能引发线上问题 |
| P2 中 | 11 | 健壮性、边界、规范偏离 |
| P3 低 | 8 | 工程质量、文档、可选未实施 |

---

## 1. P0 严重问题（模块未接入主链路 / 合规缺口）

### P0-1【A1】分厂商脱敏引擎完全未接入主链路（"死代码"）

- **现象**：`VendorSanitizer` / `GetDefaultVendorSanitizer()` 仅在其自身文件与 `vendor_sanitizer_test.go` 中被引用，**任何生产路径都不调用**；`logger` 新增的 `WithVendor()/WithCommand()/ContextSanitizer` 也没有任何生产调用点。
- **证据**：
  - `internal/report/vendor_sanitizer.go:47`（单例定义）、`:227`（`Sanitize`）——全仓无生产调用点（`internal/report/vendor_sanitizer_test.go:13,30,77` 为唯一调用）。
  - `internal/logger/sanitizer.go:185-214`：`ContextSanitizer.Sanitize()` 只转发 `globalSanitizer.Sanitize(msg)`，**`vendor` / `command` 字段被存储但从未使用**。
  - 方案 §2.1.5 要求的数据流 `logger.Sanitize → VendorSanitizer.Sanitize → CheckContentSanitized` 未实现。
- **实证**（临时测试，已复现）：

  | 输入回显 | 全局 17 条规则后 | `ValidateExportContent` | 厂商引擎（手工调用对照） |
  |---|---|---|---|
  | `dldp authentication-mode simple PlainKey999` | **原文泄露** | **不阻断**（nil） | `simple ****` ✅ |
  | `local-user admin password irreversible-cipher $1a$X#K$...` | `irreversible-cipher ****` | 不阻断 | 同左 |

- **影响**：华为/华三场景中大量"厂商特有形态"口令（如 `dldp authentication-mode simple X`、`snmp-agent ... securityname X`）在导出 CSV/JSON/原始日志时**明文泄露且不被阻断**，直接违反一期"合规基座"目标与 P7 红线精神。
- **建议**：在 `report` 侧提供注册钩子，或将厂商脱敏接入 `VerifyExportContent` 之前的统一入口（`inspection_service`/`hardware_inventory_service`/`bizcompare_service` 等所有导出点），并把 `device.Identity.Series`/`commandKey` 透传进来；`ContextSanitizer` 需真正调用厂商规则或删除以免误导。

### P0-2【A1】分级阻断边界：明文口令被降级为 WARN，可正常导出

- **现象**：`unmaskedPatterns` 把"通用密码未掩码"（`password\s+(\S{2,})`）归为 `SeverityWarn`，而 `ValidateExportContent` 只对 CRITICAL 阻断。
- **证据**：`internal/report/sanitizer_check.go:36-40`（WARN 定义）、`:107-125`（只拦 CRITICAL）。
- **实证**：`password Admin@12345` → `WARN`，`ValidateExportContent` 返回 `nil`（**允许导出**）；`snmp-agent community read public123` → **完全未检出**。
- **影响**：方案 §2.1.6 验收项"脱敏后无 CRITICAL 级阻断"被满足，但真实明文口令仍可能出现在导出物中。
- **建议**：将"`password <非掩码值>`"提升为 CRITICAL（或按"值形态"判定：包含大小写+数字/特殊字符视为真实口令）；补充 `community`、`securityname`、`authentication-mode simple` 等高危形态。

### P0-3【A5】能力准入（eligibility）未接入编译期，"unsupported 标记"未实现

- **现象**：`CheckDeviceEligibility` 只有单测调用（`internal/taskexec/eligibility_test.go:16,25,37,42`），**无任何 Compiler 调用它**；`ProfileRegistry` 的唯一生产调用者 `eligibility.go` 本身也是死代码。
- **证据**：`internal/taskexec/eligibility.go:13`、`internal/device/profile_registry.go:59`（`GetDefaultProfileRegistry` 仅测试 + eligibility 调用）；全仓无 "unsupported" 设备标记逻辑（`grep unsupported` 命中的均为 `unsupported target type` 旧错误分支）。
- **方案条款**：§2.2.6 "能力准入在 `PlanCompiler.Compile()` 阶段生效（不支持的设备标记 `unsupported` 而非失败）"。
- **影响**：190 款型能力矩阵、`device_capabilities` 表（seed 5 条）对任务编排无任何约束作用。
- **建议**：在 `BizCompare`/`Inspection`/`Topology` 编译器中调用 `CheckDeviceEligibility`，产物中增加 `unsupported` 状态并输出可读原因；补 190 款型比对测试。

### P0-4【A6】matcher 策略外置与 Shadow Mode 全部未接线

- **现象**：`NewStreamMatcherWithPolicy`、`ApplyPolicy`、`EnableShadowMode`、`GetRulesForVendor` 均**只在 `matcher/policy_test.go:42,48` 被调用**；生产仍走 `executor/executor.go:107 matcher.NewStreamMatcher()` + 硬编码 `DefaultRules`。
- **证据**：`internal/matcher/matcher.go:48-97`、`internal/matcher/rules.go:81`、`internal/matcher/policy.go:51`；`internal/executor/executor.go:107` 未替换为策略注入。
- **方案条款**：§3.2.4 Shadow Mode 三阶段、§3.2.5 验收（"新增厂商仅靠 JSON 生效"、"Shadow 差异率 < 1%"）。
- **影响**：执行主链路无任何行为变化，"零代码接新厂商"、"场景维度生效"无法达成；`EnableShadowMode` 存在顺序缺陷（若先开启影子再 `ApplyPolicy`，影子对比的新规则集会过期：`matcher.go:85-93`）。
- **建议**：先接线 Shadow Phase 1（只记录），同时把 executor 创建 matcher 的位置改为 `policymatcher.Resolve(scene, vendor, deviceType)` 注入，并补差异率统计上报。

### P0-5【B2】字符集/逐流上限/代理：配置项无人赋值，"auto" 为空操作

- **现象**：`sshutil.Config` 新增 `MaxEchoBytes`、`Charset`、`ProxyAddr`（`internal/sshutil/client.go:99-105`），但全仓**无任何生产代码为其赋值**（`grep Charset: / MaxEchoBytes: / ProxyAddr:` 零命中，仅测试与声明）。
- **附加缺陷**：`NewCharsetReader(r, "auto")` **原样返回 `r`**（`internal/sshutil/charset.go:92-100`），即 `Charset="auto"` 不会做任何自动探测；`CharsetDecoder.AutoDetect` 仅存在于未被生产使用的 `Decode()` 中。
- **证据**：`client.go:692-702`（Reader 装配）；`charset.go:29-89`（auto 逻辑）。
- **影响**：GBK 中文设备回显乱码问题**依旧存在**（方案 §3.3 验收"中文设备回显正确解码"未达成）；SOCKS5 代理能力不可达（`DialWithProxy` 已被 `client.go:632,924` 调用，但 `ProxyAddr` 恒为空 → 等同直连）。
- **建议**：在设备模型/连接配置中增加 `Charset` 字段并透传到 `sshutil.Config`；`NewCharsetReader` 支持 `auto`（可基于首块字节探测或使用 `transform` + 判定重试）；设备"高级设置"中增加代理地址。

### P0-6【B9】告警归并管线"断头"：没有任何写入告警记录的代码

- **现象**：`AlarmService.AnalyzeAndMerge` 从 `alarm_records` 表按 `run_id` 读取记录（`internal/ui/alarm_service.go:96-104`），但**全仓无任何 `AlarmRecord` 生产写入点**（grep 仅命中模型定义、迁移、只读查询与测试）；`RuleRegistry.MatchLine` 也仅在测试中被调用（`internal/alarm/rules.go:105`），规则从不作用于真实回显。
- **附加**：`MergerConfig.TimeWindow`（`internal/alarm/merger.go:15,21`）定义后从未参与归并逻辑；19 条内置规则只存在于内存（无 `EnsureAlarmSeeds` 落库）。
- **方案条款**：§5.2 验收"规则按族加载 / 归并引擎可将原始告警聚合为故障现象 / 结果可接入巡检报告与 bizcompare 影响清单"。
- **影响**：UI 上"告警归并"功能对真实数据永远返回空，告警能力实际不可用。
- **建议**：在采集/解析链路（如巡检解析阶段或拓扑采集阶段）对 `display alarm active` 类回显调用 `MatchLine` 落库 `alarm_records`；归并结果接入巡检报告与 bizcompare 影响清单。

### P0-7【A3-β】XmlConfig 引擎路由与方案不符 + 跨厂商兜底/非确定性匹配

- **现象 1（路由优先级反转）**：方案要求 `GetParserForDevice` 路由顺序为 `XmlConfig → 用户模板 → Aggregate → Tree → Regex`；实际实现是"模板命中优先，模板缺失时才兜底 XmlConfig"，且 `internal/parser/manager.go` **未做任何改动**（方案文件清单中的 `[MODIFY] manager.go` 未执行）。
  - 证据：`internal/parser/composite_parser.go:75-87`（`if !ok { ...xmlconfig... }`）、`:163-166`。
- **现象 2（跨厂商兜底）**：`ResolveConfig` 的兜底分支"尝试任何 vendor 下匹配同名命令"会把 A 厂商规则套用到 B 厂商设备。
  - 证据：`internal/parser/xmlconfig_engine.go:132-138`。
- **现象 3（非确定性）**：`ResolveConfig` 的模糊匹配分支遍历 `map`（Go map 迭代顺序随机），当同厂商存在多个前缀相关配置（如 `show interface` 与 `show interface brief`）时，结果不稳定。
  - 证据：`internal/parser/xmlconfig_engine.go:123-130`。
- **影响**：解析结果可能随机/串厂商，直接污染拓扑、巡检与比对的解析数据；且"路由正确性"验收项不成立。
- **建议**：`ResolveConfig` 改为显式索引（vendor→有序命令键列表 + 最长匹配），去掉跨厂商兜底；将 XmlConfig 优先级调整到方案约定位置（或更新方案并说明理由）。

### P0-8【A4/B11】DSL 判定链路未上线，PreCollect/IsBig 无消费点

- **现象**：
  - `DSLInterpreter.Evaluate` **无生产调用**（仅 `dsl_test.go`），巡检判定仍走 `inspection.EvaluateItem`（`internal/taskexec/inspection_executor.go:183,386`）。
  - `EvaluateInput.ContextVars`（`internal/inspection/engine.go:20`）生产构造点从未赋值。
  - `DSLRule.PreCollects` / `ExecutePreCollect`（`dsl.go:72,216-235`）无生产调用、无规则数据使用；`DSLRule.IsBig`（`dsl.go:74`）**全仓零消费**。
  - `inspection_pipeline.go`、`report/raw_logger.go` 未按方案改造（`raw_logger.go` 本身已是流式写入，属既有能力）。
- **影响**：A4 的"≥50 条规则跑通、与 Python 结论一致率 ≥90%"与 B11 的"PreCollect 复用、isBig 流式落盘"均未真正交付；62 条规则数据目前只是"可导入导出的静态资产"。
- **建议**：在巡检判定阶段支持"规则来源=DSL"的分支（按 `CheckNo` 命中则 `Evaluate`），补 `isBig` 落盘路径，或在方案中显式降级该模块的完成定义。

### P0-9【B5】CDP 发现源未实现，端口角色/发现方式前端未消费

- **现象**：
  - `topology_builder.go` **无任何 CDP 逻辑**（发现源仅 LLDP / FDB+ARP），`topology_compiler.go` 未改造（无"发现源可选配置 + ARP 风险提示"）。
  - 新增的 `internal/parser/templates/builtin/cisco_cdp.json` 不在加载路径上（`manager.go:388` 按 `<vendor>.json` 取模板），真正的 CDP 模板被塞进了 `cisco.json:63-89`，`cisco_cdp.json` 属**死文件**。
  - `DiscoveryMethods` 仅由候选特征填充（`topology_builder.go:1262`），前端**零引用**；`GraphEdge.Role` 已赋值（`topology_query.go:135`，来源 `topology_builder.go:1245` 的 `InferPortRole`），但前端只渲染节点 role，**边 role 未展示**。
- **方案条款**：§4.4 验收（"CDP 发现源上线"、"DiscoveryMethods 落库前端可勾选"、"ARP 风险提示"、"边 Role 前端分色显示"）。
- **建议**：补 CDP 采集字段（field_key=cdp_neighbor）与解析映射，`DiscoveryMethods` 与 `Role` 接入前端；删除或接入 `cisco_cdp.json` 以避免双份模板来源。

---

## 2. P1 高优先级问题

### P1-1【通用】数据文件成对重复，工具输出路径与实际 embed 路径不一致

- **证据**（MD5 完全相同的三对 + 一对）：

  | 实际被 embed/使用 | 方案指定/工具输出 | 状态 |
  |---|---|---|
  | `internal/device/profiles/{product_items,domains,capabilities}.json`（`profile_registry.go:12-19`） | `internal/config/device_profiles/*.json`（`tools/convert_profiles/main.go:55`） | **两份完全相同的副本**，`config/device_profiles` 无任何读取方 |
  | `internal/report/rules/sensitive_cmd.json`（`vendor_sanitizer.go:12`） | `internal/config/sanitize_rules/sensitive_cmd.json`（`tools/convert_sensitive/main.go:51`） | **两份完全相同（296,712 字节）**，后者无读取方 |

- **影响**：**再跑一次转换工具会写到"没人读"的路径**，实际生效的规则文件不会被更新 —— 下一次规则升级时极易出现"改了没生效"或双份数据漂移。违反方案 §8.3 数据文件管理规范。
- **建议**：二选一（推荐把 embed 放到方案指定目录 `internal/config/...`），删除多余副本，工具输出路径与 embed 路径统一，并在 CI 加"重复数据文件检测"。

### P1-2【B7】baseline SQL 不是"当前全量表结构快照"，且与模型不一致

- **证据**：`internal/config/migrations/0001_baseline.sql` 仅创建 `schema_migrations` + `device_assets` 两张表；`device_assets` 的 `id` 为 `TEXT PRIMARY KEY`，而模型 `models.DeviceAsset.ID` 为 `uint autoIncrement`（`internal/models/models.go:13`），且基线缺少 `esn/model_series/form_factor/connect_mode/jump_host_id/tags` 等列。
- **影响**：新装库先建"窄表/异型表"，再靠 `AutoMigrate` 补列（`internal/config/db.go:79-86`）；方案 §4.2 分工设计（migration 管演进、AutoMigrate 只补新实体）被打破，后续真实迁移脚本缺少可信基线。
- **建议**：用"当前全量表结构"重新生成基线（或用 `sqlite3 .schema` 导出），保持与 GORM 模型一致；后续结构变更全部走 `0002_xxx.sql`。

### P1-3【A2】比对快照语义偏离方案：存原始回显行 + 失败占位项制造假差异

- **证据**：`internal/taskexec/bizcompare_executor.go:146-152`：`key = CommandKey + "." + 整行文本`，快照即"原始回显行集合"，未按方案 §3.5.2 的"采集项→命令→模板→归一化"结构化；`:159-164`：连接失败/无回显时写入 `"executed"` 占位项。
- **影响**：
  - diff 结果噪声大（配置类回显的任意行变化都会成为差异项），无法按"业务指标"判断；
  - 一次采集失败、一次成功的两次运行之间会产生**大量假差异**（占位 vs 真实行）。
- **建议**：快照项改为"解析后的归一化字段（含指标名/值）"，失败时不写占位项而是标记设备级失败并在 diff 中跳过；`differ` 的 `drift` 判定补充可配置阈值。

### P1-4【A2】比对服务健壮性与合规问题

- **证据**：`internal/ui/bizcompare_service.go:83,94,96`（DB 错误全部 `_ =` 吞掉；`db == nil` 时 `CompareTaskID=0`（`:88`）产生孤儿差异项）、`:47-66`（未校验 before/after 两次运行的 domain/scene/phase 是否一致）、`:124-156`（`ExportDiffCSV` **未调用 `report.ValidateExportContent`**，与巡检/硬件清单导出（`inspection_service.go:493,510`）不一致）。
- **影响**：比对结果可能被误读；差异 CSV 可能包含未脱敏的口令类配置片段（叠加 P0-1/P0-2 风险放大）。
- **建议**：错误显式返回；比对前校验 domain/scene 一致性并在结果中标注；导出前统一走脱敏自检。

### P1-5【B1】信任清单可"永不过期"，逃生闭环未闭合

- **证据**：`internal/models/risk_trust_list.go:21-26`（`ExpiresAt` 零值 → `IsExpired()=false`，即永久白名单）；`internal/ui/risk_command_service.go:73-93`（新增信任条目不校验正则合法性、不强制填写过期时间）；`internal/executor/bypass_policy.go:114-130`（"紧急放行"只写留痕，**不放行**——再次执行仍会被拦截/告警）。
- **影响**：安全性依赖使用者自觉；"逃生"操作对操作者表现为"点了没用"。
- **建议**：`ExpiresAt` 必填并设置最长时限（如 24h）；`AddTrustEntry` 预编译校验正则；`BypassRiskCommand` 落库同时写入一条短期信任记录（或写入内存 pending 放行集合），保证"二次确认 + 理由"后可执行。

### P1-6【B1】留痕 Action 取值与模型注释不一致

- **证据**：模型注释为 `blocked | confirmed | warned | bypassed`（`internal/models/risk_command_log.go:12`），实际写入 `block|confirm|warn`（`internal/executor/stream_engine.go:423-435`）。前端/报表若按注释取值会查不到数据。
- **建议**：统一枚举并抽常量。

### P1-7【B5】厂商字段命令新增 `scene` 与旧唯一索引冲突

- **证据**：`internal/models/topology_command.go:48-49` 唯一索引为 `(vendor, field_key)`，`:50` 新增 `Scene` 并另建普通索引 `idx_topology_vendor_field_scene`。同厂商同一字段**无法为不同场景配置不同命令**（唯一约束冲突），与方案 §4.3 "LLDP 场景包（enterprise/service/ipmaster）"的设计目标矛盾；且 `AutoMigrate` 不会修改已存在的唯一索引。
- **建议**：唯一索引改为 `(vendor, field_key, scene)`，并写迁移脚本重建索引。

### P1-8【B3】新增 ANSI 序列被消费端静默丢弃，且缺 Golden 测试集

- **证据**：`internal/terminal/ansi.go` 已解析 DEC 私有序列（`?1049`/`?25`，`:58-69,241-247`）与 OSC（`:177-190`），但 `replayer.processCommand`（`internal/terminal/replayer.go:93-133`）**没有 `CmdDecSet/CmdDecReset/CmdOSC` 分支，也无 default** —— 序列被静默吞掉；`testdata/ansi/` 目录**不存在**（方案 §4.6 要求）。
- **影响**：终端回放的"备用屏/光标显隐/标题"行为无实际改善；Golden 验收缺失。
- **建议**：replayer 增加消费策略（alt-screen 状态记录、OSC 标题提取、DEC 模式忽略并计数），补 `testdata/ansi/` Top 10 高频序列 Golden。

### P1-9【C3/C5】入参校验函数无调用点；风险命令种子未扩充

- **证据**：`internal/security/param_validator.go`（8 个校验函数）**生产零 import**（仅 `param_validator_test.go`）；`internal/models/risk_command.go:43` 仍为 **18 条**种子（方案 B1 要求导入 `riskCmd_product.xml` 扩充，`[MODIFY] risk_command.go` 未执行）。
- **影响**：C3"入参校验种子"与 C5"关键命令种子"实际未交付（虽然 C 档为可选，但方案将其列为一/二期搭车项）。
- **建议**：在 Wails 服务层写路径接入 `param_validator`；补 `riskCmd_product.xml` 转换与种子扩充，并注明 C1/C2 的取舍结论。

### P1-10【B4/B6】弱光检测与智能 Ping 无生产调用点

- **证据**：
  - `internal/optical/checker.go`（percent/absolute 双模式、LOS 判定）**仅测试调用**；`models.OpticalCheckRule` 只有 AutoMigrate，无 UI 服务/无种子（`internal/ui` 中 `optical` 零命中）→ 方案验收"前端可管理"未达成。
  - `internal/smartping/engine.go` 未与 `internal/icmp` 对接（`internal/icmp` 无 smartping 引用），`GetGlobalEngine` 仅测试调用；`rules/builtin.json` 5 条规则无消费入口。
- **影响**：三项专项能力（弱光/B6/部分 B4）对业务零暴露；B4 的"LLDP 场景切换"同样未接线（`LldParseTemplate` 模型无消费）。
- **建议**：为弱光检测提供 Wails 服务与规则管理入口，并接入巡检采集项；智能 Ping 接入批量 Ping 结果分析链路。

---

## 3. P2 中优先级问题

| 编号 | 模块 | 问题 | 证据 | 建议 |
|------|------|------|------|------|
| P2-1 | A3-α | 后顾断言改写策略与方案不符：方案要求"先捕获整段再二次提取"，实现是**直接删除断言**改为 `(?:...)`（`(?<=X)Y` → `(?:X)Y`，匹配偏移变化可能产出错误值）；改写只计数不留痕，无"改写建议"清单 | `internal/parser/xmlcfg/compat.go:84-93` | 改写结果写回配置文件并人工复核；或按方案实现二次提取 |
| P2-2 | A3-α | `parsecfg_broken.json` 只由**测试**生成（`loader_test.go:39-44`），启动时无 WARN、无读取方；方案要求"记入清单 + 启动日志 WARN" | `internal/parser/templates/parsecfg/parsecfg_broken.json` | 启动加载时输出 broken 清单日志 |
| P2-3 | A1 | 4 条脱敏规则 RE2 编译失败后被**静默丢弃**，且 `BrokenRules()` 无调用点（应输出启动 WARN）；失败规则含 `shared-key-cipher (?!\* )(.*)` 等安全相关项 | 实证清单：`NE router: snmp-agent target-host trap address udp-domain ... (?!cipher)...`、`NE router: shared-key-cipher (?!\* )(.*)`、`通用: contact "((?:(?!")[\s\S])*)"`、`JUNIPER: 同 contact 模式` | 用 `xmlcfg.CleanAndCompileRegex` 复用改写能力；启动日志输出 broken 明细 |
| P2-4 | A5 | `ResolveCategory` 会返回数据中**不存在**的 `RUIJIE`（数据仅 16 类：AR/NE/Ethernet Switch/DC/Firewall/WLAN/H3C/CISCO/ALU/NOKIA/ZTE/JUNIPER/DPTECH/RUCKUS/SANGFOR/DELL + 通用空类），锐捷设备只能命中共 73 条通用规则 | `internal/report/vendor_sanitizer.go:155-156`；`Categories()` 实测 16（`vendor_sanitizer_test.go:21`） | 对齐数据类别集合，或补 RUIJIE 规则数据 |
| P2-5 | A6 | `PolicyMatcher` 细节：`Resolve` 返回内部指针（调用方可变）；`LoadEmbedded` 静默忽略解析失败；`ToCompiledErrorRules` 静默跳过编译失败规则 | `internal/matcher/policy.go:82-88,138-179` | 返回副本；失败记录日志/metrics |
| P2-6 | B9 | 19 条告警规则仅内存注册表，`alarm_rules` 表无写入（无 `EnsureAlarmSeeds`），UI `ListAlarmRules` 读内存、`alarm_rules` 表空置 | `internal/alarm/rules.go:30-36`；`internal/config/db.go:90-117`（无告警种子调用） | 增加种子落库或明确"内置只读规则"的定位并隐藏空表 |
| P2-7 | B9 | `TimeWindow`（默认 10 分钟）不参与归并，所有同设备告警被无条件聚合，可能把不同时段的独立故障合并 | `internal/alarm/merger.go:15,21` | 归并分桶时按 `TimeWindow` 切分 |
| P2-8 | A4 | DSL 导入规则**只存内存**，重启即失效；导入不校验正则合法性/字段完整性，允许覆盖内置规则 | `internal/inspection/dsl.go:171-201` | 持久化到 DB 或文件；导入前 `regexp.Compile` 校验 |
| P2-9 | B2 | `Charset` 转码发生在 `io.TeeReader(sink)` **之前**（原始日志落盘的是转码后内容），与"原始字节流镜像"语义可能冲突，需确认预期 | `internal/sshutil/client.go:699-709` | 明确 raw sink 语义（原始 or 转码后），必要时调整 Tee 位置 |
| P2-10 | A1 | `VendorSanitizeRule.IsRegex` 字段生成后从未使用（引擎一律按正则编译），数据里 4 条 `isRegex=false` 的规则仍会被尝试编译并失败 | `tools/convert_sensitive/main.go:97-111`；`internal/report/vendor_sanitizer.go:91` | 引擎按 `IsRegex` 决定"正则 or 字面量掩码" |
| P2-11 | 通用 | 变更集中 **34 个 .go 文件未通过 `gofmt`**（含新增核心文件 `dsl.go`、`stream_engine.go`、`matcher/policy.go`、`iftype.go`、`alarm/rules.go` 等），存在对齐/缩进混乱（`stream_engine.go:443-531` 缩进错位影响可读性与后续维护） | `gofmt -l` 输出 | 提交前统一 `gofmt -w`，CI 增加格式门禁 |

---

## 4. P3 低优先级问题

| 编号 | 模块 | 问题 | 证据/说明 |
|------|------|------|-----------|
| P3-1 | 流程 | Sprint 0 六份探针产物缺失（`docs/draft/bizcompare_mapping.md`、`inspection_dsl.md`、`matcher_ssh_gap.md`、RE2 兼容率报告、ANSI UnknownCount 统计等均不存在） | 方案 §6 完成标志 |
| P3-2 | 工具 | `tools/convert_profiles/main.go:54` 硬编码本机绝对路径（无 flag），`convert_sensitive` 有 flag 但默认值同样指向本机路径 | 可移植性/协作问题 |
| P3-3 | 测试 | 验收阈值被放宽：脱敏性能测试断言 200ms（方案要求 50ms）；broken 规则断言 `≤5`（应为 0 并人工处理）；脱敏类别断言 `≥15`（方案为 17） | `internal/report/vendor_sanitizer_test.go:26,90` |
| P3-4 | A5 | 能力种子口径混用：`capabilityKey` 同时容纳业务能力（bizcompare/optical/hardware）与非业务标识（inspection/topology/batchping/configbuild），而 `CheckDeviceEligibility` 只识别其中 3 种（bizcompare/optical/hardware），其余 seed 无消费方 | `internal/models/device_capability.go:34-40`；`internal/taskexec/eligibility.go:24-35` |
| P3-5 | A2 | `BizCompareExecutor` 的 `db`/`settings` 字段赋值后从未使用；`domain` 缺省硬编码为 `"S"` | `internal/taskexec/bizcompare_executor.go:21-32,90` |
| P3-6 | A2 | ui `ListDomains` 硬编码 3 域（与 seeds 重复定义），域/场景描述维护点分散 | `internal/ui/bizcompare_service.go:32-38` |
| P3-7 | B8 | `NameToIFType` 遍历 map 做前缀匹配（多键命中时结果不确定，如小写 `eth-trunk10` 同时命中 `eth`/`eth-trunk`）；`NormalizeWithAlias` 同样依赖 map 顺序 | `internal/normalize/iftype.go:107-115,124-136`；建议改为"最长前缀优先"的有序切片 |
| P3-8 | C 档 | C1（离线命令模板）、C2（xlsx 导出）未实施——方案标注为"按需交付"，需在变更说明中明确取舍，避免被误认为已完成 | `go.mod` 无 excelize；无 offline 实现 |

---

## 5. "未接线清单"汇总（本轮审计核心结论）

> 判定标准：功能代码存在且单测通过，但**无生产调用点**（排除 `_test.go`）。

| # | 模块 | 未接线的对象 | 唯一调用方 |
|---|------|--------------|-----------|
| 1 | A1 | `report.VendorSanitizer` / `logger.ContextSanitizer` | 测试 |
| 2 | A5 | `taskexec.CheckDeviceEligibility` / `device.ProfileRegistry` | 测试（注册表另被 eligibility 调用，而 eligibility 本身无调用） |
| 3 | A6 | `matcher.NewStreamMatcherWithPolicy` / `ApplyPolicy` / `EnableShadowMode` / `GetRulesForVendor` | 测试 |
| 4 | B2 | `sshutil.Config.Charset` / `ProxyAddr` / `MaxEchoBytes`（默认可生效）| 无赋值点 |
| 5 | B8 | `normalize.NormalizeWithAlias` / `NameToIFType` / `IFTypeToName` | 测试 |
| 6 | B4 | `optical.Checker` / `models.OpticalCheckRule` / `LldParseTemplate` | 测试 / 仅建表 |
| 7 | B5 | `cisco_cdp.json`、`DiscoveryMethods`（前端）、边 `Role`（前端） | 无 |
| 8 | B6 | `smartping.Engine` | 测试 |
| 9 | B9 | `alarm.RuleRegistry.MatchLine`、`AlarmRecord` 写入 | 测试 / 无写入者 |
| 10 | B10 | `connutil.JumpConnector`、`DeviceAsset.ConnectMode/JumpHostID`、`ExecutorOptions.PreCommands` | 测试 / 无赋值点 |
| 11 | C3 | `security` 包全部校验函数 | 测试 |
| 12 | A4/B11 | `DSLInterpreter.Evaluate` / `ExecutePreCollect` / `IsBig` | 测试 |

---

## 6. 模块完成度与验收对照

| 模块 | 代码/数据落地 | 生产接线 | 方案验收达成情况 |
|------|---------------|----------|------------------|
| A1 分厂商脱敏 | ✅ 581/585 条 | ❌ 未接线 | ⚠️ 加载✅ / 华为命中率✅ / **"脱敏后无 CRITICAL"不成立（P0-2）** / 性能实测 104ms（>50ms 目标，未阻断） / broken 清单无启动 WARN |
| A5 设备画像+能力 | ✅ 18 类 / 44 域 / 190 款型矩阵 | ❌ 未接线 | ❌ 190 款型比对无自动化测试；准入未在 Compile 生效；字段已加（AutoMigrate 兼容） |
| A3-α 规则导入 | ✅ 44 XML 全加载 / 编译率 ≥90%（2 条 broken） | — | ✅ 加载率 100%；⚠️ 破规清单无启动告警、无改写建议 |
| A3-β 第四引擎 | ✅ 13 算子 / 2 策略 / 6 模型 Golden | ⚠️ 部分（仅模板缺失时兜底） | ⚠️ 13 算子✅ / Golden✅ / **路由顺序不符** / 新增厂商"零 Go 改动"未验证 |
| A6 策略外置 | ✅ 4 份策略 JSON + Shadow | ❌ 未接线 | ❌ Shadow 差异率无从统计 |
| B2 协议层 | ✅ charset/SOCKS5/LimitedEcho | ❌ 未接线（Charset/ProxyAddr 无赋值；auto 空转） | ❌ 中文回显未验证；代理无环境验证；10MB 截断逻辑已实现（等待触发） |
| B1 高危命令闭环 | ✅ 留痕/信任清单/逃生 API | ✅ 已接入 stream_engine | ⚠️ 留痕✅ / 信任清单✅（可永不过期）/ 逃生**不放行** / 种子未扩充 |
| A2 变更比对 | ✅ 5 文件 + 编译器/执行器/服务 | ✅ 已注册（Compile/Executor/UI） | ⚠️ 快照语义偏离 / 占位项假差异 / 导出未脱敏校验 / 无端到端验证 |
| A4 规则 DSL | ✅ 62 条规则 + 导入导出 | ❌ 判定未接线 | ❌ ≥50 条"跑通"仅测试；Python 一致率未验证 |
| B7 版本化迁移 | ✅ runner（事务+幂等+校验和） | ✅ 接入 InitDB | ⚠️ 基线 SQL 不完整（P1-2） |
| B4 弱光/LLDP 场景 | ✅ checker 双模式 | ❌ 未接线 | ❌ 前端可管理未达成 |
| B5 CDP/端口角色 | ⚠️ 角色推断已接线；CDP 未实现 | ⚠️ 部分 | ❌ CDP/DiscoveryMethods/ARP 提示/边角色展示缺失 |
| B6 智能 Ping | ✅ 引擎 + 5 规则 | ❌ 未接线 | ❌ 逐接口可通性矩阵未产出 |
| B3 终端仿真 | ✅ 新增 DEC/OSC 解析 | ⚠️ 消费端丢弃 | ❌ Golden 集缺失（testdata/ansi） |
| B11 编排增强 | ⚠️ DSL 字段就绪 | ❌ 未接线 | ❌ PreCollect/isBig 未生效 |
| B9 告警归并 | ✅ 19 规则 + 归并算法 | ❌ 无数据源 | ❌ 无法产出故障现象 |
| B10 跳板 | ✅ 13 命令模板 + 连接器 | ❌ 未接线 | ❌ 未交付（方案亦建议"按需"） |
| C3/C5 | ✅ 校验函数 / 18 种子 | ❌ 未接线 / 未扩充 | ❌ |

---

## 7. 合规红线与原则核查（P1~P7）

| 原则 | 结论 | 说明 |
|------|------|------|
| P1 搬数据不搬代码 | ✅ | 未发现 Java/Python/Jython 源码文本 |
| P2 借设计不借实现 | ✅ | Go 惯用法重写（算子/策略/注册表） |
| P3 只增不减 | ✅ | 未发现公开接口/结构删除；`parser/manager.go` 未改（属未执行，非破坏） |
| P4 旧行为兜底 | ✅ | 模板优先、`DefaultRules` 兜底、迁移失败回退 AutoMigrate 等均保留 |
| P5 Shadow 先行 | ❌ **未执行** | A6/B2 均未产生 shadow 运行（P0-4） |
| P6 Golden 锁定 | ⚠️ 部分 | 仅 XmlConfig 6 模型 + 脱敏 + 设备样本；弱光/Ping/ANSI/告警无 Golden |
| P7 知识产权 | ✅ | 62 条 DSL 规则为中文重写，未见 `#title_cn/#suggest_cn` 原文；`tools/convert_*` 仅读取事实型数据 |
| 合规红线 4（不含方案文档对外） | ⚠️ | 方案与评估报告已入库（`docs/`），交付时需确认不外发 |

---

## 8. 建议修复顺序（Top 10）

1. **P0-1 / P0-2（A1 合规）**：把厂商脱敏接入所有导出/落盘链路，修正分级边界；这是"合规基座"的唯一硬指标。
2. **P0-6（B9 数据源）**：补齐 `alarm_records` 写入，否则告警功能整体无效。
3. **P0-4（A6 Shadow）**：按方案 Phase 1 只记录接入策略，保留回退开关。
4. **P0-5（B2 字符集）**：让 `Charset/ProxyAddr` 有配置来源并修复 `auto`；否则中文设备问题依旧。
5. **P0-3（A5 准入）**：在编译期接入 eligibility，落地 `unsupported` 标记。
6. **P0-7（A3-β 路由）**：`ResolveConfig` 去随机化、去跨厂商兜底，明确路由优先级。
7. **P1-1 / P1-2（数据一致性）**：消除重复数据文件、重建 baseline SQL。
8. **P1-3 / P1-4（A2 语义与合规）**：结构化快照、去掉占位差异、导出前脱敏校验。
9. **P0-8 / P1-10（A4/B4/B6）**：为 DSL 判定、弱光、智能 Ping 提供服务入口，或在文档中明确降级。
10. **P2-11（gofmt）+ 补验收测试**：CI 增加 `gofmt -l` 与关键验收项测试（190 款型、GBK、Shadow 差异率、DEC Golden）。

---

## 9. 复现与验证方法

```bash
# 构建与静态检查（均通过）
go build ./... && go vet ./internal/...

# 全量测试（全绿）
go test ./internal/...

# 未接线清单核验（示例：A1 / A6 / B8）
grep -rn "GetDefaultVendorSanitizer\|VendorSanitizer" --include=*.go internal/ | grep -v _test.go
grep -rn "NewStreamMatcherWithPolicy\|EnableShadowMode\|GetRulesForVendor" --include=*.go internal/ | grep -v _test.go
grep -rn "NormalizeWithAlias\|NameToIFType" --include=*.go internal/ | grep -v _test.go

# 数据重复核验（四对文件 MD5 相同）
# internal/device/profiles/*.json  ==  internal/config/device_profiles/*.json
# internal/report/rules/sensitive_cmd.json  ==  internal/config/sanitize_rules/sensitive_cmd.json

# 格式检查
gofmt -l internal cmd tools
```

---

*本清单基于对 `9de17dc..794e483` 变更集的源码级审计，所有结论均可通过上述路径与命令复核。审计过程中使用的临时验证测试已删除，工作区保持干净。*
