# 拓扑回归测试数据集

本目录包含用于拓扑还原模块回归测试的数据集。

## 目录结构

```
testdata/topology/
├── README.md                          # 本文件
├── multi_vendor/                      # 多厂商混合拓扑样例
│   └── mixed_topology.json            # 华为、H3C、Cisco 混合环境
└── anomalies/                         # 异常样例
    ├── name_drift.json                # 名称漂移场景
    ├── single_side_lldp.json          # 单边 LLDP 场景
    ├── dirty_arp.json                 # 脏 ARP 场景
    └── port_flapping.json             # 端口震荡场景
```

## 数据集说明

### 1. 多厂商混合拓扑 (`multi_vendor/mixed_topology.json`)

测试不同厂商设备间的 LLDP 互通性：

- 华为 CE12800 核心
- H3C S12500 汇聚
- Cisco Nexus9000 接入
- 华为 S5700 接入

验证点：

- 不同厂商接口命名差异（Eth-Trunk vs Bridge-Aggregation）
- 双向 LLDP 确认
- 聚合链路映射

### 2. 异常样例 (`anomalies/`)

#### name_drift.json - 名称漂移

设备更名后，邻居设备仍缓存旧名称的场景。

- 验证 chassis_id 匹配优先级
- 验证 IP 匹配兜底机制

#### single_side_lldp.json - 单边 LLDP

只有一端能看到邻居的场景。

- 验证 semi_confirmed 状态判定
- 验证 FDB/ARP 交叉验证

#### dirty_arp.json - 脏 ARP

ARP 表中存在错误或过期映射的场景。

- 验证 IP 冲突检测
- 验证置信度降权机制

#### port_flapping.json - 端口震荡

接口状态频繁变化的场景。

- 验证接口状态一致性检查
- 验证不稳定链路标记

## 数据格式

每个测试数据文件包含以下字段：

```json
{
  "description": "场景描述",
  "scenario": "具体场景说明",
  "devices": [...],           // 设备列表
  "lldpFacts": [...],         // LLDP 事实
  "fdbFacts": [...],          // FDB 事实（可选）
  "arpFacts": [...],          // ARP 事实（可选）
  "interfaceFacts": [...],    // 接口事实（可选）
  "expectedBehavior": {       // 期望行为
    "description": "期望行为描述",
    "expectedEdges": [...]    // 期望生成的边
  }
}
```

## 使用方法

这些数据集可用于：

1. 单元测试的输入数据
2. 集成测试的场景验证
3. 回归测试的基准数据
