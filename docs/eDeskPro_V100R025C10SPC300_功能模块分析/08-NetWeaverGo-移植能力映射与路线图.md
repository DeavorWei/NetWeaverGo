# 08 · NetWeaverGo 移植能力映射与路线图

> 综合 `00`~`10`（含第二轮新增的 `09`/`10`），将 eDesk Pro 各模块资产映射到 NetWeaverGo，给出**按性价比排序**的落地路线。
> 本轮相比第一轮的变化：**新增 `IPOnlineService`（业务比对）与真实协议层 CBB 两大类资产**，并发现一批**纯配置型**高价值资产（脱敏库、LLDP 规则、接口名编码…），优先级已重排。

---

## 0. NetWeaverGo 现状对照（用于定位缺口）

| 已有模块（`internal/`） | 已有能力 |
|---|---|
| `parser/`（Regex + Aggregate + **Tree** 三引擎）、`models.UserParseTemplate` | 模板驱动回显解析（已实现二维规则树！） |
| `device/`（`Identify` + `series.go`） | 华为设备形态/系列识别 |
| `executor/`、`terminal/`、`matcher/`、`sshutil/`、`telnetutil/`、`sftputil/` | 单设备命令执行与终端仿真 |
| `taskexec/`（五层架构 + 7 编译/执行器） | 任务编排、拓扑构建、离线重放 |
| `ceas/`、`inspection/` | 硬件清单（elabel）、巡检规则引擎 |
| `plancompare/`、`forge/`、`icmp/`、`snmp/`、`fileserver/`、`report/`、`logger/` | 规划比对、配置生成、Ping、SNMP、文件服务、报告、日志 |

**缺口（本包对应资产）**：业务级双时点比对、合规审计规则库、分厂商脱敏、告警归并、端口角色语义、跳板接入、高危命令拦截、LLDP 规则外置、设备多协议并存、数据库版本化迁移。

---

## 1. 资产 → NetWeaverGo 能力映射表（全量）

| # | eDesk Pro 资产（出处） | NetWeaverGo 现状 | 建议动作 | 优先级 |
|---|---|---|---|---|
| 1 | **`sensitiveCmd.xml`** 17 产品族/厂商脱敏正则（`10` §6.1） | `logger/sanitizer.go` 仅通用脱敏 | **P0**：建分厂商脱敏管线（`map[Category][]regexp`），接 `report` 落盘与前端导出 | ★★★ |
| 2 | **`businesscompare/`**：142 采集项 + 44 款型分域 + 16 场景 + 190 款型版本矩阵（`09` §4） | `plancompare`（仅"规划 vs 实采"） | **P0**：新建 `internal/bizcompare/`，复用 `taskexec` 五层架构做「变更前/后双时点快照 → 差异 → 业务影响清单」 | ★★★ |
| 3 | EMT 审计规则库 8312 py（`02`，主体在 `Health/` 2121 + `Reliability/` 346） | `inspection` 阈值引擎（12 条种子规则） | **P0**：抽象「规则模板 = 取数命令 + 分块正则 + 判定 + 建议」，优先从 `Health/`/`Reliability/` 抽样 50~100 条 | ★★★ |
| 4 | 声明式解析规则引擎 `CmdEchoParser` + `ResultTreeTile`（`01`）+ `xmlconfig` 44 份（`03`） | `parser` 已有 Tree 引擎 | **P0**：核对现有 Tree 引擎与 `parentItem/isPath/splitRegex` 的覆盖度；把 44 份厂商规则转为内置解析模板 | ★★★ |
| 5 | `riskCmd_product.xml`（`05` §5）+ `sensitiveCmd.xml` + `HighRisk*` BO（`10` §5） | 无 | **P0**：建 `risk_commands` 表 + 执行前拦截 + 白名单/逃生/留痕 | ★★★ |
| 6 | 协议层 CBB：PTY / `gbk` 默认字符集 / 10MB 回显上限 / SOCKS5 / 指纹校验 / 错误关键字表（`10` §3.2） | `sshutil` 已有基础 PTY 与算法预设 | **P1**：对照补齐字符集协商、回显上限、SOCKS5、错误关键字识别 | ★★ |
| 7 | `nmotprotocolcbb-model/strategy/*`：And/Or/ComplexPattern + RuleParser(XML)（`10` §3.1） | `matcher` 硬编码提示符/分页/错误规则 | **P1**：把提示符识别改为**XML/JSON 规则驱动**（含 `DeviceFilterReceiveKeyModel`、`VendorFilterPattern`、`VendorCharsetCmd`） | ★★ |
| 8 | 终端仿真 116 类（`10` §3）：DP-modes / SGR / Vmto / Vmot2 / Tab / Tbc | `terminal/ansi.go` + `line_buffer.go` | **P1**：对照补齐处理器，重点 `GSetCopy`/`VtResetAndCursorSave`/`WindowRelative`/`TrackMouseh` | ★★ |
| 9 | `devicevalidate.properties` + `connectCommandPolicy.properties`（`10` §6.2/§6.5） | `matcher` 全局策略 | **P1**：matcher 增加「按任务场景 × 按设备类型」的校验/拼接策略维度 | ★★ |
| 10 | `keyCmd.xml` 12 厂商 × 2 场景命令表（`10` §6.4） | 手工配置命令组 | **P1**：内置为「厂商 → 配置/接口采集命令」默认命令组种子 | ★★ |
| 11 | `CollectItem` 语义：`PreCollectItem` / `ThresholdManagement` / `ProductVersion`（`10` §4.1） | `inspection` 已有 `IsPreCollect` | **P1**：核对并统一"前置采集项 + 阈值外置 + 款型白名单"三语义的模型 | ★★ |
| 12 | `alarm-rule-<族>.xlsx` 分产品族告警规则（`09` §5.1） | 无 | **P1**：建 `alarm_rules` 表 + 告警归并引擎（原始告警 → 可判定故障现象） | ★★ |
| 13 | `manmap/*.xlsx` 端口角色映射（`09` §5.2） | 拓扑边无角色语义 | **P1**：拓扑边增加 `role`（上联/下联/互联/堆叠/带外），前端 `TopologyGraph` 分色 | ★★ |
| 14 | `support_devices.json` 190 款型 × 版本矩阵（`09` §4.4） | `device.Identify` 无能力白名单 | **P1**：建 `device_capabilities` 表，做"该设备可执行哪些任务/模板"准入 | ★★ |
| 15 | 三套数据库 schema（`06` §0.1）+ 133 个 patch（`06` §0.1） | GORM AutoMigrate | **P1**：合并为单一模型；引入带版本号的 migration 目录 | ★★ |
| 16 | `lldparseconfig/*.json` 场景化 LLDP 规则（`10` §6.9） | `topology_builder` 硬编码 LLDP | **P2**：LLDP 解析规则外置（企业网/运营商/工具链/手工兜底） | ★ |
| 17 | `OpticalCheckRule` + `LldParseTemplate` 落表（`03` §8.3） | 无 | **P2**：弱光阈值与 LLDP 模板建表 + 前端管理界面 | ★ |
| 18 | `protocoldiscovery.properties`：CDP / OSPF / ARP 发现源（`10` §6.6） | 仅 LLDP/FDB/ARP | **P2**：补 CDP（Cisco）、OSPF 发现源；复刻"ARP 误纳用户终端"风险提示 | ★ |
| 19 | `cmd.properties` 13 条跳板/代理/VPN 命令模板 + `multihop` CBB（`10` §6.7） | 仅直连 | **P2**：新增「连接方式 → 命令模板」配置，支持跳板/堡垒机接入 | ★ |
| 20 | `InterfaceTransition.properties`（**ifType 数字编码**）+ `ifnamemapping.json`（`10` §6.8） | `normalize/` 已有接口名规范化 | **P2**：补 ifType 数字编码与厂商别名表 | ★ |
| 21 | 在线分析库 134 SQL（`09` §6） | — | **P2**：仅作 schema 参考，不照搬 | ★ |
| 22 | `kpi_detector_cli`（24 py）（`09` §5.5） | 无 | **P3**：二阶段指标分析，可子进程旁路 | ★ |
| 23 | `validate/*.xml` 55 份（`05` §9） | 无 | **P3**：Wails 服务层入参校验中间件 | ★ |
| 24 | `devDirCfg.xml`(158KB) / `SupportVersion.xml`(899KB)（`05` §8） | — | **P3**：待专题分析（设备目录树 + 版本矩阵权威源） | ★ |
| 25 | `Offline/Command/` 2261 xlsx（`02` §1.1） | `taskexec/replay_executor.go` | **P3**：离线回显分析的命令模板来源 | ★ |
| 26 | 报告模板 `template/` 311 xlsx + docx/pptx（`02` §1.1） | 报告仅 CSV/JSON/HTML | **P3**：交付物格式参考 | ★ |
| 27 | 5 个 APP 功能边界（`07` §6） | 功能菜单 | **P3**：映射为一级子系统 | ★ |
| 28 | `versionmapping/*.xlsx` / `SupportVersion.xml` / `support_devices.json` 三处同义 | — | **P3**：取 JSON 一份即可，避免重复维护 | ★ |

---

## 2. 建议优先级路线

### P0 —— 合规与"可配置化"（纯配置/低风险，收益最大）

1. **分厂商敏感信息脱敏**（#1）：`sensitiveCmd.xml` → `internal/report` 脱敏管线。**这是交付现场最刚性的合规需求**（回显中的 `cipher` 密文不可外流），且是纯正则数据，几乎零算法成本。
2. **业务比对模块**（#2）：新建 `internal/bizcompare/`。直接吃 `collectCmdList.json`（142 项）+ `domain.json`（44 条）+ `scene/*.json`（16 场景）+ `support_devices.json`（190 款型），复用现有 `taskexec` 的 Compiler/Executor/EventBus 五层架构，成本可控而能力增量大（`plancompare` 只做静态比对，缺双时点）。
3. **合规规则库 + 风险命令拦截**（#3+#5）：从 `Health/`/`Reliability/` 抽样建规则库；建 `risk_commands` 表 + 白名单/逃生/留痕。
4. **解析模板资产导入**（#4）：把 `xmlconfig/` 44 份厂商规则 + EMT 的 `collectitem/` 1007 份 XML 作为内置模板批量导入，验证现有 Tree 引擎覆盖率。

### P1 —— 协议/终端/策略的"对标硬化"

5. **matcher 规则化**（#7+#9）：`strategy/*` 的 And/Or/ComplexPattern + RuleParser 是现成的"提示符识别引擎"设计；配合 `DeviceFilterReceiveKeyModel` / `VendorCharsetCmd` / `devicevalidate.properties`，把 `matcher` 从硬编码变成**按场景 × 按设备**的规则驱动。
6. **协议层细节对齐**（#6）：`gbk` 默认字符集、10MB 回显上限、SOCKS5、指纹校验、错误关键字表——这些都是现场才会暴露的问题，直接从 CBB 抄。
7. **终端仿真补齐**（#8）：DP-modes / SGR / Vmto 处理器。
8. **告警 + 端口角色 + 能力白名单**（#12+#13+#14）：三者都让 NetWeaverGo 从"能采到"升级为"看得懂"。
9. **数据库合并与版本化迁移**（#15）。

### P2 —— 专项能力

10. LLDP 规则外置 + 弱光阈值落表（#16+#17）——对应 eDesk 的 `FiberOptics-Check` 专项。
11. 拓扑发现源扩展（CDP/OSPF）（#18）。
12. 跳板/代理/VPN 接入（#19）。
13. 接口名标准化补全（ifType 编码）（#20）。
14. 在线分析 schema 参考（#21，不照搬）。

### P3 —— 体验与交付

15. 入参校验中间件（#23）、KPI 分析（#22）、离线命令模板（#25）、报告模板（#26）、APP 功能边界（#27）、设备目录树/版本矩阵专题（#24）。

---

## 3. 关键设计借鉴（可直接用）

1. **注册表驱动**：`CollectItem`（XML）/`TBL_AUDIT_RULE_TYPE_VERSION`/`domain.json`/`support_devices.json` —— 「设备画像 → 脚本/规则/场景」白名单，比 if-else 可维护得多；**且都是"顺序敏感、首条命中"**。
2. **三层粒度递进**：`productItem.xml`（18 大类，粗）→ `domain.json`（16 场景域，细）→ `scene/*.json`（具体款型开关，最细）。
3. **阈值/规则外置**：`Template/<threshold>`、`TBL_COLLECT_PLUGIN_RULE.RULE_CONTENT`、`OpticalCheckRule`、`LldParseTemplate` —— 判定逻辑与代码解耦。
4. **场景化规则包**：`lldparseconfig/` 按"企业网/运营商/工具链/手工兜底"分文件（还带年份版本），是"同一回显多客户差异"的正确解法。
5. **前置采集项（pre-collect）**：`isPreCollect="true"`，结果可被后续项复用（CPU/内存预检）。
6. **分块保留块头**：`splitCmdEcho` 用 `match.start()` 而非 `end()`，避免吞掉块头。
7. **命令回显可靠性**：`-i` 超时默认、`[Y/N]` 自动应答、提示符正则 `-p`、回显缓存、密码脱敏。
8. **原始回显留档**：`tbl_ne_analysis.item_packets`(CLOB) 保留原始报文，便于复盘与规则迭代。
9. **协议实现要点**：默认 `gbk`、单次回显上限 10MB、PTY 配置、SOCKS5、指纹校验、错误关键字表。
10. **顺序/编码正确性**：`InterfaceTransition.properties` 的 `ifType` 数字编码（Ethernet=6/Pos=39/Vlanif=53/Tunnel=131/Trunk=161）、`devType` 的显式 `index`、`domain.json` 的顺序敏感——**归一化表必须保序**。
11. **大表分流**：`dataCategoryConfig.xml` 的 `isBig="true"`（arp/mac）→ 小表直读、大表流式落盘。
12. **数据库版本化**：133 个时间戳 patch 构成可审计迁移链。
13. **多协议并存**：同一网元可并存 SSH/Telnet/Serial/MML/SNMP/IPMI/HTTPS/VMM 凭据（`tbl_protocol_*`），NetWeaverGo 目前每设备仅一套。
14. **高危命令治理闭环**：`HighRisk*` → 拦截记录 → `WhiteList*`/`TrustList*` → `BypassPolicyScheme`（逃生）→ `EscapeLogBo`（留痕）。

---

## 4. 与旧分析的衔接

- 旧文档 `docs/华为eDeskPro脚本体系分析.md` 覆盖 `product/Script`（见本系列 `01`）。
- 第一轮新增：`02`(EMT 审计引擎)、`03`(IPDeviceCheck 解析)、`04`(NMOT 协议层)、`05`(配置)、`06`(DB)、`07`(APP)、`08`(本路线图)。
- **第二轮新增**：`09`(IPOnlineService 在线分析/业务比对)、`10`(协议层 CBB + 终端仿真 + 脱敏库 + LLDP 规则 + 接口名标准化)，并修正 `00`/`02`/`03`/`04`/`05`/`06`/`07` 共 7 处事实错误或遗漏。
- **最大增量（第一轮）**：EMT 的 8312 条审计规则库 + Java 后端的解析/弱光光纤算法。
- **最大增量（第二轮）**：
  1. **`IPOnlineService`** —— 业务比对 / 告警规则 / 端口角色 / 智能 Ping，是完全独立的第 3 套脚本与模板体系；
  2. **`lib/nmotprotocolcbb-*`** —— 推翻了"协议层不在包内"的结论，协议/终端实现可对照移植；
  3. **`config/deviceversion/sensitiveCmd.xml`** —— 17 产品族脱敏正则库，最高性价比的纯数据资产。

---

## 5. 下一步建议

1. 先打开 `services/IPOnlineService/webapps/ROOT/WEB-INF/classes/template/businesscompare/`，用 Excel 展开 `CheckAndCompareItem.xlsx` / `TaskTemplate_zh.xlsx` / `cmd.xlsx`，产出 `internal/bizcompare` 的**采集项 → 命令 → 判定**映射草案。
2. 用脚本把 `config/deviceversion/sensitiveCmd.xml` 解析为 JSON/Go 常量，评估接入 `internal/report` 的成本（预计 <1 天）。
3. 打开 `services/EMTMessageAnalyseClientService/app_data/script/Health/`，取 20 条 `analyze_*.py` + 20 条 `Reliability/*.py`，抽象规则模板。
4. 对照 `nmotprotocolcbb-model/strategy/` 与 `nmotprotocolcbb-ssh/SSHProtocol.java`（Kali `/data/src/cbb/`），列出 `matcher` / `sshutil` 的差异清单。
5. 打开 `product/config/riskCmd_product.xml` 提取首批风险命令清单。
6. 依据 `06` §0.1 设计 NetWeaverGo 的「网元/任务/采集项/结果/能力白名单」表结构草案（**三套 schema 合并**）。
7. 复核 `devDirCfg.xml`(158KB) 与 `SupportVersion.xml`(899KB)——本包最大的两份配置，尚未专题分析。
