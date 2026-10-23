# MGIN 项目文档路由与源码契约

本参考对应当前仓库 `docs/` 与 `docs/development/`。它是路由和容易误读的契约摘要，不替代源码；版本/分支变化时重新核对。

## 文档路由

| 任务 | 优先阅读 |
|---|---|
| 新人上手/环境/脚手架 | `01-getting-started/01-04`、`08-reference/commands.md`、`10-contributing/01-02` |
| 架构/启动/全局单例/依赖边界 | `02-concepts/01-04`、`03-development/01-project-layout.md` |
| 接口实现与服务调用 | `03-development/02-service-implementation.md`、`03-development/03-service-call.md`、`08-reference/api/README.md` |
| DAO/事务/缓存/序列化 | `03-development/04-data-access.md`、`03-development/06-serialization-and-context.md`、`08-reference/types.md` |
| 配置/热重载/诊断 | `04-configuration/01-03`、`08-reference/variables.md` |
| 注册发现/负载/容错 | `05-governance/01-02`、`docs/registry_optimization_jh_overview.md` |
| JWT/Casbin/CORS/XSS/敏感信息 | `05-governance/03-security.md` |
| 指标/追踪/日志/排障 | `06-observability/01-04`、`07-deployment/02-runbook.md`、`docs/victorialogs.md` |
| 部署/发布/兼容性/迁移 | `07-deployment/01-02`、`10-contributing/03-05`、`11-appendix/03-migration-and-known-issues.md` |
| 国际化 | `docs/i18n.md`、`03-development/06-serialization-and-context.md` |
| 未来能力/版本规划 | `09-advanced/01-04` |

## 稳定决策

- 基线是 jh/v1，module 为 `github.com/maczh/mgin`，Go 版本以当前 `go.mod` 为准；不要将 `github.com/maczh/mgin/v2` 的 `pkg/health`、EnableHealth/EnableMetrics 等 API 回写到 v1。
- `NewApp` 有副作用，会读配置、初始化资源/注册中心/路由；测试不要把它当纯构造函数。`Run` 会监听信号并阻塞。
- `go.config.used` 决定组件，`go.config.prefix.<component>` 定位子配置；组件名是子串匹配，容易误启用。主配置与 `<prefix>-<env>.yml` 子配置是两层，不要只修改 application.yml。
- 热重载只重载配置与资源，串行执行；不重建端口、路由、中间件和 App 身份字段。新配置加载失败保留旧连接；加载成功但新依赖连接失败可能使旧连接已关闭，需说明回滚路径。
- `RegistryClient` 必需接口是 `Register([]byte)`、`GetServiceURL(service, group...)`、`DeRegister()`；扩展接口必须可选，旧自定义实现不能因新增实例列表/watch 而无法编译。
- `NewRegistry` 对已支持后端套 `CachedRegistry`。缓存是进程内，包含 TTL、热刷新、并发合并、旧实例降级和 watcher；不要把它描述为跨副本缓存或强一致服务状态。
- 当前发现扩展包括 `InstanceProvider`、`ServiceWatcher`/离线通知和实例策略；真实 etcd watch 与其他后端轮询的时延/取消语义不同。处理空实例、服务删除、关闭和重载时必须验证。
- `client.Call` 的 `Protocol` 默认取 `go.discovery.callType`；`Query` 是 URL 参数，`Data` 是表单，`Json` 是 JSON，文件会强制 file。`Retry` 只重试一次且不区分幂等性，非幂等写操作不得默认开启。
- 调用治理顺序要明确：发现/缓存 → 超时 → 一次重试 → 熔断；fallback 返回缓存不代表下游成功，应该继续计入熔断失败。响应缓存按 service、URI 和参数指纹隔离，当前是进程内且需注意 TTL/容量风险。
- MGIN 业务响应常用 HTTP 200 承载业务错误；`models.Result` 使用 `status`，业务 `vo.Response` 使用 `code`，不得互换。
- 健康/metrics 探针应最早注册以绕过业务鉴权/限流；`/health/ready` 依赖检查可能触发连接检查或重连，必须评估探针超时。
- 日志使用 `{}` 占位符；不要输出 token、Authorization、完整请求体或敏感配置。VictoriaLogs 与 logsink 的队列、关闭排空和错误路径要按源码核对。

## 代码变更检查表

1. 先确认目标版本、真实 import、配置键和旧接口；检查文档是否描述了已实现能力还是规划能力。
2. 沿 `router → controller → service → DAO/ms/mq` 跑通调用链，分别验证 HTTP 状态、业务码、空值、错误和权限。
3. 对 registry/client 覆盖多实例、空实例、注册中心不可达、缓存命中/过期、watch 触发、取消、重载/关闭、超时、重试、熔断半开和 fallback 参数隔离。
4. 对数据库和消息补充幂等、事务边界、重复/乱序、回滚和资源关闭验证。
5. 使用项目实际命令执行 `gofmt`、相关 `go test`、`go test -race`、`go vet`/lint；若工具或外部依赖不可用，明确记录未验证项，不把静态检查称为联调通过。

## 文档与源码冲突的处理

部分历史文档仍保留“client 无超时/无熔断”等旧描述，也可能把某些后端称为单入口。遇到此类冲突：以当前源码和测试为准，保留迁移兼容，更新实际适用文档或在交付中指出差异；不要为了匹配旧文档回退已实现的新能力。

