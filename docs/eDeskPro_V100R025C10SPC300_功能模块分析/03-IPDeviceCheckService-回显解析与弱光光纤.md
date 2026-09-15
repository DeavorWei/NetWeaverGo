# 03 · `services/IPDeviceCheckService/` 回显解析与弱光光纤（源码级）

> 本文档的源码级结论来自对 `IPDeviceCheckService` WAR 内 `WEB-INF/classes` 的**反编译**（Kali 上 cfr 0.152，966 个 .java 中本服务占 321 个）。方法名/类名可信，局部变量名可能混淆。
> 职责：**命令回显正则解析 + 弱光/光纤检查 + 报表导出**。设备连接（SSH/Netconf）与 Python 采集脚本**不在此服务内**（见 `04`）。

## 1. 结论与移植价值

本服务是 **配置驱动的命令回显解析引擎**：一份 XML 规则 → JAXB 反序列化为解析树 → 对命令回显逐段/逐行正则抽取 → 字段算子 ETL → 按厂商 Saver 落地。这是 eDesk Pro 里**最适合 1:1 移植到 Go** 的模块（纯算法、无协议依赖）。

| 优先级 | 资产 | 移植动作 |
|---|---|---|
| ★★★ | 解析引擎 `CommandParseConfig/SegmentParse/ParseNode/FieldParse/RegexModel/DefaultNode` + 4 种 Policy | 移植为 Go「正则配置树 + 行/段匹配 + 有序算子管线」 |
| ★★★ | 13 个字段算子（ETL） | 直接映射为 Go 的算子接口实现 |
| ★★ | `DataSaver` 厂商路由 + `hw/InterfaceDataSaver` | 改为「属性树 → 结构体 → CSV/DB」 |
| ★★ | `OpticalCheckRule` 弱光阈值 + `LinkParseService` 链路还原 | 单独复用算法 |

## 2. 包/类结构（反编译验证）

```
com/huawei/ncetools/ipdevicecheckservice/
├── model/parse/            CommandParseConfig, SegmentParse, ParseNode, FieldParse, RegexModel, DefaultNode
│   └── policy/             ConfigParsePolicy, TableLineParsePolicy, TableColumnParsePolicy, DynamicColumnParsePolicy
├── operator/               Assign/Filter/MatchAndSet/MergeField/RenameField/ReplaceAll/SplitField/
│                           StrConcat/StrExtract/ToUpper/ToLower/ValueMapping/DefaultValue/CustomOperator
├── parsecfg/               service/(驱动) task/ datasaver/(base/hw/h3c/zte/ruijie/adapter)
├── model/                  modelcmdconfig/ category/ common/ network/ lowlight/ parse/
├── linkrestoration/        LinkParseService, BaseLinkParse, LldpLinkParse, ArpLinkParse, MacLinkParse
├── lowlight/               checkrule/ checktask/ parse/ report/ model/lowlight/(EnumLowLightCheckRule…)
├── crossconnectedfiber/    CrossConnectedFiberMgr, CrossFiberCompareUtils, CrossFiberExcelUtils
├── util/                   ExcelParserUtil, IfNameConvertUtil, InterfaceParseUtil, DispartTool, ParserMsgUtil
└── (intf/cache/enums/mapper/task/ws/parselld/file/report/validate/filter/db)
```

## 3. 解析引擎执行流程（核心）

**加载（一次）**：`CommandParseCache.getCommandParseConfig(vendor, cmd, path)` 用 JAXB（`XMLLoader`/`XmlSerialInput`）把 XML 反序列化为 `CommandParseConfig`。`CommandParseConfig.initConfig()` → `SegmentParse.init()`：若 `ParseNode` 带 `xmlConfigPath`，用 `classloader.getResourceAsStream()` **动态再装载子节点**（嵌套规则）。所有节点 `init()` 预编译正则：`RegexModel.init()` 先折叠 `\r\n + 缩进` 为单行，再 `Pattern.compile(re, 10)`（= `MULTILINE(8) | CASE_INSENSITIVE(2)`，即正则跨行匹配且忽略大小写）。

> **关键可移植性结论**：这些解析规则 XML（`parsecfg/xmlconfig/*`、`parseitem/*`、`modelCmdConfig.xml`）是**引擎无关的纯数据**，与 Java/JAXB 解耦。移植时 XML 规则文件可**原样搬入 NetWeaverGo**，只需用 Go 重写下文引擎（配置树→正则→算子→Saver）即可，规则资产零损耗复用。

**执行（每回显）**：`CommandParseConfig.startParse(model, contents, rootNode, snmp, path)` → 逐段 `executeSegmentParse()`：
1. `SegmentParse` 用自身 `RegexModel` 经 `DispartTool.dispart()/extractContent()` 把整段回显切成若干片段（`extractGroupName` 取命名组；`hasFirst` 控制是否保留首个匹配前内容；`childMutex` 命中一个子节点即停）。
2. 每个片段遍历 `List<ParseNode>`，用 `RegexModel` 做 `matcher.find()`，命中则 `ParseNode.execNodeParse()`。
3. `execNodeParse()` 先按**版本白名单**（`productVersionLst`）过滤，再选第一个非空 `ParsePolicy` 执行 `parseNode()`；否则用 `Filter/MergeField/RenameField/CustomOperator/defaultValues` 算子生成属性并 `model.addNode()`。
4. 若子节点有 `SegmentParse` 且 `needParseSubNode()==true`（无 table 策略），递归 `executeSegmentParse()`；无命中走 `DefaultNode`（固定 name+attributes）。

**行/段应用**：`TableLineParsePolicy.parseNode()` 一个正则 `while(matcher.find())` 逐行匹配，每个 `FieldParse` 按 `fieldNames`（命名捕获组）取 `matcher.group(fn)`，再用字段级 `RegexModel` 校验，然后 `FieldParse.execOperators()` 跑字段算子 → 每行一条结果节点（**逐行提取**）。`TableColumnParsePolicy` 为逐列横向提取，二者不递归子段。

## 3.1 三层配置映射链（源码 + 真实样本）

一条命令回显从「任务模型」到「解析规则文件」经过三层 XML 映射，全部位于 `webapps/ROOT/WEB-INF/classes/parsecfg/`：

```
① modelCmdConfig.xml      ModelCmdConfigRoot → ModelConfig(model=NeInterface, taskTypes=...) 
                                                    → Vendor(HUAWEI) → CommandConfig(cmd, isBig, isRegex)
     └─ 作用：把「数据模型 + 巡检任务类型」绑定到各厂商要执行的命令清单
② parseitem/<name>.xml    CommonParseItem → VendorCommon(vendor) → CommandParse(cmd, path)
     └─ 作用：把命令 cmd 映射到具体解析规则文件 path（classpath 资源）
        · 互斥命令用 @@ 分隔同序双列表：cmd="A@@B" path="a.xml@@b.xml"
        · isResultMutex="true" → 任一命中即停；isRegexCmd="true" → cmd 为正则，CommandPool.sendByRegexsWithCommand 匹配
③ xmlconfig/<vendor>/<cmd>.xml   CommandParseConfig（解析规则本体，见 §3.2）
     └─ 作用：正则分段 + 字段抽取 + 算子 ETL 的真正定义
```

实测 `modelCmdConfig.xml` 节选（华为 `NeInterface` 模型 → 命令）：

```xml
<ModelConfig model="...model.network.NeInterface" taskTypes="FIBER_CHECK,WEAK_LIGHT_CHECK">
  <Vendor vendor="HUAWEI">
    <CommandConfig cmd="display interface"/>
    <CommandConfig cmd="display interface brief"/>
  </Vendor>
  <Vendor vendor="RUIJIE"><CommandConfig cmd="show interfaces"/></Vendor>
</ModelConfig>
```

实测 `parseitem/interface.xml` 节选（命令 → 规则文件，含互斥/正则）：

```xml
<VendorCommon vendor="HUAWEI">
  <CommandParse cmd="display interface" path="parsecfg/xmlconfig/huawei/display interface.xml"/>
  <CommandParse cmd="display optical-module interface \S+" isRegexCmd="true" isResultMutex="true"
                path="parsecfg/xmlconfig/huawei/display optical-module interface.xml@@.../display transceiver diagnosis interface.xml"/>
</VendorCommon>
```

> 模型共 10 个：`NeArpTable / NeIfTransceiver / NeInterface / NeLldpNeighbor / NeBgpPeer / NeLicense / NeElement / NeAlarmInfo / NeMacData / NeTrunkPort`，分布于 `taskTypes=FIBER_CHECK,WEAK_LIGHT_CHECK` 两类任务。

## 3.2 真实解析规则范例：`display interface.xml`

反编译后于 `parsecfg/xmlconfig/huawei/display interface.xml` 取得，结构即 `CommandParseConfig → SegmentParse → (Regex 分段 + List<ParseNode> + DefaultNode)`，每个 `ParseNode` 内含 `ConfigParsePolicy/TableLineParsePolicy` + `Field` 列表 + 算子。节选：

```xml
<CommandParseConfig>
  <SegmentParse>
    <Regex>^ {0,100}([0-9A-Za-z-\|]{1,100}\d{1,10}(/\d{1,10})?...)current[ \r\n]{0,255}state.{0,1000})$</Regex>
    <ParseNode name="neInterfaceNode">
      <ConfigParsePolicy>
        <Field name="interfaceName,ifAdminStatus,ifPhyStatus">
          <Regex><![CDATA[^ {0,100}(?<interfaceName>...)\s*current\s*state\s*:\s*(?<ifAdminStatus>\w{1,255})?...(?<ifPhyStatus>DOWN|UP)]]></Regex>
          <ToUpper name="ifAdminStatus,ifPhyStatus"/>                       <!-- 算子：转大写 -->
          <ValueMapping name="ifAdminStatus" srcValue="ADMINISTRATIVELY" dstValue="Administratively DOWN|Administratively DOWN"/>
          <SplitField srcField="ifAdminStatus" delimiter="|" dstFields="ifAdminStatus,ifPhyStatus"/>
        </Field>
        <Field name="maxBandwidth">
          <Regex><![CDATA[^ *Speed *: *(?<maxBandwidth>\d+)]]></Regex>
          <Assign assignMap="unit:MBPS" filterType="NOT_NULL"/>              <!-- 算子：条件赋值 -->
        </Field>
        ...
      </ConfigParsePolicy>
      <DefaultValue name="ifAdminStatus" value="UP"/>                        <!-- 算子：缺省值 -->
      <SegmentParse extractGroupName="content">                              <!-- 嵌套递归分段 -->
        <Regex><![CDATA[^ *PortName {1,100}Status {1,100}(?<content>[ \S\r\n]{1,4000})]]></Regex>
        <ParseNode name="TrunkIfMember">
          <TableLineParsePolicy>                                           <!-- 逐行表策略 -->
            <Regex><![CDATA[^ {0,100}(?<memberIfName>...)[ \r\n]{0,255}(?<trunkPortState>down|up)...]]></Regex>
            <Field name="memberIfName,weight,trunkPortState"/>
          </TableLineParsePolicy>
        </ParseNode>
      </SegmentParse>
    </ParseNode>
  </SegmentParse>
</CommandParseConfig>
```

要点：
- **分段正则**切出每个接口块；`ParseNode` 内 `ConfigParsePolicy` 用带命名捕获组（`<Field name="...">`）的多个 `<Regex>` 抽取字段。
- **算子直接写在 `<Field>` 内**（`ToUpper`/`ValueMapping`/`SplitField`/`Assign`/`DefaultValue`），与 §5 算子表一一对应，纯数据驱动。
- `SegmentParse` 可**嵌套**（父块内再分段抽子表，如 Trunk 成员），印证 §3 的递归装载。

## 4. 解析策略（4 种 Policy）

| Policy | 行为 |
|---|---|
| `ConfigParsePolicy` | 通用键值/块解析（默认） |
| `TableLineParsePolicy` | 逐行正则，`fieldNames` 命名组 → 每行一条记录 |
| `TableColumnParsePolicy` | 逐列横向提取 |
| `DynamicColumnParsePolicy` | 动态列（列名不固定） |

## 5. 字段算子（13 个 ETL，均实现 `BaseParseOperator.operateFieldValue(fieldsMap, defaultFieldName)`）

| 算子 | 配置字段 | 行为 |
|---|---|---|
| `Assign` | `filterFieldName/filterType/filterValue/assignMap` | 满足条件时把 `assignMap` 写入；值以 `#{x}` 开头可引用其它字段 |
| `Filter` | `filterFieldName/filterType/filterValue` | 条件满足时 `fieldsMap.clear()`（丢弃该记录）；支持 `NOT_ONLY` |
| `MatchAndSet` | `name/value` | 把 `value` 写入 `name` 字段（name 空则写默认字段） |
| `MergeField` | `srcFields/delimiter/dstField` | 多源字段用分隔符拼成 `dstField` |
| `RenameField` | `fieldMap` | 旧名→新名重命名（同步主键） |
| `ReplaceAll` | `name/regex/replacement` | `String.replaceAll` |
| `SplitField` | `srcField/delimiter/dstFields` | 按分隔符拆分源字段到多个目标（长度须匹配） |
| `StrConcat` | `srcField/prefixStr/suffixStr/dstField` | 源字段加前后缀写目标 |
| `StrExtract` | `name/regex/groupId` | 对字段再正则提取第 `groupId` 组，不匹配则移除键 |
| `ToUpper` / `ToLower` | `name` | 字段大小写转换 |
| `ValueMapping` | `name/srcValue/dstValue` | `srcValue→dstValue` 映射（未命中保留原值） |
| `DefaultValue` | `name/value` | `putIfAbsent` 填默认值 |
| `CustomOperator` | `className/methodName` | 反射调用静态方法 `method.invoke(null, fieldsMap)`（任意 Java 逻辑） |

- 执行顺序由 `BaseParseOperator.OPERATOR_ORDER_MAP` 固定（`ToUpper=1 … StrConcat=14`）。
- 比较函数 `VALUE_COMPARE_FUNCTION_MAP`：`EQUALS / NOT_EQUALS / CONTAINS / IN / …`。

## 6. DataSaver（厂商路由 + 落地）

- `DataSaverFactory.getDataSaver(VendorSaver)` 用反射实例化 `VendorSaver.saverClass`（`IDataSaver`）。`VendorSaver` 由 `dataCategoryConfig.xml` 的 `<DataCategory> → <VendorSaver vendor/saverClass/adapterClass>` 定义（如 `hw.InterfaceDataSaver` / `h3c.InterfaceDataSaver` / `zte.InterfaceDataSaver` / `ruijie.InterfaceDataSaver`）。
- `BaseDataSaver` 持有 `Model`（DOM 树），`saveData(list)` 按 `ParseEnvCache` 批处理白名单校验后，超过 50000 条或即时 `CsvUtil.saveLocalCsvData()` 落地 CSV。
- 典型：`hw/InterfaceDataSaver.saveIfTransceiverInfo()` 从 `model.findNodes("/device/ifTransNode"` 和 `"/device/ifOpticalNode")` 取节点属性 → 映射到 `NeIfTransceiver` POJO（含光功率/偏置/温度 lanes，经 `IfNameConvertUtil` 转换接口名）→ `saveData(result)` 写出。`*BaseDataSaver`（接口/硬件/LLDP/ARP/MAC）为各厂商复用基类。

## 7. 弱光/光模块阈值 + 链路还原

**阈值比较**：`OpticalCheckRule` 存厂商级 `rxPowerMax/Min`、`txPowerMax/Min`、`biasMax/Min` 及各自的 `xxxMaxDifferenceRange` 与 `xxxOffsetMode`（`percent`/`absolute`，`EnumOffSetMode`）。`LowLightCalCheckRuleMgr` 是核心：
- `maxOrMinCheckRule()` 判断 `offsetMode`；
- `offSetModeRuleValue()` 在 **PERCENT** 模式下用 `diff = (high-low) * ruleValue/100` 算出阈值上下限；
- 依规则名含 `Max/Min`（`EnumLowLightCheckRule`）决定用**减/加**得到 `checkRuleValue`（`getDiffValue` 的 SUBTRACT/ADD）；
- `maxDiffCheckRule()` 处理最大差值范围。
- `WeakOpticalSignalTaskMgr` / `OpticalModelSignalCheckMgr` 用 `OpticalModuleParse` 把光模块命令回显解析成节点，再与规则比较。

**链路还原**：`LinkParseService.getLldpLink()` 读 CSV（`NeArpTable`/`NeTrunkPort`/`LldDataMgr.getLLDDataByTaskId`）→ `ArpLinkParse.arpLinkParse` / `MacLinkParse.macLinkParse` / `LldpLinkParse.lldpLinkParse` 各自生成 `LinkVo`，`arrangeLinks()` 用 `device_if_remoteDevice_remoteIf` 去重合并得物理链路；`getLLDPParseData()` 再聚合成 `FiberCheckVo`（本地/对端接口映射、IPMI 接口）。

## 8. 与 XML 配置映射（规则驱动来源）

| XML | 反序列化为 | 作用 |
|---|---|---|
| `parsecfg/xmlconfig/<vendor>/*.xml` | `CommandParseConfig>SegmentParse>ParseNode>FieldParse` | 驱动整条解析树（含正则、`ParseNode.xmlConfigPath` 嵌套装载） |
| `modelCmdConfig.xml` | `ModelCmdConfigRoot/ModelConfig/CommandConfig` | 命令清单（`isBig()`、`matchCmd`、`splitRegex`），`BigCmdParseManager.findCommandParse()` 选命令与分割正则 |
| `dataCategoryConfig.xml` | `DataCategoryConfig>DataCategory>VendorSaver/CommonParseItem` | 厂商→`saverClass`/`adapterClass` 及通用解析项 |
| `ifnamemapping.json` | 点对点别名表 | 接口名差异化（`{"mappingIfName":"Twenty-FiveGigE","originalIfName":"25GE","vendor":"H3C"}`） |
| `lldparseconfig/*.json` × 7 | LLDP 解析规则 | 企业网 / 运营商 / IPMaster / 工具链 / 手工兜底（**原文档遗漏，见 §8.2**） |
| `mapping/*.xml` × 8 | MyBatis Mapper | 揭示 8 张业务表（**原文档遗漏，见 §8.3**） |

> 这些 XML 资源虽不在 `.java` 反编译产物内，但**就以资源形式随包发布**，位于 `services/IPDeviceCheckService/webapps/ROOT/WEB-INF/classes/parsecfg/`，可直接取用（§3.1/§3.2 已抽样 `modelCmdConfig.xml` / `parseitem/interface.xml` / `xmlconfig/huawei/display interface.xml`）。`CommandParseCache` 正是用 `classloader.getResourceAsStream(path)` 加载它们，与 §3 引擎一致。

### 8.1 ⚠️ 数量修订与资源全清单（第二轮实测）

| 项 | 原文档 | 实测 |
|---|---|---|
| `parsecfg/xmlconfig/` | 「**50+ 份**，覆盖 HUAWEI/H3C/RUIJIE/ZTE」 | **44 份**：`huawei/15` + `h3c/8` + `ruijie/8` + `zte/13`，另有 `commandparse.dtd` |
| `parseitem/` | 举例 `interface.xml` | **6 份**：`arp.xml`、`bgp.xml`、`hardware.xml`、`interface.xml`、`lldp.xml`、`mac.xml` |
| `com/.../` 类文件 | 「321 个 .java」 | `.class` **347 个**，经 cfr 反编译得 **321 个 .java**（匿名/合成类被内联）——与 Kali `/data/src/ipdevice` 一致 ✅ |
| `templates/` | 未提及 | **17 个 xlsx**（`templates/zh/` 10 个 + `en/`） |
| `i18n/` | 未提及 | `filberOptics_{zh_CN,en_US}.properties`（弱光模块文案） |

**`xmlconfig/<vendor>/` 命令清单（44 份全量）**：

| 厂商 | 命令 |
|---|---|
| `huawei/` | `display version`、`display alarm active`、`display arp`、`display bgp peer`、`display esn`、`display interface`、`display interface brief`、`display interface transceiver verbose`、`display license`、`display lldp neighbor`、`display lldp neighbor brief`、`display mac-address`、`display optical-module interface`、`display transceiver diagnosis interface`、`display transceiver verbose` |
| `h3c/` | `display arp all`、`display interface`、`display lldp neighbor-information list`、`display mac-address`、`display sysname`、`display transceiver diagnosis interface`、`display transceiver interface`、`display version` |
| `ruijie/` | `show arp`、`show interfaces`、`show interfaces brief`、`show interfaces transceiver`、`show lldp neighbors`、`show mac address table`、`show running config include hostname`、`show version` |
| `zte/` | `show arp`、`show hostname`、`show interface`、`show interface brief`、`show lldp entry`、`show mac`、`show mac table`、`show optical-inform brief shelf 0`、`show optical-inform details rx-power shelf 0`、`show optical-inform details tx-power shelf 0`、`show optical-inform shelf 0`、`show opticalinfo`、`show version` |

> **注意厂商命名差异**：华为/H3C 用 `display`，锐捷/ZTE 用 `show`；ZTE 的光模块命令带 `shelf 0` 且**分 rx-power / tx-power 两个命令**（华为是一条 `display transceiver verbose` 全量）。移植时不能假设"同一命令跨厂商"。

### 8.2 ★ `lldparseconfig/` —— LLDP 解析规则外置（原文档遗漏）

```
lldparseconfig/
├── enterpriseruleconfig.json        企业网场景
├── serviceruleconfig.json           运营商场景
├── ipmasterlldruleconfig.json       IPMaster 场景
├── ipmasterlldruleconfig2026.json   IPMaster 2026 版
├── manuallylldruleconfig.json       手工/兜底
├── manuallylldruleconfig2026.json   手工 2026 版
└── toolchainruleconfig.json         工具链场景
```

对应 `com/huawei/ncetools/ipdevicecheckservice/parselld/`（**13 个类**，原文档未列出该包）。

> **关键设计思想**：华为把"**同一份 LLDP 回显在不同客户场景下的解析差异**"（企业网 / 运营商 / IPMaster / 工具链 / 手工）**外置为规则文件**，而不是写死在代码分支里；且带年份版本（`...2026.json`）。
> **对 NetWeaverGo 的意义**：`internal/taskexec/topology_builder.go` 目前把 LLDP 解析**硬编码**，且只有一套规则。这是拓扑模块走向"多客户可配置"的核心范式。

### 8.3 `mapping/*.xml` —— MyBatis Mapper 揭示的 8 张业务表（原文档遗漏）

| Mapper | 业务表语义 |
|---|---|
| `CheckTaskBatchMapper.xml` | 检查任务批次 |
| `TaskBatchProgressMapper.xml` | 批次进度 |
| `ParseTaskMapper.xml` | 解析任务 |
| `ParseDataPrepareMapper.xml` | 解析数据准备 |
| `ProcessMessageMapper.xml` | 处理消息 |
| `IfNameMappingMapper.xml` | 接口名映射（对应 `ifnamemapping.json`） |
| `LldParseTemplateMapper.xml` | **LLDP 解析模板**（对应 §8.2） |
| `OpticalCheckRuleMapper.xml` | **光模块检查规则**（对应 §7） |

> 修正 §7 的隐含假设：弱光阈值（`OpticalCheckRule`）与 LLDP 规则**都是库表驱动**，不是硬编码常量。移植时二者都应落为**可编辑的表 + 前端管理界面**。

### 8.4 `dataCategoryConfig.xml` 实测内容（补充 §6）

6 个 `DataCategory`，各自绑定 `configFileName`（`hardware.xml`/`lldp.xml`/`interface.xml`/`bgp.xml`/`arp.xml`/`mac.xml`，即 §8.1 的 `parseitem/`）与厂商 Saver：

| DataCategory | 厂商 Saver 实现 |
|---|---|
| `hardware` | `HUAWEI→base.HardwareBaseDataSaver + hw.AlarmDataSaver`；`ZTE→zte.HardwareDataSaver`；`H3C→base.HardwareBaseDataSaver`；`RUIJIE→ruijie.HardwareDataSaver` |
| `lldp` | `HUAWEI→base.LldpBaseDataSaver`（+`adapter.hw.LldpDataSaverAdapter`）；`H3C/RUIJIE→base`；`ZTE→zte.LldpDataSaver` |
| `interface` | `hw.InterfaceDataSaver` / `zte.*` / `h3c.*` / `ruijie.*` |
| `bgp` | 仅 `HUAWEI→hw.BgpDataSaver` |
| `arp` | 四厂商共用 `base.ArpDataBaseSaver`（标 `isBig="true"`） |
| `mac` | 四厂商共用 `base.MacBaseDataSaver`（标 `isBig="true"`） |

> **`isBig="true"`** 是关键标记：`arp`/`mac` 为大表数据，走**批量/流式**路径（对应 §6 的"超过 50000 条落地 CSV"）。NetWeaverGo 采集 ARP/MAC 时同样需要区分"小表直读"与"大表流式"。

## 9. 移植要点（NetWeaverGo）

1. 解析引擎：把 `CommandParseConfig/SegmentParse/ParseNode/FieldParse/RegexModel/DefaultNode` + 4 种 Policy + `DispartTool` 移植为 Go「正则配置树 + 行/段匹配 + 有序算子管线」。`RegexModel` 对应 `regexp.Regexp` 预编译；`SegmentParse` 对应分块（`find_pos_by_pattern` 思路）。
2. 算子：`OPERATOR_ORDER_MAP` 固定顺序 → Go 用 `[]Operator` 切片顺序执行；`CustomOperator` 对应 Go 的插件/函数注册表。
3. Saver：`VendorSaver` 厂商路由 → Go 的 `map[Vendor]Saver`；DOM/属性树 → 结构体；落地复用 `CsvUtil`/GORM。
4. 弱光/光纤：`OpticalCheckRule` + `LinkParseService` 算法单独成包，阈值用 `percent`/`absolute` 两种模式计算。
5. **配置即资产、原样复用**：`parsecfg/xmlconfig/<vendor>/*.xml`（**44 份**，覆盖 HUAWEI 15 / H3C 8 / RUIJIE 8 / ZTE 13）、`parseitem/*.xml`（6 份）、`modelCmdConfig.xml`、`dataCategoryConfig.xml` 是引擎无关纯数据，**直接作为 Go 的结构化配置加载**（用 `encoding/xml` + struct tag 映射 `CommandParseConfig/SegmentParse/ParseNode/FieldParse/RegexModel` 即可，无需 JAXB）。规则资产零损耗迁移，NetWeaverGo 只需实现引擎与 4 种 Policy + 13 算子 + 厂商 Saver。
6. **LLDP 解析规则外置（§8.2）**：把 `lldparseconfig/*.json` 的"场景化规则包"思想引入 `topology_builder`，让 LLDP 解析从硬编码变为可配置。
7. **弱光与 LLDP 规则落表（§8.3）**：`OpticalCheckRule` / `LldParseTemplate` 都是库表驱动，移植时应建表 + 前端管理界面，而非写死在 Go 常量里。
8. **大表分流（§8.4）**：`dataCategoryConfig.xml` 的 `isBig="true"`（arp/mac）提示采集需区分"小表直读 / 大表流式落盘"。
