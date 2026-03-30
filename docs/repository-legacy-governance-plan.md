# 仓库级兼容与 legacy 治理实施方案

> 目标：在不保留无效兼容层思维的前提下，对主仓业务源码进行一次架构级收口，删除旧接口、旧配置入口、旧 UI 适配层与废弃 API；同时明确保留业务上必须存在的 SSH 老旧设备兼容能力与配置入口，确保仓库只保留一套主语义、一套主入口、一套主配置。
>
> 范围约束：排除第三方与生成物，不处理 `frontend/dist`、`node_modules`、Wails 生成 bindings。

---

## 1. 治理目标

本次治理不是局部打补丁，而是围绕以下四个结果进行收口：

1. **唯一接口入口**：删除只为旧调用路径保留的接口别名与旧服务方法。
2. **唯一配置来源**：删除执行链中无真实业务价值的兼容回退配置，避免双轨配置并存。
3. **唯一执行语义**：前端执行视图直接消费运行时 `run/stage/unit/snapshot` 模型，不再通过旧设备视图二次翻译。
4. **唯一能力边界**：保留业务上必须存在的 SSH 老旧设备兼容能力与配置入口，但清理无业务价值的兼容包装语义与无效回退链路。

---

## 2. 当前已识别的治理对象

### 2.1 接口别名与旧入口

- [`SSHAlgorithmSettings`](../internal/config/settings.go:13)：`config` 层对 [`models.SSHAlgorithmSettings`](../internal/models/models.go:97) 的别名。
- [`GlobalSettings`](../internal/config/settings.go:16)：`config` 层对 [`models.GlobalSettings`](../internal/models/models.go:113) 的别名。
- [`GetCommands()`](../internal/ui/command_group_service.go:68)：旧命令文本入口。
- [`SaveCommands()`](../internal/ui/command_group_service.go:73)：旧命令文本保存入口。
- [`CommandGroupAPI.getCommands`](../frontend/src/services/api.ts:84)、[`CommandGroupAPI.saveCommands`](../frontend/src/services/api.ts:86)：前端仍暴露旧接口映射。
- [`PlannedLink`](../internal/plancompare/models.go:9)：兼容性类型重导出，需要复核仓内真实使用情况。

### 2.2 配置回退与双轨配置

- [`resolveWorkerCount()`](../internal/config/runtime_config.go:435)：仍允许执行链从 [`GlobalSettings.MaxWorkers`](../internal/models/models.go:115) 回退。
- [`ResolveEngineWorkerCount()`](../internal/config/runtime_config.go:472)：文档语义仍包含“兼容兜底”。
- [`Settings.vue`](../frontend/src/views/Settings.vue:32)：全局设置页仍保留“最大并发数”入口。
- [`RuntimeConfigPanel.vue`](../frontend/src/components/settings/RuntimeConfigPanel.vue:20)：文案仍说明全局并发仅作为兼容回退值。
- [`fallbackEventCapacity`](../internal/config/runtime_config.go:45)：已确认无独立业务消费，已从运行时配置结构、默认值、持久化键、DTO 与前端表单全链路删除。
- [`RuntimeConfigData`](../internal/ui/settings_service.go:188)：已与运行时配置字段同步收口（移除 `fallbackEventCapacity`）。

### 2.3 前端旧执行视图适配层

- [`DeviceViewState`](../frontend/src/services/api.ts:228)：旧设备视图类型。
- [`execDevices`](../frontend/src/views/TaskExecution.vue:492)：把 [`UnitSnapshot`](../frontend/src/views/TaskExecution.vue:512) 翻译成旧设备视图。
- [`mapUnitStatusToDeviceStatus()`](../frontend/src/views/TaskExecution.vue:517)：旧状态映射适配。

### 2.4 废弃 API 与残留调用

- [`WriteChunk()`](../internal/report/detail_logger.go:116)：已删除。
- [`WriteNormalizedText()`](../internal/report/detail_logger.go:74)、[`WriteNormalizedLines()`](../internal/report/detail_logger.go:94)：已成为唯一写入入口。

### 2.5 SSH 老旧设备兼容能力与配置入口（保留项）

- [`CompatiblePreset`](../internal/sshutil/presets.go:69)：承载老旧设备所需算法集合，当前确认属于业务保留项。
- [`GetPreset()`](../internal/sshutil/presets.go:144)：仍支持 `compatible`，当前确认保留该入口。
- [`Settings.vue`](../frontend/src/views/Settings.vue:196)：设置页暴露“兼容模式”，当前确认保留该配置入口。
- [`logSSHHandshakeError()`](../internal/sshutil/client.go:155)：错误提示中涉及兼容模式引导，需要复核文案是否准确，但不作为删除目标。
- [`applyAlgorithmConfig()`](../internal/sshutil/client.go:262)：需要复核默认算法策略与显式配置优先级，确保保留的是业务兼容能力，而不是无边界放宽。

---

## 3. 治理原则

### 3.1 删除原则

以下内容应直接删除，而不是继续保留注释说明：

- 仅用于兼容旧调用路径的接口、别名、前端 API 映射。
- 仅用于双轨兜底的配置项和解析逻辑。
- 仅用于把新执行模型伪装成旧执行模型的前端适配层。
- 已有替代能力且调用方可迁移的废弃 API。
- 无真实业务价值、仅增加认知负担的兼容包装语义与回退文案。

### 3.2 保留原则

以下内容不因“名字旧”而误删：

- 仍真实参与任务编排的任务级并发、超时等业务字段。
- 当前执行页面中直接消费新快照模型的逻辑。
- 老旧设备实际需要的 SSH 加密算法、密钥交换算法、MAC 与主机密钥算法能力，以及对应配置入口。
- 第三方依赖、构建输出、生成 bindings。

### 3.3 实施原则

- 必须**先改调用方，再删旧入口**。
- 必须**前后端一起收口**，不能只删 UI 或只删后端字段。
- 必须**删除配置读写链全链路**，包括默认值、结构体、持久化键、接口 DTO、前端表单。
- 必须**补测试与全链路验证**，避免删完后留隐式 fallback。

---

## 4. 分阶段实施方案

## 阶段 A：接口与类型别名收口

### 目标

删除仓库中仅为兼容旧调用路径保留的接口别名与旧服务方法。

### 动作

1. 将 `config` 层所有 [`GlobalSettings`](../internal/config/settings.go:16) 与 [`SSHAlgorithmSettings`](../internal/config/settings.go:13) 引用替换为 [`models.GlobalSettings`](../internal/models/models.go:113) 与 [`models.SSHAlgorithmSettings`](../internal/models/models.go:97)。
2. 删除 [`GetCommands()`](../internal/ui/command_group_service.go:68) 与 [`SaveCommands()`](../internal/ui/command_group_service.go:73)。
3. 删除 [`CommandGroupAPI.getCommands`](../frontend/src/services/api.ts:84) 与 [`CommandGroupAPI.saveCommands`](../frontend/src/services/api.ts:86)。
4. 改造 [`CommandEditor.vue`](../frontend/src/components/task/CommandEditor.vue:171) 与 [`CommandEditor.vue`](../frontend/src/components/task/CommandEditor.vue:215)，改为基于命令组模型工作，而不是旧文本接口。
5. 盘点 [`PlannedLink`](../internal/plancompare/models.go:9) 一类兼容重导出；若仓内无必要使用，直接删除。

### 验收标准

- 仓库业务源码中不再通过 `config` 别名访问核心设置模型。
- 前端不再调用旧命令文本接口。
- 旧入口删除后编译通过。

---

## 阶段 B：配置回退与双轨入口收口

### 目标

让执行链配置只认一套来源，彻底移除兼容回退逻辑。

### 动作

1. 重构 [`resolveWorkerCount()`](../internal/config/runtime_config.go:435) 与 [`ResolveEngineWorkerCount()`](../internal/config/runtime_config.go:472)，移除对 [`GlobalSettings.MaxWorkers`](../internal/models/models.go:115) 的兜底依赖。
2. 评估 [`Settings.vue`](../frontend/src/views/Settings.vue:32) 中“最大并发数”是否仍承担其他业务含义：
   - 如果仅为执行链兜底，则删除。
   - 如果仍服务于非执行链，则需重新命名与重新归属，避免误导为执行链主配置。
3. 收口 [`RuntimeConfigPanel.vue`](../frontend/src/components/settings/RuntimeConfigPanel.vue:20) 文案，删除兼容回退表述。
4. 盘点 [`fallbackEventCapacity`](../internal/config/runtime_config.go:45) 是否真实参与业务：
   - 若无真实消费，则删除字段、默认值、数据库键、DTO 映射、前端表单项。
   - 若仍有真实消费，则重命名为业务语义字段，删除 `fallback` 命名。
5. 同步更新 [`RuntimeConfigData`](../internal/ui/settings_service.go:188) 与对应前端表单结构。

### 验收标准

- 执行链工作协程数只由运行时配置决定。
- 不再存在“全局设置兜底运行时配置”的双轨语义。
- 所有被删除配置项的前后端字段、持久化键、文案同步移除。

---

## 阶段 C：前端旧执行视图适配层删除

### 目标

前端执行页直接消费统一运行时快照，不再通过旧设备模型中转。

### 动作

1. 删除 [`DeviceViewState`](../frontend/src/services/api.ts:228)。
2. 删除 [`execDevices`](../frontend/src/views/TaskExecution.vue:492) 与 [`mapUnitStatusToDeviceStatus()`](../frontend/src/views/TaskExecution.vue:517)。
3. 将执行页设备列表、日志面板、状态展示直接绑定到 [`UnitSnapshot`](../frontend/src/views/TaskExecution.vue:512) 与 [`StageSnapshot`](../frontend/src/views/TaskExecution.vue:508)。
4. 清理执行页中“兼容旧 UI”注释与由旧视图模型驱动的过滤逻辑。

### 验收标准

- 执行页面不再依赖旧设备视图类型。
- 单位状态、日志、进度全部直接来源于运行时快照模型。

---

## 阶段 D：废弃日志 API 删除

### 目标

删除废弃但仍在使用的日志写入接口，统一到规范化文本写入链路。

### 动作

1. 将 [`WriteDetailChunk()`](../internal/report/log_storage.go:56) 调整为调用 [`WriteNormalizedText()`](../internal/report/detail_logger.go:74) 或 [`WriteNormalizedLines()`](../internal/report/detail_logger.go:94)。
2. 复核所有 [`WriteChunk()`](../internal/report/detail_logger.go:116) 调用点与测试。
3. 删除 [`WriteChunk()`](../internal/report/detail_logger.go:116) 及仅为其存在的测试逻辑。

### 验收标准

- 仓库中无业务调用 [`WriteChunk()`](../internal/report/detail_logger.go:116)。
- 日志写入链统一到规范化接口。

---

## 阶段 E：SSH 兼容能力边界收敛

> 本阶段已确认：**保留老旧设备所需的 SSH 兼容算法能力与配置入口，不做删除。**

### 目标

保留业务上必须存在的老旧设备兼容能力，同时收敛 SSH 配置的边界表达、优先级和错误提示，避免把“业务兼容能力”误治理为“历史残留”。

### 动作

1. 保留 [`CompatiblePreset`](../internal/sshutil/presets.go:69)、[`GetPreset()`](../internal/sshutil/presets.go:144) 与 [`Settings.vue`](../frontend/src/views/Settings.vue:196) 中的兼容模式入口。
2. 复核 [`applyAlgorithmConfig()`](../internal/sshutil/client.go:262) 的默认算法装配逻辑，确认显式配置优先级、默认策略边界和日志说明一致。
3. 复核 [`logSSHHandshakeError()`](../internal/sshutil/client.go:155) 中的建议文案，确保它表达的是业务兼容能力，而不是含混的“旧架构兼容层”。
4. 在文档中把 SSH 部分明确标注为“业务保留项”，与应删除的仓库级 legacy 残留区分开。

### 验收标准

- SSH 老旧设备兼容能力与配置入口保持可用。
- SSH 相关代码与文档不再被误判为应删除的 legacy 残留。
- 默认算法优先级、配置入口与错误提示语义一致。

---

## 5. 验证与回归要求

实施完成后，需要至少执行以下验证：

1. Go 侧受影响包单测回归：`internal/config`、`internal/ui`、`internal/report`、`internal/sshutil`、`internal/taskexec`。
2. 前端编译与类型检查回归，确认删除 API 后无遗留调用。
3. 根目录 [`build.bat`](../build.bat) 全链路构建验证。
4. 更新 [`docs/execution-event-pipeline-residual-issues.md`](./execution-event-pipeline-residual-issues.md)，补充“仓库级兼容治理”状态。

---

## 6. 风险与控制点

### 风险 1：误删仍有业务价值的配置

控制方式：在删除 [`maxWorkers`](../frontend/src/views/Settings.vue:32) 或 [`fallbackEventCapacity`](../internal/config/runtime_config.go:45) 前，先确认真实消费点与业务归属。

### 风险 2：前端执行页删适配层后出现展示断裂

控制方式：先补基于 [`UnitSnapshot`](../frontend/src/views/TaskExecution.vue:512) 的展示逻辑，再删除 [`execDevices`](../frontend/src/views/TaskExecution.vue:492)。

### 风险 3：误将 SSH 业务兼容能力当作 legacy 残留删除

控制方式：已确认 SSH 老旧设备兼容算法能力与配置入口属于业务保留项；治理时仅收敛边界表达、文档与默认策略说明，不删除底层能力。

---

## 7. 已确认决策与待核实事项

### 已确认决策

1. **SSH 范围**：保留 [`CompatiblePreset`](../internal/sshutil/presets.go:69) 代表的老旧设备兼容算法能力，以及 [`Settings.vue`](../frontend/src/views/Settings.vue:196) 中的配置入口。
2. **全局最大并发数**：删除 [`Settings.vue`](../frontend/src/views/Settings.vue:32) 中的 `maxWorkers` 全局入口，执行链只保留运行时配置中的 [`workerCount`](../internal/config/runtime_config.go:43)。

### 待核实事项

3. **事件后备容量字段**：已核实无独立业务消费，已删除 [`fallbackEventCapacity`](../internal/config/runtime_config.go:45) 及其前后端配套字段。

---

## 8. 建议执行顺序

建议按以下顺序落地，降低回归风险：

1. 阶段 A：接口与类型别名收口
2. 阶段 B：配置回退与双轨入口收口
3. 阶段 C：前端旧执行视图适配层删除
4. 阶段 D：废弃日志 API 删除
5. 阶段 E：SSH 兼容能力边界收敛
6. 核实 [`fallbackEventCapacity`](../internal/config/runtime_config.go:45) 真实消费链
7. 测试、构建、文档回写

---

## 9. 当前状态（已落地）

1. 已完成 [`Settings.vue`](../frontend/src/views/Settings.vue:32) `maxWorkers` 入口删除与执行链兜底依赖清理。
2. 已完成 [`fallbackEventCapacity`](../internal/config/runtime_config.go:45) 全链路删除（配置结构/DB 键/DTO/UI）。
3. 已完成废弃日志 API 清理：[`WriteDetailChunk()`](../internal/report/log_storage.go:56) 已切换到 [`WriteNormalizedText()`](../internal/report/detail_logger.go:74)，[`WriteChunk()`](../internal/report/detail_logger.go:116) 与对应测试已删除。
4. SSH 兼容能力与配置入口保持保留：[`CompatiblePreset`](../internal/sshutil/presets.go:69)、[`GetPreset()`](../internal/sshutil/presets.go:144)、[`Settings.vue`](../frontend/src/views/Settings.vue:196)。
5. 验证结果：
   - [`go test ./internal/config ./internal/ui ./internal/report ./internal/executor ./internal/taskexec ./internal/sshutil`](../internal/config/runtime_config.go:1) 通过。
   - [`build.bat`](../build.bat) 全链路构建通过。
