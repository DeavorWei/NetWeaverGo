# 华为 eDesk Pro 巡检脚本体系分析

> 分析对象：`D:\ICSLite_download\eDesk_Pro_V100R025C10SPC300\eDeskPro_V100R025C10SPC300-windows-x64\product`
> 版本：eDesk Pro V100R025C10SPC300（windows-x64）
> 目的：理清各目录存放脚本的职责，抽取可复用的处理逻辑，为 NetWeaverGo 的能力迁移提供依据。

---

## 1. 总体结论（先看这个）

这套东西是华为 **eDesk Pro（原 ICSLite / eDesk）设备巡检工具** 的脚本仓库，共 **4148 个 Python 脚本 + 169 个 XML + 30 个 CSV**。它的本质是：

> **一套以 XML 为注册表、以 Python(Jython) 为执行体的"采集项插件框架"**。

四个核心特征：

1. **注册表驱动**：`CollectItem/**/*.xml` 描述「采集项 × 厂商 × 产品款型/版本 → 脚本路径」，是唯一的编排层，没有 Python 侧的 runner/orchestrator。
2. **薄脚本、厚框架**：单个采集脚本只做「发命令 → 正则取值 → 回写结果」，所有公共能力（设备识别、命令下发、分块、CSV 输出、结果判定）都在 `Script/common/`。
3. **产品差异用「分层 + 兜底」消化**：款型/版本 → 脚本路径的精确映射为主，脚本内部再用 `if version.startswith(...)` 二次分叉。
4. **输出是给后端系统吃的**：不是给人看的报告，而是 `CEAS-SG/*.csv`、`data.csv` 这类结构化数据，最终导入华为 IBMS 做 EOX / 维保 / 批次预警核算。

**对 NetWeaverGo 最有价值的三块**（按性价比排序）：

| 优先级 | 能力 | 来源 | 说明 |
|---|---|---|---|
| ★★★ | 声明式解析规则引擎（`ParseRule` 树 + 分块 + 拍平） | `Script/common/CmdEchoParser.py` + `ResultTreeTile.py` | 可直接升级现有 `UserParseTemplate`。约 400 行，能力远超"单正则抽字段" |
| ★★★ | elabel 电子标签解析器（新框架） | `Script/ceas/Common/elabel_parser.py` | 一套正则表覆盖 6 大产品族，是"多产品解析"的最佳范式 |
| ★★ | 设备形态识别（款型/版本/系列/补丁） | `Script/common/device/device.py` + `getversion/` | 20+ 组正则的 `_REG2HANDLER` 表 + 系列归一化算法 |
| ★★ | 槽位/单板筛选（BOM 白名单 → 涉事槽位） | `Script/common/public_utils.py` | 把"批次预警"落到具体槽位的核心逻辑 |
| ★ | 命令下发可靠性 + 回显缓存 + 脱敏 | `common/sender.py`、`ceas/Common/cmd_echo_writer.py` | `-i/-m/-p/-c` 参数体系、Y/N 自动应答、密码脱敏 |

---

## 2. `product/` 目录全景

```
product/
├── CollectItem/    采集项注册表（121 XML）      ← 编排层，"谁在什么设备上跑哪个脚本"
├── Script/         脚本库（4148 py）            ← 执行体
│   ├── common/     框架公共库（25 py）          ← 所有脚本的依赖
│   ├── ceas/       CEAS 电子标签采集（71 py）   ← 硬件清单专项
│   ├── inspector/  巡检脚本（3303 py）          ← 主力历史脚本库
│   ├── combination/ 按任务单的采集脚本（737 py） ← 扁平、单文件、轻量
│   ├── getversion/ 设备形态识别（13 py + 3 xml）← 多厂商款型识别
│   └── fault/      故障信息采集（仅目录，IP/ 为空）
├── Template/       巡检模板（44 XML）           ← "一次巡检包含哪些检查项"
├── Resource/       多语言资源（29 CSV）         ← 检查项名称/描述 中英文
├── config/         运行配置（风险命令、一屏展示）
└── distributeCollectItemMapping.properties     分布式采集映射（3055 行）
```

一句话概括四者的关系：

```
Template（巡检模板：选哪些项）
    │  引用
    ▼
CollectItem（采集项注册表：每项在哪些设备上用哪个脚本）
    │  指向
    ▼
Script（脚本：发命令、解析、回写）
    │  查
    ▼
Resource（资源：检查项名称/描述的多语言文案）
```

---

## 3. 各目录详解

### 3.1 `CollectItem/` —— 采集项注册表（编排层）

按产品线分目录：

| 目录 | 文件数 | 说明 |
|---|---|---|
| `IP/` | 17 xml | 企业网数通（**主力**）：`AR.xml` `CE.xml` `Switch.xml` `Route.xml` `FW.xml` `WLAN.xml` `CEAS.xml` `Fault.xml` `CISCO.xml` `H3C.xml` ... |
| `unetbuilder/` | 73 xml | 网络构建/规划类采集项 |
| `flashEver/` | 13 xml | 长期留存（flash）类采集 |
| `smartnos/` | 11 xml | SmartNOS 智能运维（周期采集 + 服务） |
| `ipmaster/`、`ipcrystal/` | 目录存在，xml 未统计 | 其它产品线 |

`CollectItem/IP/readme.txt` 原文：
> 企业网采集项，当前该目录保留，后续将该目录下所有 xml 分类到巡检、质检、周期采集目录下，该目录不保留

**注册表的 XML 结构**（以 `IP/Route.xml` 为例）：

```xml
<CollectItemMapping name="custom_IP_IPBoardInfo">
  <CollectItemSet name="custom_IP_IPBoardInfo" description="...">
    <CollectItem name="NeMemCheck" Level="3" description="..." lmtNo="202406240011S">
      <Vendor name="Huawei">
        <ProductVersion lmtNo="202406240011S"
                        name="NE20E-2/common,NE20E-4/common,...,SIG9800-X8A/common">
          <Script scriptType="python">
            <Command name="" scriptPath="IP/Huawei/Route/Route/pre_check_memory.py"/>
          </Script>
        </ProductVersion>
      </Vendor>
    </CollectItem>
  </CollectItemSet>
</CollectItemMapping>
```

关键字段：

| 字段 | 含义 |
|---|---|
| `CollectItem name` | 采集项名。命名规律：历史项用业务名（`NeMemCheck`），新增项用 `YYYYMMDD_<lmtNo>_<族>`（如 `20250731_202503270005S_NE`） |
| `lmtNo` | 预警单号 / 需求单号，是华为内部的需求追踪 ID |
| `Level` | 采集级别（1~3），影响执行优先级与权限 |
| `ProductVersion name` | **逗号分隔的「款型/版本」列表**，常见 `/common`（全版本）、`/V200R019C00`（指定版本）、`/V600`（云杉平台大版本） |
| `scriptPath` | 脚本相对 `Script/inspector/` 或 `Script/combination/` 的路径 |
| `Command name` | 也可直接写命令（不写脚本），如 `display ptn-mode` + `viewName="diagnose"` |

> **值得借鉴点**：`ProductVersion` 用「逗号分隔的款型/版本白名单」做精确匹配，比"按产品族 if-else"可维护得多。NetWeaverGo 若要支持多产品，值得引入同款「设备画像 → 脚本/模板」的注册表。

### 3.2 `Script/common/` —— 框架公共库（**价值最高**）

25 个 py，是所有采集脚本的依赖。核心文件：

| 文件 | 大小 | 作用 |
|---|---|---|
| `pre_parse.py` | 23.7 KB | **门面层**。采集脚本 `import pre_parse` 就能用全部能力，内部转调 sender/external_function |
| `sender.py` | — | 命令下发。`send_dc_command` / `_router` / `_genery` / `_ott` / `_ce1800v` / `send_no_record` |
| `external_function.py` | — | 与 Java 宿主交互的唯一出口（`PythonCallback`），含 `send` / `save_result` / `get_language` / `send_super_password` |
| `common_util.py` | — | Tcl 兼容工具：`match` / `compare` / `equal` / `search` / `matchstr` / `escape_xml` |
| `blocklist.py` | — | **按正则分块**（返回 `[块数, 块列表]`） |
| `CmdEchoParser.py` | 8 KB | **声明式解析规则引擎**（见 §4.1） |
| `ResultTreeTile.py` | 3 KB | **树形结果拍平**成表格行（见 §4.1） |
| `public_utils.py` | 58 KB | `Constants`（各产品族 BOM 白名单）+ 槽位/单板筛选工具（见 §4.4） |
| `global_variable.py` | — | 全局状态（中英文、结果码、设备信息…，其实是被 `device/__init__.py` 的 `GlobalVariable` 类逐步替换的遗留） |
| `get_dev_type.py` | — | 单一款型（E600）识别，历史遗留 |
| `device/` | 10 py | **设备形态识别**（见 §4.3） |
| `cr_common/` | 2 py | 路由器芯片/槽位公共函数（`get_chip_list`、`is_NE5000E_ngsf_mode`、`is_BTB`…） |
| `np_common/` | 2 py | NP（网络处理器）采集公共函数 |
| `sstlib_fw.py` | — | 防火墙小工具：款型版本、运行模式、双行合并 |
| `CmdEchoParser.py` 同级还有 `ResultTreeTile.py` | | 这两个由 EasyCoding 生成，注释明确"禁止私自修改" |

### 3.3 `Script/ceas/` —— CEAS 电子标签采集（硬件清单）

分两部分：

- **`ceas/Common/`（15 py）**：新框架 + 工具
  - `elabel_parser.py` —— **核心**，通用 elabel 解析（见 §4.5）
  - `collect_ceas.py` —— `MessagePool`（命令缓存）、`gen_data_rows`（层级 ID）、`parse_version_info`、`convert_series`
  - `esn_parse.py` —— ESN 提取（CE/AR/WLAN 三套策略）
  - `collect_ap_ceas.py` —— AP 电子标签批量采集
  - `cmd_echo_writer.py` —— 大回显流式写 CSV + **密码脱敏**
  - `buildresultnode.py` / `buildtransresultnode.py` —— XML 节点拼装（老框架用）
  - `regbetween.py` / `getregstr.py` / `isincludereg.py` / `replacebyreg.py` —— 正则小工具
  - `checkcmdecho.py` / `getversionecho.py` / `message.py` / `putslog.py` —— 辅助

- **`ceas/CEAS/`（56 py）**：**旧框架**，按产品分目录 `ac/ ar/ ce/ route/ s8700/ sw/ FW_GX/ FW_high/ FW_low/ default/ Common/`
  - 入口 `ceasload.py`：`importlib.import_module(DEV_TYPE.lower())`，找不到就 `default`
  - **现状：只有 `s8700` 一个产品还在用**（`inspector/IP/Huawei/S/common/s8700_ceas.py` 调 `ceasload("s8700")`），其余已被 `Common/elabel_parser.py` 取代
  - 结论：**不要照它重写**

### 3.4 `Script/inspector/` —— 巡检脚本主力库

`readme.txt` 原文：
```
采集项脚本路径，分类划分目录
Distributed：分布式采集项脚本，内部分产业划分目录，如Router，CE等
general：公共采集项脚本，内部分产业划分目录，如Router，CE等
schedule：周期性采集项脚本，内部分产业划分目录，如Router，CE等
statistics：统计项脚本，内部分产业划分目录，如Router，CE等
IP：历史脚本，无特殊情况不新增
```

| 子目录 | 脚本数 | 说明 |
|---|---|---|
| `IP/` | ~3164 | 主力。`IP/<Vendor>/<产品族>/<款型>/<版本>/<脚本>.py` |
| `DC/` | 139 | 数据中心/第三方厂商：ALU、aruba、Cisco、common、custom、Extreme、H3C、hirschmann、huawei、Juniper、moxa、tellabs、tplink、ZTE |
| `Distributed/`、`general/`、`schedule/`、`statistics/` | **0** | 只有 0 字节 readme.txt，是**预留未落地**的目录 |

`IP/` 的组织规律：

- **Vendor**：`Huawei`(3161) / `Cisco`(1) / `H3C`(1) / `ZTE`(1) —— 非华为基本只剩占位
- **产品族**：`Route`(1109) `FW`(673) `S`(625) `CE`(391) `AR`(229) `WLAN`(114) `FLASH_EVER`(18) `Statistics`(1) `common`(1)
- **族内子目录规律**：
  - `common/` —— 全族通用（**CEAS 入口就在这里**：`CE/common/ce_ceas.py`、`S/common/sw_ceas.py`、`AR/common/ar_ceas.py`、`WLAN/common/ac_ceas.py`、`Route/common/route_ceas.py`、`FW/common/fw_ceas.py`）
  - `flash_ever/` —— 长期留存采集（`period_board_*.py`、`period_optical_module_info_*.py`）
  - `smart_nos/` / `Smart_NOS_Service/` —— 智能运维
  - `Yunshan/` / `yunshan/` —— 云杉（V600）平台
  - `<款型>/<版本>/` —— 精确适配，如 `S/S7700/V200R003/`、`AR/AR1000V/V200R008C20/`

**脚本类型与命名**：

| 类型 | 数量 | 入口 | 职责 |
|---|---|---|---|
| `*_collect.py` | 137 | `def collect(dev_id)` | 纯采集：发命令、写文件，**无业务返回值** |
| `*_check.py` | 210 | `def analysis(dev_id)` → `exec_check()` | 检查判定：`set_test_result/problem/advice` 回写，返回 `"结果码,问题,建议"` |
| 其它（`_1451873677694.py` 等数字 ID 命名） | ~2900 | `def collect(dev_id)` | 历史脚本，命名与职责不严格对应 |

### 3.5 `Script/combination/` —— 按任务单的扁平采集脚本

737 个 py，全部在 `pyscripts/` 下，**完全扁平、无子目录、无 `__init__.py`**。

- 命名编码了一切：`[collect_]YYYYMMDD_<lmtNo>_<产品族>.py`
  - `YYYYMMDD` = 生成日期；`<lmtNo>` = 预警单号；产品族尾缀 `_NE`(308) `_CE`(162) `_FW`(69) `_S`(66) `_AR`(50) `_WLAN`(20)
- **不是"组合子脚本"**：全文 `import collect_*` 命中 0 次、`^class` 命中 0 次。所谓"组合"= **在一个脚本里组合多条命令 + 条件分支**
- 结构高度一致：
  ```python
  import re
  import pre_parse
  # 若干纯函数
  def collect(dev_id):
      pre_parse.init_test_info(dev_id)
      pre_parse.send_dc_command("display ...", "-m 3", "user")
  ```
- 头部 docstring 记录 `rule name / lmtNo / collect item / ProductVersion / Level / 命令集合`
- 与 `inspector/` **零 import 关系**，但共用 `pre_parse` 框架；注册表中二者并存

### 3.6 `Script/getversion/` —— 设备形态识别

`Huawei`（默认）+ 12 个第三方厂商各一个 `getversion.py`：
`ALU / Cisco / Extreme / Fortinet / H3c / HillStone / Inspur / Juniper / RedBack / Ruijie / Tellabs / ZTE`

外加 `getVendor.xml`、`getNeighbor.xml`、`getversion.xml`。

统一入口签名 `get_version(dev_id)`，返回 8 元组：
```python
dev_info_arr = [DEV_TYPE, DEV_NAME, DEV_VERSION, PATCH_VERSION, '', '', DEV_VRBD, DEV_DETTYPE]
result = '"' + '","'.join(dev_info_arr) + '"'
```
即：`"款型","设备名","版本","补丁版本","","","VRBD版本","详细款型"`

第三方实现是"**前置解析 + 厂商特判**"模式，例如 Cisco ASA 只在前置解析为空时才介入：
```python
# getversion/Cisco/getversion.py
def parse_asa_device_type(ver_msg, dev_type_list):
    if dev_type_list[0]:        # 前置已解析出设备类型即认为设备不是ASA防火墙
        return
    device_type_result = re.compile(r"^ *Hardware *: *(?P<deviceType>(ASA|FPR)[\S\s]+?),", re.I|re.M).search(ver_msg)
```

### 3.7 `Template/` —— 巡检模板

`Template/inspector/DefaultTemplate/IP/` 下 25 个 xml，每个是一套"巡检场景"：
`custom_IP_InspectionTemplate.xml`（通用）、`custom_CE_InspectionTemplate.xml`、`custom_S_InspectionTemplate.xml`、`custom_WLAN_InspectionTemplate.xml`、`CEASTemplate.xml`、`custom_QInspectionTemplate.xml`（质检）、`custom_Install_assuranceTemplate.xml`（安装保障）、`SASE_Solution_Inspection_Template.xml`（方案级）…

结构（`CEASTemplate.xml`）：
```xml
<Template name="CEASTemplate" domain="IP" ...>
  <Category name="AR router">
    <TemplateItem name="pretreatment" ...>
      <TemplateItem name="20250702_202507010015S_AR" isPreCollect="true" .../>
      <TemplateItem name="PRE_CHECK_CPU_USAGE_AR" ...>
        <threshold name="cpu_usage" DataType="float" defaultValue="70.00"
                   maxValue="100.00" minValue="0.00" rangeType="bound"/>
      </TemplateItem>
    </TemplateItem>
    <TemplateItem name="CEAS" ...>
      <TemplateItem name="CEAS_COLLECT" .../>
    </TemplateItem>
  </Category>
</Template>
```

关键设计：
- `Category` = 产品族；`TemplateItem` 可无限嵌套形成分组树
- **`isPreCollect="true"`** = 前置采集项（结果可被后续项复用，如 CPU/内存预检）
- **`<threshold>`** = 阈值外置，与脚本解耦（`DataType` / `defaultValue` / `maxValue` / `minValue` / `rangeType="bound"`）
- `description` / `deselectDescription` 都是 key，实际文案在 `Resource/*.csv`

> **值得借鉴点**：阈值外置 + 前置采集项（pre-collect）机制。NetWeaverGo 目前阈值若写死在解析逻辑里，可考虑外置。

### 3.8 `Resource/` —— 多语言资源

29 个 csv，与 `CollectItem/IP/*.xml` 一一对应（`AR.csv` `CE.csv` `Route.csv` `Switch.csv` `WLAN.csv` `FW.csv` `Statistics.csv` `fault.csv` `Template.csv` …）。

格式：`key,中文文案,英文文案`
```
SVN2200@V200R001@IpService_Ipv6Forwarding_DESCRIPTION,"操作说明：1. 执行display current-configuration | include ipv6命令...","Operation Instruction: 1. Run the ... command..."
USG5300@V100R002@USG5300_03RunState_02Enviromnt,设备环境,Enviroment State
```

key 的构成：`款型@版本@检查项名` 或 `检查项名_DESCRIPTION`。**`_DESCRIPTION` 后缀 = 操作说明（含排查步骤）**，这是很有价值的运维知识库内容。

`Resource/IP/readme.txt` 原文：
> 周期采集资源文件，按产业划分文件，XXX_NE_zh.properties，XXX_NE_en.properties等，使用properties文件

### 3.9 `config/` 与根目录配置

| 文件 | 作用 |
|---|---|
| `config/riskCmd_product.xml` | **风险命令清单**。按 `TaskType(INSPECTOR,SMARTNOS)` → `Scope(Query/Non-Query)` → `Category(产品族)` → `ProductVersion` → `<Cmd name="..."/>` 组织。例：防火墙禁止 `reset ike sa`、`undo ipsec policy`、`packet-capture all-packet`；WLAN 禁止 `display diagnostic-information` |
| `config/one_screen_display_cfg.properties` | 一屏展示配置 |
| `distributeCollectItemMapping.properties` | 分布式采集项映射（3055 行） |
| `Script/readme.txt` | `collect/common/DC/getversion` 四类目录说明 |

---

## 4. 可复用的核心处理逻辑（**重点**）

### 4.1 ★★★ 声明式解析规则引擎（`CmdEchoParser.py` + `ResultTreeTile.py`）

这是整套体系里**最值得搬**的设计。它把"从回显里抽结构化数据"变成了一份 JSON 配置。

**规则结构**（`ParseRule`，扁平列表 + `parentItem` 形成树）：

| 字段 | 含义 |
|---|---|
| `parseItem` | 字段名 |
| `parentItem` | 父字段名（空 = 根） |
| `isList` | 是否列表（0/1） |
| `itemType` | `string` / `number`（自动转型） |
| `parseRegex` / `parseFlag` / `groupIndex` | 取值正则、标志（`"1,1"`=`re.M\|re.I`）、捕获组序号 |
| `splitRegex` / `splitFlag` | **分块正则**（把一个回显切成多段，每段产一条记录） |
| `index` | 顺序 |

**执行流程**：

```
parseCmdEcho(cmd, cmdEcho, parseRuleList)
  ├─ getParseRuleTree()        扁平规则 → 树（parentItem 关联），并继承父级 regex/flag
  ├─ fillSubResult()           递归填充
  │    ├─ 叶子：getParseValue()  /  getParseValues()（isList）
  │    └─ 非叶子：
  │         ├─ isList=0 → 单个 dict 递归
  │         ├─ isList=1 且无 splitRegex → finditer(parseRegex) 每个 match 一条
  │         └─ isList=1 且有 splitRegex → splitCmdEcho() 按位置切块，每块一条
  ├─ flatResultMap()           level>1 的项在 root 也平铺一份数组（跨层引用用）
  └─ fillDefaultValue()        未命中字段按 itemType 补默认值（""/None/[]）
```

**分块算法**（`splitCmdEcho`）—— 注意它**保留了块头**（用 `match.start()` 而非 `end()`）：
```python
def splitCmdEcho(echo, splitRegex, splitFlag, singleFlag):
    matchList = re.finditer(splitRegex, echo, splitFlag)
    start, end, locationList = 0, None, []
    for match in matchList:
        end = match.start(0)
        locationList.append([start, end])
        start = end
        if singleFlag: break
    locationList.append([start, len(echo)])
    return locationList
```

**拍平算法**（`ResultTreeTile.tileResultTree`）—— 把树形结果展开成表格行（笛卡尔积 + 父子字段继承）：

```python
def getTileResult(resultMap, parseRuleTree, maxOutputLevel, upperResult, resultArr):
    result = extendsUpperResult(upperResult)          # 继承父层字段
    for subParseRule in parseRuleTree["subList"]:
        if subParseRule.get("isOutput") == 1:          # 只输出标记为输出的字段
            setParseItemValueToResult(result, ...)     # 缺失补 ""/[]
    if currentLevel == maxOutputLevel:
        resultArr.append(result); return
    for subParseRule in parseRuleTree["subList"]:
        if subParseRule.get("isPath") == 1:            # 沿 isPath 链下钻
            getNextTileResult(values, subParseRule, maxOutputLevel, result, resultArr)
```

`isPath` 的计算很巧：**从每个输出字段往上回溯，把祖先全部标记为 isPath**，这样递归时只走"能到达输出字段"的分支：
```python
for outputParam in outputParamList:
    parent = parseRuleMap[outputParam].get("parent")
    while parent and parent.get("isPath") != 1:
        parent["isPath"] = 1
        parent = parent.get("parent")
```

**对 NetWeaverGo 的意义**：现有 `models.UserParseTemplate`（`Pattern` + `FieldMapping` + `Aggregation`）是"单正则 → 多字段"的一维模型。引入 `parentItem`/`splitRegex`/`isPath` 三个概念后，就能表达"分块 → 树 → 多行"的二维结构，足以覆盖 90% 的表格型回显（`display interface`、`display device`、`display transceiver`、`display ap all` …），且无需写 Go 代码。

### 4.2 ★★★ 分块工具三件套

同一件事（按正则切回显）在这套代码里有 **4 种实现**，值得统一：

| 实现 | 位置 | 特点 |
|---|---|---|
| `blocklist(rule, data)` | `common/blocklist.py` | 返回 `[块数, [块...]]`；**用 `match.end()+1` 推进，会吞掉块头的最后一个字符**；无匹配返回 `[-1, "Failed to pasre data!"]` |
| `dispart(content, regular)` | `common/public_utils.py` | 返回 `[块...]`；**从后往前切再反转**，保证最后一块完整；**保留块头** |
| `splitCmdEcho` | `common/CmdEchoParser.py` | 返回 `[[start,end]...]` 位置对，**保留块头** |
| `find_pos_by_pattern` | `common/pre_parse.py` | **生成器**，yield `(key, (start, end))`；带 `keep_matched` 开关（是否保留匹配文本）；**末尾补 `(last, (..., None))` 保证最后一块不丢** |

`pre_parse.find_pos_by_pattern` 是设计最完善的一个，推荐作为移植蓝本：
```python
def find_pos_by_pattern(pattern, msg, keep_matched=False):
    span_idx = 0 if keep_matched else 1
    prev_name, prev_name_span = "", (0, 0)
    name_idx = 1 if pattern.groups >= 1 else 0
    for match in pattern.finditer(msg):
        name = match.group(name_idx)
        cur_span = match.span()
        if prev_name:
            yield prev_name, (prev_name_span[span_idx], cur_span[0])
        prev_name, prev_name_span = name, cur_span
    yield prev_name, (prev_name_span[span_idx], None)     # 收尾
```

配套还有：
- `pre_parse.split_message_blocks_by_pattern(blocks, pattern, only_matched, keep_matched)` —— **支持跨块拼接**（大回显分片场景）
- `pre_parse.split_config_blocks(msg_config, pattern=^\s*#\s*$)` —— 按 `#` 切配置段

### 4.3 ★★ 设备形态识别（`common/device/`）

**三层结构**：

```
device/__init__.py   get_device_info(vendor=None) → GlobalVariable 实例
    ├─ _parse_device()    先探测特殊款型：OTT 盒子（HwSKU xxx-W）、CE1800V（vsw show systeminfo，需提权）
    ├─ get_device_parser(vendor)   import_module("." + mod_name) 拿 assemble()
    └─ device.py       get_dev_base_type_n_ver()  华为通用识别（20+ 组正则）
```

`get_dev_base_type_n_ver` 的判定顺序（`device.py:1057`）：

1. 取 4 条命令：`display patch-information`、`return`、`display version`、`display device`
2. **防火墙优先**：`fw_regx` 匹配 `E1000|Eudemon|ET1D2|CE-FW|CE-IPS|NIP|IPS|AntiDDoS|ASG|SVN|USG|SeMG|LE1D2|HiSecEngine Probe` → 用 `display device` 的 `'s Device status` 取款型
3. 其次按 `_REG2HANDLER` **有序表**逐条匹配 `display version`：
   ```python
   _REG2HANDLER = [
       [r"VRouter\S+ +uptime", handle_result14],
       [r"(AntiDDoS)([^\s]*)\s[^\n]*uptime", handle_result18],
       [r"\n(SVN)([^\s]*)\s[^\n]*uptime | ...", handle_result19],
       [r"\n((ET|LE)1D2(IPS|FW)0+S\d+)\s*uptime", handle_result20],
       [r"LE1D2FW00S01\s+uptime", handle_result21],
       [r"\n(CE-(FW|IPS)A)\s+uptime", handle_result23],
       [r"NIP6\d+\s+uptime|...", handle_result26],
       [r"IPS6\d+[A-Z]+\s+uptime|...", handle_result28],
       [r"\nVRP.* Software, Version \d\.\d+, (Release|Feature|RELEASE) \S+", handle_result30],
       [r"(HUAWEI)\s*(SRG)\d{4}\s+uptime", handle_result31],
       [r"\nVRP\s+\(R\)\s+software, Version \d\.\d+\((SRG|AR|NE16EX)\S+\s+", handle_result32],
       [r"...\((ME60)\s+(\S+)\)", handle_result40],
       [r"...\((MultiserviceEngine\s+60\S+)\s+(\S+)\)", handle_result41],
       [r"...\((NE9000\S*)\s+(\S+)\)", handle_result42],
       [r"...\((NE5000\S*)\s+(\S+)\)", handle_result43],
       [r"...\((NetEngine|AdtecRouter|TGDataCom)(8|6)\d00\S*...)", handle_result44],
       [r"...\((NE40E&80E|NE40E|NE80E|CX600|NE20[E]|NE05E|CX66xx...)", handle_result45],
       [r"...\(([A|E]TN\s*\S*\s*\S*)\s+(\S+)\)", handle_result46],   # ATN/ETN
       [r"...\((PTN\s*\S+)\s+(\S+)\)", handle_result47],
       [r"...\((VNE\s*\S+)\s+(\S+)\)", handle_result48],
       [r"(HUAWEI)\s*(SIG)\d+\s+\S+\s+uptime", handle_result49],
   ]
   ```
4. 全不匹配 → `handle_else()` 通用兜底（Quidway / CloudEngine / SWITCH / FutureMatrix / eKitEngine / HUAWEI）
5. `handle_final()` 归一化：NE5000E 多框统一、AC6005-8→AC6005、CE16804/08/16→CE16800、WLAN 的 FIT/CLOUD 形态后缀、AirEngine 去空格

**输出的 6 个关键字段**：`DEV_TYPE`（款型）/ `DEV_DETTYPE`（详细款型）/ `DEV_VERSION`（版本）/ `DEV_VRBD`（VRBD 版本）/ `DEV_NAME`（sysname）/ `PATCH_VERSION`（补丁）

**系列归一化算法**（`collect_ceas.convert_series`）很实用，把款型归到"系列"：
```python
pattern_series = re.compile(r"^(?:(E6)|(AR|AC|AP|AD|R)|(CE|FM|S|AirEngine *))(\d+)")
# 规则：第1组→数字全转0；第2组→保留前1位；数字越长保留越多
offset = len(num) - 4
if offset > 0: index += offset
series_num = num[:index] + "0" * len(num[index:])
# 特例：型号含 "9700D" → 系列加 "D"
```
例：`S5735-S` → `S5700`；`CE6866` → `CE6800`；`AR6280` → `AR6000`。

### 4.4 ★★ 槽位/单板筛选（`common/public_utils.py`）

这是**"批次预警 → 定位到具体槽位"**的核心逻辑，对 NetWeaverGo 做硬件风险分析很有价值。

**数据基础**：`Constants` 类里的 4 个 BOM 白名单集合（BOM 编码 = 华为物料编码）
- `ar_items`（45 个）、`s_items`（~290 个）、`ce_items`（~230 个）、`ne_items`

**核心算法**：
```python
def get_involved_elabel_slots(msg, items):
    """从 elabel 回显里找出 BOM 命中白名单的槽位"""
    block_result_array = dispart(msg, r"^\[(Slot|slot)_\S+\]")     # 按 [Slot_x] 分块
    slot_list = []
    for vercmd in block_result_array:
        result1 = re.compile(r"^\s*Item\s*=\s*(\S+)", re.I|re.M).findall(vercmd)   # 块内所有 Item
        result2 = re.search(r"^\[(Slot|slot)_(\S+)\]", vercmd, re.I|re.M)          # 槽位号
        if result1 and result2:
            for item in result1:
                if item in items:          # Item(BOM) 命中白名单
                    slot_list.append(result2.group(2)); break
    return slot_list

get_slot_item_ar(data) → get_involved_elabel_slots(data, Constants.ar_items)
get_slot_item_s(data)  → ...s_items
get_slot_item_ce(data) → ...ce_items
get_slot_item_ne(data) → ...ne_items
```

**配套工具**：

| 函数 | 作用 |
|---|---|
| `get_slot_list_from_display_device(data)` | 从 `display device` 提取在位的 LPU/NPU/SFU/VSU/BSU/MPU/NSU/SPU/MSU/IPU/CXP 槽位（要求 `Registered\|NA` + `Normal`） |
| `get_slot_id_bytype(data, slottype)` | 按单板类型取槽位：`(\S+)\s+<TYPE>\s+\S+\s+(registered\|NA)\s+Normal` |
| `get_slot_id_by_rule_device(data, rule)` | 传入任意正则，逐行取 group(1) |
| `get_slot_type(data, typelist)` | 从 `display version` 按 `PCB Version` 反查槽位（先按 `uptime is` 分块） |
| `get_pic_type` / `get_pic_type1` | 定位子卡（PIC）所在单板 + 子卡号 |
| `get_slot_list(data)` / `brackets_slots_list` / `get_data` | **槽位列表展开**：`<1,3,5-8>` → `[1,3,5,6,7,8]`（含区间展开、跳过 display 行） |
| `check_packer_zone(data)` | 国内外大包判断（`cfcard:/xxx-OC`、`-OC.cc`） |
| `dispart` / `get_block` | 分块（见 §4.2） |

### 4.5 ★★★ elabel 电子标签解析（新框架）

见前一轮对话的详细分析，这里只列关键资产：

**块头识别总表**（`elabel_parser.py` 的 `handlers`，OrderedDict，**顺序即优先级**）：

| 节点类型 | 正则 | `_level` |
|---|---|---|
| `frame` | `\[BackPlane_(\d+)]` | 1 |
| `fanframe` | `\[\S*((?:FanSlot\|FanFrame\|FAN)_?(\S+))]` | 2 |
| `power` | `\[\S*((?:PowerFrame\|PWR)_?(\S+))]` | 2 |
| `mainboard` | `\[Main_Board[^\d\r\n]*(\d*)]` | 3 |
| `motherboard` | `\[Mother_Board[^\d\r\n]*(\d*)]` | 3 |
| `daughterboard` | `\[(Daughter_Board_[^]]+)]` | 3 |
| `ofccard` | `\[(OfcCard_[^]]+)]` | 3 |
| `port` | `\[(Port_\S+)]` | 3 |
| `card` | `\[Slot_?\d\S*\s*Card_?(?:\S*\d+/)?(\d+)]` | 3 |
| `slot` | `\[(?:Slot_\|Unit_)(\S+)]` | 2 |

三个精巧设计（详见前一轮分析）：`handle_extra_properties` 装饰器（块内多段属性 → 自动拆 daughterboard）、`last_slot` 继承、`Slot_1/3` 归一化。

> **补充：更新的一代实现**。`inspector/IP/Huawei/Route/Route/20260625_202603250004S.py` 里有 `ELabelParserPortBased`，采用**"3 段式 ID"**（`Slot_X`→`X/0`，`Daughter_Board_X/Y`→`X/Y`，最终 `X/Y/Port`），并带 **Item/BarCode 白名单过滤**。这是 2026 年新写的，说明华为自己也在迭代这块，值得跟踪。

### 4.6 ★ 命令下发与回显可靠性

**参数体系**（`external_function.send(cmd, cmd_para, viewname, dev_id)`）：

| 参数 | 含义 |
|---|---|
| `-m N` | 回显保存模式。`-m 1` 不保存；`-m 2` 保存到设备侧；`-m 3` 保存到工具侧（默认）；`-m 4` 用于流式 |
| `-i N` | 超时（秒）。`send_dc_command` 默认补 `-i 60`，`_router` 补 `-i 75` |
| `-p <regex>` | **自定义命令提示符正则**（用于 Linux 虚机/第三方设备，如 CE1800V、OTT） |
| `-c <list>` | 采集前置检查项（如 `-c PRE_CHECK_CPU_USAGE_AR;PRE_CHECK_MEMORY_USAGE_AR`） |
| `viewname` | 视图：`user` / `diagnose` / `system` / `hide` / 空 |

**特殊设备适配**（`sender.py`）：
```python
REG_CE1800V = r"(^\[*[^\n]*~*\]*\s*(\#|\]\$|>)\s*$)"   # CE1800V 是 Linux 虚机
REG_OTT     = r"^[^\n]*[#$]\s?$"                        # OTTBOX
```
这两个设备必须带 `-p` 指定提示符，否则框架收不到回显结束信号。

**回显可靠性**（`collect_ceas.MessagePool`）：
```python
@staticmethod
def send_command_reliably(cmd, param, view):
    max_size = 500
    msg = pre_parse.send_dc_command(cmd, param, view)
    new_param = param.replace("-m 3", "-m 2").replace("-m 1", "-m 2")
    no_need2send = new_param == param
    if "[Y/N]" in msg[-max_size:]:              # 只看尾部 500 字符，性能好
        if not no_need2send:
            msg = pre_parse.send_dc_command(cmd, new_param, view)
        if no_need2send or "[Y/N]" in msg[-max_size:]:
            msg += pre_parse.send_dc_command("Y", new_param, "")
    return msg
```
外加：
- **命令缓存** `MessagePool._pool[view][cmd]`，同一命令不重复下发
- **回显校验** `ceas/Common/checkcmdecho.py`：识别 `password`(1) / 命令不可识别(4) / 设备忙(3) / 正常(0)
- **密码脱敏** `cmd_echo_writer.hide_echo_password`：`password irreversible-cipher .*` → `*`
- **大回显流式写盘** `CMDEchoCsvWriter`：分块写 CSV，避免一次性占内存；并支持 `register(cmd, parse, anomynize)` 在写盘过程中顺带解析/脱敏

### 4.7 结果回写协议

采集/检查脚本通过三个全局函数回写结论（`pre_parse.py`）：

```python
set_test_result("TEST_FAIL")     # 结论
set_test_problem("问题描述")      # 问题
set_test_advice("修复建议")       # 建议
```

结果码映射（`pre_parse.get_test_flag`）：
```python
{"TEST_PASS":0, "TEST_FAIL":-1, "TEST_UNACCORD":5, "TEST_PERIODIC_DISCONN":2,
 "TEST_IGNORE":3, "TEST_EXCEPT":-2, "TEST_MANUAL":4, "TEST_UNTEST":6, "TEST_ERROR":6}
```
默认值 6（未测试）。另有 `manual_test_dlg(msg)` 直接置 4（需人工确认）。

### 4.8 输出规范

**CEAS 三件套**（`CEAS-SG/<pathid>/`）：
- `data.csv` — 硬件树，22 列，层级 ID 形如 `0_1_2`
- `baseinfo.csv` — 设备画像，8 列
- `cmdecho.csv` — 原始回显，4 列（`NE_ID,KEY,COMMAND,MESSAGE`），KEY ∈ `DIS_VER/DIS_ELABEL/DIS_CONF/DIS_ESN`

**CSV 转义**（`cmd_echo_writer`）：仅当值含 `",\r\n` 时才加引号，内部 `"` → `""`。

---

## 5. 对 NetWeaverGo 的落地建议

### 5.1 能力映射表

| eDesk Pro 资产 | 对应 NetWeaverGo 现状 | 建议动作 |
|---|---|---|
| `CollectItem/*.xml` 注册表 | 无（脚本/模板硬绑定） | 新增 `DeviceProfile → 模板集` 的匹配表，键用「款型/版本」白名单 |
| `CmdEchoParser` 规则树 | `models.UserParseTemplate`（单正则） | **扩展**：加 `parentItem` / `splitRegex` / `isPath` / `groupIndex`，升级为二维解析 |
| `ResultTreeTile` 拍平 | `internal/parser/mapper.go` | 对齐"输出字段回溯标 isPath + 笛卡尔展开"算法 |
| `device.py` 形态识别 | 无 / 分散 | 抽 `internal/parser/device/` 包，移植 `_REG2HANDLER` 有序表 + `convert_series` |
| `public_utils.Constants` BOM 白名单 | 无 | 作为数据资产导入（4 个集合约 600 条 BOM），支撑"批次预警→槽位" |
| `elabel_parser.handlers` | 无 | 新建 `internal/parser/ceas/`，正则表转为 Go 的 `[]Handler` |
| `sender` 参数体系 | `internal/executor` / `sshutil` | 补 `-i` 超时默认值、`[Y/N]` 自动应答、提示符正则（`-p`） |
| `riskCmd_product.xml` | 无 | 建 `risk_commands` 表，执行前拦截 |
| `Resource/*.csv` 多语言 | 无 | 可作为检查项描述的种子数据 |

### 5.2 建议的优先级

1. **P0 — 解析引擎升级**（收益最大）：把 `CmdEchoParser` + `ResultTreeTile` 的规则树模型搬进 `internal/parser`，让 `UserParseTemplate` 支持 `parentItem`/`splitRegex`。这一项能让绝大多数表格型回显变成纯配置，无需写代码。
2. **P1 — CEAS 硬件清单**：按前一轮方案落地 `internal/parser/ceas/`，先做 CE/SW/AR/WLAN（新框架），FW/Route 放二期。
3. **P2 — 设备形态识别**：`internal/parser/device/`，先支持华为（CE/S/AR/FW/Route/WLAN），第三方按需。
4. **P3 — 注册表与模板**：引入「设备画像 → 采集项/检查项」的匹配表 + 阈值外置。

---

## 附录 A：关键文件索引

| 能力 | 路径（相对 `product/`） |
|---|---|
| 框架门面 | `Script/common/pre_parse.py` |
| 命令下发 | `Script/common/sender.py` |
| Java 宿主桥接 | `Script/common/external_function.py` |
| Tcl 兼容工具 | `Script/common/common_util.py` |
| 分块（4 种实现） | `Script/common/blocklist.py`、`Script/common/public_utils.py:512`、`Script/common/CmdEchoParser.py:91`、`Script/common/pre_parse.py:780` |
| **解析规则引擎** | `Script/common/CmdEchoParser.py` |
| **结果拍平** | `Script/common/ResultTreeTile.py` |
| 设备形态识别 | `Script/common/device/device.py`、`Script/common/device/__init__.py` |
| 多厂商版本识别 | `Script/getversion/<Vendor>/getversion.py` |
| BOM 白名单 + 槽位筛选 | `Script/common/public_utils.py` |
| **elabel 新框架** | `Script/ceas/Common/elabel_parser.py` |
| CEAS 采集公共逻辑 | `Script/ceas/Common/collect_ceas.py` |
| ESN 提取 | `Script/ceas/Common/esn_parse.py` |
| AP 电子标签 | `Script/ceas/Common/collect_ap_ceas.py` |
| 大回显写盘 + 脱敏 | `Script/ceas/Common/cmd_echo_writer.py` |
| CEAS 旧框架入口 | `Script/ceas/CEAS/ceasload.py`（**仅供 s8700**） |
| 各产品 CEAS 入口 | `Script/inspector/IP/Huawei/{CE,S,AR,WLAN,FW,Route}/common/*_ceas.py` |
| 采集项注册表 | `CollectItem/IP/CEAS.xml`（CEAS 专项）、`CollectItem/IP/Route.xml`（典型全量） |
| 巡检模板 | `Template/inspector/DefaultTemplate/IP/*.xml` |
| 多语言资源 | `Resource/IP/*.csv` |
| 风险命令 | `config/riskCmd_product.xml` |

## 附录 B：术语表

| 术语 | 含义 |
|---|---|
| **CEAS** | 设备电子标签/硬件清单采集，输出给 IBMS 做 EOX/维保核算 |
| **lmtNo** | 华为内部预警单/需求单号，形如 `202503270005S`。尾字母：S=Switch/Service，C=CE，N=NE |
| **EOX** | End of X（停产/停止服务等生命周期状态） |
| **BOM** | 物料编码（如 `02353LGH-016`），elabel 里的 `Item=` 字段 |
| **elabel** | 电子标签，设备/单板的出厂信息（型号、条码、BOM、生产日期等） |
| **BKP** | 背板（Backplane） |
| **PIC / OfcCard** | 子卡 / 光子卡 |
| **VRBD** | 单板/设备的详细版本号（含 SPC 补丁） |
| **DEV_TYPE / DEV_DETTYPE** | 款型 / 详细款型（如 `S5735` / `S5735-L24P4S-A2`） |
| **云杉 / Yunshan / V600** | 华为新一带网络操作系统平台，命令从 `display elabel` 变为 `display device elabel` |
| **LPU / MPU / SFU / SFU / NPU** | 线卡 / 主控 / 交换网板 / 转发芯片 |
| **MDCLI** | 华为新命令行（YANG/JSON 风格），`handle_lite()` 分支处理 |
