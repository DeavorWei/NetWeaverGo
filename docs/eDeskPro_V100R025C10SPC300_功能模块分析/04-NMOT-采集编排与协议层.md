# 04 · `services/NMOT*` + `NetCareInsideService` 采集编排与协议层

> 这组 Java 服务是 eDesk Pro 的**采集编排 / 设备协议 / 任务调度 / Agent 推送**层。它是真正"连设备、跑脚本"的地方（`product/Script` 与 EMT 的脚本都经此执行）。

## 1. 服务拓扑与层级

| 服务 | 包 | 层级 | 关键类 |
|---|---|---|---|
| **NMOTBusinessService** | `...nmot/nmotbusinessservice` | 编排/业务 | `resource/impl/BuildSSHData`/`BuildTelnetData`/`BuildSNMPData`/`NeProtocolService`/`DeviceLoginInfoService`/`NeService`；`resource/proxy/ProxyService`/`ProxyConnectService`；`protocol/ConfigurationService`/`MultihopInitService`；`task/collect/CollectExecute`；`upgrade/frameagent/FrameExecutor` |
| **NMOTCollectAppService** | `...nmotcollectappservice` | 采集应用 | `service/impl/CollectRoaServiceDelegateImpl`(43KB)/`CollectService`(44KB)/`CrtService`(27KB)/`DeviceHandleService`；`executor/action/CollectTaskAction`(30KB)/`DistributedCollectAction`(26KB)/`PreCollectAction`/`SnmpInitAction`/`RegisterInterpreterAction`；`executor/executormgr/CollectExecutorMgr`/`InspectTaskThreadPool`；`distribute/mgr/DistributeScriptMgr`/`xftp/XftpActionMgr`；`template/TemplateMgr`；`service/upgrade/ScriptPgkWin`/`ScriptPkgLinux` |
| **NMOTPlatformService** | `...nmotplatformservice` | 平台/网关 | `login/WebSecurityConfig`/`authentication`；`gateway/CsrfFilter*`/`ParamCheckFilterConfig`/`TomcatConfiguration`；`filetransfer/UploadAndDownloadService`(40KB)；`platform/management/AppOperationManagement` |
| **NetCareInsideService** | `com.huawei.netcareinside` | **设备协议驱动** | `schedule/CommonScheduleTask`(25KB)/`executor/NetCareInsideExecutor`/`async/AsyncCommand`；`WEB-INF/lib/netcareinside-driver-2.2.6.jar` |
| **NmotLicenseService** | `...nmotlicenseservice` | License | `LicenseServiceDelegateImpl`(19KB)/`LicenseParser`/`LicenseUtils` |

## 2. 设备连接 / 协议层（关键结论）

**SSH/Telnet/Netconf/SNMP 的底层 Session 类不在上述 5 个服务的 `.class` 中，而集中在 `NetCareInsideService/lib/netcareinside-driver-2.2.6.jar`**（+ `us-common/us-os/us-file-21.0.0.5.jar`）。业务/采集服务只负责"构造参数 + 调用驱动"：

- **参数构造**：`BuildSSHData`/`BuildTelnetData`/`BuildSNMPData`/`BuildSshKeyInfoData`；模型 `NeProtocolCliParamPO`/`NeProtocolSnmpParamPO`/`NeProtocolMmlParamPO`；枚举 `SNMPVersionEnum`/`SNMPAuthModeEnum`/`SnmpTypeEnum`/`Vender4A`。
- **凭据/代理**：`resource/proxy/`（ProxyService/ProxyConnectService/ProxyDeviceService）；`DeviceLoginInfoService`/`DevicePwdCleanService`；`ProxyConstant`。
- **采集调用入口**（采集应用层触发驱动）：`CollectTaskAction`/`DistributedCollectAction`/`PreCollectAction`/`SnmpInitAction`/`RegisterInterpreterAction`。
- **大命令策略**：`cfg/bigCommandConfig.xml` 列出 `display current-configuration`、`display arp all` 等"大命令"（流式/特殊解析）。

## 3. 脚本执行（Python/Jython 桥）

并非 `ProcessBuilder` 起独立进程，而是 **Jython**（脚本内 `from java.lang import System`、`import com.huawei.unb.ip...`）。桥配置在 `EMTMessageAnalyseClientService/.../config/inspection.properties`：

```
python.server.ip=127.0.0.1
python.server.port=38887
timeOut=100000   soTimeOut=600000
sendBufferSize=5242880
script.inspect.pool.size=12      # 并发脚本池
```

- **入口**：
  - `collectscript/pythonEntrance/PythonEntrance.py::collect()`：动态 `__import__` 脚本 → 注入 `cli/model/test` → 调 `module.collect()` → `finally` 中 `saveDB()`。
  - `app_data/script/EvalEntrance.py::check()`：注入 `threshold/networkModel/api/rules` → 调 `module.check()`。
- **Java↔Python 桥**：每个服务都有 `resource/external/ExternalFunc.class`（即 `external_function`/`PythonCallback`），Python 回调 Java 服务；配套 `ExternalFuncUtils`。

## 4. 任务调度 / 并发 / Agent 推送

- **编排层**：`task/collect/CollectExecute`、`task/connecttest/*`、`task/taskschedule/strategy/ProxyScheduleStrategy`/`SmartProxySchedule`、`TaskProgressObserver`。
- **采集层**：`executor/executormgr/CollectExecutorMgr`/`InspectTaskThreadPool`/`TaskThreadPool`、`service/util/TaskUtil`(21KB)/`TaskConfigUtil`、`flashever/scheduled/TaskExecutePool`。
- **NetCare**：`schedule/CommonScheduleTask`/`async/AsyncCommand`/`NetCareInsideExecutor`。
- **Agent 推送**：`NMOTCollectAppService/distribute/`（DistributeScriptMgr 派发、XftpActionMgr 经 XFTP 推送、DistributeCollectUtils）、`CrtService`（CRT 证书/agent 下发）、`service/upgrade/ScriptPgkWin|ScriptPkgLinux|IScriptPgk|CollectScriptPkgUtil`（构建 agent 脚本包）；`NMOTBusinessService/upgrade/frameagent/FrameExecutor` + `UpgradeService`（框架 agent 升级）。

## 5. 容量/并发配置（`config/inspect.properties`）

```
connect_retry_count=3            collect_task_create_maxnum=80
collect_selected_ne_maxnum=30000  DEFAULT_CORE_POOL_SIZE=300  DEFAULT_MAX_POOL_SIZE=300
COMMAND_TIMEOUT_DEFAULT=21600    distribute_file_size=5242880
```

## 6. 移植要点（NetWeaverGo）

1. **协议层**：Go 版需自实现 SSH/Telnet/Netconf/SNMP 连接器（替代 `netcareinside-driver`），但可**直接复用参数模型** `Build*Data` + `NeProtocol*PO`（CLI/SNMP/MML 参数结构）。
2. **脚本执行**：保留 38887 端口 Jython 桥语义，或改为 Go 子进程调 Python；`script.inspect.pool.size=12` → Go `semaphore`/worker pool。
3. **调度**：`CollectExecutorMgr`/`CommonScheduleTask` 模式 → Go `errgroup` + 定时任务。
4. **Agent 推送**：`DistributeScriptMgr`+`XftpActionMgr` 对应交付工具的"下发脚本到设备/采集器"能力。
5. **大命令**：`bigCommandConfig.xml` 的大输出命令清单与流式处理策略应纳入连接器的回显读取逻辑。
