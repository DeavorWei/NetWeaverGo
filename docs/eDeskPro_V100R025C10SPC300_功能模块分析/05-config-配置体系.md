# 05 · `config/` + `bin/` + `launcher/` 配置体系

> 运行期配置：设备映射、版本映射、命令模板、入参校验、风险命令、安全白名单、启动脚本。

## 1. `config/` 子目录与关键文件

| 子目录/文件 | 作用 | 移植价值 |
|---|---|---|
| `devicemapping/*.xml` | 各厂商 `productItem.xml`（CISCO/H3C/Huawei/RUIJIE/ZTE）：**款型前缀 → 设备类型**映射 | ★★ 设备形态归一化规则 |
| `versionmapping/inspect/`、`versionmapping/unetbuilder/` | 版本映射（产品版本矩阵） | ★★ 设备画像→能力匹配 |
| `script/` | `ciscoInterfaceTransition.ini`、`InterfaceFiltrate.properties`、`InterfaceTransition.properties`、`InterfaceTransitionblack.properties` | 接口名转换/过滤（对应 IPDeviceCheck 的 `ifnamemapping.json`） |
| `validate/` | **55 个 XML 入参校验规则**（`validate-rule-*.xml` 按服务分，`validate-template-*.xml` 按资源分，`validate-common-config-*.xml` 公共） | ★ 请求/参数校验模板 |
| `faultConfig/`、`multihop/`、`filetransfer/`、`log4j/`、`customconfig/`、`certification/`、`codesignature/`、`neimport/`、`snimport/`、`template/`、`upgrade/`、`blankblocklist/`、`backup/` | 故障配置、多跳、文件传输、日志、证书、导入、模板、升级等 | 按场景取用 |
| `cmd.properties` | **VPN/SSH/Telnet 命令模板**（跳板/代理连接命令，如 `hw_proxy_ssh_cmd=stelnet %s %s`、`telnet_vpn_cmd`） | ★★ 代理/跳板连接命令生成 |
| `inspect.properties` | 连接重试、任务数、线程池、超时、分布式采集阈值（见 `04`） | ★★ 采集运行参数 |
| `connect.properties`、`taskConfig.properties`、`script.properties`、`serialCreateTaskcfg.properties` | 连接/任务/脚本配置 | ★ |
| `security_whitelist.properties`、`security_whitelist_product.properties` | **REST API 白名单**（HTTP 安全，非风险命令） | 端点清单（可见全部微服务） |
| `policy.properties`、`audit_log_backup.properties`、`check_core_integrity.properties`、`kmc.properties`、`lockscreen.properties`、`schedule.properties` | 策略/审计/完整性/KMC 锁屏/调度 | 运维合规 |
| **`deviceversion/`（10 文件）** | **协议/命令/回显级配置**（原文档遗漏，见 §8） | ★★★ 多项可直接搬 |
| `protocol/` | 5 properties + 3 json：协议相关配置 | ★ |
| `help/`、`customconfig/`、`install/`、`license/` | 帮助文档(16)、自定义配置(59，含 45 png)、安装、许可 | 低 |
| `certification/`、`codesignature/`（37 文件：12 zip + 12 cms + 12 crl） | 证书与代码签名（CMS/PKCS#7 + CRL） | 供应链合规 |
| `backup/`、`blankblocklist/`、`neimport/`、`snimport/`、`template/`、`upgrade/`、`faultConfig/`、`filetransfer/`（27 xml）、`log4j/`（11 xml）、`multihop/`（18） | 备份/黑名单/网元导入/SN 导入/模板/升级/故障/文件传输/日志/多跳 | 按场景取用 |
| 根级其它 properties | `cmd_authentication.properties`、`configurationshow.properties`、`dcn_config.properties`、`downloadcheckitem.properties`、`electron.properties`、`globaldb.properties`、`h2_driver_upgrade_min_version.properties`、`menusshow.properties`、`resource.properties`、`security_resource.properties`、`SystemConfiguration.properties`、`unzip_config.properties`、`inspect_thread_application.properties`、`service-configure.properties` | 运维参数 |
| `area_info_zh.json`/`area_info_en.json`、`technicalsupport.json` | 区域、技术支持信息 | — |

> **统计**：`config/` 实测 **315 文件**（112 xml + 70 properties + 51 png + 20 json + 16 xlsx + …），25+ 个子目录。

## 2. `devicemapping/productItem.xml` 形态归一化（可借鉴）

```xml
<devType name="AR router"   index="1" type="AR|SRG13|SRG23|SRG33|NE16EX|SRG5|RU-5G"/>
<devType name="NE router"   index="2" type="NE|ME|CX|Ne|ATN|ETN|PTN|VNE|SIG|TGDataCom|AdtecRouter"/>
<devType name="Firewall"    index="3" type="USG|SVN|ET|IPS|LE|CE-FWA|CE-IPSA|NIP|ANTIDDOS|...|HiSecEngine|ASG|E1000|XH6655|HSIC"/>
<devType name="DC"          index="6" type="CE|FM|XH|DX|SF"/>
<devType name="Ethernet Switch" index="4" type="S|E6"/>
<devType name="WLAN"        index="7" type="AC|AT|AD|FitAP|CloudAP|AirEngine|AP|R|WA"/>
```
→ `type` 是一组**款型前缀正则**，可整体转为 NetWeaverGo 的「款型前缀 → 设备类型」映射表（与 `Script/common/device/device.py` 的 `_REG2HANDLER` 同源互补）。

### 2.1 实测全量 `devType`（**原文档仅列 6 条，实际 18 条**）

| index | `name` | `type` 前缀正则 | `isSoftware` | 中文名 |
|---|---|---|---|---|
| — | `AR router` | `USG6521G-DW5G`（**单型号特例，必须排在最前**） | false | AR企业路由器 |
| 1 | `AR router` | `AR\|SRG13\|SRG23\|SRG33\|NE16EX\|SRG5\|RU-5G` | false | AR企业路由器 |
| 2 | `NE router` | `NE\|ME\|CX\|Ne\|ATN\|ETN\|PTN\|VNE\|SIG\|TGDataCom\|AdtecRouter` | false | NE路由器 |
| 3 | `Firewall` | `USG\|SVN\|ET\|IPS\|LE\|CE-FWA\|CE-IPSA\|NIP\|ANTIDDOS\|AntiDDoS\|EUDEMON\|Eudemon\|SSM_FW\|SSM_IPS\|SRG22\|NGFW\|SEMG\|SeMG\|HiSecEngine\|ASG\|E1000\|XH6655\|HSIC` | false | 防火墙 |
| 4 | `Ethernet Switch` | `S\|E6` | false | 以太网交换机 |
| **5** | **`DC(SOFT)`** | `CE1800V-SOFT` | **true** | 数据中心交换机(软件类) |
| 6 | `DC` | `CE\|FM\|XH\|DX\|SF` | false | 数据中心交换机 |
| 7 | `WLAN` | `AC\|AT\|AD\|FitAP\|CloudAP\|AirEngine\|AP\|R\|WA` | false | 无线局域网 |
| 8 | `Agile Controller` | `CONTR` | true | 敏捷控制器 |
| 9 | `eSight Platform` | `ESIGHT` | true | eSight管理平台 |
| 10 | `VCN` | `VCN` | true | 视频云节点 |
| 11 | `VCM` | `VCM` | true | 视频内容管理 |
| 12 | `IPC` | `IPC` | true | IP摄像机 |
| 13 | `LogCenter` | `LOGCENTER` | true | LogCenter |
| 14 | `AnyOffice` | `ANYOFFICE` | true | AnyOffice |
| 15 | `eLog` | `ELOG` | true | eLog |
| 16 | `FireHunter` | `FIREHUNTER` | true | FireHunter |
| 17 | `CIS` | `CIS` | true | CIS |
| 18 | `NCE` | `NCE` | true | NCE |

**原文档遗漏的 3 个关键属性**：

1. **`index` 是显式序号**（1~18），不是数组下标——`DC(SOFT)=5` 插在 `Ethernet Switch=4` 与 `DC=6` 之间，说明**分类顺序被业务依赖**（如报表排序/菜单顺序）。
2. **`isSoftware="true"`** 区分硬件设备与软件形态（`DC(SOFT)`/eSight/VCN/IPC/LogCenter/NCE…）——NetWeaverGo 的 `DeviceAsset` 缺少"软/硬形态"字段，做映射时应补。
3. **`<lang value="zh|en">`** 双语名称内嵌在 XML 中（对应 `product/Resource/IP/*.csv` 的另一套多语言方案）。

> **与 `09` 篇 §4.3 的分工**：本文件是**粗粒度大类**（18 类，含软件/管理类），`IPOnlineService/template/businesscompare/domain.json` 是**细粒度场景域**（44 条正则 → 16 域）。二者叠加才构成完整「设备画像」，**且都要注意"顺序敏感、首条命中"**。

## 3. `cmd.properties` 代理/跳板命令模板（可借鉴）

```
hw_vpn_cmd=telnet vpn-instance %s %s %s
hw_proxy_ssh_cmd=stelnet %s %s
unix_ssh_cmd=ssh -l %s %s -p %s
cs_ssh_cmd=ssh %s %s
```
→ 可转为 NetWeaverGo 的「连接方式 → 命令模板」配置，支撑跳板/代理/VPN 接入。

## 4. `validate/` 入参校验规则（可借鉴）

`validate-rule-inspect-task.xml`、`validate-rule-collecttask-er.xml`、`validate-rule-protocol.xml`、`validate-rule-lowlightrule-er.xml`、`validate-rule-connecttest.xml` 等，按服务/资源精细划分。结构为通用 `validate-common-config-*.xml` + 各业务 rule/template。可作为 NetWeaverGo「任务/设备/采集项入参校验」的参考规则集。

## 5. 风险命令清单

风险命令不在 `config/` 顶层，而位于 **`product/config/riskCmd_product.xml`**（见 `01`）：按 `TaskType(INSPECTOR,SMARTNOS)` → `Scope(Query/Non-Query)` → `Category(产品族)` → `ProductVersion` → `<Cmd>` 组织（如防火墙禁止 `reset ike sa`、`undo ipsec policy`、`packet-capture all-packet`）。**NetWeaverGo 应建 `risk_commands` 表，执行前拦截。**

## 6. `bin/` + `launcher/` 启动与运维

| 文件 | 作用 |
|---|---|
| `start.bat` / `stop.bat` / `uninstall.bat` / `get_log.bat` | 一键启动/停止/卸载/取日志 |
| `bin/*.bat`（18）、`bin/*.ps1`（4） | 各服务启动脚本、环境初始化 |
| `launcher/` | Electron 主进程启动器 |

> 移植关注点不在启动脚本本身，而在脚本揭示的**服务依赖顺序与端口分配**（Electron 前端 36888/38888；**UDP 38887 = SNMP 回调端口，非 "Jython 服务"**，修正见 `04` 篇 §8；REST 微服务 80/443 等）。
>
> `bin/` 实测 22 个脚本：`start/stop/uninstall/get_log`、`h2_import/h2_export`（H2 库导入导出）、`upgrade/upgrade_package/lotsuser_upgrade`、`diskremain`（磁盘剩余）、`check_proc_start`、`autostartapp`、`easycrt/easycrt_NCE`（SecureCRT 集成）、`get_info_service`、`log4operation`，另有 `Instance/` 与 `script/` 子目录。

## 7. 移植要点（NetWeaverGo）

1. 把 `devicemapping/productItem.xml`（**18 类，含 `isSoftware`**）+ `getversion/` + `device.py` + `IPOnlineService/businesscompare/domain.json`（44 条细粒度）四处形态规则**统一**为一份「设备画像」配置。
2. `cmd.properties` 代理模板 → 连接器接入方式配置（见 §8.4）。
3. `validate/*.xml` 校验规则 → 入参校验中间件。
4. `riskCmd_product.xml` → 风险命令拦截表（**必做**）；与 `IPOnlineService/config/sensitiveCmd.xml`（**脱敏**）配合使用（见 §8.1）。
5. `script/Interface*.properties` + `ifnamemapping.json` → 接口名标准化模块（**含 ifType 数字编码**，见 §8.5）。
6. `deviceversion/*`（§8）→ `matcher` / `executor` 的策略外置（**性价比最高，纯配置**）。

---

## 8. ★★★ `config/deviceversion/`（10 文件）—— 原文档遗漏的"命令/回显级配置"

这是 `config/` 下**最被低估**的目录：它把"命令下发与回显处理"的所有策略从代码里抽了出来。

| 文件 | 作用 | 移植价值 |
|---|---|---|
| `sensitiveCmd.xml` | **17 产品族/厂商的敏感信息脱敏正则库**（`<Category>`→`<Cmd>`→`<Filter>`） | ★★★ **最高**，见 `10` 篇 §6.1 |
| `keyCmd.xml` | **12 厂商 × 2 场景**的配置/接口采集命令表 | ★★ 见 `10` 篇 §6.4 |
| `devicevalidate.properties` | 按**任务场景**决定回显校验模型与异常提示 | ★★ 见 `10` 篇 §6.5 |
| `connectCommandPolicy.properties` | 回显"拼接命令"策略（`CE1800V=2`：回显须以命令开始） | ★★ 见 `10` 篇 §6.2 |
| `reciveFilterKey.properties` | 接收端特殊字符过滤（Linux suse11 的 ANSI 残留） | ★ 见 `10` 篇 §6.3 |
| `protocoldiscovery.properties` | 拓扑发现协议（**LLDP / CDP / OSPF / ARP**）+ 厂商白名单 + 业务风险提示 | ★★ 见 `10` 篇 §6.6 |
| `collectCmd.properties` | 采集命令参数 | ★ |
| `devDirCfg.xml`（158KB） | **设备目录/分类配置**（大文件，按目录树组织设备能力） | ★★ 待深挖 |
| `SupportVersion.xml`（899KB） | **支持版本矩阵**（与 `09` 篇 `support_devices.json` 同族） | ★★ 待深挖 |
| `reciveFilterKey.json` | `reciveFilterKey.properties` 的 JSON 版 | ★ |

> `devDirCfg.xml`（158KB）与 `SupportVersion.xml`（899KB）是本包中**体积最大的两份配置**，是「设备目录树 + 版本支持矩阵」的权威来源，建议后续单独专题分析。

## 9. `config/validate/` 实测清单（55 XML + 1 properties）

按**服务**（`validate-rule-*`，44 个）与**资源**（`validate-template-*`，11 个）两类组织：

- 服务类：`inspect-task`、`inspect-parser`、`inspect-report`、`collecttask-er`、`collect-upgrade`、`parse-task-er`、`parsedataprepare-er`、`devicecheckfile-er`、`lldparsefile-er`、`lldparsetemplate-er`、`lowlightrule-er`、`ifnamemapping-er`、`onlineAnalysis`、`onlinetask`、`upgrade`、`license`、`filetransfer`、`xftpService`、`protocol`、`connecttest`、`templateService`、`systemconfig`、`platform`、`logger`、`manual`、`offlinefile`、`anticracks`、`lockstreen`、`resource-discovery`、`resource-neconnectlock`、`resource-network`、`resource-service`、`resource-systemcall`、`inspectApp`、`inspectApp-systemcall`、`checktaskbatch-er`、`taskexecute-er`、`authorisation`…
- 资源类：`validate-template-{authorisation,configurationParams,filetransfer,inspectapp,logger,neconnectlock,neworkService,platform,proxyService,resourceService,templateService,xftpService}`

> 覆盖了**每一个 REST 资源**的入参契约，可作为 NetWeaverGo Wails 服务层"入参校验中间件"的规则来源。

## 10. `config/versionmapping/` 实测结构

```
versionmapping/
├── inspect/          en/  zh/  → Version_Mapping_zh.xlsx
└── unetbuilder/      en/  zh/
```

> 版本映射以 **xlsx** 承载（非 XML/properties），`09` 篇 `support_devices.json` 是其 JSON 化版本。三处（`versionmapping/*.xlsx`、`SupportVersion.xml`、`support_devices.json`）表达同一语义，移植时取**最容易解析的 JSON**即可。
