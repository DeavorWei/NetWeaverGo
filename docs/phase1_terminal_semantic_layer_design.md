# 阶段一：终端语义层最终设计

## 1. 目标

建立一个对当前网络设备 CLI 足够准确的轻量终端语义层，替代现有“正则删除 ANSI + 字符串拼接”的处理方式。

本阶段只解决一件事：

- 把 SSH 字节流正确转换为**规范化逻辑文本**

不在本阶段解决：

- 命令状态推进
- 厂商初始化
- discovery / parser 集成

---

## 2. 已确认的问题

当前代码中的问题已经被证实：

1. `detail_logger.go` 中的 `cleanDetailChunk()` 只删除 ANSI、分页符、退格，不解释终端语义。
2. `matcher.go` 仍对“清洗后的脏流”做 prompt / pager 判断。
3. 分页续页时设备发送的光标移动和覆盖写没有被模拟，导致正文截断、提示符错位、表头破坏。

因此本阶段必须把“终端解释”从 `report` 层迁出，建立独立 `terminal` 层。

---

## 3. 设计原则

1. 第一版必须轻量，不做完整 VT100 二维终端仿真。
2. 第一版以**逻辑行语义正确**为目标，不以屏幕绘制完整为目标。
3. 所有字符处理必须按 `rune`，不能按 `byte`。
4. 未支持的 ANSI 序列必须可观测，不能静默吞掉。
5. 本阶段只输出规范化事件和文本，不直接写日志、不直接驱动状态机。

---

## 4. 模块边界

目录调整为：

```text
internal/terminal/
  replayer.go
  ansi.go
  line_buffer.go
  event.go
  replayer_test.go
```

不单独引入 `screen.go` 作为第一版核心结构。  
如后续确实需要调试快照，可在第二轮增加 `snapshot.go`，但不作为第一版主实现。

---

## 5. 核心模型

### 5.1 Replayer

`Replayer` 负责消费原始文本块，产出规范化事件。

建议接口：

```go
package terminal

type Replayer struct {
    ansi      *ANSIParser
    lineBuf   *LineBuffer
    committed []string
    width     int
}

func NewReplayer(width int) *Replayer
func (r *Replayer) Process(data string) []LineEvent
func (r *Replayer) ActiveLine() string
func (r *Replayer) Lines() []string
func (r *Replayer) Reset()
```

说明：

1. 第一版不维护完整二维屏幕，只维护：
   - 当前活动行
   - 已提交逻辑行
2. `width` 只用于可选换行保护和调试，不用于复杂屏幕裁剪。

### 5.2 LineBuffer

`LineBuffer` 是第一版的核心数据结构。

```go
package terminal

type LineBuffer struct {
    cells  []rune
    cursor int
}

func NewLineBuffer() *LineBuffer
func (l *LineBuffer) Put(r rune)
func (l *LineBuffer) MoveLeft(n int)
func (l *LineBuffer) MoveRight(n int)
func (l *LineBuffer) CarriageReturn()
func (l *LineBuffer) Backspace()
func (l *LineBuffer) EraseToEnd()
func (l *LineBuffer) EraseAll()
func (l *LineBuffer) String() string
func (l *LineBuffer) Reset()
```

约束：

1. `cursor` 永远表示 `rune` 索引。
2. 覆盖写必须生效，不能只做追加。
3. `EraseToEnd` 与 `EraseAll` 必须直接修改底层内容。

### 5.3 ANSIParser

`ANSIParser` 负责把控制序列解析成命令。

```go
package terminal

type ANSIParser struct {
    state        parseState
    buffer       []byte
    params       []int
    unknownCount int
}

type ANSICommand struct {
    Type   CommandType
    Params []int
    Raw    string
}

func NewANSIParser() *ANSIParser
func (p *ANSIParser) Parse(data string) (plain []rune, commands []ANSICommand)
func (p *ANSIParser) UnknownCount() int
func (p *ANSIParser) Reset()
```

第一版必须支持：

1. `ESC[nD`
2. `ESC[nC`
3. `ESC[K`
4. `ESC[2K`
5. `ESC[m`

第一版可以先不完整支持，但要正确跳过：

1. `ESC[nA`
2. `ESC[nB`
3. `ESC[H`
4. `ESC[f`
5. `ESC[J`

对于未支持序列：

1. 正确消费原始输入
2. 计入 `unknownCount`
3. 允许上层打 debug 日志

### 5.4 LineEvent

```go
package terminal

type EventType int

const (
    EventLineCommitted EventType = iota
    EventActiveLineUpdated
    EventControlSequence
)

type LineEvent struct {
    Type EventType
    Line string
    Raw  string
}
```

第一版只要求上层稳定使用：

1. `EventLineCommitted`
2. `ActiveLine()`

---

## 6. 处理规则

### 6.1 基础控制字符

必须支持：

1. `\r`
   行首归位，不提交行
2. `\n`
   提交当前逻辑行，清空活动行
3. `\b`
   光标左退一位，不自动删字符，后续写入可覆盖
4. `\t`
   按 8 列展开，第一版可简化为空格补齐

### 6.2 覆盖写规则

必须支持类似故障样本中的场景：

```text
\x1b[16D                \x1b[16DGE1/0/8...
```

语义必须是：

1. 光标左移
2. 用空格覆盖旧内容
3. 再次左移
4. 在同一位置写入新内容

不能退化为：

- 删除控制字符后把文本直接追加到尾部

### 6.3 分页提示处理

本阶段不负责“识别分页后发空格”，那是状态机职责。  
本阶段只负责把分页覆盖写正确还原为逻辑文本。

---

## 7. 对现有代码的接入方式

第一版接入点：

1. `executor.go` 在读取到 `chunk` 后，先交给 `terminal.Replayer`
2. `matcher.go` 后续逐步改为基于 `LineEvent` 和活动行判断
3. `detail_logger.go` 不再解释终端行为，只消费规范化文本

第一阶段完成后，`detail_logger.go` 中应只保留：

1. 换行标准化
2. 零字节清理
3. 脱敏

不再保留：

1. ANSI 删除
2. 分页符删除
3. 退格删除

---

## 8. 实施步骤

1. 新建 `internal/terminal/ansi.go`
2. 新建 `internal/terminal/line_buffer.go`
3. 新建 `internal/terminal/event.go`
4. 新建 `internal/terminal/replayer.go`
5. 用真实故障样本编写 `replayer_test.go`
6. 在 `executor` 中先接一条实验性调用路径，验证输出正确性

---

## 9. 测试要求

本阶段必须有以下测试：

1. `LineBuffer` 单元测试
2. ANSI 解析单元测试
3. 真实分页覆盖写样本回放测试

最低样本：

1. `abc\rXYZ`
2. `hello\b\bXY`
3. `abcdef\x1b[3D123`
4. `---- More ----\x1b[16D                \x1b[16DGE1/0/8`

---

## 10. 验收标准

满足以下条件即视为阶段完成：

1. 故障样本中的 `PHY: Physical` 恢复完整
2. 分页覆盖写场景恢复正确
3. 提示符不再与正文拼接
4. `unknownCount` 可观察到未支持序列
5. 本阶段代码不依赖 `detail_logger` 做终端修复

---

## 11. 本阶段完成后的输出

阶段一完成后，应向阶段二提供：

1. `Replayer`
2. `LineEvent`
3. `ActiveLine()`
4. `Lines()`

阶段二不得再直接对原始 chunk 做 prompt / pager 语义判断。
