# 01 · `product/` 采集脚本体系

> 本目录是旧分析 `docs/华为eDeskPro脚本体系分析.md` 的主体，此处只做**移植视角的精炼**，细节请回看旧文档。
> 本包实际内容：`product/` 共 4513 文件 = 4148 py + 169 xml + 149 json + 30 csv + 9 txt（相比旧文档多了 149 个 json，多为运行态产物/新版注册表）。

## 1. 目录职责

```
product/
├── CollectItem/   采集项注册表（XML）  ← 编排层："谁在什么设备上跑哪个脚本"
├── Script/        脚本库（4148 py）
│   ├── common/     框架公共库（pre_parse / sender / CmdEchoParser / device …）
│   ├── ceas/       CEAS 电子标签采集（elabel_parser 新框架）
│   ├── inspector/  巡检脚本主力库（~3164，IP/<Vendor>/<族>/<款型>/<版本>/）
│   ├── combination/ 按任务单的扁平采集脚本（737，命名含 lmtNo）
│   └── getversion/  设备形态识别（多厂商 getversion.py）
├── Template/      巡检模板（IP/*.xml：选哪些检查项、阈值外置）
├── Resource/      多语言文案 CSV（key,中文,英文；_DESCRIPTION=操作说明）
└── config/        riskCmd_product.xml（风险命令清单）、一屏展示
```

## 2. 四者关系（注册表驱动）

```
Template（巡检模板：选哪些项）
   → CollectItem（注册表：每项在哪些设备用哪个脚本，ProductVersion=逗号分隔款型/版本白名单）
      → Script（脚本：发命令、解析、回写）
         → Resource（检查项名称/描述多语言文案）
```

## 3. 对 NetWeaverGo 最有价值的三块（来自旧文档结论）

| 优先级 | 能力 | 来源 | 落地建议 |
|---|---|---|---|
| ★★★ | 声明式解析规则引擎（`ParseRule` 树 + 分块 + 拍平） | `Script/common/CmdEchoParser.py` + `ResultTreeTile.py` | 升级现有 `UserParseTemplate` 为支持 `parentItem`/`splitRegex`/`isPath` 的二维模型 |
| ★★★ | elabel 电子标签解析器 | `Script/ceas/Common/elabel_parser.py` | 一套正则表覆盖 6 大产品族，是多产品解析范式 |
| ★★ | 设备形态识别（款型/版本/系列/补丁） | `Script/common/device/device.py` + `getversion/` | `_REG2HANDLER` 有序表 + `convert_series` 系列归一化 |

## 4. 注册表 XML 关键字段（可借鉴的"设备画像→脚本"匹配）

```xml
<CollectItem name="NeMemCheck" Level="3" lmtNo="202406240011S">
  <Vendor name="Huawei">
    <ProductVersion name="NE20E-2/common,NE20E-4/common">
      <Script scriptType="python">
        <Command name="" scriptPath="IP/Huawei/Route/Route/pre_check_memory.py"/>
```

- `ProductVersion.name`：逗号分隔「款型/版本」白名单，`/common`=全版本，`/V600`=云杉大版本。
- `lmtNo`：华为内部预警/需求单号（运维追踪 ID）。
- **借鉴**：用「设备画像 → 采集项/检查项」注册表替代硬绑定。

## 5. 脚本类型与入口

| 类型 | 入口 | 职责 |
|---|---|---|
| `*_collect.py` | `collect(dev_id)` | 纯采集：发命令、写文件 |
| `*_check.py` | `analysis(dev_id)`→`exec_check()` | 检查判定：回写 `set_test_result/problem/advice` |
| `combination/*` | `collect(dev_id)` | 命名含 `YYYYMMDD_<lmtNo>_<族>`，单文件组合多条命令 |

## 6. 结果回写协议（脚本侧）

```python
set_test_result("TEST_FAIL")   # 结论
set_test_problem("问题描述")    # 问题
set_test_advice("修复建议")     # 建议
```
结果码：`TEST_PASS=0, TEST_FAIL=-1, TEST_UNACCORD=5, TEST_IGNORE=3, TEST_EXCEPT=-2, TEST_MANUAL=4, TEST_UNTEST=6`。

## 7. 与 `services/EMT*` 的关系

`product/Script` 与 `services/EMTMessageAnalyseClientService` **是两套独立脚本体系**（见 `02`）。`product/Script` 由 `CollectItem` 注册表派发、偏"发命令取数"；EMT 由 `app_data/pdk` 框架 + `TBL_AUDIT_RULE` 规则表派发、偏"配置合规审计"。迁移时建议**两条线都收**，但 EMT 的规则库分类更系统、知识密度更高。

> 注意：本包 `product/` 下出现 149 个 `*.json`，多为运行态生成的采集项/结果元数据，非源码，迁移时以 `Script/` + `CollectItem/` 为主。
