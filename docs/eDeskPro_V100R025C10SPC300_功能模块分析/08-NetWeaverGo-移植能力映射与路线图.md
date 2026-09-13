# 08 · NetWeaverGo 移植能力映射与路线图

> 综合 `00`~`07`，将 eDesk Pro 各模块资产映射到 NetWeaverGo，给出**按性价比排序**的落地路线。

## 1. 资产 → NetWeaverGo 能力映射表

| eDesk Pro 资产（文档） | 对应 NetWeaverGo 现状 | 建议动作 |
|---|---|---|
| EMT 配置审计规则库（8312 py，`02`） | 无（或零散规则） | **P0**：抽象「规则模板=取数命令+分块正则+判定+建议」，按 BGP/OSPF/QOS/MPLS… 建规则库 |
| 声明式解析规则引擎（`01` CmdEchoParser + `03` xmlconfig） | `models.UserParseTemplate`（单正则） | **P0**：升级为支持 `parentItem`/`splitRegex`/`isPath`/`groupIndex` 的二维模型 |
| elabel 电子标签解析（`01` ceas/Common） | 无 | P1：建 `internal/parser/ceas/`，正则表转 `[]Handler` |
| IPDeviceCheck 正则解析三段式（`03`） | 解析零散 | P1：用 `regexp`+结构体 tag 复刻 xmlconfig + 字段算子 ETL |
| NMOT 连接参数模型（`04`） | `internal/executor`/`sshutil` | P1：复用 `Build*Data`+`NeProtocol*PO` 参数结构；自实现 SSH/Telnet/Netconf/SNMP |
| NMOT 脚本桥 38887/Jython（`04`） | 无 | P2：保留桥语义或改 Go 子进程调 Python；`pool.size=12`→semaphore |
| 设备形态/版本映射（`05` + `01` getversion） | 分散 | P2：统一 `devicemapping/productItem.xml`+`device.py`+`getversion` 为一份「设备画像」 |
| 数据库模型（`06`） | 部分 | P2：按 `06` 建「网元/任务/采集项/模板/结果/规则」表（以 unicnetwork/unicollect 为权威源） |
| 风险命令 `riskCmd_product.xml`（`05`） | 无 | **P0**：建 `risk_commands` 表，执行前拦截 |
| `config/validate/*.xml`（`05`） | 无 | P3：入参校验中间件 |
| 弱光/光纤专项（`03` lowlight + `07` FiberOptics-Check） | 无 | P2 专项：光模块阈值模型 + 链路还原 + 交叉光纤比对 |
| 接口名标准化（`05` Interface*.properties + ifnamemapping.json） | 无 | P3：`ifName` 归一化模块 |
| 5 个 APP 功能边界（`07`） | 功能菜单 | P3：映射为 5 个一级子系统 |

## 2. 建议优先级路线

### P0 — 解析引擎升级 + 风险拦截（收益最大，纯配置化）
1. 升级 `UserParseTemplate` 为二维解析（parentItem/splitRegex/isPath/groupIndex），覆盖 90% 表格型回显（`display interface/device/transceiver/ap all`）。
2. 从 EMT 抽样 50~100 条 `check()` 规则，抽象为「规则模板」结构，建立规则库雏形。
3. 建 `risk_commands` 表并接执行前拦截。

### P1 — CEAS 硬件清单 + 解析 ETL + 协议层
4. 落地 `internal/parser/ceas/`（先做 CE/SW/AR/WLAN 新框架，FW/Route 二期）。
5. 复刻 IPDeviceCheck 的 xmlconfig 正则 + 字段算子 ETL（ToUpper/ValueMapping/SplitField/MatchAndSet…）。
6. 复用 NMOT 连接参数模型，自实现协议连接器。

### P2 — 设备画像 + 数据模型 + 弱光光纤
7. 统一设备形态/版本映射为「设备画像 → 采集项/检查项」注册表。
8. 按 `06` 建核心表（网元/任务/采集项/模板/结果/规则）。
9. 弱光/光纤专项立项（阈值模型 + 链路还原 + 交叉光纤比对）。

### P3 — 校验/标准化/功能边界
10. 入参校验中间件（validate/*.xml）。
11. 接口名标准化模块。
12. 映射 5 个 APP 为一级子系统。

## 3. 关键设计借鉴（直接可用）

- **注册表驱动**：`CollectItem`/`TBL_AUDIT_RULE_TYPE_VERSION` 的「设备画像 → 脚本/规则」白名单，比 if-else 可维护得多。
- **阈值/规则外置**：`Template/<threshold>`、`TBL_COLLECT_PLUGIN_RULE.RULE_CONTENT` 把判定逻辑与代码解耦。
- **前置采集项（pre-collect）**：`isPreCollect="true"`，结果可被后续项复用（CPU/内存预检）。
- **分块保留块头**：`splitCmdEcho` 用 `match.start()` 而非 `end()`，避免吞掉块头。
- **命令回显可靠性**：`-i` 超时默认、`[Y/N]` 自动应答、提示符正则 `-p`、回显缓存、密码脱敏。
- **原始回显留档**：`tbl_ne_analysis.item_packets`(CLOB) 保留原始报文，便于复盘与规则迭代。

## 4. 与旧分析的衔接

- 旧文档 `docs/华为eDeskPro脚本体系分析.md` 覆盖 `product/Script`（见本系列 `01`）。
- 本系列新增：`02`(EMT 审计引擎)、`03`(IPDeviceCheck 解析)、`04`(NMOT 协议层)、`05`(配置)、`06`(DB)、`07`(APP)、`08`(路线图)。
- **最大增量**：EMT 的 8312 条审计规则库 + Java 后端的解析/弱光光纤算法，是旧分析完全缺失、且最该搬的"知识资产"。

## 5. 下一步建议

1. 先打开 `services/EMTMessageAnalyseClientService/app_data/script/BGP/`、`OSPF/`、`QOS/` 抽样规则，评估规则抽象成本。
2. 打开 `services/IPDeviceCheckService/parsecfg/xmlconfig/huawei/display interface.xml` 评估正则规则可移植性。
3. 打开 `product/config/riskCmd_product.xml` 提取首批风险命令清单。
4. 依据 `06` 设计 NetWeaverGo 的「网元/任务/采集项/结果」表结构草案。
