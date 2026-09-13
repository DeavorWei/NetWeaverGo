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
| `area_info_zh.json`/`area_info_en.json`、`technicalsupport.json` | 区域、技术支持信息 | — |

## 2. `devicemapping/productItem.xml` 形态归一化（可借鉴）

```xml
<devType name="AR router"   index="1" type="AR|SRG13|SRG23|SRG33|NE16EX|SRG5|RU-5G"/>
<devType name="NE router"   index="2" type="NE|ME|CX|Ne|ATN|ETN|PTN|VNE|SIG|TGDataCom|AdtecRouter"/>
<devType name="Firewall"    index="3" type="USG|SVN|ET|IPS|LE|CE-FWA|CE-IPSA|NIP|ANTIDDOS|...|HiSecEngine|ASG|E1000"/>
<devType name="DC"          index="6" type="CE|FM|XH|DX|SF"/>
<devType name="Ethernet Switch" index="4" type="S|E6"/>
<devType name="WLAN"        index="7" type="AC|AT|AD|FitAP|CloudAP|AirEngine|AP|R|WA"/>
```
→ `type` 是一组**款型前缀正则**，可整体转为 NetWeaverGo 的「款型前缀 → 设备类型」映射表（与 `Script/common/device/device.py` 的 `_REG2HANDLER` 同源互补）。

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

> 移植关注点不在启动脚本本身，而在脚本揭示的**服务依赖顺序与端口分配**（Electron 前端 36888/38888，Jython 38887，REST 微服务 80/443 等）。

## 7. 移植要点（NetWeaverGo）

1. 把 `devicemapping/productItem.xml` + `getversion/` + `device.py` 三处形态规则**统一**为一份「设备画像」配置。
2. `cmd.properties` 代理模板 → 连接器接入方式配置。
3. `validate/*.xml` 校验规则 → 入参校验中间件。
4. `riskCmd_product.xml` → 风险命令拦截表（**必做**）。
5. `script/Interface*.properties` + `ifnamemapping.json` → 接口名标准化模块。
