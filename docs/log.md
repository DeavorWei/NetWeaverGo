路由追踪IP地区解析模块日志优化实施计划
为了方便排查路由追踪中 IP 地区解析（如 API 限频、请求超时、被运营商拦截、解析报错等）相关问题，我们计划在地区解析模块中引入更丰富的 Debug 和 Verbose 日志。

用户评审要求
IMPORTANT

本次日志添加中使用了 logger.Verbose 级别记录最为详细的数据（如 API 返回的原始 HTML/JSON 内容、缓存跳过细节等），此级别仅在 logger.EnableVerbose 开启时输出，不会影响日常性能。
我们将新建单元测试 tracert_geo_resolver_test.go 来验证地区解析过程中的日志输出是否正确。
待讨论问题
无（如有关于日志格式或需要额外监控指标的具体要求，请在评审中指出）。

拟做的变更
路由追踪 IP 地区解析器
[MODIFY] 
tracert_geo_resolver.go
在 internal/ui/tracert_geo_resolver.go 中，为以下关键步骤添加日志：

collectNewIPs (IP 过滤阶段)

添加 Verbose 日志：记录每一个被过滤的 IP 及其原因（如：空或 *、已有成功缓存、已有未过期的失败缓存、正在 pending 等）。
示例：logger.Verbose("TracertGeoResolver", ip, "IP 过滤跳过: 已有成功缓存")
ResolveAsync (异步触发阶段)

添加 Debug 日志：记录传入的 IP 列表以及过滤后最终需要发起请求的 IP 列表。
示例：logger.Debug("TracertGeoResolver", "-", "收到异步解析请求，输入IP数: %d, 过滤后需要查询的IP数: %d, IP列表: %v", len(ips), len(newIPs), newIPs)
resolveSingleIP (单IP并发任务阶段)

添加 Verbose 日志：在进入限流等待前、成功获取限流器许可后、以及向前端推送事件前分别记录日志。
示例：logger.Verbose("TracertGeoResolver", ip, "开始等待限流器许可")、logger.Verbose("TracertGeoResolver", ip, "成功获取限流器许可，开始执行查询")
queryGeoInfo & doQueryAttempt (网络请求与重试阶段)

添加 Debug 日志：开始查询、API 限频（429）、重试触发等关键生命周期。
添加 Verbose 日志：在读取到 API 响应体后记录响应大小，在 JSON 解析失败时打印原始响应体内容（极其重要，用于排查网络被拦截返回了 HTML 登录页或跳转页的情况）。
示例：logger.Verbose("TracertGeoResolver", ip, "读取响应体成功，长度: %d 字节", len(body))
示例：logger.Verbose("TracertGeoResolver", ip, "解析 JSON 失败: %v, 原始响应体: %s", err, string(body))
doSingleRequest (底层 HTTP 阶段)

添加 Verbose 日志：记录发起的 HTTP 请求 URL。
添加 Debug 日志：记录非 200 响应状态码及网络请求失败的错误信息。
[NEW] 
tracert_geo_resolver_test.go
创建针对 TracertGeoResolver 日志记录的单元测试，模拟各种网络响应（成功、429限频、网络错误、非JSON异常返回等），验证日志在 debug/verbose 模式下是否正确输出且信息完整。

验证计划
自动化测试
运行单元测试验证逻辑：

powershell

go test -v -run TestTracertGeoResolver ./internal/ui/...
手动验证
在系统配置文件或启动参数中开启调试模式（logger.EnableDebug = true，logger.EnableVerbose = true）。
在前端界面启动路由追踪（Tracert）功能。
检查 logs/app.log，验证是否按预期记录了 IP 地区解析阶段的详细 debug 和 verbose 日志。