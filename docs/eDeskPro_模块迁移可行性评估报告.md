# eDesk Pro V100R025C10SPC300 → NetWeaverGo 模块迁移可行性评估报告

> 评估对象：`E:\BaiduDownload\eDeskPro_V100R025C10SPC300-windows-x64`（24086 文件，完整交付包）
> 评估依据：`docs/eDeskPro_V100R025C10SPC300_功能模块分析/`（00~10 共 12 篇）+ `docs/华为eDeskPro脚本体系分析.md`
> 评估基线：对 `NetWeaverGo/internal/**` 的**源码级实测**（包结构、类型、函数、内置种子数据逐项核对）
> 报告日期：2026-09-15
> 文档定位：本报告在 `08-移植能力映射与路线图` 的基础上，做三件事——① 用**实测现状**校准优先级（修正 6 处与当前代码不符的判断）；② 给出**值得迁移 / 部分迁移 / 不建议迁移**的明确分级与理由；③ 给出**文件级落地方案 + 工作量 + 验收标准**。

---

## 0. 执行摘要（TL;DR）

### 0.1 一句话结论

eDesk Pro 里真正值得搬进 NetWeaverGo 的不是**代码**，而是三类东西：
1. **事实型数据资产**（分厂商脱敏正则、142 项业务快照采集项、44 份解析规则 XML、190 款型×版本矩阵、12 厂商关键命令表、接口名 ifType 编码表）——纯数据、零算法风险、可直接搬；
2. **经过现场验证的设计范式**（场景化规则包、注册表驱动、库表驱动阈值、高危命令治理闭环、版本化迁移链）——搬"设计"，不搬实现；
3. **协议/终端/调度层的实现细节清单**（gbk 默认字符集、10MB 回显上限、SOCKS5、指纹校验、116 类终端仿真处理器、内存自适应背压）——**对照补齐**，不逐行移植。

**不建议搬**的：Jython/Python 脚本引擎本体（8312 个 py 只抽规则语义）、Java 微服务基础设施（Spring/MyBatis/H2/Redis/Electron/JRE）、三套数据库 schema 原文、133 个 patch 原文、MML/串口/IPMI 协议、EVA 设备端代理推送、4A/SecureCRT 集成。

### 0.2 分级结论总览

| 档 | 含义 | 模块数 | 建议动作 |
|---|---|---|---|
| **A 档 · 强烈建议** | 缺口明确、资产现成、成本可控、收益可量化 | 6 | 立项，本季度排期 |
| **B 档 · 建议** | 有价值但需配套改造，或收益依赖前置模块 | 11 | 立项，下季度排期 / 随 A 档搭车 |
| **C 档 · 可选** | 锦上添花或体量小 | 6 | 按需取用，不单独立项 |
| **D 档 · 不建议** | 架构不匹配 / 维护成本 > 收益 / 有合规风险 | 9 | 明确放弃，记录理由 |

### 0.3 优先级速览表（按 性价比 = 价值 ÷ 成本 排序）

| 排名 | 模块 | 档 | 价值 | 成本(人日) | 期次 |
|---|---|---|---|---|---|
| 1 | 分厂商敏感信息脱敏管线（`sensitiveCmd.xml`） | A1 | 极高（合规刚需） | 2~3 | 一期 |
| 2 | 设备画像统一 + 能力白名单（`productItem`+`domain`+`support_devices`） | A5 | 高 | 8~10 | 一期 |
| 3 | 解析规则资产导入 + 算子管线（`xmlconfig` 44 份 + 13 算子） | A3 | 极高 | 12~15 | 一期~二期 |
| 4 | matcher/executor 策略外置（`deviceversion/*` + `strategy/`） | A6 | 高 | 8~10 | 二期 |
| 5 | 变更前后业务比对 `bizcompare`（142 采集项 + 16 场景） | A2 | 极高（新能力） | 15~20 | 二期 |
| 6 | 审计规则库模板化（`Health/` 2121 + `Reliability/` 346 抽样） | A4 | 高 | 10~15 | 二期 |
| 7 | 高危命令治理闭环（拦截+白名单+逃生+留痕） | B1 | 高 | 6~8 | 二期 |
| 8 | 协议层细节对齐（gbk / 10MB / SOCKS5 / 指纹 / 错误关键字） | B2 | 中高 | 5~8 | 二期 |
| 9 | 智能 Ping 规则（`smartping` / `pingtest`） | B6 | 中 | 5~8 | 三期 |
| 10 | LLDP 场景化规则外置 + 弱光阈值落表 | B4 | 中高 | 8~10 | 三期 |
| 11 | 数据模型合并 + 版本化迁移 | B7 | 中高 | 10~15 | 三期 |
| 12 | 拓扑发现源扩展（CDP/OSPF）+ 端口角色 | B5 | 中高 | 8~12 | 三期 |
| 13 | 终端仿真补齐（DP-modes / SGR / Vmto） | B3 | 中 | 6~10 | 三期 |
| 14 | 接口名标准化补全（ifType 编码 + 别名表） | B8 | 中 | 3~5 | 三期 |
| 15 | 采集编排语义（`PreCollectItem` / `ThresholdManagement` / `isBig`） | B11 | 中 | 5~8 | 四期 |
| 16 | 跳板/代理/VPN 接入 | B10 | 中（场景依赖） | 8~12 | 四期 |
| 17 | 告警规则与归并引擎 | B9 | 中（依赖告警采集） | 10~15 | 四期 |

---

## 1. 评估方法

### 1.1 评估维度

| 维度 | 说明 | 权重 |
|---|---|---|
| **缺口强度** | NetWeaverGo 当前是否真的没有 / 明显不足（源码实测，不凭文档描述） | 30% |
| **资产成熟度** | eDesk 侧资产是否为结构化数据、是否被现场验证过（体量、版本迭代次数） | 20% |
| **迁移成本** | 数据转换 / 引擎重写 / 前端改造 / 回归测试 | 25% |
| **复用度** | 能否复用现有 `taskexec` 五层架构、`parser` 三引擎、`models`/`repository` | 15% |
| **风险** | 合规、稳定性（是否触碰执行主链路）、维护负担 | 10% |

### 1.2 与 `08` 篇的关系

`08` 篇给出了 28 条「资产 → NetWeaverGo 映射」。本报告对其做**现状校准**，发现 6 处判断需要修正（详见 §5），并据此重排优先级：

- `08` #1 认为"`logger` 仅通用脱敏" → 实测 `logger/sanitizer.go` 已有 17 条规则 + `report/sanitizer_check.go` 有导出前阻断。**结论不变（仍 P0），但工作量从"从零建"降为"加厂商维度"**。
- `08` #5 认为"无高危命令拦截" → 实测已有 `models.RiskCommand` + `executor/risk_validator.go` + 20 条种子 + 三级动作。**从"新建"降级为"治理闭环补全"**。
- `08` #16 认为"LLDP 解析硬编码在 Go 里" → 实测 `topology_builder.go` 走的已是 `parser` 的 aggregate 模板（`huawei.json` 的 `lldp_neighbor`）。**问题不是"硬编码"，而是"只有一套规则、没有场景维度"**。
- `08` #4 认为 `parser` 已有 Tree 引擎即可承接 → 实测 Tree 引擎**无任何内置模板在用**，且缺少算子管线（13 个 ETL 算子）与 4 种解析策略。**工作量被低估**。
- `08` #3 认为要在 `inspection` 上"从零抽象规则模板" → 实测 `inspection` 已是**数据驱动**（DB 模板 + `ThresholdsJSON` 阈值外置 + 6 种判定类型 + 8 种结论码 + 中英导出）。**真正缺口是"规则体量"（12 条 vs 8312 条），不是"框架"**。
- `08` #19/#20 把跳板接入、接口名标准化列为 P2/P3 → 结合 `normalize/` 已有基础与 `icmp` 已有引擎，接口名标准化成本被高估（降为 3~5 人日），跳板接入因**无真实需求验证**维持后置。

---

## 2. NetWeaverGo 现状基线（源码实测）

> 本节是全部判断的锚点。所有结论来自对 `internal/**` 的逐包核对，非文档推断。

### 2.1 能力矩阵

| 领域 | 现状 | 强度 | 关键代码位置 |
|---|---|---|---|
| 回显解析 | Regex / Aggregate / **Tree** 三引擎；内置 `huawei/h3c/cisco/default` 4 份 JSON 模板（10/6/6/3 键）；DB 侧 `UserParseTemplate` 带 `appliesTo` 款型版本白名单 | 中 | `internal/parser/{manager,regex_parser,aggregate_engine,tree_engine}.go` |
| 设备画像 | `DeviceProfile`（PTY/Prompt/Pager/Init/Commands）+ 文件 `profiles/*.json`（仅 2 份）+ DB 覆盖表；`device.Identify` + `series.go` | 中 | `internal/config/device_profile.go`、`internal/device/` |
| 命令执行 | `StreamEngine` + `SessionReducer`（8 类事件状态机）+ 自动应答 + 分页续页 + 单设备挂起 | 强 | `internal/executor/` |
| 任务编排 | **五层架构**；5 个 Compiler（Normal/Topology/Backup/CEAS/Inspection）；9 个 Executor；EventBus + SnapshotHub；三阶段管线 | 强 | `internal/taskexec/` |
| 巡检 | 引擎 + 12 条内置种子 + DB 模板/检查项/分组树 + 阈值外置 + 6 判定类型 + 8 结论码 + 中英 CSV/JSON 导出 | 中（框架强、规则少） | `internal/inspection/` |
| 硬件清单 | CEAS elabel 解析 + 硬件树 + BOM 批次预警 | 中 | `internal/ceas/` |
| 规划比对 | 规划 CSV → 链路 → 与实采拓扑比对（missing/unexpected/mismatch） | 中（仅静态） | `internal/plancompare/` |
| 报告 | CSV + JSON + 原始日志；**导出前脱敏阻断** | 中 | `internal/report/` |
| 脱敏 | 17 条通用规则，可增删启停 | 弱（无厂商/命令维度） | `internal/logger/sanitizer.go` |
| 终端仿真 | ANSI 基础 CSI（光标/擦除/SGR），行缓冲 + 重放器 | 弱 | `internal/terminal/` |
| SSH | PTY 配置 + 算法预设（secure/compatible/custom）+ 主机密钥三模式（strict/accept_new/insecure）+ known_hosts | 中 | `internal/sshutil/` |
| Telnet | IAC 协商 + ECHO + TerminalType + 窗口尺寸 | 中 | `internal/telnetutil/` |
| SNMP | v1/v2c/v3 全安全级 + 独立加密库（AES-256-GCM） | 中（无 WALK 封装、无 MIB） | `internal/snmp/` |
| 拓扑 | `topology_builder.go`(45KB) + 事实/边候选/决策追溯；LLDP 走解析模板；ARP/FDB 融合 | 中（无 CDP、无角色语义） | `internal/taskexec/topology_*.go` |
| 高危命令 | `risk_commands` 表 + `RiskValidator`（block/confirm/warn，厂商专属>通配）+ 20 条种子 + 热更新；全局开关默认 `warn`（非强制） | 中（无审批/逃生/留痕） | `internal/executor/risk_validator.go` |
| 数据层 | GORM AutoMigrate（18+21 表 + SNMP 库） | 弱（无版本化） | `internal/config/db.go` |
| UI 服务 | 20 个 Wails 服务 | 强 | `internal/ui/` |
| 归一化 | 接口名 / LLDP 远端端口 / 聚合口 / 设备名 / MAC | 中 | `internal/normalize/` |
| 网络工具 | Ping + Tracert + GeoIP + 网络计算器 + 文件服务器（SFTP/FTP/TFTP/HTTP） | 强 | `internal/icmp/`、`internal/fileserver/` |

### 2.2 明确的能力缺口（eDesk 侧有资产可填）

| # | 缺口 | 现状证据 | 影响 |
|---|---|---|---|
| G1 | 脱敏无「厂商 × 命令」维度 | `logger` 17 条为全局正则 | 回显中的 `cipher` 密文可能外流，**交付合规风险** |
| G2 | 无双时点（变更前/后）比对 | `plancompare` 只比「规划 vs 实采」；`TaskRun` 无 baseline 字段 | 割接验收场景完全无法覆盖 |
| G3 | 解析引擎无**算子管线**与**多种解析策略** | 三引擎无 Operator；Tree 引擎零内置模板 | 复杂表（Trunk 成员、光模块 lanes）无法表达 |
| G4 | 规则体量小 | `inspection/seeds.go` 仅 12 条 | 与 eDesk 8312 条不是一个量级 |
| G5 | 设备无「能力准入」 | 无 `device_capabilities`；只有画像 `TopologyEnabled` | 无法回答"这台设备能不能跑这个模板" |
| G6 | matcher 策略硬编码 | `DefaultRules` 7 条、`DefaultPrompts` 3 个、分页 5 条写死在 Go 里 | 接新厂商必改代码 |
| G7 | 无场景维度（按任务场景切换校验/提示策略） | matcher 全局策略 | 采集/巡检/故障三类场景共用一套判定 |
| G8 | 单设备仅一套凭据 | `DeviceAsset` 内联 `Protocol/Username/Password/Port` | 无法同时 SSH + SNMP + Telnet 并存 |
| G9 | 数据库无版本化 | 纯 AutoMigrate | 升级不可审计、回滚不承诺 |
| G10 | 无告警/端口角色/跳板/多发现源 | — | 拓扑"能连线"但不"懂语义" |

---

## 3. A 档：强烈建议迁移（6 个模块）

### A1 · 分厂商敏感信息脱敏管线 ★★★

| 项 | 内容 |
|---|---|
| **资产** | `config/deviceversion/sensitiveCmd.xml`：17 个 `<Category>`（AR router / NE router / Ethernet Switch / DC / Firewall / WLAN / H3C / CISCO / ALU / NOKIA / ZTE / JUNIPER / DPTECH / RUCKUS / SANGFOR / DELL / 通用）× 每类 N 条 `<Cmd>`（`,,,` 分隔多命令）× N 条 `<Filter>` 正则 |
| **互补资产** | `services/IPOnlineService/.../config/sensitiveCmd.xml`；`product/Script/ceas/Common/cmd_echo_writer.py::hide_echo_password`（脚本侧简化版） |
| **现状** | `internal/logger/sanitizer.go`：17 条**全局**规则（password cipher/simple/plain、secret/token/credential、JSON 字段、URL 口令），支持 `AddRule/RemoveRule/SetRuleEnabled`；`internal/report/sanitizer_check.go`：`CheckContentSanitized` 在导出前校验 4 类未掩码特征并**阻断** |
| **差距** | 缺少「按厂商 + 按命令」的**分级脱敏**：`display current-configuration` 与 `display interface` 的脱敏强度应不同；`snmp-agent community read cipher (\S*)`、`local-user \S+ password irreversible-cipher (.*)`、`pre-shared-key cipher (.*)` 这类**厂商特有**密文特征未覆盖 |
| **方案** | 1) 新增 `internal/report/vendor_sanitizer.go`，结构 `map[Category]map[CmdKey][]SanitizeRule`，规则源为内置 `sanitize_rules/sensitive_cmd.json`（由 XML 一次性转换，附转换脚本）；2) 在 `logger.Sanitizer` 上增加 `WithVendor(vendor)` / `WithCommand(cmd)` 上下文，命中 `Category+Cmd` 时追加厂商规则集；3) `report/sanitizer_check.go` 增加分级：CRITICAL（cipher 类）阻断，WARN（口令提示类）仅告警；4) 前端「设置 → 脱敏规则」提供开关与命中计数 |
| **落点** | `internal/report/vendor_sanitizer.go`（新）、`internal/logger/sanitizer.go`（改）、`internal/report/sanitizer_check.go`（改）、`internal/config/sanitize_rules/*.json`（新，数据） |
| **成本** | 2~3 人日（XML→JSON 转换脚本 0.5、引擎 1、接入与测试 1） |
| **风险** | 低。纯数据 + 只增不删；需防正则回溯（对 17×N 条正则做编译期超时/复杂度检查，建议在启动时预编译并 `regexp.Compile` 失败即跳过并告警） |
| **验收** | ① 17 个 Category 全量加载；② 对 `display current-configuration` 华为回显样本，cipher/irreversible-cipher/pre-shared-key 命中率 100%；③ 脱敏后 `CheckContentSanitized` 无阻断；④ 性能：单条 1MB 回显脱敏 < 50ms |

---

### A2 · 变更前后业务比对（`bizcompare`） ★★★

| 项 | 内容 |
|---|---|
| **资产** | `services/IPOnlineService/.../template/businesscompare/`：`collectCmdList.json`（**142 个采集项键**）、`domain.json`（44 条「款型正则 → 产品域」）、`support_devices.json`（190 款型 × `VxRXXX` 版本矩阵）、`deviceList.json`、`scene/*.json`（16 个场景开关，含 `sceneTemplate` 复用指向）、18 个产品域 xlsx（含 `cmd.xlsx` 命令定义）、中英 TaskTemplate/CheckAndCompareItem/MOP_FAQ |
| **现状** | `internal/plancompare/`：仅「规划链路 vs 实采拓扑」静态比对，输出 `missing_link / unexpected_link / interface_mismatch`；`taskexec` 的 `TaskRun` **无 baseline/双时点概念**，但有 `Scheduler`（`robfig/cron`）、`EventBus`、`SnapshotHub`、`TaskArtifact` 可复用 |
| **差距** | 完全缺失「变更前快照 → 变更 → 变更后快照 → 差异 → 业务影响清单」链路 |
| **方案** | 1) 新建 `internal/bizcompare/`：`SnapshotSpec`（采集项键 → 命令 → 解析模板 → 归一化口径）、`SnapshotStore`（按 `deviceID + sceneID + 时点` 存快照，复用 `TaskArtifact` 表或新表 `biz_snapshots`）、`Differ`（逐项 diff：新增/消失/变更/阈值漂移）、`ImpactAnalyzer`（按采集项归属域打标：路由/BGP/MPLS/接口/可靠性…）；2) 新增 `internal/taskexec/bizcompare_compiler.go`（第 6 个 Compiler）+ `BizCompareExecutor`，**完全复用**现有 `CompilerRegistry`/`StageExecutor`/`EventBus`；3) 前端新增「变更比对」视图：三次运行选择（前/后）+ 差异树 + 影响清单导出 |
| **关键设计借鉴** | ① `scene` ≠ 模板（多个款型共享一个 `sceneTemplate`）；② `itemList` 是**按域裁剪后的子集 + 布尔开关**（S 系列关掉 `bgpAdvertisedRoutes` 这类重命令）；③ `domain.json` **顺序敏感、首条命中**（`^CE12800$` 必须排在 `^CE\d+` 前） |
| **落点** | `internal/bizcompare/`（新包，5~7 文件）、`internal/taskexec/bizcompare_compiler.go`+`bizcompare_executor.go`（新）、`internal/models/`（新增 `BizSnapshot`/`BizCompareTask`/`BizCompareItem`）、`internal/ui/bizcompare_service.go`（新）、前端 `views/BizCompare.vue` |
| **成本** | 15~20 人日（数据转换 3、后端 8、前端 5、联调 4） |
| **前置依赖** | A3（解析）、A5（画像/能力白名单）——建议 A5 先行 |
| **风险** | 中。**142 项采集项对应的具体命令在各产品域 xlsx 里**，需先做「采集项键 → 命令」的抽取（Excel 解析，约 20%~30% 项在不同域命令不同）；建议**首期只落地 3 个域**（S / NE-SR / CE），其余后续补齐 |
| **验收** | ① 支持对同一批设备跑两次快照并产出 diff；② 至少 3 个产品域的开箱即用；③ 差异项可按域过滤、可导出 CSV；④ 与 `plancompare` 结果互不干扰 |

---

### A3 · 解析规则资产导入 + 算子管线 ★★★

| 项 | 内容 |
|---|---|
| **资产** | ① `services/IPDeviceCheckService/.../parsecfg/`：`xmlconfig/{huawei 15, h3c 8, ruijie 8, zte 13}` 共 **44 份**解析规则 XML + `parseitem/` 6 份 + `modelCmdConfig.xml` + `dataCategoryConfig.xml` + DTD；② 引擎语义：`CommandParseConfig > SegmentParse > ParseNode > FieldParse > RegexModel` + **4 种 Policy**（Config/TableLine/TableColumn/DynamicColumn）+ **13 个字段算子**（Assign/Filter/MatchAndSet/MergeField/RenameField/ReplaceAll/SplitField/StrConcat/StrExtract/ToUpper/ToLower/ValueMapping/DefaultValue/CustomOperator，有固定执行序 `OPERATOR_ORDER_MAP`）；③ `product/Script/common/CmdEchoParser.py` 的二维模型（`parentItem` / `splitRegex` / `isPath` / `groupIndex` / `splitFlag`） |
| **现状** | `internal/parser/` 已有三引擎（Regex / Aggregate / Tree）、`block_splitter.go`、`result_tile.go`、`mapper.go`（→ `LLDPFact`/`FDBFact`/`ARPFact`）；内置模板 4 份。**但**：无 Operator 概念、无 Policy 概念、Tree 引擎无内置模板在用、仅 4 厂商 |
| **差距** | 复杂回显（Trunk 成员子表、光模块 lanes、`display interface` 的多段结构）无法用现有模板表达；新增厂商需写 Go 代码 |
| **方案** | 1) **数据先行**：把 44 份 XML 用 `encoding/xml` + struct tag 直接反序列化（`CommandParseConfig` 等结构体定义在 `internal/parser/xmlcfg/`），**规则文件原样内置**（`//go:embed parsecfg/`），零损耗；2) **引擎增强**：在 `parser` 中新增 `Operator` 接口 + 13 个实现 + 有序管线（对齐 `OPERATOR_ORDER_MAP`）；新增 4 种 Policy（先实现 `ConfigParsePolicy` + `TableLineParsePolicy`，另两种按需求触发）；3) 与现有三引擎并列为**第四引擎「XmlConfig 引擎」**，由 `ParserManager.GetParserForDevice` 按厂商/命令路由（优先 XmlConfig → 回退 Aggregate → 回退 Regex）；4) 抽取 6 个高频模型（interface / lldp / mac / arp / hardware / transceiver）做 Golden 回归，落在 `testdata/regression/vendor_golden/` |
| **关键设计借鉴** | ① `RegexModel.init()` 先折叠 `\r\n + 缩进` 再 `Pattern.compile(re, 10)`（= MULTILINE|CASE_INSENSITIVE）——Go 侧用 `(?im)` 前缀对齐；② `isBig="true"`（arp/mac）→ 大表走流式；③ `@@` 分隔互斥命令、`isResultMutex` 任一命中即停；④ `ifnamemapping.json` 点对点别名 + `InterfaceTransition.properties` 正则→类型编码，二者互补 |
| **落点** | `internal/parser/xmlcfg/`（新，结构体定义）、`internal/parser/operators.go`（新，13 算子）、`internal/parser/policy.go`（新）、`internal/parser/xmlconfig_engine.go`（新）、`internal/parser/manager.go`（改，路由）、`internal/parser/templates/parsecfg/`（新，44+6+2 份数据） |
| **成本** | 12~15 人日（结构体映射 2、算子 3、Policy + 引擎 4、数据导入与 Golden 4、回归 2） |
| **风险** | 中。**Go 的 RE2 不支持后顾断言 `(?<=...)` 与反向引用**，44 份规则中可能少量不兼容（参考 `09` 篇 §5.3 的 smartping 案例）。处理：加载时校验编译，失败的规则进入 `parsecfg_broken.json` 清单并告警，不阻断启动；对不兼容项人工改写为「先捕获整段再二次提取」 |
| **验收** | ① 44 份规则 ≥ 90% 成功编译；② 6 个模型 Golden 通过；③ 新增一个厂商解析能力**无需改 Go 代码**（仅加规则文件）；④ `parser` 既有测试全绿（不回归） |

---

### A4 · 审计规则库模板化（EMT 规则知识资产） ★★★

| 项 | 内容 |
|---|---|
| **资产** | `services/EMTMessageAnalyseClientService/app_data/script/` 共 3007 py，其中 **`Health/` 2121 条（70.6%）**、`Reliability/` 346 条、`BASE/` 111、`OSPF/` 61、`BGP/` 59、`ISIS/` 47、`MPLS_TE/` 45……；`collectscript/` 1322 py（`huawei/common/` 1019）+ `collectitem/` 1007 XML；规则元数据表 `TBL_AUDIT_RULE` / `TBL_AUDIT_RULE_TYPE_VERSION` / `TBL_AUDIT_RULE_COMMAND_RELATION` / `TBL_AUDIT_RULE_GROUP` / `TBL_BASELINE_INFO` |
| **规则形态** | `def check()` → `ParserTool.dispart(回显, 分块正则)` → 逐块正则取值 → `CheckResult.addResult(status, neId, detail)`；`status ∈ {SUCCESS, FAIL, IGNORE, EXCEPTION}`；文件头注释 `#title_cn / #checkno / #risklevel / #suggest_cn` |
| **现状** | `internal/inspection/`：`engine.go`（`EvaluateItem`）+ `seeds.go` 内置 **12 条**种子（GEN_CPU/MEM/TEMP/FAN/POWER/UPTIME/IF_ERR/NTP_SYNC + CE_CPU/CE_BGP/S_STP/AR_OSPF）；判定类型 6 种（threshold/must_contain/must_not_contain/regex/equals/bound）；规则**已数据驱动**（`inspection_templates` / `inspection_items` + `ThresholdsJSON{Name,DataType,DefaultValue,Min,Max,RangeType,Unit}` 外置）；结论码 8 种 + 4 级严重度；中英双语导出 |
| **差距（真实）** | **不是框架，是规则体量**：12 vs 8312。且现有 12 条偏"资源健康"，缺"配置合规"（BGP/OSPF/MPLS/QOS/安全基线） |
| **方案（关键：只抽语义，不搬 py）** | 1) 定义**规则 DSL（JSON）**：`{checkno, title{zh,en}, category, riskLevel, commands[], scope{vendor[], models[], versions[]}, extract{blockRegex, fields[]}, assert{type, expr}, advice{zh,en}}`；2) `internal/inspection/dsl.go` 实现解释器（复用现有 6 种判定类型 + `EvidenceJSON`/`Advice` 字段，**无需改数据模型**）；3) 抽样转换：首批 **50~100 条**，抽样顺序固定为 `Health/` → `Reliability/` → `BASE/` → `BGP/OSPF`（**不要按 BGP/OSPF 优先，它们各仅 11~61 条**）；4) 状态映射：`SUCCESS→TEST_PASS`、`FAIL→TEST_FAIL`、`IGNORE→TEST_IGNORE`、`EXCEPTION→TEST_EXCEPT`，`detail` 行 → `EvidenceJSON`，`suggestion` → `Advice` |
| **配套** | `TBL_AUDIT_RULE_TYPE_VERSION`（checkno × 产品 × 版本 × 例外设备）→ 直接映射为 `inspection_items` 的 `appliesTo` 白名单，与 `UserParseTemplate` 的 `appliesTo` 复用同一套匹配器 |
| **落点** | `internal/inspection/dsl.go`（新）、`internal/inspection/rules/builtin/*.json`（新，数据）、`internal/ui/inspection_service.go`（改，支持导入/导出规则包）、前端规则管理页（改） |
| **成本** | 10~15 人日（DSL 设计 + 解释器 5、抽样转换 50~100 条 5、前端 3、验证 2） |
| **合规红线** | **只抽取"判定逻辑的事实表达"（命令、正则、阈值、建议文案的语义），不复制 Python 源码文本**。规则标题/建议文案建议**自行重写**为中文表述，避免逐字搬运 |
| **验收** | ① ≥ 50 条规则通过 DSL 跑通，与 Python 原规则在 10 台样本设备上结论一致率 ≥ 90%；② 规则可导入/导出（JSON 包）；③ 不改动 `inspection` 既有表结构（或仅新增，只增不减） |

---

### A5 · 设备画像统一 + 能力白名单 ★★★

| 项 | 内容 |
|---|---|
| **资产** | ① `config/devicemapping/productItem.xml`：18 个 `devType`（含 `index` 显式序号 1~18、`isSoftware` 软硬形态标记、内嵌中英 `<lang>`）；② `IPOnlineService/.../businesscompare/domain.json`：44 条「款型正则 → 产品域」（细粒度，**顺序敏感**）；③ `support_devices.json`：190 款型 × `VxRXXX` 版本矩阵；④ `product/Script/getversion/`（12 厂商，统一返回 8 元组）+ `common/device.py::_REG2HANDLER`（20+ 组有序正则）+ `convert_series`（`S5735-S→S5700`、`CE6866→CE6800`、`AR6280→AR6000`）；⑤ `config/deviceversion/SupportVersion.xml`（899KB，权威版本矩阵，暂缓深挖） |
| **现状** | `internal/device/{Identify, series.go}`；`internal/config/device_profile.go`：`DeviceProfile{Vendor,Name,TopologyEnabled,Selector,PTY{Prompt,Pager,Init,Commands}}`；文件画像仅 `huawei_ce.json`/`huawei_s.json` 两份（其余代码内置）+ DB 覆盖表 `device_profiles`。**无 `device_capabilities`** |
| **差距** | 三套形态规则分散；无法回答"该设备能否执行某模板/某采集项"；`DeviceAsset` 缺 `esn` / `patch_version` / 软硬形态字段 |
| **方案** | 1) 建立**统一设备画像配置** `internal/device/profile_registry.go`：`[{order, regex, category, domain, isSoftware, zh, en}]`，数据源 = `productItem`（粗）+ `domain.json`（细）+ `convert_series`（归一化），**保序、首条命中**；2) 新增 `device_capabilities` 表（`model_pattern`, `versions[]`, `capability_key`, `enabled`），数据来自 `support_devices.json`；3) 在 `taskexec` 编译期做准入校验（设备不在白名单 → 标记 `unsupported` 而非失败）；4) `DeviceAsset` 补 `ESN` / `PatchVersion` / `FormFactor`（hardware|software）字段（只增不减，符合现有升级策略） |
| **关键设计借鉴** | ① 三层粒度递进：大类（18）→ 场景域（44）→ 款型开关（190）；② `index` 显式序号被业务依赖（排序/报表），**不可改用数组下标**；③ `isSoftware` 区分软硬形态 |
| **落点** | `internal/device/profile_registry.go`（新）、`internal/device/series.go`（改，合并 `convert_series`）、`internal/models/device_capability.go`（新）、`internal/models/device_asset.go`（改，3 字段）、`internal/config/device_profiles/*.json`（新，数据）、`internal/taskexec/eligibility.go`（新，准入） |
| **成本** | 8~10 人日（数据合并 3、注册表 2、能力表 + 准入 3、迁移校验 2） |
| **风险** | 低~中。**顺序敏感**：合并多源规则时必须固化顺序并在单测中锁定（建议写 `TestProfileOrderLocked` 固定前 20 条）；`DeviceAsset` 加字段走 AutoMigrate（现有升级流程已支持只增不减） |
| **验收** | ① 190 款型识别结果与 `support_devices.json` 一致；② 能力准入在编译期生效；③ 既有设备数据不丢失（升级前有自动备份） |

---

### A6 · matcher / executor 策略外置（规则驱动） ★★★

| 项 | 内容 |
|---|---|
| **资产** | ① `config/deviceversion/`：`devicevalidate.properties`（**按任务场景**决定 `needValidate` / `validateModel` / `needPrompt`）、`connectCommandPolicy.properties`（回显"拼接命令"策略：`CE1800V=2` 表示回显须以命令开始）、`reciveFilterKey.properties`（接收端 ANSI 残留过滤，如 Linux suse11 的 `[1m`/`[m`）、`keyCmd.xml`（12 厂商 × 2 场景命令表）、`protocoldiscovery.properties`（见 B5）；② `nmotprotocolcbb-model/strategy/`：`SimplePattern` / `ComplexPattern` / `AndOption` / `OROption` / `RegStrategy` / `PatternFactory` / `RuleParser(XML)` / `PatternSpecialChar`；配套模型 `RuleProtocolRegex`、`VendorFilterPattern`、`DeviceFilterReceiveKeyModel`、`VendorCharsetCmd`、`CommandConfig` |
| **现状** | `internal/matcher/`：`StreamMatcher` + `rules.go`（`DefaultRules` **7 条硬编码**，generic/huawei_h3c/cisco，Warning/Critical）+ `confirm.go` + `view.go`；提示符 `DefaultPrompts = [">", "#", "]"]`（硬编码）、分页符 5 条（硬编码）；**支持** `ConfigureFromProfile()` 从画像注入 |
| **差距** | ① 规则硬编码，接新厂商必改代码；② **无场景维度**（采集/巡检/故障应不同）；③ 无「回显首行命令对齐」策略；④ 无厂商字符集命令 |
| **方案** | 1) 定义 `MatchPolicy{ Scene, Vendor, DeviceType, PromptPatterns[], PagerPatterns[], ErrorRules[], ConfirmPatterns[], ValidateModel, NeedPrompt, ReceiveFilterKeys[], ConnectCommandPolicy }`；2) 数据源：内置 `internal/matcher/policies/*.json`（由上述 properties/XML 转换）+ DB 覆盖（复用 `device_profiles` 表模式）+ 画像 `ConfigureFromProfile`；3) `matcher` 改为**策略解析**：`NewStreamMatcher(policy)`，运行时按 `Scene × Vendor × DeviceType` 选取（最长前缀匹配 + 首条命中）；4) `strategy/` 的 And/Or/ComplexPattern 思想落地为 `PromptExpr` 结构（`{all:[...]} / {any:[...]} / {regex:"..."}`），支持 JSON 表达；5) `executor` 侧新增「回显首行对齐校验」开关（吃 `connectCommandPolicy`） |
| **落点** | `internal/matcher/policy.go`（新）、`internal/matcher/policies/*.json`（新，数据）、`internal/matcher/rules.go`（改，改为加载而非硬编码，**保留 DefaultRules 作为兜底**）、`internal/executor/stream_engine.go`（改，回显对齐校验）、`internal/models/match_policy.go`（新，可选 DB 化） |
| **成本** | 8~10 人日 |
| **风险** | 中。**这是执行主链路**，改动风险最高。缓解：① 策略加载失败自动回退到现有 `DefaultRules`；② 新旧策略并行运行一个版本（shadow mode），对比命中差异后再切换；③ 现有 `executor` 集成测试必须全绿 |
| **验收** | ① 现有测试全绿；② 新增厂商（如 RUIJIE/ZTE）策略仅靠 JSON 生效，零 Go 改动；③ 场景维度生效（同一设备在不同 scene 下校验行为不同）；④ shadow 对比差异率 < 1% |

---

## 4. B 档 / C 档 / D 档

### 4.1 B 档：建议迁移（11 个）

| # | 模块 | 资产 | 现状缺口 | 方案要点 | 成本 | 备注 |
|---|---|---|---|---|---|---|
| **B1** | 高危命令治理闭环 | `netcareinside-sdk` 的 `HighRiskCommandInfoBo`/`HighRiskInterceptRecordBo`/`WhiteListCommandInfoBo`/`TrustListBo`/`BlockListBo`/`BypassPolicySchemeBo`/`EscapeLogBo`；`product/config/riskCmd_product.xml`（TaskType→Scope→Category→ProductVersion→Cmd）；`TBL_PROXY_DEVICE.EXTEND_CMD` | **已有** `risk_commands` + `RiskValidator`（block/confirm/warn、厂商专属>通配、20 种子、热更新）；缺：审批流、逃生（Bypass）、留痕（EscapeLog）、默认仅 `warn` 不强制 | ① 新增 `risk_command_logs` 表（拦截/放行/逃生全留痕）；② 引入 `TrustList`（用户×命令×时限）+ `BypassPolicy`（紧急放行需二次确认 + 理由必填）；③ 把 `GlobalSettings.RiskCommandMode` 的**默认值从 `warn` 改为 `enforce`**（需评估存量任务）；④ 导入 `riskCmd_product.xml` 扩充种子 | 6~8 | **改默认值为强制有行为变更风险**，建议先灰度一版只记录不阻断 |
| **B2** | 协议层细节对齐 | `nmotprotocolcbb-ssh`（Apache MINA SSHD 2.18.0）：`SSHReceiver` 默认 `gbk` 字符集、`MAX_LENGTH = 0xA00000`（10MB 单次回显上限）、`PtyChannelConfiguration`、SOCKS5（`Socks5Utils`）、主机密钥指纹（`KeyUtils`/`BuiltinDigests`）、错误关键字表（`Permission denied`/`auth fail`/`Username or password invalid`）；`BuildSSHData`→`ProtocolSSH`（`authenticationMode` USER/KEY/KEY_PWD、`superPassword`、`proxyId`、密钥 ≤ 5KB） | `sshutil` 已有 PTY + 三模式主机密钥 + 算法预设；**缺**：字符集转换（现纯 UTF-8，中文设备回显可能乱码）、逐流回显上限（现为全局 `RawBufferLimitMB`=8）、代理/SOCKS5、错误关键字识别 | ① `sshutil` 增加 `Charset` 配置（gbk/utf-8 自动探测，用 `golang.org/x/text` 转码）；② 增加 `MaxEchoBytes`（默认 10MB）逐流上限 + 超限截断告警；③ 支持 `ProxyDialer`（SOCKS5，复用 `golang.org/x/net/proxy`）；④ 错误关键字表外置为 JSON，接入 `matcher` 错误检测 | 5~8 | 中文乱码是**现场才会暴露**的问题，优先级建议提前 |
| **B3** | 终端仿真补齐 | `nmotprotocolcbb-msgfilter` **116 类**：`TerminalInterpreter`/`Screen`/`TerminalBuffer`/`TerminalXTerm`；`interprethandler/`（AnsiPrinter/Bell/Bs/Cbt/Tab/Tbc/TrackMouseh/Vmto/Vmot2/XTermSeq）；`dpmodeshandler/`（DpModeBreak/GSetCopy/GSetCopyAndCursorSave/GSetsReset/VtResetAndCursorSave/WindowRelative）；`sgrmodeshandler/`（AttributeSet/AttributesClear/BackgroudColor/ForegroudColor/GSets/SgrModeBreak） | `internal/terminal/ansi.go` 仅支持基础 CSI（光标 A/B/C/D/H/f、擦除 K/J、SGR）；**无** OSC / DEC 私有序列 / 滚动 / 插入删除行；`replayer.go` 已有宽度可配的行缓冲 | ① 补齐 DEC 私有序列（`?1049` 等 alt screen、`?25` 光标显隐）；② 补齐 DP-modes（GSetCopy 系列）与 Tab/Tbc；③ 补齐 OSC（标题/剪贴板）；④ 建立 ANSI 序列 Golden 测试集（`testdata/ansi/`） | 6~10 | 建议先做「未知序列统计」：现网跑一批设备，按 UnknownCount 排序后再决定补哪些（避免为 1% 场景写 100% 代码） |
| **B4** | LLDP 场景化规则外置 + 弱光阈值落表 | `services/IPDeviceCheckService/.../lldparseconfig/` 7 份 JSON（enterprise / service / ipmaster / ipmaster2026 / manually / manually2026 / toolchain）；`OpticalCheckRule`（rx/tx/bias 的 Max/Min + `xxxMaxDifferenceRange` + `OffsetMode` percent\|absolute）；`LldParseTemplateMapper.xml` / `OpticalCheckRuleMapper.xml`（**库表驱动**）；算法 `LowLightCalCheckRuleMgr`（PERCENT 模式 `diff=(high-low)*ruleValue/100`，按规则名含 Max/Min 决定 SUBTRACT/ADD） | `topology_builder.go` 的 LLDP 已模板驱动（`huawei.json` aggregate），但**只有一套规则、无场景维度**；无光模块/弱光能力 | ① 把 LLDP 解析规则抽为**场景包**（默认 `enterprise`），`TopologyVendorFieldCommand` 增加 `scene` 维度；② 新建 `optical_check_rules` + `lld_parse_templates` 表 + 前端管理页；③ 实现 `internal/optical/`（percent/absolute 两种阈值模式 + 差值检查） | 8~10 | 弱光是新专项能力（对应 eDesk `FiberOptics-Check`），若无明确需求可拆分为 B4a（LLDP 外置，3~4 人日）+ B4b（弱光，5~6 人日） |
| **B5** | 拓扑发现源扩展 + 端口角色语义 | `config/deviceversion/protocoldiscovery.properties`（LLDP / **CDP(CISCO)** / **OSPF** / ARP + 厂商白名单 + **ARP 误纳用户终端的业务风险提示**）；`IPOnlineService/.../template/manmap/` 5 xlsx（端口 → 角色：上联/下联/互联/堆叠/带外） | 发现源仅 LLDP + ARP/FDB 推断，**无 CDP**；`DiscoveryMethods` 字段已预留但未填充；拓扑边**无 role 语义** | ① 增加 CDP 发现源（复用 `parser` 的 cisco 模板 + 新增 `cdp_neighbors`）；② `DiscoveryMethods` 落库并在前端可勾选；③ 复刻 ARP 风险提示（勾选 ARP 时提示"可能纳入大量用户终端，降低探测效率"）；④ 拓扑边增加 `role` 字段（来源：manmap 默认模板 + 用户覆盖），前端 `TopologyGraph` 分色 | 8~12 | 端口角色对"看得懂拓扑"提升明显，建议与 B4 合并排期 |
| **B6** | 智能 Ping 规则 | `IPOnlineService/.../template/smartping/`（3 文件，`rule/ipv6Interface.json`：`cmd` + `splite` + `filter` + `values[{regexName, regex, patterFlag, all}]`）+ `pingtest/`（21 文件，16 xlsx + 5 json，含 `rule/`、`en/`、`zh/`） | `internal/icmp/` 已有 Ping + Tracert + GeoIP，**但无"先解析出对端地址再 Ping"的联动** | ① 新增 `internal/smartping/`：规则结构 `cmd → 分块正则 → 过滤 → 取值` 产出目标地址集；② **注意 RE2 不支持后顾断言 `(?<=...)`**，需改写为「先捕获整段再二次提取」（`09` 篇已标注）；③ 与现有 `icmp.Engine` 对接，产出「逐接口可通性矩阵」 | 5~8 | 结构与 `UserParseTemplate`（Pattern + FieldMapping）**高度同构**，几乎可直接反序列化，成本低 |
| **B7** | 数据模型合并 + 版本化迁移 | `dbScript/`（57 SQL，5 组）+ `IPOnlineService/db/init/`（1 init + **133 个时间戳 patch**）+ EMT 规则库 SQL；关键表：`tbl_ne_common`（含 `patch_version` / `base_protocol_*`）、`tbl_protocol_{ssh,telnet,serial,mml,snmp,ipmi,https,vmm}`（**多协议并存**，FK 级联）、`tbl_ne_analysis`（含 `item_packets` CLOB **原始回显**）、`tbl_collect_task_item_entity`、`tbl_whitelist`、`TBL_COLLECT_PLUGIN_RULE`（rule_content CLOB）、`tbl_pipeline_entity` | 纯 GORM AutoMigrate（18+21 表），**无版本化、无审计链**；设备单凭据 | ① 引入 `internal/config/migrations/`：`0001_init.sql` 基线 + 有序 `YYYYMMDDHHMMSS_*.sql` 补丁（**只借鉴命名与组织方式，不照搬 133 个 patch 内容**）；② 保留 AutoMigrate 作为"新表补齐"的兜底，**二者分工明确**：migration 管结构演进，AutoMigrate 只补全新实体；③ 按 A5 补 `ESN`/`PatchVersion`/`FormFactor`；④ 多协议凭据**暂不拆表**（改动面大），改为在 `DeviceAsset` 增加可选 `AltProtocol` JSON 字段 | 10~15 | 只借鉴**范式**，切勿照搬三套 schema（06 篇已明确警告 `network/1.2.2.sql` 与其他重复） |
| **B8** | 接口名标准化补全 | `config/script/InterfaceTransition.properties`（`<标准名>=<数字>\|<正则>`，**ifType 编码：Ethernet=6 / Pos=39 / Vlanif=53 / Tunnel=131 / Trunk=161**）+ `InterfaceFiltrate.properties` + `InterfaceTransitionblack.properties` + `ciscoInterfaceTransition.ini` + `ifnamemapping.json`（点对点别名，如 `25GE → Twenty-FiveGigE`） | `internal/normalize/` 已有接口名归一化，**但无 ifType 数字编码、无厂商别名表** | ① 新增 `normalize/iftype.go`（数字编码双向映射）；② 合并别名表为 `normalize/data/ifname_alias.json`；③ **保序**（归一化表顺序敏感） | 3~5 | 成本被 `08` 篇高估，实际很小；建议搭 A3 的车一起做 |
| **B9** | 告警规则与归并引擎 | `IPOnlineService/.../template/alarm/` 9 xlsx：`alarm-rule.xlsx`（总表）+ `-template` + 分族 `-CE` / `-Router` / `-S` / `-USG` / `-VNE` + `alarmTemp-cn/en` | 无告警采集、无告警表、无归并引擎；`inspection` 的 `alarmActive` 类能力不存在 | ① 前置：先有告警采集（`display alarm active`，A3 的 44 份规则已含 `huawei/display alarm active`）；② 新建 `internal/alarm/`：`alarm_rules` 表（分族）+ 归并引擎（原始告警 → 可判定故障现象）；③ 结果接入巡检报告与 `bizcompare` 影响清单 | 10~15 | **依赖告警采集先行**，否则空有规则；建议与 A3 同批 |
| **B10** | 跳板/代理/VPN 接入 | `config/cmd.properties` 13 条模板（`hw_vpn_cmd=telnet vpn-instance %s %s %s`、`hw_proxy_ssh_cmd=stelnet %s %s`、`unix_ssh_cmd=ssh -l %s %s -p %s`、`cs_ssh_cmd=ssh %s %s`、ipv6 变体…）+ `nmotprotocolcbb-multihop`（34 类）+ `config/multihop/`（18）+ `TBL_PROXY`/`TBL_PROXY_DEVICE` | 仅直连；`sshutil` 无代理支持 | ① `DeviceAsset` 增加 `ConnectMode`（direct / jump / proxy / vpn）+ `JumpHost` 引用；② 新增 `internal/connutil/jumphost.go`：先连跳板 → 执行模板命令 → 转入目标会话（`executor` 需支持"登录后前置命令序列"）；③ 命令模板外置为 `connect_commands.json` | 8~12 | **无真实客户需求验证**，且改动连接主链路（风险高）。建议：先只做 **SOCKS5 代理**（B2 的一部分，成本低），跳板机留到有明确需求 |
| **B11** | 采集编排语义增强 | `nmotbusinesscbb-scriptmgr/collectitem/`：`PreCollectItem`（`isPreCollect="true"`，结果可被后续复用）/ `ThresholdManagement`（阈值外置）/ `ProductVersion`（款型版本白名单，`/common` 全版本）/ `UnsupportProductVersion`（黑名单）；`dataCategoryConfig.xml` 的 `isBig="true"`（arp/mac 走流式）；`product/Template/inspector/.../*.xml`（`TemplateItem` 可无限嵌套 + `threshold{DataType,defaultValue,maxValue,minValue,rangeType}`） | `inspection` 已有 `IsPreCollect`，但语义未统一；大表（ARP/MAC）与小表未分流；阈值已外置但无嵌套模板 | ① 统一 `PreCollectItem` 语义：前置采集项结果进入上下文，后续项可引用（避免重复下命令）；② 大表标记 `isBig` → 采集走流式落盘（复用 `report/raw_logger.go`）；③ 模板树支持嵌套 + 继承 | 5~8 | 与 A4 的 DSL 有重叠，建议**合并设计**：A4 的规则 DSL 直接吃 `TemplateItem` 嵌套 + `threshold` 结构 |

### 4.2 C 档：可选（按需取用，不单独立项）

| # | 模块 | 资产 | 说明 |
|---|---|---|---|
| C1 | 离线命令模板 | `EMT/Offline/Command/` 2261 xlsx + `offlineCommandConfig.xml` | 对应 `taskexec/replay_executor.go` 的离线重放。价值：给"导入回显 → 重新判定"提供官方命令清单。成本 3~5 人日（抽取 + 内置）。**前提：离线重放确有使用** |
| C2 | 报告导出格式 | `EMT/template/` 311 xlsx + 8 docx + 6 pptx | 现报告仅 CSV/JSON。可增加 **xlsx 导出**（`excelize`）。**只借鉴格式与字段组织，不搬模板文件**。成本 3~5 人日（若只做 xlsx） |
| C3 | 入参校验中间件 | `config/validate/` 55 XML（服务类 44 + 资源类 11） | 可作为 Wails 服务层入参校验的规则来源。**建议只抽取与"任务/设备/采集项"相关的 5~8 份**做种子，不做全套。成本 3~5 人日 |
| C4 | KPI 指标分析 | `IPOnlineService/tools/kpi_detector_cli/` 27 文件（24 py：models/parsers/src/plot） | 采集数据的二次指标分析 + 绘图。**建议先以子进程旁路验证价值，确认后再 Go 化**。成本：旁路 1~2 人日 / Go 化 8~10 人日 |
| C5 | 关键命令种子 | `config/deviceversion/keyCmd.xml` 12 厂商 × 2 场景 | 作为「配置备份 / 拓扑采集」的**默认命令组种子**内置。成本 1~2 人日，搭 A5 的车 |
| C6 | APP 功能边界 | `apps/` 5 个 APP（`Easy-Health` / `Easy-Diagnosis` / `Easy-RFC` / `FiberOptics-Check` / `Platform-App`）+ 各自的四段式菜单（资源管理/任务/模板/分析） | 用作**前端信息架构参考**。NetWeaverGo 已有 15 个路由视图，不必照搬；仅在做 A2/B9 时参考其"变更/故障"分类。成本 ~0 |

### 4.3 D 档：明确不建议迁移（9 项）

| # | 模块 | 理由 |
|---|---|---|
| D1 | **Jython / Python 脚本引擎本体**（8312 py、`jython-standalone-2.7.4`） | 引入 Python 运行时 = 引入分发体积、版本兼容、安全沙箱三重负担。NetWeaverGo 已有声明式解析 + 巡检 DSL（A3/A4），**规则语义可通过 DSL 表达，无需脚本引擎** |
| D2 | **Java 微服务基础设施**（Spring Boot 3.5 / Tomcat / MyBatis / HikariCP / H2 / Redis / Netty） | 与 Wails 单体桌面架构完全不匹配 |
| D3 | **Electron / JRE / launcher / bin 启动脚本**（58 pak、156 JRE 文件、22 bat） | 桌面壳已被 Wails v3 取代 |
| D4 | **三套数据库 schema 原文照搬**（`dbScript/` 57 + `IPOnlineService` 134 + EMT 71 SQL） | 06 篇已确认存在重复镜像（`network/1.2.2.sql`）；且 NetWeaverGo 是单库 GORM 模型。**只借鉴实体关系与字段设计（B7）** |
| D5 | **133 个 patch 原文** | 只借鉴"时间戳命名 + init 基线 + 有序补丁"的**组织范式**，内容全部重写 |
| D6 | **MML / 串口 / IPMI / VMM / HTTPS 协议**（`nmotprotocolcbb-{mml,serialport}` 等） | 目标设备为数通交换机/路由器，SSH/Telnet/SNMP 已覆盖；引入串口/IPMI 无场景 |
| D7 | **EVA 设备端分布式代理推送**（`nmotdcscriptcbb-service` 247 类、`DistributeScriptMgr`、`eva.cliArray(view,cmd)` 脚本 JSON + XFTP 上传） | 依赖设备端 EVA 运行时（华为私有），NetWeaverGo 无对应生态；且需要设备侧安装脚本，运维风险高 |
| D8 | **4A / SecureCRT 集成、License 鉴权、邮件通知、锁屏、KMC 密钥管理** | 企业内流程集成，与工具核心能力无关；且依赖华为私有服务 |
| D9 | **`devDirCfg.xml`(158KB) / `SupportVersion.xml`(899KB) 深挖** | 体量最大但结构未明，投入产出比不确定。**列为"待专题"**：若 A5 落地后仍发现版本/能力判定不准，再立项（预估 5~8 人日） |

---

## 5. 与 `08` 篇的校准差异（6 处）

| # | `08` 篇判断 | 实测校准 | 影响 |
|---|---|---|---|
| 1 | "logger 仅通用脱敏，需建分厂商脱敏管线" | 已有 17 条规则 + `report/sanitizer_check.go` 导出前阻断 | 结论不变（仍 P0），**工作量 5~8 → 2~3 人日** |
| 2 | "无高危命令拦截，需建 `risk_commands` 表" | 已有表 + `RiskValidator` + 20 种子 + 热更新 | 从"新建"→"**治理闭环补全**"，优先级 ★★★ → ★★ |
| 3 | "LLDP 解析硬编码在 Go 里" | 已走 `parser` aggregate 模板 | 问题重定义为"**缺场景维度**"，方案从"重构"→"加 scene 维度" |
| 4 | "`parser` 已有 Tree 引擎，核对覆盖度即可" | Tree 引擎**零内置模板在用**，且缺算子与 Policy | **工作量被低估**，从"核对"→"新增第四引擎 + 13 算子"，升为 A 档 |
| 5 | "`inspection` 需抽象规则模板" | 已数据驱动（DB 模板 + 阈值外置 + 6 判定 + 8 结论码） | 方向修正：不是"建框架"，而是"**灌规则体量 + 补配置合规域**" |
| 6 | 跳板接入列为 P2、接口名标准化 P3 | 跳板无需求验证且改连接主链路（后置）；接口名标准化因 `normalize/` 已有基础，成本仅 3~5 人日（**提前**） | 顺序调整 |

**新增（08 篇未列）**：A6 的「按任务场景的校验/提示策略」（`devicevalidate.properties`）与 B2 的「gbk 字符集」——后者是中文设备回显乱码的直接成因，建议在 B 档中**优先**。

---

## 6. 落地路线图

### 一期（约 25~30 人日）— 合规 + 数据底座

| 模块 | 目标 |
|---|---|
| A1 分厂商脱敏 | 17 Category 全量接入，导出前分级阻断 |
| A5 设备画像统一 + 能力白名单 | 三源合并保序；`device_capabilities` 准入；`DeviceAsset` 补 3 字段 |
| A3 阶段一：数据导入 + 结构体映射 | 44 份 XML 可加载、可编译（先不接引擎），产出不兼容清单 |

### 二期（约 45~55 人日）— 解析与执行硬化

| 模块 | 目标 |
|---|---|
| A3 阶段二：算子 + Policy + XmlConfig 引擎 | 第四引擎上线，6 模型 Golden 通过 |
| A6 matcher/executor 策略外置 | 策略 JSON 化 + 场景维度；shadow mode 验证后切换 |
| B2 协议层细节（**含 gbk 字符集优先**） | 字符集协商、10MB 上限、SOCKS5、错误关键字表 |
| A2 变更前后业务比对（首期 3 个域） | 新 Compiler + Executor + 快照与 diff + 前端视图 |
| B1 高危命令治理闭环 | 留痕 + 逃生 + TrustList；默认强制视灰度结果定 |
| B8 接口名标准化（搭车） | ifType 编码 + 别名表 |

### 三期（约 40~50 人日）— 语义与专项能力

| 模块 | 目标 |
|---|---|
| A4 审计规则库模板化（50~100 条） | DSL + 解释器 + 规则包导入导出 |
| B4 LLDP 场景包 + 弱光（可拆 a/b） | 场景化规则 + `optical_check_rules` 表 + 前端 |
| B5 拓扑发现源扩展 + 端口角色 | CDP + `DiscoveryMethods` 落库 + 边 role 分色 |
| B6 智能 Ping | RE2 改写 + 逐接口可通性矩阵 |
| B7 数据模型 + 版本化迁移 | migration 目录 + AutoMigrate 分工 |
| B3 终端仿真补齐（按 UnknownCount 排序后定范围） | DEC 私有 + DP-modes + OSC |

### 四期（约 25~35 人日）— 编排增强与可选能力

| 模块 | 目标 |
|---|---|
| B11 采集编排语义（与 A4 DSL 合并设计） | PreCollectItem 复用 + 大表流式 + 嵌套模板 |
| B9 告警规则与归并（依赖 A3 告警采集） | 分族规则 + 归并引擎 |
| B10 跳板接入（**仅限有真实需求时**） | ConnectMode + 跳板命令模板 |
| C1/C2/C3/C5 按需 | 离线命令 / xlsx 导出 / 入参校验 / 命令种子 |

**合计：135~170 人日**（不含需求细化与回归缓冲；建议按 1.3 系数计入项目管理成本）。

---

## 7. 风险登记

| 风险 | 等级 | 说明 | 缓解措施 |
|---|---|---|---|
| **知识产权合规** | **高** | eDesk Pro 为华为商业软件；反编译产物、Jython 源码、Excel 规则模板均受著作权保护 | ① 只吸收**事实型数据**（CLI 命令、正则、阈值、OID、ifType 编码）与**设计范式**；② 规则标题/建议文案**自行重写**；③ 不复制任何源码文本；④ 本报告及后续产出仅限内部评估使用，不随产品分发；⑤ 若涉及对外交付，建议法务评审 |
| **执行主链路改动**（A6 / B2 / B10） | 高 | matcher/sshutil 改动直接影响所有任务 | shadow mode 并行验证 → 灰度 → 全量；保留旧行为作为回退默认值；现有集成测试作为准入门槛 |
| **RE2 兼容性**（A3 / B6） | 中 | Go `regexp` 不支持后顾断言与反向引用 | 加载期编译校验 + 不兼容清单 + 人工改写为二次提取；`09` 篇已提供改写思路 |
| **规则顺序敏感**（A5 / A3） | 中 | `domain.json` / `productItem.xml` / 归一化表均"首条命中"，错序即误判 | 单测锁定顺序（`TestProfileOrderLocked`）；数据文件加 `order` 显式字段而非依赖数组下标 |
| **默认行为变更**（B1 强制拦截） | 中 | 把 `RiskCommandMode` 从 `warn` 改 `enforce` 会改变存量任务行为 | 先只记录不阻断一个版本；提供全局开关与按任务覆盖 |
| **规则体量失控**（A4） | 中 | 8312 条若全量转换，维护成本极高 | 严格按 A4 抽样策略（50~100 条起步）；DSL 只覆盖可用声明式表达的**判定型**规则，过程型规则放弃 |
| **数据质量**（A2） | 中 | 142 采集项的具体命令分散在 18 个 xlsx 中，部分域差异大 | 首期只做 3 个域；建立"采集项键 → 命令"的映射校对表 |
| **升级兼容** | 低 | 新增表/字段 | 现有策略已支持"只增不减 + 启动前自动备份（保留 5 份）"，沿用即可 |

---

## 8. 复用度说明（为什么成本可控）

NetWeaverGo 已有的三块"地基"使多数迁移可以**搭车**而非从零：

1. **`taskexec` 五层架构**（Compiler Registry / RuntimeManager / StageExecutor / EventBus / SnapshotHub）：A2 只需**新增**第 6 个 Compiler + 1 个 Executor，不改框架；B11 只需扩展 `InspectionPipelineMode` 已有的三阶段模型。
2. **`parser` 三引擎 + `ParserManager` 路由**：A3 的 XmlConfig 引擎作为**第四引擎并列接入**，既有模板与调用方零改动。
3. **`inspection` 数据驱动模型**（DB 模板 + `ThresholdsJSON` + 6 判定 + 8 结论码 + 中英导出）：A4 的 DSL 只需新增一个**解释器**，数据模型、报告导出、前端展示全部复用。

此外：`models`/`repository` 的分层使新增实体（如 `device_capabilities`、`optical_check_rules`、`risk_command_logs`）的成本约为 0.5 人日/表；`ui` 已有 20 个服务与前端 15 个路由视图的既有范式，新增页面可参照 `PlanCompare.vue` / `Inspection*.vue`。

---

## 9. 建议的下一步动作

| # | 动作 | 产出 | 耗时 |
|---|---|---|---|
| 1 | 编写 `sensitiveCmd.xml` → JSON 的转换脚本（一次性），评估规则条数与 RE2 兼容率 | `tools/convert_sensitive/main.go` + 兼容率报告 | 0.5 天 |
| 2 | 解析 44 份 `xmlconfig/*.xml`，产出 Go 结构体映射草案 + 编译失败清单 | `internal/parser/xmlcfg/` 草案 + `parsecfg_broken.json` | 1 天 |
| 3 | 用 Excel 解析 `businesscompare/` 的 `cmd.xlsx` + 3 个 `scene/*.json`，产出「采集项 → 命令」映射草案（限 S / NE-SR / CE 三域） | `docs/draft/bizcompare_mapping.md` | 1 天 |
| 4 | 从 `EMT/app_data/script/Health/` 取 20 条 + `Reliability/` 取 20 条，抽象 DSL 字段集 | `docs/draft/inspection_dsl.md` | 1 天 |
| 5 | 对照 `nmotprotocolcbb-model/strategy/` 与 `SSHProtocol.java`（Kali `/data/src/cbb/`），列出 `matcher`/`sshutil` 差异清单 | `docs/draft/matcher_ssh_gap.md` | 1 天 |
| 6 | 现网采集一批 ANSI UnknownCount 统计（决定 B3 范围） | 统计报告 | 0.5 天 |

> 上述 6 项合计约 **5 人日**，完成后即可对 A1/A2/A3/A4/A6 的估算做一次 ±20% 精度的重估，再进入一期开发。

---

## 10. 附：资产 → 结论速查（全量）

| eDesk Pro 资产 | 出处 | 结论 | 档 |
|---|---|---|---|
| `sensitiveCmd.xml`（17 族脱敏正则） | `config/deviceversion/` | 直接搬，接 `report`+`logger` | A1 |
| `businesscompare/`（142 项 + 44 域 + 16 场景 + 190 款型） | `IPOnlineService` | 新建 `bizcompare` | A2 |
| `parsecfg/xmlconfig/` 44 份 + 13 算子 + 4 Policy | `IPDeviceCheckService` | 原样搬规则 + Go 重写引擎 | A3 |
| `EMT/app_data/script/`（Health 2121 / Reliability 346…） | `EMTMessageAnalyseClientService` | **只抽语义** → DSL | A4 |
| `productItem.xml` + `domain.json` + `support_devices.json` + `device.py` | `config/` `IPOnlineService` `product/` | 合并为统一画像 + 能力表 | A5 |
| `deviceversion/*` + `strategy/`（And/Or/ComplexPattern） | `config/` `nmotprotocolcbb-model` | matcher 规则化 | A6 |
| `HighRisk*`/`WhiteList*`/`Bypass*`/`EscapeLog*` + `riskCmd_product.xml` | `netcareinside-sdk` `product/config` | 治理闭环（拦截已存在） | B1 |
| `nmotprotocolcbb-ssh`（gbk/10MB/PTY/SOCKS5/指纹） | `lib/` | 对照补齐 | B2 |
| `nmotprotocolcbb-msgfilter`（116 类） | `lib/` | 按 UnknownCount 排序后补齐 | B3 |
| `lldparseconfig/` 7 JSON + `OpticalCheckRule` | `IPDeviceCheckService` | 场景包 + 落表 | B4 |
| `protocoldiscovery.properties` + `manmap/` 5 xlsx | `config/` `IPOnlineService` | CDP/OSPF + 边 role | B5 |
| `smartping/` + `pingtest/` | `IPOnlineService` | 需 RE2 改写 | B6 |
| 三套 schema + 133 patch + 多协议凭据 | `dbScript/` `IPOnlineService/db` | **只借鉴范式** | B7 |
| `InterfaceTransition.properties`（ifType）+ `ifnamemapping.json` | `config/script` | 补全 normalize | B8 |
| `alarm/` 9 xlsx | `IPOnlineService` | 依赖告警采集先行 | B9 |
| `cmd.properties` 13 模板 + `multihop` CBB | `config/` | 需求未验证，后置 | B10 |
| `CollectItem` 语义（PreCollect/Threshold/isBig） | `nmotbusinesscbb-scriptmgr` `product/` | 与 A4 DSL 合并设计 | B11 |
| `Offline/Command/` 2261 xlsx | `EMT` | 可选，供离线重放 | C1 |
| `template/` 311 xlsx + docx/pptx | `EMT` | 只借鉴格式（xlsx 导出） | C2 |
| `validate/` 55 XML | `config/` | 抽 5~8 份做种子 | C3 |
| `kpi_detector_cli` 24 py | `IPOnlineService/tools` | 旁路验证后再定 | C4 |
| `keyCmd.xml` 12 厂商 | `config/deviceversion` | 命令组种子 | C5 |
| `apps/` 5 APP 边界 | `apps/` | 信息架构参考 | C6 |
| Jython 引擎 / 8312 py 本体 | `lib/` `EMT` | **不搬引擎** | D1 |
| Java 微服务基础设施 | `lib/` `services/` | 不搬 | D2 |
| Electron / JRE / bin / launcher | 根目录 | 不搬 | D3 |
| 三套 schema 原文 | `dbScript/` 等 | 不照搬 | D4 |
| 133 patch 原文 | `IPOnlineService/db/init` | 只借鉴命名 | D5 |
| MML / Serial / IPMI / VMM | `nmotprotocolcbb-*` | 无场景 | D6 |
| EVA 分布式代理 | `nmotdcscriptcbb-service` | 依赖设备端生态 | D7 |
| 4A / SecureCRT / License / 邮件 | `services/` | 与核心能力无关 | D8 |
| `devDirCfg.xml` / `SupportVersion.xml` | `config/deviceversion` | 待专题 | D9 |

---

*本报告基于对 `docs/eDeskPro_V100R025C10SPC300_功能模块分析/` 全部 12 篇文档，以及对 `NetWeaverGo/internal/**` 的源码级实测。所有"现状"结论均可通过对应代码位置复核。*
