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
├── init/rules/.../sql/   规则库表结构(DDL + 初始化规则)
└── webapps/ROOT/WEB-INF/classes/db/  recreate.sql initTable.sql initRules/*.sql
```

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
4. **结果协议**：`CheckResult(SUCCESS/FAIL/IGNORE/EXCEPTION + detail + suggestion)` 可直接映射为 NetWeaverGo 的检查结论结构。
