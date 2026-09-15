# 07 · `apps/` 应用模块

> 5 个 APP 定义（每个含 `app.json` + `*.properties` + 前端 `*.svg` + 启动 `*.bat` + 路由 `*.yml`）。它们定义了 eDesk Pro 的**功能边界**，对应交付工具的「功能菜单 / 子系统」划分。

## 1. 五个 APP 一览

| APP | nameZh / nameEn | 功能定位 | 前端 URL | 核心菜单 |
|---|---|---|---|---|
| **Easy-Health** | 数据采集 / Data Collector | 采集网元健康信息（**巡检主模块**） | `nmotcollectappwebsite` | 资源管理 / 采集任务 / 模板管理 / 巡检分析 |
| **Easy-Diagnosis** | 故障诊断 / Fault Diagnosis | 看守设备健康、故障定界 | `nmotcollectappwebsite` | 资源管理 / 采集任务 / 模板管理 / 故障定界 |
| **Easy-RFC** | 网络变更 / Network Change | 版本/补丁/MOD/license 升级 + 变更前后业务比对（高竹） | `uUpgrade` | 资源管理 / 升级工具 / 高竹工具 / 辅助工具 |
| **FiberOptics-Check** | 错纤弱光排查 | 海量光纤场景的连纤/光模块排查（工程交付） | `nmotcollectappwebsite` | 资源管理 / 采集任务 / 模板管理 / 错纤弱光排查 |
| **Platform-App** | 首页 / Home Page | 平台首页与 App 管理 | `nmotplatformwebsite` | 无（首页聚合） |

> 版本：均为 `V100R025C10`；Easy-Health/Easy-Diagnosis/Easy-RFC 的 patch 为 `SPC300`，FiberOptics-Check 为 `SPC210`，Platform-App 为 `SPC100`。

## 2. 各 APP 详解

### 2.1 Easy-Health（数据采集 / 巡检主模块）
- 描述：帮助工程师快速准确采集网元健康情况，提升巡检能力与运维效率。
- 菜单：`resourceManagement`(资源管理) / `collectTask`(采集任务) / `templateManagement`(模板管理) / `Inspection`(巡检分析)。
- **对应 NetWeaverGo**：主力「巡检/采集」子系统。其「采集任务 + 模板管理 + 巡检分析」正是 NetWeaverGo 的核心交付能力。

### 2.2 Easy-Diagnosis（故障诊断）
- 描述：看守数通域设备健康状态，故障时辅助快速定位、定界。
- 菜单：资源管理 / 采集任务 / 模板管理 / `faultManage`(故障定界)。
- **对应 NetWeaverGo**：故障诊断/健康看护模块（可复用采集框架 + 故障定界规则）。

### 2.3 Easy-RFC（网络变更）
- 描述：面向变更场景的自动化操作与业务检查比对；支持版本/补丁/MOD/license 升级，以及变更前后自动业务检查比对（高竹工具）。
- 菜单：`upgrade`(升级工具) / `gaozhu`(高竹工具) / `auxiliaryTools`(辅助工具)。
- **对应 NetWeaverGo**：变更/升级验收模块（变更前快照 → 变更 → 变更后比对）。

### 2.4 FiberOptics-Check（错纤弱光排查）
- 描述：工程交付中针对海量光纤场景的连纤排查、光模块排查等集成调测能力。
- 菜单：资源管理 / 采集任务 / 模板管理 / `commissioning`(错纤弱光排查)。
- **对应 NetWeaverGo**：弱光/光纤专项（底层即 `IPDeviceCheckService` 的 `lowlight/` + `crossconnectedfiber/` + `linkrestoration/`）。

### 2.5 Platform-App（首页）
- 描述：Home Page，聚合各 APP。
- `houpQueryParam`：`productFamily=Service Router, productGroupType=Platform-App, productType=IP Toolkit`。
- **对应 NetWeaverGo**：工作台/首页。

## 3. 公共模块结构（每个 app 的目录）

```
apps/<AppName>/
├── app.json              应用定义(id/name/version/菜单/前端URL)
├── <app>.properties      运行配置(4 个)
├── <app>.json            附加配置(2 个)
├── start_logo_zh.svg / start_logo_en.svg   启动 Logo
├── uninstallApp.bat / start*.bat           启停脚本
└── <app>.yml             路由/服务注册
```

## 4. 安全白名单揭示的微服务拓扑

`config/security_whitelist.properties` 暴露了全部后端微服务 REST 前缀：
- `/rest/nmot/platform/*`（平台）、`/rest/nmot/systemconfig/*`（系统配置）
- `/rest/nmot/ir/**`（巡检/采集）、`/rest/nmot/inspect/v1/**`（巡检分析）
- `/rest/nmot/licenseservice/*`（License）、`/rest/nmot/filetransfer/*`（文件传输）
- `/rest/networkdataingestionservice/*`（数据接入）、`/rest/unicollect*service/**`、`/rest/uniconnector*service/**`、`/rest/collectoragent/**`（采集/连接/代理）
- `/netcareinsideservice/**`（设备驱动）、`/rest/upgrade/*`（升级）

## 5. 移植要点（NetWeaverGo）

1. **功能边界可直接映射**：5 个 APP = NetWeaverGo 的 5 个一级子系统（巡检、故障、变更、光纤、工作台）。
2. **菜单即能力目录**：每个 APP 的 `menuCategories` 已是「资源管理 / 任务 / 模板 / 分析」的标准四段式，NetWeaverGo 可沿用。
3. **FiberOptics-Check / Easy-Diagnosis** 是差异化专项，建议作为独立功能模块立项，底层复用巡检采集框架 + `IPDeviceCheckService` 专项算法。

---

## 6. ✅ APP ↔ 后端服务映射（第二轮实测确证）

`§1`/`§2` 的映射原为**推测**。本轮通过各服务 `app_define.json` 的 `menuIds` / `description` 得到**确证**：

| APP | 承载服务 | 证据（`app_define.json`） |
|---|---|---|
| **Easy-Health**（数据采集/巡检） | `NMOTPlatformService`、`NMOTBusinessService`、`NMOTCollectAppService`、**`EMTMessageAnalyseClientService`** | `NMOT*`：`menuIds={resourceManagement, templateManagement}`、`{collectTask, aIHardware}`<br>`EMT`：`description="Inspection"`，`menuIds={Inspection: /PMIAnalysis/index.html#/pmiAnalysis}` |
| **Easy-Diagnosis**（故障诊断） | **`IPOnlineService`** ★ | `callbacks.networkoccupation="/easydiagnosis/external/v1/getExecutingTaskCounts"`；<br>`menuIds={businessCompare, starWay, ipStorm, faultManage, taskManage}` |
| **Easy-RFC**（网络变更/高竹） | **`uupgrade`** | `menuIds={upgrade: /uUpgrade/index.html#/upgrade, gaozhu: /uUpgrade/gaozhu.html#/gaozhu, auxiliaryTools}` |
| **FiberOptics-Check**（错纤弱光排查） | **`IPDeviceCheckService`** + `NMOT*` | `description="continuous relocation inspection app"`，`menuIds={commissioning: /ipdevicecheckwebsite/index.html#/}` |
| **Platform-App**（首页） | 平台底座（`NmotLicenseService` 等） | 无 `menuIds` |
| （横切） | `NetCareInsideService` | 无 `menuIds`（`type: microService`，后端↔客户端 REST 控制面） |

> **两条重要推论**：
> 1. **Easy-Diagnosis 的全部差异化能力集中在 `IPOnlineService`**（此前 `01`~`08` 均未覆盖！）——其业务比对 / 告警规则 / 端口角色 / 智能 Ping 资产见新增的 **`09` 篇**。
> 2. **`EMTMessageAnalyseClientService` 服务的不是"独立审计产品"，而是 Easy-Health 的「巡检分析(Inspection)」菜单**——即审计规则库的输出直接进入巡检报告，与 `product/Script` 的检查项**同级并列**。这解释了为何两套脚本体系并存：`product/Script` 负责"发命令取数 + 简单判定"，EMT 负责"配置合规深度审计"。

### 6.1 `apps/Platform-App/` 的嵌套结构（实测）

```
apps/Platform-App/
├── app.json
└── Platform-App/                ← 同名嵌套目录
    ├── config/
    ├── process.json
    └── start.bat
```

> 其余 4 个 APP 均为扁平结构（`app.json` + 4 properties + 2~3 json + 2 svg + 1~3 bat + 1 yml，实测各 10~12 文件）；只有 `Platform-App` 多一层同名目录与独立 `process.json`（进程编排定义）——因为它负责**拉起其他 APP**。
