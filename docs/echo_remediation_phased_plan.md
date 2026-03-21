# SSH 回显截断问题分阶段调整计划

## 1. 文档目的

基于当前项目实际架构，以及已有分析文档：

- `docs/echo_truncation_deep_analysis.md`
- `docs/echo_fix.md`

对 SSH 设备回显截断问题制定一份**面向彻底解决**的分阶段调整计划。

本计划不采用“止血式补丁”思路，而是直接按新项目建设期标准，对现有回显处理链路进行结构化重建，目标是一次性解决以下问题：

- 命令回显被吞
- 分页后输出截断
- 提示符错位
- ANSI 控制字符导致的覆盖写错乱
- 结构化解析链路吃到脏数据后失败

---

## 2. 当前项目实际情况与方案调整原则

结合现有代码实现，修复方案需要做如下调整，不能完全照搬 `echo_fix.md` 中的理想化模块拆分：

### 2.1 当前项目已有可复用边界

项目当前已经存在以下核心边界：

- `internal/executor/`：单设备命令执行生命周期
- `internal/sshutil/`：SSH Shell / PTY 管理
- `internal/matcher/`：错误、分页、提示符匹配
- `internal/report/`：raw/detail/summary 日志输出
- `internal/discovery/`：同步命令采集与结果落盘
- `internal/parser/`：TextFSM 结构化解析

因此本次调整应优先沿着现有主链路重构，而不是在短期内另起一套完全平行的新执行框架。

### 2.2 当前项目中需要特别注意的事实

#### 事实一：当前存在两条命令采集链路

当前项目至少有两条会消费 SSH 输出的主链路：

- `DeviceExecutor.ExecutePlaybook()`：批量命令执行
- `DeviceExecutor.ExecuteCommandSync()`：单命令同步采集，discovery/backup 依赖该路径

因此修复不能只覆盖 `ExecutePlaybook()`，必须抽出公共的终端输出处理内核，统一两条链路。

#### 事实二：当前 parser 不是直接消费 detail.log

发现任务的解析链路是：

`discovery.Runner -> RawCommandOutput.FilePath -> parser.Service`

也就是说，当前 parser 的输入来源是 discovery 阶段落盘的命令输出文件，而不是 `detail.log`。因此本次方案不能简单改成“parser 读 detail.log”，而应该明确让 discovery 落盘**规范化输出**。

#### 事实三：当前 detail logger 承担了错误的职责

`internal/report/detail_logger.go` 里的 `cleanDetailChunk()` 当前做了：

- ANSI 删除
- 分页符删除
- 退格符删除
- 换行与空白压缩

这意味着展示层正在尝试修复终端语义，这是当前问题的根源之一。后续必须把“终端语义解释”迁出 detail logger。

#### 事实四：当前 PTY 参数偏保守

当前 `internal/sshutil/client.go` 实际请求的是：

- `vt100`
- 宽 `80`
- 高 `40`

这比问题分析文档中提到的更容易触发分页与重绘，因此后续需要统一终端参数策略。

---

## 3. 总体改造目标

将当前链路从：

`SSH 字节流 -> 字符串拼接/正则清洗 -> prompt/pager 判断 -> detail/raw 落盘 -> parser`

调整为：

`SSH 字节流 -> 终端语义回放 -> 会话状态机 -> 规范化输出 -> 日志/落盘/解析`

最终要求如下：

1. 原始字节流必须保真保存
2. 控制字符必须先执行语义，再产出文本
3. prompt / pager / echo 判断必须基于规范化输出
4. parser 只能消费规范化后的命令输出
5. detail.log 只能做展示，不能再修复终端行为
6. discovery、backup、普通执行三条链路使用统一输出处理模型

---

## 4. 分阶段调整计划

## 阶段一：建立终端语义层

### 阶段目标

建立一个对网络设备 CLI 足够准确的轻量终端回放器，用来接管控制字符解释工作。

### 主要工作

1. 在 `internal/terminal/` 下新增终端处理模块。
2. 实现轻量终端回放器，优先支持以下语义：
   - `\r`
   - `\n`
   - `\b`
   - `\t`
   - `ESC[nD`
   - `ESC[nC`
   - `ESC[K`
   - `ESC[2K`
   - `ESC[m`
3. 设计统一的终端事件输出模型，至少包括：
   - 已提交逻辑行
   - 当前活动行
   - 屏幕更新事件
4. 将真实故障样本中涉及的分页覆盖写作为第一优先级场景实现。

### 建议新增模块

- `internal/terminal/replayer.go`
- `internal/terminal/ansi.go`
- `internal/terminal/line_buffer.go`
- `internal/terminal/event.go`

### 与现有架构的衔接方式

- 不立即重写 `engine`
- 先由 `executor` 调用 `terminal.Replayer`
- `matcher` 暂时保留，但输入逐步切换为 replayer 输出

### 阶段交付物

- 轻量终端语义回放器
- 基础 ANSI/控制字符单元测试
- 使用真实 `raw.log` 的回放测试

### 阶段验收标准

- 回放真实故障样本时，`PHY: Physical` 不再被截断
- 分页覆盖写后，续页内容位置正确
- prompt 不再与正文串连成一行

---

## 阶段二：重构执行器为统一会话状态机

### 阶段目标

将当前依赖 `streamBuffer` 的字符串驱动执行逻辑，重构为显式状态机。

### 主要工作

1. 在 `internal/executor/` 中引入命令执行状态模型，至少包括：
   - 等待首提示符
   - 终端预热
   - 发送命令
   - 等待 echo
   - 收集正文
   - 处理分页
   - 等待结束 prompt
   - 完成 / 失败
2. 抽取公共的流驱动命令执行内核，供以下两条链路复用：
   - `ExecutePlaybook()`
   - `ExecuteCommandSync()`
3. 删除当前不可靠逻辑：
   - 分页命中后清空 `streamBuffer`
   - 仅依赖 `HasPrefix` 的 echo 过滤
   - 按“最后一段未换行内容”模拟终端行为
4. 将 prompt / pager 检测从“原始脏流”切换为“终端语义层输出的逻辑行或当前活动行”。

### 计划调整点

`echo_fix.md` 中提出单独新建 `session/` 目录是合理方向，但结合当前项目规模，可以先把状态机落在 `executor` 内部，后续再视需要独立成 `session` 层，避免早期目录重构过大。

### 建议涉及文件

- `internal/executor/executor.go`
- 可新增：
  - `internal/executor/session_state.go`
  - `internal/executor/command_context.go`
  - `internal/executor/stream_engine.go`

### 阶段交付物

- 统一的命令执行状态机
- Playbook / Sync 共用执行内核
- 输出采集行为一致化

### 阶段验收标准

- 同一命令在批量执行和同步执行中得到一致输出
- 不再出现命令回显误吞正文
- 分页继续后不再丢失前后文

---

## 阶段三：建设设备画像与会话初始化流程

### 阶段目标

将厂商差异从执行主链路中抽离，形成统一的设备画像配置与初始化流程。

### 主要工作

1. 在现有 `internal/discovery/command_profile.go` 基础上扩展设备画像能力，至少增加：
   - 禁分页命令
   - prompt 匹配规则
   - pager 匹配规则
   - 分页继续字节
   - 建议 PTY 宽高
   - 必要的初始化命令
2. 将 `matcher` 中的分页符/提示符规则改为可由设备画像驱动。
3. 在 SSH 会话建立后，统一执行初始化流程：
   - 等待稳定 prompt
   - 发送禁分页命令
   - 验证 prompt 恢复
   - 再进入业务命令阶段
4. 统一 `sshutil` 中的 PTY 策略，提升终端宽高，降低分页和自动换行概率。

### 计划调整点

本项目已经有 discovery 侧 vendor profile，不建议平地再起一个完全独立的 profile 子系统。更合适的方式是扩展现有 profile，使其同时服务 discovery 和 executor。

### 建议涉及文件

- `internal/discovery/command_profile.go`
- `internal/matcher/matcher.go`
- `internal/sshutil/client.go`
- `internal/executor/executor.go`

### 阶段交付物

- 统一设备画像结构
- Huawei / H3C / Cisco 初始化策略
- 标准化 PTY 请求参数

### 阶段验收标准

- 各厂商 prompt / pager 逻辑不再硬编码散落在执行器中
- 普通执行、discovery、backup 都使用统一初始化策略

---

## 阶段四：重构输出模型与日志职责

### 阶段目标

将原始输出、规范化输出、展示日志、解析输入彻底分层。

### 主要工作

1. 为单条命令定义统一结果结构，至少包含：
   - Command
   - RawChunks
   - RawText
   - NormalizedLines
   - NormalizedText
   - EchoConsumed
   - PagerCount
   - PromptMatched
   - StartedAt / EndedAt
   - Err
2. 重定义日志职责：
   - `raw.log`：保留原始字节流
   - `detail.log`：展示规范化输出
   - 必要时增加 normalized 输出文件用于调试和解析
3. 将 `detail_logger.go` 从“终端修复器”改为“格式化展示器”。
4. 对 `report` 层进行适配，使其输出的详细日志基于 normalized output，而不是 ANSI 清洗结果。

### 计划调整点

当前 `ExecutionLogStore` 已经具备日志会话管理能力，后续重点是调整数据来源，不必重写日志框架本身。

### 建议涉及文件

- `internal/report/detail_logger.go`
- `internal/report/log_storage.go`
- `internal/report/raw_logger.go`
- `internal/executor/executor.go`

### 阶段交付物

- 统一命令结果模型
- 新日志职责划分
- detail log 改为展示 normalized output

### 阶段验收标准

- `raw.log` 与 `detail.log` 各自职责明确
- detail log 中不再依赖正则删除 ANSI 来“伪修复”输出
- 问题排障时可以同时查看 raw 与 normalized 两类输出

---

## 阶段五：打通 discovery 与 parser 的规范化输出链路

### 阶段目标

让结构化解析链路完全建立在规范化输出上，而不是建立在带控制字符污染的命令输出上。

### 主要工作

1. 调整 `internal/discovery/runner.go` 中的命令输出保存逻辑，使 discovery 落盘内容改为规范化后的命令输出。
2. 明确 parser 的输入契约：
   - parser 只负责字段解析
   - parser 不再承担 ANSI 修复、分页修复、提示符修复职责
3. 如有必要，扩展 `models.RawCommandOutput` 字段，区分：
   - 原始审计文件路径
   - 规范化解析文件路径
4. 校验 topology / parser 依赖的命令模板是否符合新输出格式。

### 计划调整点

`echo_fix.md` 中“parser 改读 normalized output”的方向是正确的，但结合当前项目实现，不应直接让 parser 去读 `detail.log`，而应由 discovery 在保存阶段直接落规范化输出文件。

### 建议涉及文件

- `internal/discovery/runner.go`
- `internal/parser/service.go`
- `internal/models/discovery.go`

### 阶段交付物

- discovery 规范化输出落盘
- parser 输入契约明确化
- 必要的数据模型扩展

### 阶段验收标准

- `interface_brief`
- `interface_detail`
- `lldp_neighbor`
- `arp_all`
- `mac_address`
- `eth_trunk`

以上命令在真实样本下可稳定通过解析，不再因脏输出而失败。

---

## 阶段六：建立长期回归与样本驱动测试体系

### 阶段目标

把“终端语义正确性”纳入项目的长期回归体系，避免后续重构再次退化为字符串拼接模式。

### 主要工作

1. 建立终端控制序列单元测试，覆盖：
   - 覆盖写
   - 退格
   - 清行
   - 左右移动
   - 分页擦除重绘
2. 建立真实 `raw.log` 回放 golden tests。
3. 建立多厂商回归样本：
   - Huawei
   - H3C
   - Cisco
4. 建立执行器状态机集成测试，验证：
   - echo
   - pager
   - prompt
   - completion
5. 建立 discovery 与 parser 端到端回归测试。

### 建议涉及测试位置

- `internal/terminal/*_test.go`
- `internal/executor/*_test.go`
- `internal/matcher/*_test.go`
- `internal/discovery/runner_test.go`
- `internal/parser/golden_test.go`

### 阶段交付物

- 样本驱动回放测试集
- 多厂商 golden case
- 执行器状态机回归测试

### 阶段验收标准

- 后续任何对 executor / terminal / matcher / sshutil 的变更，都会自动验证终端语义正确性
- 真实故障样本永久纳入回归，不再重复引入同类问题

---

## 5. 推荐实施顺序

建议按以下顺序推进：

1. 阶段一：建立终端语义层
2. 阶段二：重构执行器为统一状态机
3. 阶段三：建设设备画像与初始化流程
4. 阶段四：重构输出模型与日志职责
5. 阶段五：打通 discovery 与 parser 的规范化输出链路
6. 阶段六：建立长期回归体系

这个顺序的原因是：

- 先有终端语义层，后续状态机和日志分层才有稳定输入
- 先统一 executor，再去打通 discovery/parser，避免重复适配
- 测试体系放在最后成型，但每个阶段都应同步补对应测试

---

## 6. 各阶段完成标志

| 阶段 | 完成标志 |
| ---- | -------- |
| 阶段一 | 终端回放器可正确处理故障样本中的控制字符 |
| 阶段二 | Playbook / Sync 两条链路统一使用状态机执行 |
| 阶段三 | 设备厂商差异进入 profile，不再散落在主逻辑中 |
| 阶段四 | raw / normalized / detail 输出职责清晰分离 |
| 阶段五 | parser 只消费规范化输出，解析稳定恢复 |
| 阶段六 | 真实样本与多厂商用例全部进入自动回归 |

---

## 7. 最终结论

本问题不应继续沿用“字符串拼接 + 正则清洗 + 局部补丁”的路线修补。

对于当前这个处于建设期的新项目，正确的调整方向是：

1. 建立终端语义回放器，先解释控制字符，再产出文本
2. 建立统一命令执行状态机，显式处理 echo / pager / prompt
3. 建立设备画像与初始化流程，收敛厂商差异
4. 建立规范化输出链路，彻底分离 raw、detail、parser 的职责
5. 建立真实样本驱动的长期回归体系

只要按以上阶段完成重构，SSH 回显截断问题可以从根上解决，并为后续 discovery、topology、parser 等上层能力提供稳定输入基础。
