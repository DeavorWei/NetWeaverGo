# 10 · 新发现资产：协议层 CBB、终端过滤器与可复用配置

> **本篇为新增**，用于记录本轮核实中发现的、`00`~`08` 系列**未覆盖或结论有误**的资产。
> 其中最重要的一条是：**`04` 篇"协议层无法从本包取得"的结论不成立**——真正的 SSH/Telnet/SNMP/CLI/终端仿真实现在 `lib/` 的 `nmotprotocolcbb-*` jar 族里。

---

## 1. ⚠️ 对 `04` 篇的重大修正

| 04 篇原结论 | 修正后（本轮反编译实证） |
|---|---|
| §1「真正的 SSH/Telnet/Netconf/SNMP/Serial 引擎在桌面客户端二进制里，**不在本交付包内**」 | ❌ **不成立**。`lib/` 下存在 `nmotprotocolcbb-{ssh,telnet,snmp,cli,mml,xftp,multihop,serialport,model,msgfilter}` 共 **10 个 jar**，含**真实协议实现**：<br>· SSH → `org.apache.sshd.client.SshClient` / `ChannelShell` / `PtyChannelConfiguration`（Apache MINA SSHD 2.18.0）<br>· SNMP → `snmp4j-2.8.18-h2.jar`<br>· 终端仿真 → `unicollectprotocolcbb/ssh/msgfilter/*` |
| §9「NetWeaverGo 若要做实机协议连接，需另寻桌面客户端二进制（或抓其与后端 51943 的 REST 报文）做逆向」 | ⚠️ 降级为**可选**。CBB 协议层已可 1:1 参考移植，无需逆向客户端 |
| §1「`netcareinside-driver` 是后端↔客户端 REST 控制面」 | ✅ **仍然成立**（`netcareinside-sdk-2.2.4.jar` 的 `bo/RequestData`、`async/AsyncService` 印证） |

**边界说明**：CBB 是**可被后端直接调用的协议组件库**（`INetProtocol` / `ISession` 接口 + 工厂），而 `netcareinside-driver`/`netcareinside-sdk` 是**另一条**通往桌面客户端的 REST 通道。两条路径并存——前者用于后端自采集，后者用于托管客户端会话。

---

## 2. `lib/` 真实构成（实测 257 jar）

```
lib/
├── 压缩工具     7z.dll / 7z.exe / 7za* / 7zxa.dll
├── BSP 框架     baize-*.jar（资源/安全/Web 基础）、com.huawei.bsp.*（26.455.*）
├── 安全         bcprov/bcpkix/bcmail/bcpg/bcutil-jdk18on-1.84、esapi、huawei-secure-*、cmsverifycbb、com.huawei.cms.verify
├── 数据库       h2-2.4.240、HikariCP-6.3.3、mybatis-3.5.19 + mybatis-spring-*、jedis/lettuce（Redis）
├── Web/序列化   netty-all-4.1.135、httpclient5-5.5、jackson-2.19.1、fastjson2-2.0.63、gson-2.11.0
├── 脚本引擎     jython-standalone-2.7.4.jar          ← Jython 2.7 实证
├── 协议/终端    sshd-core|common|sftp|putty-2.18.0、snmp4j-2.8.18-h2、jSerialComm-2.11.0、commons-net-3.11.1
├── NMOT 协议 CBB nmotprotocolcbb-*.jar × 10          ← ★ 本轮新发现
├── NMOT 业务 CBB nmotbusinesscbb-*.jar × 7、nmotdcscriptcbb-service、unicollectutilcbb
├── 平台 CBB     nmotplatformcbb-*.jar × 12、nmotplatformlauncher-*.jar × 6、nmlicensecbb
├── 客户端 SDK   netcareinside-sdk-2.2.4、nicprovidersdk-{public,business}、nicproviderxdeskdriver-driver-{public,business}
└── 报表         commons-csv-1.13.0、commons-math3、fontbox/jai-imageio（PDF/图）
```

---

## 3. ★★★ `nmotprotocolcbb-*` —— 协议层（反编译共 260 个类）

> 反编译方式：`lib/*.jar` → scp 至 Kali → `cfr 0.152`（与 `03`/`04` 篇同一套流程）。
> 包根：`com.huawei.ncetools.nmot.nmotprotocolcbb.*`

| 模块 | 类数 | 核心类 | NetWeaverGo 对应 |
|---|---|---|---|
| **`-ssh`** | 5 | `SSHProtocol`(INetProtocol)、`SSHReceiver`(Runnable)、`SSHProtocolUtil`、`SSHVersionUtils`、`SSHReceiverErrorHandle` | `internal/sshutil/client.go` ★ |
| **`-telnet`** | 2 | `TelnetProtocol`、`MyTelnet` | `internal/telnetutil/` ★ |
| **`-snmp`** | 17 | `SnmpProtocol`、`SnmpSession`、`SnmpSessionFactory`、`SnmpParam`、`SnmpConfig`/`SnmpDefaultConfig`、`SnmpType`、`SnmpCollectModeEnum`、`StampValue` + `util/{CipherManagerProxy,InetAddressUtil,NetUtil,NetworkInterfaceUtil,SnmpProtocolUtil}` | `internal/snmp/querier.go` ★ |
| **`-cli`** | 9 | `CliSession`、`CliReceiver`、`CliFtpReceiver`、`CliSessionUtils`、`MsgCheckUtils`、`ProtocolFactory`、`SpecialCharactersFilterProxy`、`util/{MMLSessionUtils,ProtocolCfgForCommand}` | `internal/executor/session_*.go` ★ |
| **`-msgfilter`** | 116 | `TerminalInterpreter`、`Screen`、`TerminalBuffer`、`TerminalXTerm`、`TerminalXTermUtil`、`CompatTerminal`、`MsgFilter`、`MsgFilterPolicy`、`AsciiCode`<br>· `interprethandler/`：`AnsiPrinter`、`Bell`、`Bs`、`Cbt`、`Tab`、`Tbc`、`TrackMouseh`、`Vmto`、`Vmot2`、`XTermSeq`…<br>· `dpmodeshandler/`：`DpModeBreak`、`GSetCopy`、`GSetCopyAndCursorSave`、`GSetsReset`、`VtResetAndCursorSave`、`WindowRelative`<br>· `sgrmodeshandler/`：`AttributeSet`、`AttributesClear`、`BackgroudColor`、`ForegroudColor`、`GSets`、`SgrModeBreak` | `internal/terminal/{ansi.go,line_buffer.go,replayer.go}` ★★ **厂商级终端仿真** |
| **`-model`** | 47 | `args/{ConnectionArgs,CmdArgs,SerialPortConnectArg}`、`cfg/ProtocolCfg`、`constant/{ProtocolType,SSHAuthMode,ProtocolConstant,FlowControlEnum,ParityEnum,NumStopBitsEnum}`、`inf/{IProtocol,INetProtocol,ISession,IFTPProtocol,IFilterSpecialReceive}`、`filter/DeviceFilterReceiveKeyModel`、`model/{CommandConfig,RuleProtocolRegex,VendorFilterPattern,VendorCharsetCmd}`、`strategy/{AndOption,OROption,ComplexPattern,SimplePattern,RegStrategy,PatternFactory,RuleParser,XmlHelper,PatternSpecialChar}` | `internal/sshutil` + `internal/matcher` ★★ |
| **`-multihop`** | 34 | 跳板/多跳代理（配 `config/multihop/`） | 待建（NetWeaverGo 无跳板）|
| **`-xftp`** | 7 | `FTPProtocol`、`SFTPProtocol`、`SFTPClientHelper`、`XftpProtocolUtils`、`XftpConstant` | `internal/sftputil/` ★ |
| **`-mml`** | 21 | MML 会话 + `cert/auth/internal/{KeyStoreFactory,Ssl...}` 证书认证 | 参考 |
| **`-serialport`** | 2 | `SerialProtocol`、`ReceivedDataListener`（`jSerialComm`） | 参考 |

### 3.1 `model/strategy/` —— **提示符/回显正则策略引擎（最值得移植）**

```java
IPattern / IRegStrategy / IOption / IMatchResult          // 抽象
├── SimplePattern          // 单正则
├── ComplexPattern         // 复合模式
├── AndOption / OROption   // 逻辑组合（与/或）
├── RegStrategy            // 正则策略
├── PatternFactory / PatternFactoryUtils
├── RuleParser / XmlHelper // 从 XML 解析策略（规则驱动）
└── PatternSpecialChar     // 特殊字符处理
```

配套数据模型：

| 类 | 语义 |
|---|---|
| `RuleProtocolRegex` | 「协议 + 正则」规则项 |
| `VendorFilterPattern` | **厂商级回显过滤正则** |
| `DeviceFilterReceiveKeyModel` | **设备级接收关键字过滤**（驱动 `config/deviceversion/reciveFilterKey.properties`） |
| `CommandConfig` | 命令配置（超时/提示符/编码…） |
| `VendorCharsetCmd` | **厂商级字符集命令**（`gbk` ⇄ `utf-8` 切换） |

> `SSHReceiver` 默认 `Charset.forName("gbk")`，且 `MAX_LENGTH = 0xA00000`（**10MB 单次回显上限**）——这两点对 NetWeaverGo 的 `internal/terminal`（中文设备回显乱码、大回显保护）有直接参考价值。

### 3.2 `ConnectionArgs` / `ProtocolCfg` —— 连接参数模型

与 `04` 篇 §3 的 `BuildSSHData`/`ProtocolSSH` 互为上下游：`04` 篇描述的是**业务侧构造**，这里的是**协议侧消费**（`args.ConnectionArgs` + `cfg.ProtocolCfg`）。SSH 支持 **SOCKS5 代理**（`model/util/Socks5Utils`）与 PTY（`PtyChannelConfiguration`）、主机密钥指纹校验（`KeyUtils`/`BuiltinDigests`）。

---

## 4. ★★ `nmotbusinesscbb-*` —— 脚本/任务引擎（反编译共 356 个类）

| 模块 | 类数 | 关键内容 |
|---|---|---|
| `-commonmodel` | 100 | 公共业务模型（设备/任务/结果） |
| `-scriptmgr` | 100 | **脚本管理器**（见下 §4.1） |
| `-taskmgr` | 54 | `util/{TaskConstants,TaskStatusMgr,TaskMgrUtils,SecureCalculate,SerialCreateTaskCfg,SystemConfigurationCfg,ZipUtils,CollectResultUtils,CycleCollectResultUtils,QkUtil}`、`tef/thread/TefThreadErrorHandle` |
| `-collect` | 40 | 采集执行 |
| `-collect-execute` | 20 | 采集执行（线程/申请） |
| `-taskschedule` | 36 | `sdk/{JobMgr,model/{Task,Executor,CatchTaskSegment,TaskApplyInfo,TaskOperateContent,TaskOperateObject}}`、`executor/ExecutorMgr`、`jobmgr/subtaskmaker/{CollectTaskMaker,ConnectTestTaskMaker,SubTaskMakerFactory}`、`jobmgr/taskmgr/{TaskMgr,TaskDataMgr,TaskProcessMgr}`、`external/ExternalFunc` |
| `-interpreter` | 6 | 解释器入口 |

### 4.1 `scriptmgr/collectitem/` —— **`product/CollectItem/*.xml` 的 Java 数据模型**

| 类 | 对应 XML 元素（见 `01` 篇 §4） |
|---|---|
| `CollectItemSet` | `<CollectItemSet>` |
| `Vendor` | `<Vendor>` |
| `ProductVersion` | `<ProductVersion name="...">`（逗号分隔款型/版本白名单） |
| `ScriptItem` / `LinkedScriptItem` | `<Script>` |
| `Command` | `<Command>` |
| `Param` | `<Script><Param>` |
| `PreCollectItem` | `<CollectItem isPreCollect="true">`（前置采集项）★ |
| `ThresholdManagement` | `<threshold>`（阈值外置）★ |
| `UnsupportProductVersion` | 不支持版本黑名单 |

其他骨架：
- `scriptmgr/dev/`：`DeviceAgent`、`DeviceAgentUtils`、`DevDirInfo(Manager)`、`NeDataPersistentUtil`
- `scriptmgr/common/`：`AbstractCollectItemExecutor`、`AbstractGetVersionExecutor`、`AbstractParseScriptExecutor`、`AbstractReachabilityCheckExecutor`、`AbstractCheckPermissionExecutor`、`DeviceInfo`、`ModelInfo`、`SnmpNeInfo`
- `scriptmgr/enums/`：`EnumScriptType`、`EnumCollectMode`、`ExecMode`、`DeviceVendorType`、`DeviceDomainType`、`ConnectCmdPolicyType`、`EnumMessageSendMode`、`EnumCIType`、`EnumExcuteScene`

> **反向印证**：`01` 篇推断的「注册表驱动 + 前置采集 + 阈值外置」三特性，在这里找到了**代码级实现**（`PreCollectItem`、`ThresholdManagement` 是独立类）。移植 `CollectItem` 语义时可直接对照这套类名。

### 4.2 另两个 CBB

| jar | 类数 | 说明 |
|---|---|---|
| `nmotdcscriptcbb-service` | 247 | 分布式（EVA / 设备端 Python 代理）脚本 CBB —— 对应 `04` 篇 §4 的 `DistributeScriptMgr` |
| `unicollectutilcbb` | 337 | 采集公共工具 CBB（采集器侧） |

---

## 5. `netcareinside-sdk` / `nicprovidersdk` —— 高危命令与终端托管

| jar | 类数 | 关键 `bo` |
|---|---|---|
| `netcareinside-sdk-2.2.4` | 139 | `RequestData`/`RequestDataParam`（REST 信封，与 `04` 篇 §2 一致）、**`HighRiskCommandInfoBo`**、**`HighRiskFullCommandBo`**、**`HighRiskInterceptRecordBo`**、`WhiteListCommandInfoBo`、`WhiteListMenuInfoBo`、`TrustListBo`、`BlockListBo`/`BlockListMenuInfoBo`、`BypassPolicySchemeBo`、`ClientControlPolicyBo`、`CrtSchemeCommandInfoBo`、`ChangePeriodCommandBo`、`EscapeLogBo`、`FeatureOperationRecordBo`、`UserAuthenticateInfoBo`、`VerifyCodeRecordBo`、`ScheduleTaskBo`/`TaskBo`/`TaskInfoBo`/`TaskToolBo`/`TaskSubtypeBo`/`TaskDotInfoBo`/`TaskFinishBo`、`NetCareDotStepBo`/`DotInfoBo`/`DotStepBo` |
| `nicprovidersdk-public` | 126 | NIC 提供方 SDK（公共） |
| `nicprovidersdk-business` | 34 | NIC 提供方 SDK（业务） |
| `nicproviderxdeskdriver-driver-public` | 24 | eDesk 桌面驱动（公共） |
| `nicproviderxdeskdriver-driver-business` | 15 | eDesk 桌面驱动（业务） |

> **可移植点**：`HighRisk*` / `WhiteList*` / `TrustList*` / `BlockList*` 这一组 BO 定义了一套**「高危命令拦截 + 白名单/信任列表 + 逃生（Bypass）+ 操作留痕」**的完整模型。NetWeaverGo 目前对高危命令只有"风险命令清单"层面的拦截（`08` 篇 P0 项），可直接借用这套字段设计。

---

## 6. ★★ 本轮新发现的**纯配置型**可复用资产

这一节是纯数据资产，**可零代码损耗搬入 NetWeaverGo**。

### 6.1 `config/deviceversion/sensitiveCmd.xml` —— 全厂商**敏感信息脱敏正则库**（最高性价比）

结构：`<SensitiveCmd>` → `<Category name="产品族/厂商">` → `<Cmd name="命令A,,,命令B,,,"/>` → N × `<Filter name="正则"/>`

覆盖 **17 个 Category**：`AR router`、`NE router`、`Ethernet Switch`、`DC`、`Firewall`、`WLAN`、`""`（通用/多厂商）、`H3C`、`CISCO`、`ALU`、`NOKIA`、`ZTE`、`JUNIPER`、`DPTECH`、`RUCKUS`、`SANGFOR`、`DELL`。

命中场景举例（`Cmd` 用 `,,,` 分隔"多命令同规则"）：

```xml
<Category name="NE router">
  <Cmd name="display diagnostic-information,,,collect diagnostic-information,,,display this,,,display current-config*,,,display saved-configuration*,,,compare configuration,,,">
    <Filter name="snmp-agent community read cipher (\S*) alias ([^__CommunityAliasName_\d+_\d+])"/>
    <Filter name="hwtacacs-server shared-key cipher (.*)"/>
    <Filter name="username \S+ password (.*)"/>
    <Filter name="local-user \S+ password irreversible-cipher (.*)"/>
    <Filter name="rsa-key ([\S]*)"/>
    <Filter name="password hash (\S+)"/>
    <Filter name="pre-shared-key cipher (.*)"/>
    <Filter name="header login information .*User:(.*?)\-+Password:(.*?)\-+"/>
    ...
  </Cmd>
</Category>
```

> **NetWeaverGo 现状**：`internal/logger/sanitizer.go` 仅有**通用**脱敏（密码等关键词）。
> **建议**：把本 XML 转为（a）`internal/report` 落盘前的**分厂商脱敏管线**，以及（b）前端展示/导出的脱敏开关。这是**交付现场最刚性的合规需求**（回显里带 `cipher` 密文不能外流）。
> 配套：`<Cmd name="...">` 的 `,,,` 多命令语法可直接映射为 `[]string`。
>
> 注：`product/Script/ceas/Common/cmd_echo_writer.py::hide_echo_password` 是同一需求的**脚本侧简化版**（只处理 `password irreversible-cipher`），本 XML 是其完整版。

### 6.2 `config/deviceversion/connectCommandPolicy.properties` —— 回显"拼接命令"策略

```properties
# key=设备类型, value=1|2|3
# 1、回显内容必须包含命令（默认）  2、回显内容必须以命令开始  3、回显内容第一行必须是命令
CE1800V=2
```
> 解决"设备终端宽度自适应导致首行命令被截断"的判定问题——NetWeaverGo 的 `matcher` 若做"回显首行对齐"校验，需要这个策略位。

### 6.3 `config/deviceversion/reciveFilterKey.properties` —— 接收端特殊字符过滤

```properties
Linux_suse11_1=[1m
Linux_suse11_2=[m      # Linux suse11 提示符中的 ANSI 残留
```

### 6.4 `config/deviceversion/keyCmd.xml` —— **多厂商关键命令表**

12 个厂商（`HUAWEI/CISCO/JUNIPER/ALU/RUIJIE/H3C/INSPUR/EXTREME/ZTE/HIRSCHMAN/ARUBA/HPE`）× 2 个场景（`UNETBUILDER_COLLECT_TASK`、`IPCRYSTAL_COLLECT_TASK`）→ 配置/接口采集命令清单。
> 这是「**厂商 → 配置采集命令**」的最小可用映射，比 `03` 篇的 `modelCmdConfig.xml`（按数据模型组织）更粗但更全，可作为 NetWeaverGo "配置备份/拓扑采集"默认命令组的内置种子。

### 6.5 `config/deviceversion/devicevalidate.properties` —— 回显校验范围

```properties
cmd_validate_order = [{"scene":"UNETBUILDER_COLLECT_TASK","needValidate":"true","validateModel":"0"},
                      {"scene":"INSPECTOR_COLLECT_TASK","needValidate":"true","validateModel":"1"},
                      {"scene":"FAULT_COLLECT_TASK","needValidate":"false"}, ...]
cmd_prompt_scene   = [{"scene":"IPCRYSTAL_COLLECT_TASK","needPrompt":"false"}]
```
> 按**任务场景**决定"是否校验回显 / 校验模型 / 是否提示异常"——NetWeaverGo 的 `matcher` 目前是全局策略，缺这一维度。

### 6.6 `config/deviceversion/protocoldiscovery.properties` —— 拓扑发现协议与厂商白名单

```properties
discoveryprotocol=[{"protocol":"LLDP",...},{"protocol":"CDP(CISCO)",...},{"protocol":"OSPF",...},
                   {"protocol":"ARP","descriptionzh":"网关网元的ARP表项中包含大量的用户终端IP，这些设备无法采集且会降低网络探测效率，是否继续勾选？"}]
discoveryvendor=["HUAWEI","CISCO","H3C"]
```
> **CDP**（Cisco 私有发现协议）与 **OSPF** 作为拓扑发现源，NetWeaverGo 的 `topology_builder` 目前只有 LLDP/FDB/ARP；且"ARP 会把用户终端误当网元"这条**业务风险提示**非常值得在产品上复刻。

### 6.7 `config/cmd.properties` —— 跳板 / 代理 / VPN 命令模板（13 条）

```properties
hw_vpn_cmd=telnet vpn-instance %s %s %s
8011_vpn_cmd=telnet -vpn-instance %s %s %s
cs_vpn_cmd=telnet %s %s /vrf %s
telnet_cmd=telnet %s %s
ipv6_telnet_cmd=telnet ipv6 %s %s
ipv6_hw_vpn_cmd=telnet ipv6 vpn-instance %s %s %s
hw_proxy_ssh_cmd=stelnet %s %s
ipv6_hw_proxy_ssh_cmd=stelnet ipv6 %s %s
hw_proxy_ssh_vpn_cmd=stelnet %s %s -vpn-instance %s
8011_proxy_ssh_vpn_cmd=stelnet %s %s vpn-instance %s
ipv6_hw_proxy_ssh_vpn_cmd=stelnet ipv6 %s -vpn-instance %s
unix_ssh_no_uname_cmd=ssh %s -p %s
unix_ssh_no_uname_cmd_v1=ssh -1 %s -p %s
unix_ssh_cmd=ssh -l %s %s -p %s
unix_ssh_cmd_v1=ssh -1 -l %s %s -p %s
cs_ssh_cmd=ssh %s %s
```
> 支撑「**跳板机/堡垒机接入**」场景（对应 `nmotprotocolcbb-multihop` 与 `config/multihop/`）。NetWeaverGo 目前只有直连。

### 6.8 `config/script/InterfaceTransition.properties` —— 接口名归一化（含接口类型编码）

```properties
GigabitEthernet=(\d*(GigabitEthernet|xgei_|gei_|Gi|Ge|xe|Te|et|TenGig.*|FortyGig.*|XGigabitEthernet|XGE|TenGige|tengi|\d*GigabitEthernet|HundredGigE|ether|\d*\|?\d*GE?)(-*\s*\d*.*))|((\d{1,3}[/:]){1,3}\d{1,3})|(\d+)
Ethernet=6|((Ethernet|fei_|fe:?|Fa|MEth|FastEthernet|eth)(-*\s*\d+.*))
Pos=39|((POS|so|po)(-*\s*\d+.*))
Serial=(serial|Serial|se)(-*\s*\d+.*)
Trunk=161|((\w+-Trunk|smartgroup|bundle|ae|as|port+-channel|Ether|Bundle-Ether)(-*\s*\d+.*))|(PC\d+)|(lag\d+)
Vlanif=53|((vlanif|Vlanif|vlan|vl)(-*\s*\d+.*))
Tunnel=131|((tunnel|Tunnel|Tu|ip)(-*\s*\d+.*))
sub=.*(-+|\.+)(\d+|mpls)
```

**格式约定**：`<标准名>=<数字>|<正则>`，`<数字>` 是**接口类型编码**（`Ethernet=6`、`Pos=39`、`Vlanif=53`、`Tunnel=131`、`Trunk=161`——与 IANA ifType 体系同源）。配套文件：`InterfaceFiltrate.properties`、`InterfaceTransitionblack.properties`、`ciscoInterfaceTransition.ini`。

> **与 `03` 篇的 `ifnamemapping.json` 分工**：`ifnamemapping.json` 是**点对点别名**（`{"mappingIfName":"Twenty-FiveGigE","originalIfName":"25GE","vendor":"H3C"}`），`InterfaceTransition.properties` 是**正则→类型编码**的归一化。二者合起来才是完整的接口名标准化模块（`08` 篇 P3 项）。

### 6.9 `services/IPDeviceCheckService/.../lldparseconfig/` —— **LLDP 解析规则（7 份 JSON）**

```
lldparseconfig/
├── enterpriseruleconfig.json          企业网规则
├── serviceruleconfig.json             运营商规则
├── ipmasterlldruleconfig.json         IPMaster 规则
├── ipmasterlldruleconfig2026.json     IPMaster 2026 版
├── manuallylldruleconfig.json         手工/兜底规则
├── manuallylldruleconfig2026.json     手工 2026 版
└── toolchainruleconfig.json           工具链规则
```

> `03` 篇只提到 `linkrestoration/`（反编译类），**未提及这批规则文件**。它们位于 `parsecfg` 同级，是 `com/huawei/ncetools/ipdevicecheckservice/parselld/`（13 个类）的规则来源。
> **对 NetWeaverGo 的意义**：`internal/taskexec/topology_builder.go` 目前把 LLDP 解析**硬编码在 Go 里**；这批 JSON 说明华为把"不同客户场景的 LLDP 解析差异"**外置为规则配置**（企业网 / 运营商 / 工具链 / 手工兜底）。这是拓扑模块走向"多客户可配置"的关键范式。

### 6.10 `services/IPDeviceCheckService/.../mapping/*.xml` —— MyBatis Mapper 揭示的表

| Mapper | 对应业务表（`03` 篇未覆盖） |
|---|---|
| `CheckTaskBatchMapper.xml` | 检查任务批次 |
| `TaskBatchProgressMapper.xml` | 批次进度 |
| `ParseTaskMapper.xml` | 解析任务 |
| `ParseDataPrepareMapper.xml` | 解析数据准备 |
| `ProcessMessageMapper.xml` | 处理消息 |
| `IfNameMappingMapper.xml` | 接口名映射 |
| `LldParseTemplateMapper.xml` | **LLDP 解析模板** |
| `OpticalCheckRuleMapper.xml` | **光模块检查规则** |

> `03` 篇 §7 讲了 `OpticalCheckRule` 的**算法**，这里补上了它的**持久化表**与 `LldParseTemplate`（LLDP 可配置模板）——即弱光阈值与 LLDP 规则都是**库表驱动**而非硬编码。

---

## 7. 其余目录（低移植价值，仅备查）

| 目录 | 内容 |
|---|---|
| `platform/electron/` | 58 个 `.pak` + `electron.dll` 等 —— Electron 渲染进程资源包（**仅反编译产品用**，如 `app.asar` 类） |
| `jre/` | 内嵌 JRE（156 无后缀 + 86 dll + 13 exe） |
| `bin/` | 22 个 bat/ps1：`start/stop/uninstall/get_log`、`h2_import/h2_export`、`upgrade`、`diskremain`、`check_proc_start`、`easycrt`、`get_info_service`、`log4operation`、`lotsuser_upgrade`、`Instance/`、`script/` |
| `launcher/lib/` | `nmotplatformlauncher-launcher-5.6.782.jar`（单 jar 启动器） |
| `apps/<App>/` | `app.json` + `*.properties`(4) + `*.json` + `*.svg`(2) + `*.bat` + `*.yml`；`Platform-App` 为嵌套 `Platform-App/Platform-App/{config/,process.json,start.bat}` |

---

## 8. 移植要点（NetWeaverGo）

| # | 资产 | 动作 | 优先级 |
|---|---|---|---|
| 1 | `sensitiveCmd.xml` | 转 `internal/report` 的分厂商**脱敏管线**（17 Category × N 正则） | **P0** |
| 2 | `connectCommandPolicy.properties` + `devicevalidate.properties` + `reciveFilterKey.properties` | `internal/matcher` 增加「按场景/按设备」的提示符与回显校验策略 | **P0** |
| 3 | `nmotprotocolcbb-model/strategy/*` | 用 Go 复刻「And/Or/ComplexPattern + RuleParser(XML)」的提示符识别策略引擎，替换 `matcher` 的硬编码规则 | **P1** |
| 4 | `nmotprotocolcbb-ssh/telnet/snmp` | **对照移植**（非逐行）：PTY 配置、`gbk` 默认字符集、10MB 回显上限、SOCKS5、指纹校验 | **P1** |
| 5 | `nmotprotocolcbb-msgfilter`（116 类） | 对照 `internal/terminal` 补齐：`Vs/Vmto/Vmot2/Tab/Tbc` 等处理器、`SGR` 颜色、`DP modes`（GSetCopy/VtReset/WindowRelative） | **P1** |
| 6 | `nmotbusinesscbb-scriptmgr/collectitem/*` | 为 `CollectItem` 语义（`PreCollectItem`/`ThresholdManagement`/`ProductVersion`）建立 Go 模型 | **P1** |
| 7 | `lldparseconfig/*.json` | LLDP 解析规则外置（企业网/运营商/工具链/手工兜底） | **P2** |
| 8 | `keyCmd.xml` | 内置「厂商 → 配置/接口采集命令」种子 | **P2** |
| 9 | `cmd.properties` + `config/multihop/` | 跳板/代理/VPN 接入方式 | **P2** |
| 10 | `InterfaceTransition.properties` + `ifnamemapping.json` | 合并为接口名标准化模块（含 **ifType 数字编码**） | **P2** |
| 11 | `netcareinside-sdk` 的 `HighRisk*`/`WhiteList*` BO | 高危命令拦截 + 白名单 + 逃生 + 留痕的字段设计 | **P2** |
| 12 | `protocoldiscovery.properties` | 拓扑发现源增加 **CDP / OSPF**，并复刻"ARP 会误纳用户终端"的风险提示 | **P2** |
