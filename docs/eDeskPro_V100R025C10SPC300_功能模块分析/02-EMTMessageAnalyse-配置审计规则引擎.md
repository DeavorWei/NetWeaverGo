# 02 · `services/EMTMessageAnalyseClientService/` 配置审计规则引擎

> 这是本完整包**最大的可移植资产**（8312 py + 2591 xlsx + 1054 xml + 114 sql）。它是独立的**配置合规审计引擎**，按网络协议功能分类，远超 `product/Script` 的覆盖面。
> 对应服务名：`HCIPCfgAuditService` / `HCIPParserService`。

## 1. 顶层结构

```
EMTMessageAnalyseClientService/
├── app_define.json              服务元信息(menu=/PMIAnalysis)
├── app_data/
│   ├── pdk/                     框架("Python Dev Kit")
│   │   ├── CmdEchoParser.py     声明式解析规则引擎（与 product/Script/common 同源）
│   │   ├── ParserTool.py        dispart(content, regular) 分块工具
│   │   └── NeAssistant/ ServiceAssistant/ NeElementTools/ Logger/ ResultTreeTile
│   └── script/                  审计规则库（按功能 + 版本两级）
│       ├── BASE/ BGP/ CLOCK/ OSPF/ ISIS/ L2/ L3VPN/ Health/ Reliability/
│       ├── RoutePolicy/ QOS/ MPLS/ MPLS_L2VPN/ MPLS_TE/ ControlSecurity/
│       ├── DataSecurity/ ManageSecurity/ V600R008/ V800R006/ V800R010C10/ ...
│       └── EvalEntrance.py      评估调度入口(check)
├── collectscript/               采集/解析脚本（从 CLI 回显抽结构化数据）
│   ├── pythonEntrance/          PythonEntrance.py(运行器) + ThreadPool.py
│   ├── common/                  AbstractParser / ParserTool / CrCommon
│   └── huawei/                  ATN/CX600/ME60/NE40E/NE5000E/NE80E/NE9000/<族>/common|V8R10
├── productExcel/  AR.xlsx CE.xlsx FW.xlsx S.xlsx WLAN.xlsx  规则能力表(二进制)
├── productResource/ AR.csv                               规则中/英文本元数据
├── productXML/ AR.xml                                    设备/命令→脚本映射
├── cfg/  dataCategoryConfig.xml SupportVersion.xml(899KB) devDirCfg.xml(158KB)
│         hwSegmentParser.xml bigCommandConfig.xml enumCfg.ini
├── pyService/  main.exe + *.tcl                          Jython/Java 启动器
├── init/rules/basic/  3885 py + 43 sql + 10 properties   ★ 规则初始化/内置规则数据（原文档误记为"仅 sql"）
├── Offline/Command/   2261 xlsx + offlineCommandConfig.xml ★ 离线命令配置（原文档遗漏）
├── Inspector/DC/      184 zip                            ★ 巡检插件包（原文档遗漏）
├── template/          311 xlsx + 8 docx + 6 pptx         ★ 规则/报告模板（原文档遗漏）
└── webapps/ROOT/WEB-INF/  classes/db/(71 sql 等) + 26 jar + 32 xml
```

### 1.1 实测各子目录规模（第二轮核实）

| 子目录 | 规模 | 说明 |
|---|---|---|
| `app_data/` | **3115 文件**（3105 py + 4 json + 4 txt + 2 xml） | `pdk/`(31 py) + `product/`(62 py) + `script/`(**3007 py**) + `scripttools/` |
| `collectscript/` | **2329 文件**（1322 py + 1007 xml） | `collectitem/`(**1007 xml** 采集项定义) + `huawei/`(1314 py) + `common/` + `pythonEntrance/` |
| `init/rules/basic/` | **3950 文件**（3885 py + 43 sql + 10 properties + 7 json + 3 tcl） | 规则初始化载体 |
| `Offline/` | **2262 文件**（2261 xlsx + `offlineCommandConfig.xml`） | 离线回显分析的命令定义 |
| `template/` | **334 文件**（311 xlsx + 8 docx + 6 pptx + 4 txt + 2 properties） | 规则/交付物模板 |
| `Inspector/` | 184 zip（`Inspector/DC/`） | 巡检插件包 |
| `webapps/` | 177 文件（71 sql + 32 xml + 26 jar + 14 xlsx + 13 properties） | 服务端资源 |
| `pyService/` | 53 文件（51 tcl + 1 exe + 1 ini） | `main.exe` + Tcl 启动脚本 |
| `productExcel/` `productResource/` `productXML/` | 各 1~5 文件 | `{AR,CE,FW,S,WLAN}.xlsx` / `AR.csv` / `AR.xml` |

**py 总数校验**：`app_data 3105` + `collectscript 1322` + `init 3885` = **8312** ✅ 与总量一致。

### 1.2 `app_data/script/` 功能目录真实体量（**原文档严重低估**）

| 目录 | py 数 | 占比 | 说明 |
|---|---|---|---|
| **`Health/`** | **2121** | **70.6%** | 健康类规则（告警/光模块/单板/温度/电源/授权…），文件多为 `analyze_<日期>_<lmtNo>_NE.py` 形式的厂内需求沉淀 |
| `Reliability/` | 346 | 11.5% | 可靠性类（BFD/APS/Trunk/LSP/环网/主备/FRR/SRv6…） |
| `BASE/` | 111 | 3.7% | 基础规则 |
| `OSPF/` | 61 | 2.0% | |
| `BGP/` | 59 | 2.0% | |
| `ISIS/` | 47 | 1.6% | |
| `MPLS_TE/` | 45 | 1.5% | |
| `MPLS_L2VPN/` | 31 | 1.0% | |
| `ManageSecurity/` | 29 | 1.0% | |
| `ControlSecurity/` | 23 | 0.8% | |
| `L3VPN/` | 22 / `MPLS/` 22 / `L2/` 20 / `CLOCK/` 17 / `RoutePolicy/` 15 / `QOS/` 11 | — | |
| 版本目录 `V600R008/` `V800R006/` `V800R009/` `V800R010C10/` `VersionCheck/` `DataSecurity/` `offlineCollectEntrance/` | 少量 | — | 按**版本**分叉的规则 |

> **结论修正**：`Health/` 一个目录就是 2121 条规则，**是"规则库=8312 条"这一结论的主体**。抽样与建模时优先看 `Health/` 与 `Reliability/`，它们代表华为在"设备健康看护"上的知识密度最高区。

### 1.3 `collectscript/` 采集器分布

| 目录 | py 数 | 说明 |
|---|---|---|
| `huawei/common/` | **1019** | 华为主力公共采集器 |
| `huawei/NE40E/` | 71 | |
| `huawei/NE5000E/` | 66 | |
| `huawei/CX600/` | 52 | |
| `huawei/VNE9000/` | 38 | |
| `huawei/ATN/` | 27 | |
| `huawei/ME60/` | 19 | |
| `huawei/{NE20E,NE40E-X2-M8,NE80E,NE9000,PTN}/` | 少量 | |

`collectitem/` 另有 **1007 个 XML**（采集项→命令→脚本的注册表，与 `01` 篇 `product/CollectItem/` 同构但独立）。

### 1.4 `app_data/product/Script/` —— 第二套脚本目录

`app_data/product/` 下同时存在 `CollectItem/` 与 `Script/`（62 py + 4 json + 4 txt），说明 EMT **内嵌了一份精简版 product 脚本子集**，用于离线/评测场景。

## 2. 两类脚本与通用模式

### 2.1 审计规则（`app_data/script/<功能>/rule_*.py`）

```python
def check():
    checkResult = CheckResult()
    msm = modelScriptManager
    for neElement in msm.getNeElements():
        currData = msm.getDataByCmd(neId, "display current-configuration")
        segmentList = ParserTool.dispart(currData, r"^bgp(\s)*(\S)+\s*$")
        bgpReg = re.compile(r"^\s*bgp\s*(\S+)\s*$", re.I|re.M)
        ...
        checkResult.addResult(Constants.FAIL, str(neId), detailResult)
```
- 文件头注释：`#title_cn / #checkno / #risklevel / #suggest_cn`。
- 返回：`[status, detailResult, suggestion]`，`status ∈ {SUCCESS, FAIL, IGNORE, EXCEPTION}`。
- `detailResult` 是 `ArrayList<LinkedHashMap>`（key/value 结构化行）。

### 2.2 采集器（`collectscript/huawei/<族>/*.py`）

```python
def analyze(data):
    aclReg = re.compile(r"^\s*acl(\s+number|\s+name|...)\s+(\S+)...")
    attrs = HashMap(); attrs.put("aclNum", ...); attrs.put("srcIpAddr", ...)
    model.addNode("/device", "aclInfo", attrs)     # 写入结构化模型树
def collect():
    data = cli.send("display current-configuration", 1); analyze(data); return model
```

### 2.3 注入的全局对象（框架桥）

```
cli, model, test, modelScriptManager, neElement, threshold,
networkModel, language, Constants, CheckResult, ParserTool, NeAssistant
```

## 3. `.py` 与 `.xlsx/.xml/.sql` 的关系

| 文件 | 角色 |
|---|---|
| `.py` | 分析/采集逻辑（check/collect） |
| `.xlsx`（AR/CE/FW/S/WLAN） | 规则能力表二进制版，内容与 CSV/SQL 同构 |
| `productResource/AR.csv` | 规则 ID、中/英标题、描述元数据 |
| `productXML/AR.xml` | `<CollectItemMapping>`：产品版本 → CollectItem(lmtNo) → Script → `<Command>` 派发映射 |
| `cfg/hwSegmentParser.xml` | `<ParserConfig>`：key → Java 解析类（interface/ospf/bgp/acl…） |
| `init/rules/.../sql` | 规则库表：`TBL_AUDIT_RULE` 等 |

## 4. 调度入口与派发（由 Java 服务调用）

- **采集运行器** `collectscript/pythonEntrance/PythonEntrance.py::collect()`：遍历 `scriptList` → `__import__` 脚本 → 注入 `cli/model/test` → 调 `module.collect()` → `finally` 中 `saveDB()`。
- **审计运行器** `app_data/script/EvalEntrance.py::check()`：注入 `threshold/networkModel/modelScriptManager/language/projectId/api/rules` → 调 `module.check()` → `CheckResult`。
- **派发目标**：`productXML/AR.xml` + DB 规则表（`TBL_AUDIT_RULE_COMMAND_RELATION`、`TBL_AUDIT_RULE_TYPE_VERSION`）按 `checkno`+产品/版本过滤（`RuleTypeVersionUtil.compareTypeVersion`）。

## 5. 规则库核心数据表

| 表 | 关键列 | 说明 |
|---|---|---|
| `TBL_AUDIT_RULE` | AUDITRULEINDEX, CHECKNO, RULERISKLEVEL, NAME, NAMEEN, CATEGORYINDEX, SUGGESTION, VENDOR, RULETYPE, EXTEND2(脚本相对路径) | 每条检查规则 |
| `TBL_AUDIT_RULE_TYPE_VERSION` | CHECKNO, PRODUCTTYPE, PRODUCTVERSION, INTRODUCINGPATCHVERSION, DEVICETYPE, EXCEPTIONDEVICETYPE | 规则适用产品/版本 |
| `TBL_AUDIT_RULE_COMMAND_RELATION` | RULEID, COMMAND | 命令↔规则 |
| `TBL_AUDIT_RULE_GROUP` | — | 规则分组 |
| `TBL_BASELINE_INFO` | BASELINENAME, DIMENSION, RULECLASSIFICATION, RISKTYPE | 基线/维度 |

## 6. 移植要点（NetWeaverGo）

1. **规则库即知识资产**：8312 条 `check()` 是最该搬运的内容。建议先按 `功能目录`（BGP/OSPF/QOS/MPLS/...）抽样 50~100 条，抽象为「规则模板 = 取数命令 + 分块正则 + 判定表达式 + 建议文案」。
2. **框架可复用**：`CmdEchoParser` 规则树与 `product/Script/common` 同源，统一升级到 NetWeaverGo 的二维解析模型即可。
3. **派发表可转配置**：`TBL_AUDIT_RULE_TYPE_VERSION` 即「设备画像 → 规则集」注册表，对应 NetWeaverGo 的「设备画像 → 检查项」匹配。
4. **结果协议**：`CheckResult(SUCCESS/FAIL/IGNORE/EXCEPTION + detail + suggestion)` 可直接映射为 NetWeaverGo 的检查结论结构。NetWeaverGo 的 `internal/inspection` 已定义 **8 种结论码 + 4 级严重度**，可直接承接：`SUCCESS→TEST_PASS`、`FAIL→TEST_FAIL`、`IGNORE→TEST_IGNORE`、`EXCEPTION→TEST_EXCEPT`，`detail` 行 → `EvidenceJSON`，`suggestion` → `Advice`。
5. **抽样顺序修正**：不要按 `BGP/OSPF/QOS` 顺序抽样（这些目录各仅 11~61 条）。**应从 `Health/`（2121 条）与 `Reliability/`（346 条）入手**——它们才是规则主体，也最贴近 NetWeaverGo 的"设备健康巡检"定位。
6. **`Offline/Command/`（2261 xlsx）是被忽略的第三类资产**：它定义了"离线回显分析"所需的命令集合，正好对应 NetWeaverGo 的 **离线重放**（`internal/taskexec/replay_executor.go`）。建议按产品族抽样，作为"无网环境导入回显 → 重新判定"的模板来源。
7. **`template/`（311 xlsx + 8 docx + 6 pptx）** 是交付物模板（报告/PPT/Word），可作为 NetWeaverGo 报告导出（当前仅 CSV/JSON/HTML）的**格式参考**。
8. **`init/rules/basic/`（3885 py）** 体量与规则库同级，是"规则初始化/内置规则数据"载体，移植时需判断它是**规则副本**还是**离线评测基线**，避免与 `app_data/script/` 重复搬运。
