发现（按严重级别）

[P1] FDB 交换机弱推断在真实流程中基本不可用
interface_detail 模板能解析 mac/ip，但映射层没有写入 InterfaceFact.MACAddress/IPAddress，导致 ownerByMAC 几乎为空，inferRemoteDeviceByFDBMAC 无法命中。
证据：
mapper.go (line 45)
interface_detail.textfsm (line 1)
builder.go (line 404)
[P1] 规划比对会漏报“同设备对的额外链路”
Compare 中对 plannedByPair 的全量跳过会吞掉同一设备对下多余实际链路。
典型场景：规划 1 条，实际 2 条；第二条不会进 unexpected_link。
证据：
service.go (line 236)
[P2] 设备凭据明文存储并直接返回前端
DeviceAsset.Password 明文入库，ListDevices 直接返回全字段。
证据：
models.go (line 12)
config.go (line 46)
device_service.go (line 24)
[P2] 构建拓扑时吞掉解析错误
解析失败被忽略后继续建图，前端难以感知“数据不完整导致的拓扑偏差”。
证据：
topology_service.go (line 40)
discovery_service.go (line 134)
[P2] 原始采集文件默认 0644，可被同机其他用户读取
含配置与邻居信息的 raw 输出落盘权限偏宽。
证据：
runner.go (line 512)
[P3] 拓扑页目前是“链路表格”，不是方案里的图形拓扑
未接入 cytoscape/dagre 等图组件，缺少图布局与交互展示层。
证据：
Topology.vue (line 107)
package.json (line 1)
[P3] 发现任务状态未实现文档中的分阶段状态机
当前是 pending/running/partial/completed...，没有 collecting/parsing/building 等阶段可观测性。
证据：
discovery.go (line 10)
实施方案完成度（对 docs 的实现判断）

已实现：模块分层（discovery/parser/normalize/topology/plancompare）、DB 模型与迁移、原始输出双写、LLDP 双向/单向建边、聚合逻辑边、服务器弱推断、Excel 导入/比对/导出、前后端页面与 API。
部分实现：拓扑前端展示（有过滤与详情，但不是图谱渲染）、冲突/证据链可见性（有，但缺少解析失败直观反馈）。
未完全实现：文档强调的阶段化任务状态、拓扑图形化展示、FDB 交换机弱推断闭环（受 P1 缺陷影响）。
测试与验证

已执行：go test ./...，全部通过。
但测试缺口明显：未覆盖“interface_detail→MAC/IP 映射→FDB 交换机推断”、未覆盖“同设备对多链路漏报”。

修复方案（按优先级）

先修功能正确性（P1，建议本周完成）

修复 interface_detail 的 MAC/IP 丢失，打通 FDB 弱推断输入。
改动点：
mapper.go (line 45) 在 ToInterfaces 中补齐 row["mac"]、row["ip"] 映射到 InterfaceFact.MACAddress/IPAddress。
验收：
ownerByMAC 不再长期为空；FDB switch inference 能在样例数据产出链路。
测试：
在 mapper_test.go 增加 interface_detail 映射测试；在 integration_test.go 增加“基于接口 MAC 的交换机弱推断”场景。

修复规划比对漏报“同设备对额外链路”。
改动点：
service.go (line 236) 去掉 plannedByPair 的整体跳过逻辑，改为“按实际边逐条消费（used edge ids）”，未消费的都归类 unexpected_link。
验收：
规划 1 条、实际同设备对 2 条时，额外 1 条必须告警。
测试：
在 service_test.go 增加回归用例。

解析错误可见化，避免“静默构图”。
改动点：
discovery_service.go (line 134)、topology_service.go (line 40) 将 ParseTask 错误写入 TopologyBuildResult.Errors，前端提示。
验收：
模板缺失/解析失败时，构图可继续，但 UI 能明确展示失败设备和原因。

再修安全与数据暴露（P2，建议紧跟）

前端接口脱敏，不返回密码。
改动点：
device_service.go (line 24)、query_service.go (line 33) 返回前清空 Password，或新增 DeviceAssetView。
验收：
任何设备列表接口都不含明文密码。

原始采集文件权限收紧。
改动点：
runner.go (line 512) 将目录改 0700，文件改 0600。
验收：
同机普通用户不可直接读取 raw 输出。

凭据加密（可作为第二阶段）。
改动点：
在 config 层加入 encrypt/decrypt（AES-GCM），写库加密、读库解密；提供一次性迁移脚本。
验收：
数据库内不再存储明文密码。

最后补齐体验与实施文档差距（P3）

拓扑页改成图形渲染而非表格。
改动点：
Topology.vue (line 107) 接入 cytoscape + dagre，表格保留为明细视图。
验收：
支持布局、缩放、节点拖拽、按状态着色边。

增加发现阶段状态机。
改动点：
在 discovery.go (line 10) 增加 phase（collecting/parsing/building/...），避免复用终态 status。
验收：
前端可看到当前阶段和阶段耗时。
