# 09 · `services/IPOnlineService/` 在线分析（业务比对 / 智能 Ping / 端口角色）

> **本篇为新增**：`00`~`08` 系列**完全遗漏**了该服务。实测 `services/IPOnlineService/` = **389 文件**（`app_define.json` + `webapps/ROOT/` 388）。
> 它是 **Easy-Diagnosis（故障诊断 / 网络变更）→ 高竹工具** 的功能后端，也是本包中**除 EMT 之外知识密度最高的模块**。

---

## 1. 结论与移植价值

`IPOnlineService` 是"**变更/割接场景下的业务快照采集与前后比对 + 网络侧诊断工具集**"，全部能力都靠 **Excel/JSON 模板 + SQL 规则** 驱动，几乎不依赖 Java 代码逻辑——**这批模板本身就是可搬的知识资产**。

| 优先级 | 资产 | 体量 | 对 NetWeaverGo 的意义 |
|---|---|---|---|
| ★★★ | `businesscompare/` 业务比对模板族 | 44 文件（22 xlsx + 20 json + 2 docx） | 「变更前快照 → 变更 → 变更后比对」的**完整采集项清单 + 款型分域 + 场景开关**，是 `plancompare`（规划比对）的天然升级方向 |
| ★★★ | `template/alarm/` 告警规则 | 9 xlsx | 分产品族（CE/Router/S/USG/VNE）的**告警过滤与归并规则** |
| ★★ | `template/manmap/` 端口角色映射 | 5 xlsx | 「端口 → 角色（上联/下联/互联/堆叠）」映射，是拓扑语义化的关键 |
| ★★ | `template/smartping/`、`pingtest/` | 3 + 21 文件 | **智能 Ping** 的 IPv6/接口规则 + 用例模板 |
| ★★ | `tools/kpi_detector_cli/` | 27 文件（24 py） | **独立 Python KPI 指标探测 CLI**（含 models/parsers/src/plot）——可直接作为 Go 侧的算法参照或旁路调用 |
| ★★ | `db/init/` 在线分析库 schema | 134 SQL（1 init + 133 patch） | 与巡检库**不同**的第二套数据模型 |
| ★ | `template/routeDiagnosis/`、`trace/`、`telco/`、`ipstorm/` | 4 + 1 + 3 + 1 | 路由诊断、路径追踪、电信场景命令、IP 风暴脚本 |
| ★ | `i18n/` 14 properties | — | bisCompare / MOP / onlineReport / smartPing / telco 的中英文案 |

> **一句话**：`01` 篇说 `product/Script` 是"发命令取数"，`02` 篇说 EMT 是"配置合规审计"，那么 `IPOnlineService` 就是**第三种脚本体系**——"**业务态快照与差异**"。

---

## 2. 实测目录结构

```
services/IPOnlineService/
├── app_define.json
└── webapps/ROOT/
    ├── static/                                  83 文件（39 js + 28 css + 8 png + 6 svg + 1 ico）
    └── WEB-INF/
        ├── lib/                                 25 jar
        └── classes/                             280 文件
            ├── config/                           10（7 properties + 1 sql + 1 xml + 1 dat）
            ├── db/init/                          134 SQL（init.sql + patch/133）
            ├── i18n/                             14 properties
            ├── rule/                             4 yaml（参数校验规则）
            ├── template/                         90 文件（58 xlsx + 27 json + 2 properties + 2 docx + 1 xml）
            └── tools/                            28（24 py + 1 md + 1 txt + 1 sh + 1 json）
```

---

## 3. 功能边界（`app_define.json` 实证）

```json
{
  "version": "1.0.222",
  "name": "IPOnlineService",
  "description": "IP Online Service",
  "callbacks": { "networkoccupation": "/easydiagnosis/external/v1/getExecutingTaskCounts" },
  "menuIds": {
    "businessCompare": "/IPOnline/index.html#/businessCompare",
    "starWay":         "/IPOnline/index.html#/starWay",
    "ipStorm":         "/IPOnline/index.html#/ipStorm",
    "faultManage":     "/IPOnline/index.html#/faultManage",
    "taskManage":      "/IPOnline/index.html#/taskManage"
  }
}
```

| 菜单 | 语义 | 对应 `template/` 资产 |
|---|---|---|
| `businessCompare` | **变更前后业务比对**（MOP 校验 / 高竹） | `businesscompare/` |
| `starWay` | 链路/路径类诊断（名称源自"星路"，即路径溯源） | `trace/`、`routeDiagnosis/` |
| `ipStorm` | IP 风暴/风暴抑制分析与脚本下发 | `ipstorm/ipStormConfig.properties` |
| `faultManage` | **故障定界**（Easy-Diagnosis 主菜单） | `alarm/`、`smartping/`、`pingtest/` |
| `taskManage` | 任务管理 | `db/init/` |

**关键判定**：`callback` 与 `settings.properties` 均指向 `easydiagnosis`：

```properties
url.base=/rest/easydiagnosis/
app.path=online
operation.log.default.user=online
# 参数校验规则文件名模板
parameter.validator.rule.yaml=rule/validate-rule-%s.yaml
parameter.validator.define=actionlib|custom|execution|project
# 普通设备保活 300s；动态密码设备保活 1800s
normal.device.timeout=300
dynamic.device.timeout=1800
# 异步获取回显
get.echo.async.flag=true
get.echo.async.timeout=300
```

→ **`IPOnlineService` 就是 `apps/Easy-Diagnosis`（故障诊断）的服务化落地**，`07` 篇只能从 `app.json` 猜测，此处得到确证（见 `07` 篇 §6 修正表）。

---

## 4. ★★★ `template/businesscompare/` —— 变更前后业务比对（核心资产）

### 4.1 目录内容

```
businesscompare/
├── collectCmdList.json            142 个采集项键（见 4.2）
├── domain.json                    44 条「款型正则 → 产品域」映射（见 4.3）
├── support_devices.json           近 190 个款型 × 版本矩阵（见 4.4）
├── deviceList.json                参与比对的设备清单定义
├── scene/                         16 个场景开关（见 4.5）
│   ├── ATN.json  CE.json  CX600.json  ME60.json  SIG.json  S9300.json
│   ├── Eudemon.json  Eudemon1000E-F.json  Eudemon8000.json  Eudemon9000.json
│   ├── IPS12000.json  NE40E(MSE).json  NE40E(SR).json  NE5000E.json
│   ├── NE8000(IPRAN).json  NE8000(SR).json
├── en/  zh/                       TaskTemplate、CheckAndCompareItem、MOP_FAQ（中英各 3 份）
└── 18 个产品域 xlsx                ATN / CE / CE-CR / CE12800 / CX600 / FR / FW1000E /
                                   FW8000 / FW9000 / IPS12000 / ME60 / NE-CR / NE-SR /
                                   NE8000(IPRAN) / S / SIG / cmd / CheckAndCompareItem
```

### 4.2 `collectCmdList.json` —— 142 个业务快照采集项

结构极简：`{ "<采集项键>": "" }`，键名即语义。覆盖范围（按域归类）：

| 域 | 采集项示例 |
|---|---|
| 版本/许可/配置基线 | `startup`、`version`、`currentConfiguration`、`licenseInfo`、`licenseResourceUsed`、`pafinfo`、`clock` |
| 接口/光模块 | `ipInterfaceBrief`、`ipv6InterfaceBrief`、`interfaceTransceiver`、`ifnetindex-map` |
| 路由表 | `ipV4RoutingTable`、`allV4RoutingTable`、`specificV4RoutingTable`、`DefaultRoute`、`routingTableProtocolStatic/Direct`、`v4FibStatisticsAll`、`ipRoutingTableAllRoutes`、`ipRoutingTableVpnInstance` |
| OSPF / OSPFv3 | `routerId`、`ospfInterface`、`ospfLsdbRouter`、`ospfLsdbNetwork`、`ospfv3Interface` |
| BGP / EVPN / VPNv4 / VPNv6 | `bgpRoutingTable`、`bgpReceivedRoutes`、`bgpAdvertisedRoutes`、`bgpVpnv4RoutingTable`、`bgpSpecificVpnv4AdvertisedRoute`、`bgpEvpnAllRouting-tableStatistics`、`evpnMacRoutingTableAllEvpnInstanceStatistics`、`bridgeDomain` |
| MPLS / SR / SRv6 | `mplsLdpPeer`、`mplsLsp`、`mplsTeTunnel`、`segmentRouting`、`segmentRoutingAdjacencyMplsForwarding`、`srv6-tePolicy`、`segment-routingIpv6Local-sidForwarding` |
| 可靠性 | `bfdStatistics`、`vrrp`、`vrrpBrief`、`vrrpAdmin`、`stp`、`defendFlag`、`tunnelAll` |
| 组播 | `multicastRoutingTable`、`pimPeer`、`pimRoutingTable`、`pimAllInstanceNeighbor`、`igmpRoutingTable`、`multicastIpV4Fib` |
| 用户接入 / BRAS | `accessUser`、`accessUserSlot`、`basInterface`、`l2tpTunel`、`l2tpSession`、`domain`、`userDaa`、`addedServicePolicy` |
| 防火墙 | `natSessionAgingTime`、`firewallSessionStatistics`、`firewallSessionTable`、`firewallServerMap`、`ikePeerInfo`、`IpsecInfo`、`ikeProposalInfo`、`rbpStatus` |
| 二层 / 堆叠 | `arpAll`、`arpStatistics`、`macAddress`、`macAddressStatistics`、`macAddressVsi`、`stack`、`stackConfigurationAll`、`macAddressSummary` |
| 邻居/ND | `lldpNeighborBrief`、`ipv6NeighborsStatisticsAll`、`ndEntry` |

> **借鉴要点**：这是一份可直接映射为 NetWeaverGo `CommandGroup` 的**"变更前后业务快照标准命令清单"**，且已被华为按产品域裁剪验证过。注意键名与具体 CLI 解耦——具体命令在各产品域 xlsx 里（`cmd.xlsx`）。

### 4.3 `domain.json` —— 款型正则 → 产品域（44 条）

```json
{
  "^CE12800$": "CE12800",
  "^CE98.*": "CE",
  "^NE8000$": "NE8000(IPRAN)",
  "^NE5000E$": "NE-CR",
  "^CX600$": "CX600",
  "^ME60$": "ME60",
  "^MA5200G$": "ME60",
  "^PTN\\d+": "ATN",
  "^ETN\\d+": "ATN",
  "^Eudemon9.*|^USG12|^AntiDDoS12": "FW9000",
  "^Eudemon1000E-.*|^AntiDDoS19|^IPS6.*F|^USG6.*F|^HiSecEngine": "FW1000E",
  "^Eudemon8.*|^USG9.*|^NIP68.*|^AntiDDoS8": "FW8000",
  "^IPS12": "IPS12000",
  "^Eudemon|^USG|^IPS|^AntiDDoS.*|^CE-FWA|^CE-IPSA": "FR",
  "^CE\\d+": "CE",
  "^NE[1-9]\\d": "NE-SR",
  "^S\\S+": "S",
  "^XH80|^XH90|^XH168": "CE",
  "NetEngine 8000E.*": "NE8000(IPRAN)",
  "AdtecRouter|TGDataCom 8000E.*": "NE8000(IPRAN)"
}
```

> **与 `05` 篇 `config/devicemapping/productItem.xml` 的分工**：`productItem.xml` 是**粗粒度**（款型前缀 → 设备大类：路由器/防火墙/交换机/WLAN…），`domain.json` 是**细粒度**（款型 → 比对场景域）。二者叠加即为完整的「设备画像」。**顺序敏感**：数组/对象按序匹配首条命中，`^CE12800$` 必须排在 `^CE98.*`/`^CE\d+` 之前。

### 4.4 `support_devices.json` —— 款型 × 版本支持矩阵

```json
{
  "deviceTypes": ["AntiDDoS1900", "ATN905", ..., "SIG16000 X8", "NetEngine 8000E F1A-S", ...],
  "deviceVersions": {
    "S9300":    ["V100RXXX", "V200RXXX", "V600RXXX"],
    "NE40E":    ["V300RXXX", "V600RXXX", "V800RXXX"],
    "CE12800":  ["V100RXXX", "V200RXXX", "V300RXXX"],
    "USG12000": ["V600RXXX"]
  }
}
```

- `deviceTypes` 近 **190 个款型**；`deviceVersions` 用 `VxRXXX` 通配版本号（只约束大版本 R）。
- **这是「设备画像 → 能力可用性」白名单的最佳范式**：`03` 篇 `CollectItem/ProductVersion@name` 用逗号列表，这里用 JSON 两段式，都优于 if-else。

### 4.5 `scene/*.json` —— 16 个场景开关

```json
{
  "scene": "S9300",
  "sceneTemplate": "S",       // 复用哪个产品域模板（S9300 → S）
  "id": "9",
  "itemList": [ { "startup": "true" }, { "currentConfiguration": "true" },
                { "ospfLsdbRouter": "false" }, { "bgpReceivedRoutes": "false" }, ... ]
}
```

设计要点：
1. **场景 ≠ 模板**：`scene` 是具体款型，`sceneTemplate` 指向复用的产品域模板（多个款型共享一套采集项与命令）；
2. **`itemList` 是按域裁剪后的 `collectCmdList` 子集 + 布尔开关**，用于按设备能力关闭重命令（如 S 系列关掉 `bgpAdvertisedRoutes`）；
3. 16 个场景恰好覆盖了 `domain.json` 里的全部产品域。

---

## 5. 其余子模块

### 5.1 `template/alarm/` —— 分产品族告警规则（9 xlsx）

| 文件 | 作用 |
|---|---|
| `alarm-rule.xlsx` | 告警规则总表 |
| `alarm-rule-template.xlsx` | 规则模板 |
| `alarm-rule-CE.xlsx` / `-Router.xlsx` / `-S.xlsx` / `-USG.xlsx` / `-VNE.xlsx` | 分产品族规则 |
| `alarmTemp-cn.xlsx` / `alarmTemp-en.xlsx` | 中英告警模板 |

→ 对应「故障定界」：把原始告警（`alarmAll`/`alarmActive`）按规则归并成**可判定的故障现象**。

### 5.2 `template/manmap/` —— 端口角色映射（5 xlsx）

`manmapTemplate_zh/en.xlsx`、`PortMappingTable_zh/en.xlsx`、`DefaultRoleTemplate.xlsx`
→ 「端口 → 角色（上联/下联/互联/堆叠/带外）」映射表，可让 NetWeaverGo 拓扑图从"连线"升级为"**带语义的组网结构**"。

### 5.3 `template/smartping/` + `template/pingtest/`

```json
// smartping/rule/ipv6Interface.json —— 智能 Ping 的"取 IPv6 对端地址"规则
{
  "cmd": "display ipv6 interface brief",
  "type": 1,
  "useRegex": 1,
  "splite": "\\S{1,15}\\s{1,20}up\\s{1,30}up\\S{0,5}\\s{1,15}\\S{1,15}\\s{0,30}\\S{1,5}\\s{0,2}\\S{3,10}\\s{1,3}\\S+",
  "patterFlag": 10,                       // 10 = Pattern.MULTILINE|CASE_INSENSITIVE，与 03 篇一致
  "filter": "\\[IPv6 Address\\]\\s+FE80",  // 排除链路本地
  "values": [
    { "regexName": "ip",            "regex": "(?<=\\[IPv6 Address\\]\\s{0,2})\\S+", "patterFlag": 2, "all": false },
    { "regexName": "interfaceName", "regex": "\\S+",                                "patterFlag": 2, "all": false },
    { "regexName": "vpnName",       "regex": "(?<=\\S{1,15}\\s{1,20}up\\s{1,30}up\\S{0,5}\\s{1,15})\\S{1,15}", "patterFlag": 2, "all": false }
  ]
}
```

> ⚠️ **注意**：该规则用了 `(?<=...)` **后顾断言**，Go 的 `regexp`（RE2）**不支持**。移植时需改写（改写思路参见 `tools/re2scan/`）。
> 但结构（`cmd` + `splite` + `filter` + `values[regexName/regex/all]`）与 NetWeaverGo 的 `UserParseTemplate`（Pattern + FieldMapping）高度同构，**几乎可直接反序列化**。

`pingtest/` 另有 `rule/`、`en/`、`zh/`（16 xlsx + 5 json）——Ping 用例模板与判定规则。

### 5.4 `template/routeDiagnosis/` / `trace/` / `telco/` / `ipstorm/`

| 目录 | 文件 | 说明 |
|---|---|---|
| `routeDiagnosis/` | `routeDiagnosisTemplate.xlsx`、`DestIpTable.xlsx` | **路由诊断**：给定目的 IP → 逐跳/逐表诊断 |
| `trace/` | `rulesTrace.xlsx` | 路径追踪判定规则 |
| `telco/` | `telcoCmd.xlsx` + `en/zh NetworkFileTemplate.xlsx` | 电信场景命令与网络文件模板 |
| `ipstorm/` | `ipStormConfig.properties` | IP 风暴检测参数（另有 `config/ipstorm_script.properties`） |

### 5.5 `tools/kpi_detector_cli/` —— 独立 KPI 探测 CLI

```
tools/kpi_detector_cli/
├── run_detector.py        入口
├── plot_kpi.py            指标绘图
├── requirements.txt
├── README.md
├── models/
├── parsers/
└── src/                   20 py
```

> 这是**本包中唯一的纯 Python 数据科学工具链**（探测 + 解析 + 绘图）。NetWeaverGo 目前无对应能力，可作为「**采集数据的二次指标分析**」参考实现；也可先以子进程方式旁路复用。

### 5.6 `config/` —— 运行参数与敏感命令

| 文件 | 内容 |
|---|---|
| `db.properties` / `dbupdate.properties` | 在线分析库连接与升级 |
| `intelligent.properties` | `easyPingThreadSize=100`、`destIp=8.8.8.8`、`ipv6DestIp=240c::6666`、`yundestIp=114.114.114.114`、`traceInterval=1900`、17 条 `MASK_*` 掩码常量 |
| `ne.settings.properties` | 网元级设置 |
| `ip_business_compare_postgresql.sql` | **业务比对库的 PostgreSQL 建表脚本**（与 `db/init/` 的 H2 版本并列） |
| `ipstorm_script.properties` | IP 风暴脚本参数 |
| `mail.properties` | 邮件通知 |
| `sensitiveCmd.xml` | **敏感/风险命令清单**（与 `product/config/riskCmd_product.xml` 互补） |
| `custom/aiSceneCmd.dat` | AI 场景命令（二进制） |
| `rule/*.yaml` | `validate-rule-{actionlib,custom,execution,onlineAnalysisERService}.yaml` 参数校验规则 |

---

## 6. 数据模型：`db/init/`（134 SQL）

```
db/init/
├── init.sql                    全量建表
└── patch/                      133 个增量补丁
    ├── 20210927 ~ 2023         早期（按月-旬）
    ├── 2024                    20240108 … 20241224
    ├── 2025                    20250218 … 20251230
    └── 2026                    20260209 … 20260813（本包最新快照）
```

> **两点提示**：
> 1. 这是**在线分析（Easy-Diagnosis）专属库**，与 `06` 篇的巡检库（`dbScript/network|unicnetwork|unicollect`）**不是一套 schema**；同一交付包内共存在 **3 套数据库建模**（巡检 / 采集 / 在线分析）。
> 2. `patch/` 的 133 个文件按时间戳排序即为**版本升级路径**，可作为 NetWeaverGo「数据库迁移版本化」的参考（当前项目用 GORM AutoMigrate，缺少可审计的增量升级链）。

---

## 7. 移植要点（NetWeaverGo）

| # | 动作 | 落点 |
|---|---|---|
| 1 | **业务比对模块（新）**：`变更前快照 → 快照差异 → 业务影响清单`。直接用 `collectCmdList.json` 生成快照命令组，用 `domain.json` + `scene/*.json` 做设备→场景裁剪 | 新建 `internal/bizcompare/` + `internal/taskexec/bizcompare_compiler.go`（复用现有 `CompilerRegistry`/`StageExecutor` 五层架构） |
| 2 | **`support_devices.json` → 能力白名单表**：把「款型 × 大版本」下沉为 `device_capabilities` 表，供"该设备能执行哪些任务/模板"做准入 | `internal/models/` + `internal/config/device_profile.go` |
| 3 | **`domain.json` → 场景分域规则**：与现有 `internal/device/series.go` 的系列归一化合并，统一为一份「设备画像」 | `internal/device/` |
| 4 | **告警规则表**：`alarm-rule-<族>.xlsx` 转为 `alarm_rules` 表 + 归并引擎 | 新建 `internal/alarm/` |
| 5 | **端口角色映射**：`manmap` 模板 → 拓扑图边的 `role` 属性，让 `TopologyGraph` 展示上联/下联 | `internal/taskexec/topology_builder.go` + `frontend TopologyGraph.vue` |
| 6 | **智能 Ping**：移植 `smartping/rule/*.json` 的规则结构（**须先把后顾断言改写为 RE2 兼容**） | `internal/icmp/` 或新 `internal/smartping/` |
| 7 | **掩码/诊断参数常量**：`intelligent.properties` 的 17 条掩码 → `internal/utils/` | `internal/utils/` |
| 8 | **数据库版本化迁移**：参考 `db/init/patch/` 时间戳链 | `internal/config/db.go` |
| 9 | `kpi_detector_cli`：作为二阶段指标分析模块的可选旁路 | 暂不移植，留作参考 |

> **与 `plancompare` 的区别**：`plancompare` 是「**规划文件 vs 实采拓扑**」的静态比对；`businessCompare` 是「**变更前实采 vs 变更后实采**」的双时点比对。二者互补，后者更贴近割接验收场景。
