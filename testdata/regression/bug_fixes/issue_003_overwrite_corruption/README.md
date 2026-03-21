# Issue 003: 覆盖写错乱问题

## 问题描述

进度条或状态更新使用回车覆盖写，原始字符串清洗会导致内容错乱。

## 输入特征

- 使用 `\r` 回车符进行行首覆盖
- 多次覆盖写同一行

## 期望输出

- 只保留最终覆盖写的内容
- 不保留中间状态

## 当前状态

**状态**: ✅ 已实现

`terminal.Replayer` 的 `LineBuffer` 已正确实现回车覆盖写语义。
`\r` 字符会触发光标移动到行首，后续字符覆盖原有内容。

## 实现说明

输入内容（可视化后）:

```
Processing...           Done
Status: OK              Status: COMPLETE
```

实际语义（终端模拟后）:

```
Doneessing...
Status: COMPLETE
```

覆盖写过程：

1. `Processing...` 先写入
2. `\r` 回车后 `Done` 从行首覆盖，结果为 `Doneessing...`
3. `Status: OK` 写入新行
4. `\r` 回车后 `Status: COMPLETE` 覆盖，结果为 `Status: COMPLETE`
