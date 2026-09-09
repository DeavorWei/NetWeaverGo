# SNMP 查询模块功能和逻辑

## 1 模块概述

SNMP 查询模块面向**交付现场的一次性采集场景**：设备上架后的上线确认、资产信息（型号/版本/序列号）核对。

模块刻意**不提供**周期性轮询调度、Trap 值守监听和 MIB 库管理 —— 这些属于长期值守型网管系统（Zabbix/Prometheus）的职责，交付工具一旦承担会带来部署负担（固定 IP、162 特权端口长期占用、数据留存策略）却无法替代专业 NMS。

### 1.1 架构位置

```
前端 SNMPQuery.vue
  → frontend/src/services/snmpApi.ts
  → Wails 绑定 internal/ui/snmp_query_service.go (SNMPQueryService)
      → internal/snmp.Querier.Get / Walk / GetDeviceInfo / BatchGetDeviceInfo
          → createSNMPClient（v1/v2c/v3）
          → gosnmp.Get / Walk
      → internal/repository.CredentialRepository（凭据 CRUD，凭据库 snmp.db）
```

### 1.2 职责边界

| 职责 | 归属 |
|------|------|
| 单次 GET / WALK、设备信息采集、批量上线确认、凭据管理 | 本模块 |
| 周期性轮询、趋势图、历史留存 | 不提供（交由专业监控系统） |
| Trap 接收与告警 | 不提供（交由专业监控系统） |
| MIB 文件导入与 OID 树浏览 | 不提供（OID 含义查阅外部文档） |

---

## 2 核心数据结构

### 2.1 后端运行时结构（`internal/snmp/querier.go`）

```go
// Querier SNMP 查询器，无状态、无后台任务
type Querier struct {
    crypto *CredentialCrypto
    config QuerierConfig
}

// QuerierConfig 查询器配置
type QuerierConfig struct {
    Timeout     time.Duration // 单次 SNMP 请求超时（默认 5s）
    Concurrency int           // 批量查询并发数（默认 10）
}

// DeviceInfoOIDs 设备基本信息 OID 集合
var DeviceInfoOIDs = []OIDEntry{
    {OID: "1.3.6.1.2.1.1.1.0", Name: "sysDescr"},
    {OID: "1.3.6.1.2.1.1.3.0", Name: "sysUpTime"},
    {OID: "1.3.6.1.2.1.1.5.0", Name: "sysName"},
}
```

### 2.2 传输类型（`internal/snmp/types.go`）

```go
// SNMPResult 单条查询结果
type SNMPResult struct {
    OID       string `json:"oid"`
    OIDName   string `json:"oidName"`
    Value     string `json:"value"`
    ValueType string `json:"valueType"`
    Error     string `json:"error,omitempty"`
}

// BatchQueryResult 批量查询中单台设备的结果
type BatchQueryResult struct {
    Address   string       `json:"address"`
    Reachable bool         `json:"reachable"`
    Results   []SNMPResult `json:"results,omitempty"`
    Error     string       `json:"error,omitempty"`
    Latency   int64        `json:"latencyMs"`
}
```

### 2.3 持久化模型（`internal/models/snmp.go`）

仅一张表 `snmp_credentials`（存于 `snmp.db`），字段见 `SNMPCredential`。
`Community` / `AuthPassword` / `PrivPassword` 以 AES-256-GCM 加密存储（`ENC1:` 前缀）。

---

## 3 工作流程

### 3.1 单次查询时序

```
用户填写地址/操作/OID/凭据
  → SNMPQueryService.Query(req)
      → resolveCredential()     凭据 ID 优先，其次临时凭据，都没有则为 nil（默认 public）
      → context.WithTimeout(30s)
      → Querier.connect()       getCommunity 解密 → createV1V2Client / createV3Client
      → 按 operation 分派：
            device_info → GetDeviceInfo()
            get         → Get()
            walk        → Walk()
      → pduToResult() 逐条转换（normalizeOID / formatPDUValue / getPDUTypeString）
      → 返回 SNMPQueryResponse
```

### 3.2 设备信息采集的容错策略

`GetDeviceInfo` 逐个采集三个默认 OID：

- 单个 OID 失败不影响其余，失败项仍占据一行但 `Error` 字段标注原因；
- **全部三个都失败**才返回 error，判定设备不可达；
- 这样即使设备未配置 `sysName`，也能拿到 `sysDescr` 用于版本核对。

### 3.3 批量上线确认

`BatchQueryDeviceInfo` 使用 **channel 信号量**而非 worker pool：

- 并发上限取自 `QuerierConfig.Concurrency`（默认 10）；
- 每台设备一个 goroutine，互相隔离，单台超时/失败不影响整体；
- 全流程受 `context` 控制（总超时 10 分钟），取消时未完成项标记「任务已取消」；
- 返回数组与入参地址顺序一致（按 index 回填），前端无需再做映射。

### 3.4 凭据加解密

- 写入时（`CreateCredential` / `UpdateCredential`）：仅当字段非空且尚未加密时才加密，幂等；
- 读取时（`getCommunity` / `decrypt`）：识别 `ENC1:` 前缀后解密，密文为空则回落到默认值 `public`；
- 列表查询**不返回**敏感字段明文；更新时敏感字段留空表示保持原值不变。

---

## 4 UI 服务接口（`internal/ui/snmp_query_service.go`）

| 分类 | 方法 |
|------|------|
| 查询 | `Query(req SNMPQueryRequest)`、`BatchQueryDeviceInfo(req SNMPBatchQueryRequest)`、`GetDeviceInfoOIDs()` |
| 凭据 | `GetCredentials()`、`GetCredential(id)`、`CreateCredential(vm)`、`UpdateCredential(vm)`、`DeleteCredential(id)` |

`operation` 取值：`device_info` / `get` / `walk`。

---

## 5 关键设计权衡

| 设计点 | 取舍理由 |
|--------|----------|
| 返回 `SNMPResult` 而非数据库实体 | 查询结果无需持久化，避免引入数据留存与清理负担 |
| 不依赖 `OIDResolver` | OID 名称由调用方预置（如 `DeviceInfoOIDs`），去掉 MIB 解析依赖 |
| 不依赖 `EventNotifier` | 即时查询同步返回结果，无需 Wails 事件推送 |
| 批量用 channel 信号量 | 场景简单，避免引入 worker pool 的生命周期管理复杂度 |
| 凭据独立 `snmp.db` | 凭据含加密密钥隔离诉求，且与主库业务数据解耦 |

---

## 6 关键文件索引

| 文件 | 职责 |
|------|------|
| `internal/snmp/querier.go` | 查询器：GET/WALK/设备信息/批量采集/客户端构造/结果转换 |
| `internal/snmp/types.go` | 结果类型与加密器结构定义 |
| `internal/snmp/crypto.go` | AES-256-GCM 凭据加解密、密钥文件管理 |
| `internal/models/snmp.go` | `SNMPCredential` 模型 |
| `internal/repository/credential_repository.go` | 凭据数据访问 |
| `internal/config/snmp_db.go` | SNMP 凭据库初始化与迁移 |
| `internal/ui/snmp_query_service.go` | Wails UI 服务层 |
| `frontend/src/views/SNMP/SNMPQuery.vue` | 查询页面（设备查询 / 批量上线确认 / 凭据管理） |
| `frontend/src/services/snmpApi.ts` | 前端 API 封装 |
| `frontend/src/types/snmp.ts` | 前端类型与选项常量 |

---

## 7 安全建议

1. 生产环境优先使用 SNMPv3（`authPriv`），避免 community 明文在网内传输；
2. 确需 v2c 时，不要沿用默认 `public` / `private`；
3. 凭据密文与密钥文件（`snmp_key.bin`）随数据目录一同备份，密钥丢失将导致已存凭据不可解密；
4. 查询目标建议限定在管理网段，避免对业务网设备发起高频 WALK。
