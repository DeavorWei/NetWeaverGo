# 04 · `services/NMOT*` + `NetCareInsideService` 采集编排与协议层（源码级）

> 源码级结论来自对 `NMOTBusinessService`(291 java)、`NMOTCollectAppService`(169 java)、`NetCareInsideService`(183 java) 的**反编译**（Kali + cfr 0.152）。**重要修正**：此前（及 `00` 总览）把 `netcareinside-driver-2.2.6.jar` 当作协议层，经源码验证**该判断错误**，见 §1。

## 1. ⚠️ 重大修正：`netcareinside-driver` 并非协议层

反编译 `netcareinside-driver-2.2.6.jar` 后确认：
- **它不含任何设备协议代码**（无 jsch/sshj/MINA-SSH/trilead、无 snmp4j、无 netconf、无 `org.apache.commons.net.telnet`、无 `Channel`/`getSession`/`readUntil`/prompt 正则）。
- 它实际是 **eDesk Pro 的 Web 控制面后端**（Spring Boot + MyBatis），通过 **HTTP/REST 在 TCP 51943** 与 **eDesk Pro 桌面客户端**（独立 Windows 二进制）通信。
- **真正的 SSH/Telnet/Netconf/SNMP/Serial 引擎在桌面客户端二进制里**，不在本交付包内。后端只持有登录账号（`UserAuthenticateInfoBo`），不持有设备连接参数。

**推论**：NetWeaverGo 若要做实机协议连接，需另寻桌面客户端二进制（或抓其与后端 51943 的 REST 报文）做逆向；本 jar 唯一可 1:1 映射的是「后端↔客户端」的 REST 契约（见 §2）。

### 1.1 ⚠️⚠️ 第二轮重大修正：协议层**就在本包内**（`lib/nmotprotocolcbb-*`）

第一轮的结论「真正的 SSH/Telnet/SNMP 引擎不在本交付包内」**不成立**。第二轮把 `lib/*.jar` 推至 Kali 用 cfr 反编译（共 **1538 个 .java**）后确认：`lib/` 下存在完整的 **`nmotprotocolcbb-*` 协议层实现**（10 个 jar / **260 个类**）：

| jar | 类数 | 关键类 | 底层库 |
|---|---|---|---|
| `nmotprotocolcbb-ssh` | 5 | `SSHProtocol`(implements `INetProtocol`)、`SSHReceiver`(Runnable)、`SSHProtocolUtil`、`SSHVersionUtils`、`SSHReceiverErrorHandle` | **Apache MINA SSHD 2.18.0**（`org.apache.sshd.client.SshClient` / `ChannelShell` / `PtyChannelConfiguration`） |
| `nmotprotocolcbb-telnet` | 2 | `TelnetProtocol`、`MyTelnet` | 自研 |
| `nmotprotocolcbb-snmp` | 17 | `SnmpProtocol`、`SnmpSession`, `SnmpSessionFactory`、`SnmpParam`、`SnmpConfig`、`SnmpCollectModeEnum`、`util/*` | **snmp4j 2.8.18** |
| `nmotprotocolcbb-cli` | 9 | `CliSession`、`CliReceiver`、`CliFtpReceiver`、`MsgCheckUtils`、`ProtocolFactory`、`SpecialCharactersFilterProxy` | 自研 |
| `nmotprotocolcbb-msgfilter` | **116** | `TerminalInterpreter`、`Screen`、`TerminalBuffer`、`TerminalXTerm`、`interpret handler/*`（AnsiPrinter/Bell/Bs/Cbt/Tab/Tbc/TrackMouseh/Vmto/Vmot2/XTermSeq）、`dpmodeshandler/*`、`sgrmodeshandler/*` | 自研**终端仿真器** |
| `nmotprotocolcbb-model` | 47 | `ConnectionArgs`、`CmdArgs`、`ProtocolCfg`、`ProtocolType`、`SSHAuthMode`、`RuleProtocolRegex`、`VendorFilterPattern`、`DeviceFilterReceiveKeyModel`、`VendorCharsetCmd`、`strategy/*` | 自研策略引擎 |
| `nmotprotocolcbb-multihop` | 34 | 跳板/多跳代理 | 自研 |
| `nmotprotocolcbb-mml` | 21 | MML 会话 + 证书认证（`cert/auth/internal/*`） | 自研 |
| `nmotprotocolcbb-xftp` | 7 | `FTPProtocol`、`SFTPProtocol`、`SFTPClientHelper` | 自研 |
| `nmotprotocolcbb-serialport` | 2 | `SerialProtocol`、`ReceivedDataListener` | `jSerialComm` |

**实测细节（对移植有直接价值）**：

- `SSHReceiver` 默认字符集 **`gbk`**，单次回显上限 **`MAX_LENGTH = 0xA00000`（10MB）**；
- SSH 支持 **PTY**（`PtyChannelConfiguration`）、**主机密钥指纹校验**（`KeyUtils` / `BuiltinDigests`）、**SOCKS5 代理**（`model/util/Socks5Utils`）；
- 认证模式 `SSHAuthMode`（与 §3 的 `USER/KEY/KEY_PWD` 对应）、错误关键字列表（`"Permission denied"`、`"auth fail"`、`"Username or password invalid"` 等）；
- `model/strategy/` 提供 `SimplePattern / ComplexPattern / AndOption / OROption / RegStrategy / PatternFactory / RuleParser / XmlHelper`——一套**从 XML 驱动的提示符/回显正则策略引擎**。

**修正后的结论**：

> 本包内存在两条与设备通信的路径：
> 1. **CBB 协议层**（`nmotprotocolcbb-*`）——后端**直接**建连设备，真实实现，**可直接对照移植到 Go**；
> 2. **客户端 REST 通道**（`netcareinside-driver` / `netcareinside-sdk`，51943）——用于托管 eDesk 桌面客户端的会话。
>
> 因此 NetWeaverGo **不需要**逆向桌面客户端二进制，只需参考 CBB 协议层即可。详见新增的 `10` 篇 §3。

## 2. `netcareinside-driver` 真实身份（后端↔客户端 REST，端口 51943）

- 统一信封 **`bo/RequestData`**：`url`、`header`(JSONObject)、`type`("GET"/"POST")、`requestConfig`(Apache HttpClient5 `RequestConfig`)、`param`(JSONObject body)、`isCompatibleOldVersion`。
- 发送：`utils/HttpUtil.sendRest(RequestData) → JSONObject`；POST 超时 **http 4s / https 10s**（`Timeout.ofSeconds`，`org.apache.hc.client5.http`）。
- 鉴权头：`X-HW-ID` / `X-HW-APPKEY`（从 `sysConfigDao.querySysConfig` 取），`host`（来自 `PropertyUtil.getScanQrUrl("hostEnv",site)`）。
- 客户端在线探测 `utils/TelnetUtil.hasConnectedClient()`：执行 `cmd.exe /c netstat -ano | findstr LISTENING | findstr :51943` → POST `queryConnectStatusUrl` → 解析 `{returnValue:"NetCare", obj:{netStatus:"true"|"false"}}` → `Map{isConnected,isTelnet}`。
- 调用链示例：`AsyncCommandImpl.sendHighRiskInterceptRecord()` → `TelnetUtil.hasConnectedClient()` → 在线则 `CommonDataDeal.sendHighRisk(...)` 经 `HttpUtil.sendRest` 上报高危命令拦截记录。

## 3. NMOT 连接参数模型（业务/采集服务侧）

业务/采集服务只**构造连接参数**，真正建连由 CBB 公共组件（`nmotbusinesscbb`，未在本 3 根内反编译）完成。

- `BuildSSHData.buildSSHData/buildSingleSSHData()` → 构造 `ProtocolSSH`（CBB）：`neCommonId`、`authenticationMode`(`SSHAuthMode.USER/KEY/KEY_PWD`)、`keyFileName`、`keyPath`(**AES 加密** `EncryptionEnum.DEVICE_PASSWORD`)、`fingerprint`、`username`、`password`、`superPassword`、`port`(默认 22)、`proxyId`(默认 0)。
- `BuildTelnetData` / `BuildSNMPData` 同构产出 `ProtocolTelnet` / `ProtocolSNMP`。
- `BuildSshKeyInfoData.storeKeyInfo()`：校验 SSH 密钥（上限 **5KB=5120B**），写 `keyFileName`/`keyPath`。
- `NeProtocolCliParamPO`：`neUuid`、`type`(`EnumProtocol`)、`port`、`username`、`psKey`、`connectMode`。
- `NeProtocolSnmpParamPO`：`neUuid`、`version`(`SnmpVersionEnum`)、`port`、`readCommunity`、`writeCommunity`、`username`、`encryptType`(`EncryptAlgEnum`)、`encryptPwd`、`authType`(`EncryptAlgEnum`)、`authPwd`。
- 代理以 `proxyId` 表达；VPN 见 `NetworkCloudParam`（本 3 根无显式 vpn 字段）。

## 4. 脚本执行桥（Jython / SnmpCallback / EVA 分布式）

- `CollectTaskAction.runScriptAndReturnErrorMessage()`：`SnmpCallback.setFilePath(getResultFilePath(...))` → 遍历 `IScript`，对 `PythonScript`/`XmlScript`/`JavaScript` 调 `script.run(deviceAgent, neDataPersistentUtil, scriptParameter)`；捕获 `org.python.core.PyException`（**Jython** 引擎）。`ScriptFactory.getScript(ScriptItem)` 按类型解析，`EnumScriptType.PYTHON`→`PythonCliScript.saveResult`。
- `PreCollectAction.runScript()`：同模式，`ExecMode.ANALYSIS`，执行前 `preCheck`。
- **分布式（EVA 设备端 Python 代理）**：`DistributedCollectAction` 经 `deviceAgent.send(...)` 下发 CLI：`install eva script <file> inspection`、`display eva register-status`、`display eva script status`、`uninstall eva script inspection`。
- `DistributeScriptMgr`：按 `EnumCollectMode.DISTRIBUTED` 分流，生成脚本 JSON：`events{e1: eva.singleCollect()}`、`strategy: e1`、`tasks: mainTask/taskN`，`action = eva.cliArray(${viewN}, ${cmdN})`，经 XFTP 上传（文件名 `<ip>_<id>_<ts>_script.json`）。**注入 EVA 的全局量**：`eva`(singleCollect/cliArray/register-status)、`view`、`cmd`、`tasks`。
- ⚠️ 真正的 Python 引擎与 `SnmpCallback`（**UDP 38887** 回调端口）位于 `nmotbusinesscbb` CBB 中，**未在本 3 根内**。此前 `00` 总览称「38887 是 Jython 服务」需修正为「38887 = UDP SNMP 回调端口（CBB）」。

## 5. 任务调度与并发

- `CollectExecutorMgr`（`ApplicationListener<RunnerEvent>`，`@Async onApplicationEvent`→`init()`）：读 `inspect_thread_application.properties` + `inspect.properties`；`registerCollectTaskExecutor()` 建 `ExecutorInfo`（`name=inspect_collector`，`supportTaskTypes=EnumTaskScenes.getAllCollectScenes()+INSPECT_TASK`），经 `InspectTaskThreadPool` + `TaskExecuteFramework.getInstance().startExecutor()` 注册。
- 默认池：**core=300, max=300, queue=200**，initial/interval delay=6s。
- `InspectTaskThreadPool`（`ITaskExecutor`）：`execute(TaskSegment)` 包成 `InspectTask`+`TaskSegmentStatusMonitor`+`StateNotifier`，交 `TaskThreadPool`（底层 `ThreadPoolExecutor`）执行。
- **自适应背压**：`getApplyTaskNum(curTaskNum)` 依据 free/used/max 内存、`PhysicalMemoryMonitor`（CPU/内存阈值、min/auto task num、`everyTaskGetnum*6`）动态申请；Linux 走 `getApplyTaskNumForLinux`；`getMaxTaskNum` 经 `TaskSegmentMgr` 计算（一次性/周期段权重 6/1）。

## 6. Agent 推送（DistributeScriptMgr / XftpActionMgr / CrtService）

- `DistributeScriptMgr.summarizeAndUploadScript/releaseUploadScript`：按 `type_version_vendor_template` 缓存；分流 `DISTRIBUTED` vs cmd；拼 JSON；`XftpUploadAction` 上传、`XftpDeleteFileAction` 清理。
- `XftpActionMgr`（单例）：`ThreadPoolExecutor(1,10,keepAlive 60s,queue 200)`，线程名 `XftpActionMgr thread-%d`；`executeAction` 跑 `XftpAbstractionAction` 每 1s 轮询 `resultMap`。
- `CrtService`：4A/EasyTerminal（CRT）集成——`query4AStatus`(`GET /rest/nmot/ir/easyterminal/4a/v1/judge4AStatus`)、`getActive4aSessions`、`update4ANeInfo`、`batchAddNeCommons`、`delResourceData`：经 4A 代理向设备推送/纳管终端会话。

## 7. NetCare 枚举（可直接当移植常量）

| 枚举 | 值 |
|---|---|
| `TaskTypeEnum` | `QUERY("0")`、`NON_CHANGE("1")`、`MICRO_NETWORK_CHANGE("2")`、`PROJECT("3")`、`NETWORK_CHANGE("4")`、`EMERGENCY("5")` |
| `LocalTaskTypeEnum` | `QUERY("0")`、`DAILY_NETWORK("2")`、`ENGINEERING_COMMISSIONING("3")`、`CHANGE_OPERATION("4")`、`TROUBLE_SHOOTING("5")` |
| `RiskLevelEnum` | `LOW(0)`、`MID(1)`、`HIGH(2)`、`FATAL(3)` |
| `CommandTypeEnum` | `COMMANDS("command")`、`MENUS("menu")` |

`CommonScheduleTask`（`@EnableScheduling @Scheduled`）：用 `ScheduleTaskBo`/`TaskInfoBo` + `HttpUtil` 定时与 NMOT 采集服务同步，配合 `TelnetUtil`/`RsaUtil`/`PropertyUtil`。

## 8. 端口全景修正

| 端口 | 真实含义（反编译修正后） |
|---|---|
| **51943** | `netcareinside-driver` 后端 ↔ **桌面客户端** REST 控制面 |
| **38887** | `SnmpCallback` **UDP SNMP 回调端口**（CBB `nmotbusinesscbb`-*，**第二轮已反编译**，见 `10` 篇 §4） |
| 36888 / 38888 | Electron 前端（nmotplatformwebsite / nmotcollectappwebsite） |
| REST `/rest/nmot/*` | 各微服务（platform/ir/inspect/licenseservice/filetransfer/unicollect*/netcareinsideservice…） |

## 9. 移植要点（NetWeaverGo）

1. **协议层**：~~无法从本包取得~~ → **已取得**（`lib/nmotprotocolcbb-*`，见 §1.1 与 `10` 篇 §3）。NetWeaverGo 应自建 SSH/Telnet/SNMP 连接器，并**对照移植 CBB 的实现细节**（PTY、`gbk` 默认字符集、10MB 回显上限、SOCKS5、指纹校验、错误关键字表），同时**复用 `Build*` 系列的参数模型**（`ProtocolSSH` 字段：authMode/keyPath(AES)/fingerprint/superPassword/proxyId）做 Go struct。
2. **脚本执行**：保留「外部脚本引擎」语义——Jython 进程或子进程调 Python；`SnmpCallback` 的 UDP 38887 回调可改为 Go 内部 channel/回调；EVA 分布式脚本 JSON（`eva.cliArray(view,cmd)`）协议可直接复刻为 Go 的 agent 指令格式。
3. **调度/并发**：`inspect_collector` 池（300/300/200）+ 内存自适应背压 → Go `ants`/带信号量 worker pool + `runtime.MemStats` 自适应。
4. **Agent 推送**：`DistributeScriptMgr`+`XftpActionMgr` 对应「下发脚本到设备/采集器」；`CrtService` 4A 集成按需。
5. **NetCare 枚举**直接转为 Go 常量包。
6. **后端↔客户端契约**：若 NetWeaverGo 定位为「替代后端」，可复用 `RequestData`/`HttpUtil` 契约与 `X-HW-ID/X-HW-APPKEY` 鉴权对桌面客户端发 REST（前提：客户端二进制可被对接）。

> **第二轮进展（原"未反编译部分"已补齐）**：本轮已把 `lib/` 下的 `nmotprotocolcbb-*`(10)、`nmotbusinesscbb-*`(7)、`nmotdcscriptcbb-service`、`netcareinside-sdk`、`nicprovidersdk-*`、`nicproviderxdeskdriver-driver-*`、`unicollectutilcbb` 全部反编译（**1538 个 .java**），产物在 Kali `/data/src/cbb/`。要点：
>
> - **协议层**：`nmotprotocolcbb-ssh`（Apache MINA SSHD）等 10 个 jar，见 §1.1 与 `10` 篇 §3；
> - **脚本/任务引擎**：`nmotbusinesscbb-{collect(40),collect-execute(20),commonmodel(100),interpreter(6),scriptmgr(100),taskmgr(54),taskschedule(36)}`，其中 `scriptmgr/collectitem/{CollectItemSet,Vendor,ProductVersion,ScriptItem,Command,Param,PreCollectItem,ThresholdManagement,UnsupportProductVersion}` 正是 `product/CollectItem/*.xml` 的 **Java 类模型**，见 `10` 篇 §4；
> - **分布式脚本**：`nmotdcscriptcbb-service`（247 类）；**采集公共工具**：`unicollectutilcbb`（337 类）；
> - **客户端 SDK**：`netcareinside-sdk`（139 类，含 `HighRisk*`/`WhiteList*`/`TrustList*` 高危命令拦截模型），见 `10` 篇 §5。
>
> 仍未反编译的只有 **eDesk 桌面客户端二进制**（不在本包内），但既然 CBB 协议层已具备，其必要性大幅下降。
