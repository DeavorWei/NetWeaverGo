TextFSM 迁移方案问题分析报告
一、未彻底清除 TextFSM 的地方

1. 代码依赖残留
   位置 问题 严重程度
   go.mod:9 github.com/sirikothe/gotextfsm v1.1.0 依赖仍然存在 🔴 高
   go.sum:129-130 gotextfsm 的 checksum 记录 🟡 中
   internal/parser/textfsm.go 整个文件需要删除 🔴 高
2. 代码引用残留
   位置 问题 严重程度
   internal/taskexec/executor_impl.go:692 parserEngine \*parser.TextFSMParser 🔴 高
   internal/taskexec/executor_impl.go:697 parser.NewTextFSMParser() 🔴 高
   internal/taskexec/executor_impl.go:1575 错误消息中仍提及"TextFSM模板" 🟡 中
   internal/parser/golden_test.go:13 测试使用 NewTextFSMParser() 🟡 中
   internal/config/device_profile.go:151 注释中提及"LLDP TextFSM模板" 🟢 低
3. 模板文件残留 (24个)
   全部位于 internal/parser/templates/ 目录下：

Huawei: 11个 .textfsm 文件
H3C: 6个 .textfsm 文件
Cisco: 7个 .textfsm 文件 4. 文档残留
位置 问题
README.md:319-322 文档中仍描述 .textfsm 模板位置
二、潜在问题

1. 🔴 高严重度 - CliParser 接口不兼容
   问题: 方案设计的 CliParser 接口扩展了 LoadBuiltinTemplates 和 AddTemplate 方法，但当前 internal/parser/models.go:88-92 的接口只有 Parse 方法。

影响: 代码无法编译通过。

2. 🔴 高严重度 - 接口约束检查方式错误
   问题: 方案使用 var \_ CliParser = (\*CompositeParser)(nil) 进行编译时接口检查，但设计文档中的 CliParser 接口与实际代码中的接口定义不一致。

3. 🔴 高严重度 - 并发安全问题
   问题: RegexTemplate.compiled 字段在运行时动态编译，但方案中的正则编译发生在 LoadBuiltinTemplates 阶段，而模板修改发生在 AddTemplate 阶段，需要确保并发安全。

4. 🟡 中严重度 - 缺少 DeleteTemplate 方法
   问题: 方案提供了 AddTemplate 但没有对应的删除/禁用方法，无法满足完整的 CRUD 需求。

5. 🟡 中严重度 - 数据库唯一约束冲突处理
   问题: UserParseTemplate 设置了 UNIQUE(vendor, command_key)，但方案没有说明如何处理更新时的冲突。

6. 🟡 中严重度 - 错误提示语未更新
   问题: executor_impl.go:1575 错误消息仍说"请重点检查 LLDP 采集命令输出与 TextFSM 模板是否匹配"，迁移后应改为"正则模板"。

7. 🟡 中严重度 - 测试覆盖不完整
   问题: 方案要求24个模板全部迁移，但 golden_test.go 目前只测试 lldp_neighbor 和 eth_trunk 两个命令。

8. 🟢 低严重度 - 缺少版本控制
   问题: 用户自定义模板没有版本控制机制，无法追踪模板变更历史。

三、建议修复清单
必须完成（阻塞迁移）
删除 internal/parser/textfsm.go
删除所有24个 .textfsm 模板文件
从 go.mod 移除 gotextfsm 依赖
更新 internal/parser/models.go 扩展 CliParser 接口
更新 internal/taskexec/executor_impl.go 使用 CompositeParser
更新 internal/parser/golden_test.go 使用新解析器
修正错误提示语中的"TextFSM"为"正则模板"
建议完成（提升质量）
更新 README.md 文档中的模板描述
清理 internal/config/device_profile.go 中的注释
增加所有24个模板的单元测试
完善 ParseTemplateService 的更新/删除逻辑
考虑添加模板版本控制机制
