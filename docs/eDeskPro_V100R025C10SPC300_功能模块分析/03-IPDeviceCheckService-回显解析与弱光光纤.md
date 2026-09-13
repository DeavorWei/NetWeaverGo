# 03 · `services/IPDeviceCheckService/` 回显解析与弱光光纤

> Java WAR 后端（根包 `com.huawei.ncetools.ipdevicecheckservice`，版本 5.6.134），基于华为 BSP/ROA 框架，REST 前缀 `/rest/ipdevicecheckservice/*`。
> 职责：**命令回显正则解析 + 弱光/光纤检查 + 报表导出**。设备连接（SSH/Netconf）与 Python 采集脚本**不在此 jar 内**（由 NMOT 平台负责，见 `04`）。

## 1. 包/类分层拓扑

| 层 | 关键类（推断自 .class 路径） | 职责 |
|---|---|---|
| 入口/框架 | `task/TaskExecuteMgr`、`filter/EdeskProLicenseFilter`、`db/IPMasterDataSourceConfiguration` | 任务总控 / License(`LICENSE_PERMISSION_TYPE=FiberOpticsCheck`) / 多数据源 |
| 任务/服务 | `task/service/ParseTaskService`、`CheckTaskBatchService`、`ParseDataPrepareService` | 任务批次 CRUD、解析数据准备 |
| **解析引擎** | `parse/CommandParseConfig`、`ParseNode`、`FieldParse`、`SegmentParse`、`RegexModel`、`DefaultNode` | 基于正则 + XPath 的通用命令回显解析器 |
| 数据分类 | `model/category/DataCategory`、`VendorSaver`、`DataCategoryConfig` | 数据分类 + 按厂商路由 Saver |
| **字段算子** | `operator/BaseParseOperator` + `Assign/Filter/MatchAndSet/MergeField/RenameField/ReplaceAll/SplitField/StrConcat/StrExtract/ToUpper/ToLower/ValueMapping/DefaultValue/CustomOperator` | 声明式 ETL 算子 |
| **数据落地** | `parsecfg/datasaver/BaseDataSaver`、`DataSaverFactory` + `base/Interface\|Lldp\|Mac\|Arp\|Hardware` + `hw/h3c/zte/ruijie` 各厂商实现（共 19 个） | 按厂商写入 DB |
| 链路/光纤 | `linkrestoration/LinkParseService`、`BaseLinkParse`、`LldpLinkParse`、`ArpLinkParse`、`MacLinkParse`；`crossconnectedfiber/CrossConnectedFiberMgr` | 由采集数据还原物理链路 + 交叉光纤比对 |
| 弱光/光模块 | `model/lowlight/EnumLowLightCheckRule` 等；`lowlight/report/CheckResultDetailsService`、`OpticalPerformanceService`、`RulePerformanceThresholdService`、`SummaryCheckResultService` | 弱光阈值比对与汇总 |
| 工具/校验 | `util/ExcelParserUtil`、`IfNameConvertUtil`、`InterfaceParseUtil`、`validate/InputDataValidator` | 接口名标准化 / 入参校验 |
| 数据模型 | `model/network/NeElement`、`NeInterface`、`NeIfTransceiver`、`NeLldpNeighbor`、`NeArpTable`、`NeBgpPeer`、`NeMacData`、`NeLicense`、`NeAlarmInfo`、`NeTrunkPort` | 各厂商命令→对象映射 |

## 2. 可读配置

| 文件 | 作用 |
|---|---|
| `config/config.properties` | 全局容量/线程：``PARSE_POOL_SIZE=5``、`NIC_MAX_FILE_COUNT=180000`、`NIC_MAX_UNZIP_SIZE=15G`、`SHEET_MAX_ROW_COUNT_LLD=400000`、`MAX_TASK_NUMS=50`、`LICENSE_PERMISSION_TYPE=FiberOpticsCheck` |
| `web.xml` | REST servlet、`ParamCheckFilter`（content-type 白名单）、Spring 上下文 |
| `parsecfg/modelCmdConfig.xml` | **命令字典**：每种 `NeXxx` 模型 × 厂商 × `taskTypes`(FIBER_CHECK, WEAK_LIGHT_CHECK) → 具体 CLI 命令与输出文件名 |
| `parsecfg/dataCategoryConfig.xml` | 数据分类→厂商→Saver 类路由（hardware/lldp/interface/bgp/arp/mac） |
| `parsecfg/parseitem/*.xml` + `parsecfg/xmlconfig/<vendor>/*.xml` | 每命令的逐行正则解析规则（含 SegmentParse/TableLineParsePolicy/各算子） |
| `lldparseconfig/*.json`（7） | LLD 规划表抽取（`lldSheetName`+`tableStartRow/Col`） |
| `parsecfg/ifnamemapping.json` | 接口名标准化映射 |
| `mapping/*.xml`（8 MyBatis） | `tbl_task_batch`/`tbl_parse_task`/`tbl_optical_check_rule`/`tbl_if_name_mapping` 等 SQL |

## 3. 核心：配置驱动的正则解析三段式

```
命令文本回显（NIC 压缩包内 .txt）
   → parse/ 正则引擎按 xmlconfig 逐行/逐段解析
   → operator/ 字段算子 ETL（ToUpper / ValueMapping / SplitField / MatchAndSet ...）
   → parsecfg/datasaver/* 按 dataCategoryConfig 路由，按厂商落地 DB 表
```

- **解析不是 Python**，完全由 XML 规则驱动 → Go 侧可用「`regexp` + 结构体 tag 描述 xmlconfig 规则」复刻。
- **算子可声明式组合**：`BaseParseOperator` + 14 个标准算子，覆盖绝大多数字段变换；`CustomOperator` 兜底。

## 4. 弱光 / 光纤 / 链路专项

- `lowlight/`：比对 `OpticalCheckRule`（光模块阈值）生成弱光结论；`EnumLowLightCheckRule`/`EnumOpticalModuleInfo` 枚举驱动。
- `linkrestoration/`：由 LLDP/ARP/MAC 采集数据**还原物理链路拓扑**。
- `crossconnectedfiber/`：`CrossFiberCompareUtils` + `CrossFiberExcelUtils` 做**交叉光纤比对**并输出 Excel。

## 5. 数据流（输入→处理→输出）

1. 上游（NMOT 采集）推送 **NIC 压缩包**（命令回显 `.txt`）→ `file/UploadFileUtil` 解压校验（受 `NIC_MAX_*` 约束）。
2. `ParseTaskService`/`ParseDataPrepareService` 建解析任务 → `parse/` 正则引擎解析 → `datasaver/*` 按厂商落地 DB。
3. `linkrestoration/*` 还原链路；`lowlight/report/*` 比对光模块阈值。
4. 输出：`report/DownLoadFileService` + `CrossFiberExcelUtils` 生成 Excel/CSV 报表。

## 6. 移植要点（NetWeaverGo）

- **核心可复刻**：「配置驱动正则解析 + 字段算子 ETL + 按厂商 Saver 落地」三段式。
- Go 实现建议：用 `regexp` + 结构体 tag 描述 `xmlconfig` 规则；DB 用 `sqlx/GORM` 对应 8 张表；调度用 `errgroup`/`semaphore` 替代线程池；设备连接另接 NetWeaverGo 已有协议层。
- **弱光/光纤**是 eDesk 的差异化能力（工程交付场景），值得单独立项搬运阈值模型 `EnumLowLightCheckRule`。
- 本模块**无 Python 调用、无风险命令逻辑**，命令合法性隐含在 `modelCmdConfig.xml` 受控命令字典中。
