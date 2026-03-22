# 事故样本索引

本目录包含分页竞态问题相关的事故样本，用于回归测试和问题复现。

## 样本列表

| 编号                                          | 问题名称       | 状态      | 描述                         |
| --------------------------------------------- | -------------- | --------- | ---------------------------- |
| [issue_001](issue_001_pagination_truncation/) | 分页截断问题   | ✅ 已修复 | 分页提示符后的内容被错误处理 |
| [issue_002](issue_002_prompt_misalignment/)   | 提示符错位问题 | ⏳ 待实现 | 提示符被错误拼接到正文末尾   |
| [issue_003](issue_003_overwrite_corruption/)  | 覆盖写错乱问题 | ✅ 已修复 | 回车覆盖写导致内容错乱       |
| [issue_004](issue_004_pagination_overwrite/)  | 分页覆盖写问题 | ✅ 已修复 | 分页后的 ANSI 覆盖写序列处理 |

## 样本结构

每个事故样本目录包含：

```
issue_XXX_problem_name/
├── README.md      # 问题描述、输入特征、期望输出
├── input.txt      # 原始输入数据（模拟设备输出）
└── expected.txt   # 期望的规范化输出
```

## 使用方法

### 1. 运行回归测试

```powershell
go test ./internal/terminal/... -v -run TestRegression
```

### 2. 添加新样本

1. 在 `testdata/regression/bug_fixes/` 下创建新目录
2. 命名格式：`issue_XXX_problem_name`
3. 添加 `README.md`、`input.txt`、`expected.txt`
4. 更新本索引文件

## 问题分类

### 分页相关问题

- **issue_001**: 分页截断 - 分页提示符处理不当
- **issue_004**: 分页覆盖写 - 分页后的 ANSI 序列处理

### 提示符相关问题

- **issue_002**: 提示符错位 - 提示符与正文边界识别

### 终端语义问题

- **issue_003**: 覆盖写错乱 - 回车覆盖写语义

## 修复进度

- [x] issue_001: 分页截断问题
- [ ] issue_002: 提示符错位问题（待 SessionMachine 重构）
- [x] issue_003: 覆盖写错乱问题
- [x] issue_004: 分页覆盖写问题

## 相关文档

- [分页竞态修复方案](../../docs/pagination-race-fix-plan.md)
- [分页竞态修复实施计划](../../docs/pagination-race-fix-implementation-plan.md)
