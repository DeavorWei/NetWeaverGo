# 阶段三：设备画像与会话初始化最终设计

## 1. 目标

将厂商差异从执行主链路中抽离，形成统一、可配置、可注入的设备画像与会话初始化流程。

本阶段只解决两件事：

1. 厂商差异配置化
2. SSH 会话初始化标准化

---

## 2. 已确认的问题

当前代码中已确认的问题：

1. `sshutil/client.go` 中 PTY 参数固定为 `vt100 + 80x40`，过于保守。
2. `matcher.go` 中 prompt / pager 规则是全局硬编码。
3. 普通执行链路没有标准初始化流程，备份链路中则零散执行了 `screen-length 0 temporary`。

需要修正一个判断：

- “厂商自动识别缺失”不是当前主线阻塞问题，因为设备资产模型本身已含 `Vendor` 字段。

因此：

1. 主线优先使用库存里的 `Vendor`
2. 自动识别只作为 `Vendor` 缺失时的 fallback

---

## 3. 设计原则

1. 不新建过重的独立 `profile` 顶层模块，优先扩展现有 `discovery/command_profile.go`。
2. 不使用全局单例作为核心机制，优先用可注入 provider / registry。
3. PTY 参数必须保守可落地，第一版默认 `256x200` 左右。
4. 初始化流程统一到 executor，不允许不同链路各自发初始化命令。
5. 本阶段允许直接切换，不维护长期新旧画像体系并存。

---

## 4. 模块边界

本阶段后的代码边界建议为：

```text
internal/discovery/command_profile.go   # 扩展为厂商画像主定义
internal/matcher/matcher.go             # 接收画像驱动的 prompt/pager 规则
internal/sshutil/client.go              # 根据画像请求 PTY
internal/executor/initializer.go        # 会话初始化流程
```

不单独新建：

1. `internal/profile/`
2. `internal/session/`

除非后续规模扩大，再从现有实现中拆分。

---

## 5. 核心模型

### 5.1 DeviceProfile

建议直接扩展现有 `VendorCommandProfile`，或新增与其共存的统一画像结构：

```go
package discovery

type DeviceProfile struct {
    Vendor      string
    Name        string
    PTY         PTYConfig
    Prompt      PromptConfig
    Pager       PagerConfig
    Init        InitConfig
    Commands    []CommandSpec
}
```

### 5.2 PTYConfig

```go
package discovery

type PTYConfig struct {
    TermType string
    Width    int
    Height   int
    EchoMode int
    ISpeed   int
    OSpeed   int
}
```

第一版默认值：

```go
PTYConfig{
    TermType: "vt100",
    Width:    256,
    Height:   200,
    EchoMode: 0,
    ISpeed:   14400,
    OSpeed:   14400,
}
```

说明：

1. 第一版不使用 `512x2000` 这类激进参数。
2. Huawei / H3C / Cisco 第一版可先共用 `256x200`，后续再按样本微调。

### 5.3 PromptConfig

```go
package discovery

type PromptConfig struct {
    Suffixes []string
    Patterns []string
}
```

第一版要求：

1. 继续支持常见后缀：`>`, `#`, `]`
2. 允许厂商补充正则模式

### 5.4 PagerConfig

```go
package discovery

type PagerConfig struct {
    Patterns      []string
    ContinueBytes []byte
}
```

第一版要求：

1. `ContinueBytes` 默认空格
2. 允许不同厂商配置不同的 pager 模式

### 5.5 InitConfig

```go
package discovery

type InitConfig struct {
    DisablePagerCommands []string
    ExtraCommands        []string
    PromptTimeoutSec     int
}
```

说明：

1. 第一版不搞复杂步骤 DSL
2. 初始化流程只保留必要命令列表

---

## 6. 画像提供方式

不使用全局单例 `ProfileManager` 作为主方案。  
建议改为：

```go
package discovery

type ProfileProvider interface {
    GetByVendor(vendor string) (*DeviceProfile, bool)
    DetectFallback(prompt string, banner string) string
}
```

主流程：

1. 优先使用设备资产中的 `Vendor`
2. 如果 `Vendor` 为空或 `unknown`，才调用 `DetectFallback`
3. fallback 第一版只做轻量探测，不作为主线依赖

---

## 7. 会话初始化流程

统一的初始化流程如下：

1. 建立 SSH shell 与 PTY
2. 等待首个稳定 prompt
3. 发送预热空行
4. 再次等待 prompt
5. 发送禁分页命令
6. 等待 prompt 恢复
7. 进入业务命令执行

初始化结果必须显式返回成功或失败，不能“默默忽略”。

建议接口：

```go
package executor

type Initializer struct {
    profile *discovery.DeviceProfile
}

func NewInitializer(profile *discovery.DeviceProfile) *Initializer
func (i *Initializer) Run(ctx context.Context, client *sshutil.SSHClient, machine *SessionMachine) error
```

---

## 8. matcher 的调整

`matcher` 在本阶段的职责是：

1. 接收 `PromptConfig`
2. 接收 `PagerConfig`
3. 基于逻辑行和活动行做判断

`matcher` 不再自己维护一套厂商无关的固定硬编码规则作为唯一来源。  
可以保留默认规则，但默认规则只能作为 profile 缺省值。

---

## 9. 厂商配置建议

### 9.1 Huawei

初始化建议：

1. `screen-length 0 temporary`

### 9.2 H3C

初始化建议：

1. `screen-length disable`
2. 如果设备实际不支持，再用兼容命令回退

### 9.3 Cisco

初始化建议：

1. `terminal length 0`
2. 可选 `terminal width 0`

说明：

1. 第一版厂商初始化命令应以样本验证为准
2. 不在设计阶段写入过多推测命令

---

## 10. 实施步骤

1. 扩展 `command_profile.go`，定义 `DeviceProfile`
2. 为 Huawei / H3C / Cisco 填充基础画像
3. 修改 `sshutil/client.go`，让 PTY 参数来自画像
4. 修改 `matcher.go`，让 prompt/pager 规则来自画像
5. 新增 `executor/initializer.go`
6. 普通执行、同步执行、备份链路统一走初始化流程

---

## 11. 测试要求

本阶段必须有：

1. 厂商画像单元测试
2. 初始化流程集成测试
3. PTY 参数生效测试
4. prompt / pager 规则由画像驱动的测试

最低验证项：

1. Huawei 初始化后禁分页命令生效
2. Cisco 初始化后分页减少
3. `Vendor` 已知时无需走 fallback 探测

---

## 12. 验收标准

满足以下条件即视为阶段完成：

1. 执行链路中不再硬编码 PTY 参数
2. prompt / pager 规则可由画像配置驱动
3. 普通执行、同步执行、备份链路都复用初始化流程
4. 主线流程优先使用库存 `Vendor`
5. 自动识别只作为 fallback，不影响主线落地
