# mgin v2 架构重构 · 增量 PRD

- **文档版本**：v1.0（增量，基于既有 mgin 框架，非从零设计）
- **分支基线**：`v2-arch`（从 `jh` 拉出，已含提交 `a285b65` 补齐三大生产缺口）
- **作者**：产品经理 许清楚（Xu）
- **配套文档**：`docs/mgin-architecture-options.md`（6 套架构方案与三大空白区实测，本 PRD 的起点）
- **状态**：待架构师做技术设计与任务分解

---

## 0. 一句话定位

mgin 是"单进程、单 gin.Engine、HTTP+HTTPS 同启"的**框架 + 基础设施胶水层**。v2 不是重写得像 Spring Cloud，而是**在保留全部既有能力的前提下，把散落的职责收拢成清晰的扩展点、标准化契约与可插拔插件体系，并补齐微服务必备能力**。`a285b65` 已把"健康探针 / client.Call 韧性 / HTTPS 优雅关闭"三块短板补齐，本 PRD 的 P0/P1/P2 在它之上继续推进。

---

## 1. 现状盘点（v2-arch 分支实测）

> 下列结论均来自对 `v2-arch` 分支源码的实测（`app.go` / `mgin.go` / `client/client.go` / `client/resilience.go` / `client/breaker.go` / `registry/registry.go` / `db/dao/*` / `config/configure.go` / `cmd/scaffold.go` / `health/health.go` / `go.mod`）。函数名、配置项名、接口签名均与源码一致，未编造。

### 1.1 已就绪能力（a285b65 已完成，不重复列入待办）

| 能力 | 现状（实测） |
|------|------|
| 健康探针 | `health` 包：`GET /health/live` `/health/ready` `/health/startup`，JSON 响应；由 `go.health.enabled` 或 `App.EnableHealth()` 驱动，挂载早于一切中间件；复用 `checkAll` 的 7 类数据源 `Check()` 口径 |
| client 韧性 | `client/breaker.go` 三态熔断器（全局注册表，纯标准库）；`client/resilience.go` 的 `CallResilient` 叠加超时+指数退避+jitter+熔断，写操作默认不重试（幂等保护） |
| HTTPS 优雅关闭 | `app.Run()` 将 `server`/`serverSsl` 收集进切片逐个 `Shutdown`；新增 `go.application.shutdownTimeout`（默认 5s）；信号通道改 buffered 防丢信号 |

### 1.2 分类盘点表

| 类别 | 现状（实测事实） | 对应缺口 / 改造点 |
|------|------|------|
| **A. 已有但组织方式/职责划分需优化** | 组件 Init/Close/Check 硬编码在 `mgin.go` 的 `Init()`/`SafeExit()`/`checkAll()` 一长串 `if strings.Contains(config.Config.Config.Used, "mysql")` 链；`MginPlugin` 接口仅 `Init(configData []byte)/Close()/Check() error` 三方法，且**只有 s3 真正走 `UsePlugin`**，其余组件皆硬编码 | → P0-2 统一 Plugin 抽象 |
| | 脚手架 `cmd/scaffold.go` 把 `router/controller/service/dao/model` 生成在**项目顶层**，未收纳进 `internal/`；无 `pkg/`（对外可导入）与 `internal/`（框架内部）边界 | → P0-1 目录结构优化 |
| | 配置全部扁平：`go.application.*` `go.config.*` `go.discovery.*` `go.jwt.*` `go.sys.*` `go.casbin.*`，**无 `go.framework.*` 框架层命名空间**；环境仅 `go.config.env` 单一维度，无 dev/test/prod 隔离 | → P0-5 配置分层 |
| | `db/dao.Dao[E]` 接口**仅声明 `Insert(entity *E) error` 一个方法**；而 `MySQLDao[E]` 实际有 `Where/Create/MultiCreate/Delete/Update/WithContext/Debug` 等；接口过薄，无法做端口-适配器 | → P0-4 扩展 Dao 接口 |
| | `client.Call(service, uri string, op *Options)` **无 `context.Context` 参数**；底层 `grequests` 无法被取消；取单 host 无负载均衡 | → P0-3 核心接口收敛 |
| **B. 已有但深度不足（对标 Spring Cloud 同类能力）** | 服务调用：已有 `CallResilient` 外层韧性，但 (1) `Call` 本身仍无 ctx，(2) 超时是 goroutine+timer 兜底，**底层 HTTP 无法真正取消**（resilience.go 注释自承），(3) 仍单 host 无客户端负载均衡 | 对标 OpenFeign + LoadBalancer + Resilience4j |
| | 安全：已有 `middleware/casbin` + `middleware/jwt` + `middleware/session` + `iplimit` + `xss` + `limit` + `ratelimit`，但**无 OAuth2/OIDC 资源服务器** | 对标 Spring Security OAuth2 Resource Server |
| | 限流：`middleware/ratelimit` 仅**单实例内存计数**（`Manager` 单例），无 Redis/中心化；无熔断 | 对标 Spring Cloud Gateway + bucket4j |
| | 可观测性：仅 `middleware/trace` 注入/透传 TraceId；无 OTel、无 `/metrics` 指标端点 | 对标 Micrometer + OTel |
| | 错误码：`errcode/constant.go` 有数字码（如 `URI_NOT_FOUND=1000`）；`errcode/i18n.go` 是**硬编码中文串**而非 i18n 键；`i18n.Error(code, messageId)` 能产生本地化 `models.Result`，但**响应恒为 HTTP 200，无 errcode→HTTP 状态码映射** | 对标 Spring ProblemDetail / ErrorResponse |
| **C. 完全缺失（Spring Cloud 有，mgin 无）** | API 网关/统一入口（路由/聚合/鉴权/限流/灰度） | 对标 Spring Cloud Gateway |
| | 客户端负载均衡（轮询/随机/最小连接/一致性哈希）；`registry.RegistryClient.GetServiceURL` 返回 `(string,string)` **单 host**，无 `GetServices` 多实例接口 | 对标 Spring Cloud LoadBalancer |
| | OpenTelemetry 端到端（gin/gorm/grequests 插桩、traceparent 传播） | 对标 OTel |
| | 分布式锁（Redis/etcd 可重入） | 对标 Redisson / Spring Integration Lock |
| | 指标端点 `/metrics`（Prometheus 格式） | 对标 Micrometer |
| | gRPC 服务端 + 客户端（仅 `google.golang.org/grpc` indirect 依赖） | 对标 gRPC |
| | 分布式 Session（Redis store）、多租户（Header→schema）、API 文档自动生成、集群限流、分布式事务 Saga/TCC | 对标 Spring Session / Hibernate MultiTenant / springdoc |

### 1.3 一段话总结

mgin 在"单进程内能做什么"已经做满（数据源、注册发现、配置中心、HTTP 调用、定时任务、sys 模块、权限中间件全覆盖），`a285b65` 又补齐了健康探针、调用韧性与 HTTPS 优雅关闭三块生产短板。**真正的短板在"架构层"**：组件生命周期靠 `mgin.go` 里的硬编码 if 链与过薄的 `MginPlugin` 接口，缺乏统一扩展点；脚手架生成的五层代码平铺在顶层、无 internal/pkg 边界；配置无框架/业务分层；`Dao` 接口过薄、`client.Call` 无 context 且无法负载均衡；可观测性、网关、OAuth2、分布式锁、指标、gRPC 等微服务标配能力缺失。v2 的重心就是"理顺骨架 + 补齐治理能力"。

---

## 2. 重构目标（架构层，非体验层）

1. **清晰的进程模型与扩展点**：用统一 `Plugin` 生命周期（Init/Close/Check/Order/Enabled/Health）取代 `mgin.go` 中的硬编码 `if Contains` 链；组件以声明式注册接入框架，新增能力不改 `Init`。
2. **标准化的服务契约**：扩展 `db/dao.Dao` 为完整接口集，`client.Call` 支持 `context`；业务依赖接口与契约而非全局单例与具体实现。
3. **可插拔的中间件/插件体系**：把散落在 `middleware/`、`registry/`、`storage/`、`db/` 的能力统一抽象为 Plugin；明确 `pkg/`（对外可导入 API）与 `internal/`（框架内部实现）边界。
4. **可观测性一等公民**：trace（OTel）/ metrics / health 作为框架内建能力，跨进程调用与关键路径默认埋点，而非事后补丁。
5. **配置与环境的显式分层**：框架配置（`go.framework.*`）与业务配置（`go.application.*` 等）分离，支持 dev/test/prod 隔离，且旧配置键保持向后可读。

---

## 3. 范围与功能清单

> 每条均含**验收标准**与**理由**。需要新增第三方依赖的项在 P1 显式标注 `[需要新依赖]`，以便决策（硬性要求：v2 默认不新增 3rd-party 依赖）。

### P0（v2.0 必须做，重在理顺骨架）

**P0-1 目录结构优化（internal/ 收纳 + pkg/internal 边界）**
- **做什么**：脚手架 `cmd/scaffold.go` 生成的 `router/controller/service/dao/model` 五层统一收纳到 `internal/` 下；在框架内明确 `pkg/`（对外可导入，如 `client`、`registry`、`errcode`、`i18n`、`health`、`models`）与 `internal/`（框架内部实现，如 `db`、`mgin`、`config` 内部细节）的边界，并写明各包归属。
- **验收标准**：
  1. `mgin new` 生成的工程含 `internal/router|controller|service|dao|model`；旧式顶层平铺工程 `go build` 仍通过（框架不强依赖 `internal/`）；
  2. 提供一份 `pkg/` vs `internal/` 包归属表，纳入 README；
  3. `go vet ./...` 针对脚手架与框架相关包干净。
- **理由**：当前脚手架把五层平铺在顶层（实测 `scaffold.go` 写 `write("router/router.go", ...)`），无 internal 边界，随业务增长包耦合失控，域间易越界直连（见 `mgin-architecture-options.md §3.6`）。

**P0-2 统一 Plugin 接口与注册机制**
- **做什么**：定义 `Plugin` 接口（建议 `Init(configData []byte) error` / `Close() error` / `Check() error` / `Order() int` / `Enabled() bool` / `Health() (map[string]string, error)`），提供注册表；重构 `mgin.go` 的 `Init()/SafeExit()/checkAll()`，由注册表驱动，移除硬编码 `if strings.Contains(config.Config.Config.Used, ...)` 链；mysql/redis/postgres/mongodb/clickhouse/elasticsearch/kafka/nacos/etcd/consul/s3/job 均以 Plugin 注册。
- **验收标准**：
  1. `mgin.go` 中不再出现逐组件的 `if Contains(Used,...)` 初始化/关闭链；
  2. 新增一个组件只需 `Register(plugin)` 而不改 `Init`；
  3. 现有 `MginPlugin`（s3 等）可经适配器平滑升级到新 `Plugin` 接口，行为不变；
  4. 既有 17 个测试用例仍通过（据 `a285b65`）。
- **理由**：`MginPlugin` 仅三方法且只 s3 注册（`mgin.go:19,38,140` 实测），生命周期逻辑散落硬编码，是扩展成本最高的部位。

**P0-3 核心接口收敛 · client.Call 支持 context**
- **做什么**：`client.Call`/`CallT`/`CallResilient` 增加 `context.Context` 入参（向后兼容方式：保留 `Call(service,uri,op)` 并新增 `CallCtx(ctx,...)`，或给 `Options` 增加可选 `Ctx` 字段）；把 `context`/自定义 `*http.Client` 注入底层 `grequests`，让 resilience 的超时变为**原生取消**（消除 `resilience.go` 中 goroutine+timer 兜底的"超时后连接仍占用"局限）。
- **验收标准**：
  1. 可传 `ctx` 控制超时与取消，`ctx` 取消时底层 HTTP 请求真正中断；
  2. 既有 `Call(service,uri,op)` 调用方**编译通过**（向后兼容）；
  3. `CallResilient` 复用 context 超时，不再依赖 goroutine 兜底；
  4. 新增与 context 相关的单测 ≥1。
- **理由**：`client.Call`（实测 `client.go:41`）无 ctx、底层不可取消，是跨服务调用治理的根因（§3.2）。

**P0-4 db/dao.Dao 接口扩展**
- **做什么**：将 `Dao[E]` 从仅 `Insert` 扩展为完整接口集（如 `Insert/Find/Where/Update/Delete/Count/Page` 等，签名与现有 `MySQLDao` 方法对齐）；各实现 `MySQLDao/PostgresDao/ClickhouseDao/MongoDao` 补齐；在 `db/dao` 中提供 Repository 端口-适配器范式示例。
- **验收标准**：
  1. 新 `Dao` 接口方法集有文档定义且与 `MySQLDao` 实测方法一致；
  2. 至少 `MySQLDao` 与 `PostgresDao` 全实现新接口；
  3. 业务 `service` 可仅依赖 `Dao` 接口、注入内存桩完成单测（不依赖真实库）。
- **理由**：`Dao[E]` 接口实测仅 `Insert` 一个方法（`db/dao/define.go:3`），无法做端口-适配器，强绑定全局 `db.Mysql` 单例（§3.1）。

**P0-5 配置分层（框架 / 业务 / 环境）**
- **做什么**：引入 `go.framework.*` 命名空间承载框架级配置（如 `shutdownTimeout`、`health`、`plugin` 开关、trace 开关）；保留 `go.application.*`/`go.config.*`/`go.discovery.*` 等旧键可读（向后兼容）；明确 dev/test/prod 隔离（在现有 `go.config.env` 之上规范化）。
- **验收标准**：
  1. `go.framework.shutdownTimeout` 与既有 `go.application.shutdownTimeout` 都能读取且旧键兼容；
  2. 新增配置键的加载有单测覆盖（新旧键并存）；
  3. 提供环境隔离示例配置（dev/test/prod）。
- **理由**：当前配置全扁平、无框架层命名空间（`config/configure.go` 实测字段均 `go.application.*` 等），框架升级与业务配置相互污染。

### P1（v2.1 应做，重在能力补齐；本轮建议纳入）

**P1-1 API 网关 / 统一入口（gateway 子包或新 cmd `mgin-gateway`）**
- **做什么**：基于 mgin 提供 `gateway` 子包（或独立 `mgin-gateway` 命令），支持路由转发、聚合、统一鉴权、限流、灰度（金丝雀，按权重/Header 分流）。
- **验收标准**：
  1. gateway 能将请求按服务名转发到注册中心实例；
  2. 可在网关层挂 JWT/OAuth2 鉴权与限流；
  3. 支持按 Header/权重做灰度路由的最小可用实现；
  4. 有 ≥1 个转发/灰度单测。
- **理由**：Spring Cloud Gateway 的对位；多服务化（方案 D/E）必须有统一边缘层，mgin 当前自身不是网关（§3.4 空白区）。

**P1-2 客户端负载均衡**
- **做什么**：在 `client` 包内提供轮询 / 随机 / 最小连接 / 一致性哈希 4 种策略；改造 `registry.RegistryClient`，新增 `GetServices(serviceName string, group ...string) ([]string, error)` 多实例列表接口（现有 `GetServiceURL` 仅返回单 host，实测 `registry.go:15`），`Call`/`CallResilient` 据策略选实例并剔除不健康节点。
- **验收标准**：
  1. `GetServices` 返回多实例；4 种策略可配置切换；
  2. 负载均衡与 P0-3 的 ctx、P0-2 的 breaker 可组合；
  3. 有策略选择与剔除的单测。
- **理由**：`GetServiceURL` 返回单 host（§3.2 实测），多实例下无客户端 LB，是 D/E 方案最大隐忧。

**P1-3 OpenTelemetry 接入 [需要新依赖：将 `go.opentelemetry.io/otel` 由 indirect 提升为 direct，并新增 `otel/sdk`、`otel/exporters/otlp` 等]**
- **做什么**：将 `middleware/trace` 升级为 OTel SDK；为 gin（入站 span）、gorm（DB span）、grequests（出站 span）增加插桩；以 W3C `traceparent` 头传播，保留既有 TraceId 作为 OTel `trace_id` 映射。
- **验收标准**：
  1. 一次跨服务调用的 trace_id 可被子进程/外部 APM 串联；
  2. span 覆盖 HTTP 入站、DB、出站 HTTP；
  3. `go build ./...` 通过，OTel 依赖由 indirect 提升为 direct（标注在变更说明中）。
- **理由**：仅 TraceId 透传，无 OTel/采样（§3.4 空白区 3）。otel 已在依赖图中（indirect），新增面较小，但需架构师决策是否引入 exporter。

**P1-4 OAuth2 / OIDC 资源服务器（middleware/oauth2）**
- **做什么**：在 `middleware/jwt` 基础上新增 `middleware/oauth2`，最小可用支持 `authorization_code` / `client_credentials` / `password` 流程的**资源服务器**侧校验（Bearer token 解析 + 自签发校验或 introspection）。
- **验收标准**：
  1. 可校验自签发 JWT Bearer 或经 introspection 端点校验；
  2. 能保护指定路由组；
  3. 有 ≥1 个校验单测。
- **理由**：已有 casbin+jwt 但无 OAuth2/OIDC 资源服务器（B 类深度不足）。

**P1-5 统一错误码 + 国际化三向映射**
- **做什么**：在 `errcode`（数字码）与 `i18n` 之间建立"错误码 → i18n 键 → HTTP 状态码"三向映射；将 `errcode/i18n.go` 的硬编码中文串改为 i18n 键；新增 errcode→HTTP status 映射表，使响应按错误码返回正确 HTTP 状态（当前恒为 200）。
- **验收标准**：
  1. 定义错误码目录 + HTTP 状态映射表（文档化）；
  2. 框架统一错误响应按 errcode 返回对应 HTTP 状态码；
  3. i18n 键驱动多语言文案；有映射单测。
- **理由**：`i18n.Error(code, messageId)` 已能本地化，但无 HTTP 状态映射、文案为硬编码中文（B 类深度不足）。

**P1-6 指标（Metrics）[若选 `prometheus/client_golang` 则需新依赖；推荐纯 stdlib 实现以零新增依赖]**
- **做什么**：输出 `/metrics` 端点，暴露 QPS、请求延迟、在线实例、数据源健康等核心指标。
- **验收标准**：
  1. `GET /metrics` 返回 Prometheus 文本格式（或兼容格式）；
  2. 覆盖 HTTP 请求计数/延迟；
  3. **若用纯 stdlib 方案，`go.mod` 不新增 direct 依赖**（满足硬性要求）；若选 Prometheus 则在变更说明显式标注 `[需要新依赖]`。
- **理由**：可观测性一等公民目标所需；prometheus 当前仅 transitive 出现（非 direct）。

**P1-7 分布式锁（lock/ 包，Redis/etcd 可重入）**
- **做什么**：在 `lock/` 包封装基于 Redis（`SET NX`，复用现有 `go-redis/redis/v7` direct 依赖）或 etcd（复用现有 `go.etcd.io/etcd/client/v3` direct 依赖）的可重入分布式锁，提供 `TryLock/Unlock/自动续期`。
- **验收标准**：
  1. 跨进程互斥与可重入验证通过；
  2. **零新增依赖**（复用 go-redis / etcd client）；
  3. 有 ≥1 个锁语义单测（可用 miniredis 等，若引入需标注）。
- **理由**：分布式锁/事务/Saga 完全缺失（§3.4 空白区 2）；底层依赖已具备，成本最低的一块。

### P2（v2.2 可延后，重在高级特性）

**P2-1 gRPC 服务端 + 客户端** [grpc 已是 indirect，可提升 direct]：弥补 mgin 只能 HTTP 的短板，内部高性能调用。
**P2-2 分布式 Session（Redis store）**：基于 Redis 的 session 存储。
**P2-3 多租户支持**：基于 Header 路由到不同 DB schema。
**P2-4 API 文档自动生成**：基于注释生成 OpenAPI/Swagger。
**P2-5 限流中心化**：基于 Redis 的集群限流（升级 `middleware/ratelimit` 单实例为集群）。
**P2-6 分布式事务（Saga/TCC）**：封装在 `tx/` 包。

---

## 4. 显式排除（Out of Scope）

- 不引入 Service Mesh（Istio/Linkerd）——链路治理在框架内做，不推到 sidecar。
- 不重写已有数据驱动代码（脚手架生成逻辑、`models/sys` 模块保持原样）。
- 不做分布式数据库能力（分库分表由业务侧处理；多数据源读写路由仅提供 `Tag` 选择器，不做自动路由中间件）。
- 不替换 Gin 为其他 HTTP 引擎。
- 不引入新的配置中心/注册中心协议（沿用 nacos/consul/etcd/polaris/springconfig/file）。
- 不做前端/BFF 业务层改造（`models/sys` + casbin 的 RBAC 维持现状）。

---

## 5. 向后兼容策略（关键）

mgin 已有大量存量项目（`jh` 为 1.25 系列延续），v2 必须严格向后兼容：

1. **`mgin new` 生成结构不破坏**：`internal/` 收纳是**新增目录**；旧工程平铺 `router/controller/service/dao/model` 仍可运行，框架不强依赖 `internal/`（P0-1 验收第 1 条）。
2. **`client.Call` 签名不变**：通过新增 `CallCtx(ctx, ...)` 或 `Options.Ctx` 可选字段提供 context，**既有 `Call(service,uri,op)` 调用编译通过**（P0-3 验收第 2 条）。
3. **配置项向后兼容**：`go.application.*` / `go.config.*` / `go.discovery.*` 等保持可读，且已被新分层 `go.framework.*` 接管（P0-5 验收第 1 条）。
4. **Plugin 接口升级平滑**：现有 `MginPlugin` 经适配器升级到新 `Plugin`，行为不变（P0-2 验收第 3 条）。

### 典型迁移案例（旧项目升级到 v2 的 diff 示意）

**案例 1：存量工程零代码改动，仅启用新能力**
```diff
  // main.go（旧）
  app := mgin.NewApp("application.yml", "demo", "1.0.0", true)
  app.Run()

  // main.go（v2，业务代码零改动）
  app := mgin.NewApp("application.yml", "demo", "1.0.0", true)
+ app.EnableHealth()          // 可选：暴露 /health/live|ready|startup
  app.Run()
```
```yaml
# application.yml（v2 新增，旧键全部保留）
go:
  framework:
    shutdownTimeout: 10       # 框架级；旧 go.application.shutdownTimeout 仍可读取
  health:
    enabled: true             # 探针开关（v2 新增）
  application:                 # 旧键原样保留，向后兼容
    name: demo
    port: 8080
```

**案例 2：新增一个数据源/组件（旧式改 `Init`，新式注册 Plugin）**
```go
// 旧式：需要在 mgin.go 的 Init() 里加一整段 if Contains(...) 链（破坏框架）
// 新式（v2）：组件自己实现 Plugin 接口并注册，框架 Init 由注册表驱动，无需改框架代码
func init() {
    mgin.Register(myPlugin{})   // 仅此一行，不改 mgin.go
}
```

---

## 6. 验收标准（DoD）

- [ ] `go build ./...` 通过。
- [ ] `go vet ./...` 干净（针对 v2 新增/修改的包）。
- [ ] 既有 17 个测试用例全部通过（据 `a285b65`：当前仓库含 `client/breaker_test.go`、`client/resilience_test.go`、`config/configure_test.go`、`health/health_test.go`、`job/job_test.go`、`middleware/ratelimit/ratelimit_test.go`、`registry/{consul,etcd,polaris}_test.go`、`storage/s3/client_test.go`、`utils/util_test.go` 共 11 个测试文件）。
- [ ] 新增 ≥ 5 个针对 v2 新能力的测试（建议：Plugin 注册表、Dao 新接口、client ctx 取消、配置分层新旧键、负载均衡策略选一）。
- [ ] **不新增 3rd-party 依赖（硬性要求）**；必须新增依赖的能力（如 P1-3 OTel、P1-6 若选 Prometheus）在条目中已显式标注 `[需要新依赖]`，由架构师/用户决策。
- [ ] README / CHANGELOG 更新（含 `pkg/` vs `internal/` 包归属表、新配置键说明、迁移指引）。

---

## 7. 迁移路径（三阶段）

**阶段 1 · v2.0 —— 理顺骨架**
- 范围：P0-1 目录结构 + P0-2 插件系统 + P0-5 配置分层 + P0-3/P0-4 接口扩展。
- 影响范围：**中**（改动 `mgin.go`、`client`、`db/dao`、`cmd/scaffold`、`config`）。
- 允许的破坏性变更：**默认无**；`client.Call` 通过兼容重载加 ctx，配置旧键保留。

**阶段 2 · v2.1 —— 能力补齐**
- 范围：P1-1 网关 + P1-2 客户端负载均衡 + P1-4 OAuth2 + P1-3 OTel（+ P1-5/6/7 视决策）。
- 影响范围：**中—大**（新增 `gateway`、`middleware/oauth2`、`client` LB、OTel 插桩；改造 `registry.RegistryClient` 增加 `GetServices`）。
- 允许的破坏性变更：**默认无**；`RegistryClient` 为接口，新增方法不破坏现有实现（现有 4 个实现需补 `GetServices` 默认实现）。

**阶段 3 · v2.2 —— 高级特性**
- 范围：P2-1 gRPC + P2-2 分布式 Session + P2-3 多租户 + P2-4 API 文档 + P2-5 集群限流 + P2-6 分布式事务。
- 影响范围：**大**（引入新通信协议与事务模型）。
- 允许的破坏性变更：**默认无**；均为新增包/能力，不改动既有 API。

---

## 8. 待明确问题（抛回架构师 / 用户决策）

1. **模块拆分粒度**：v2 是否保持 `mgin` 单一 module，还是拆分为 `mgin`（框架）+ `mgin-cli`（脚手架）多 module？这直接决定 `pkg/` vs `internal/` 边界与发布方式。
2. **Plugin 接口字段**：是否纳入 `Order()`（启动顺序）与 `Health()`（探针自报）？若纳入，是否要求所有内置组件补齐 `Health` 报告，否则 `/health/ready` 不完整。
3. **client.Call 加 context 的方式**：采用"新增 `CallCtx` 重载、保留旧 `Call`"（零破坏）还是"直接改 `Call` 签名"（更干净但破坏性）？取决于对存量项目破坏的容忍度。
4. **OTel 接入深度与依赖**：是否接受将 `go.opentelemetry.io/otel` 由 indirect 提升为 direct 并引入 exporter（如 OTLP）？还是仅做 trace_id 透传增强、不引入 OTel SDK（零新增依赖）？
5. **指标方案依赖**：接受 `prometheus/client_golang` 新依赖，还是坚持纯 stdlib 零新增（影响 `/metrics` 格式与生态兼容）？
6. **配置分层迁移策略**：`go.framework.*` 与既有 `go.application.*` 长期并存，还是 v2 起强制迁移？旧键的保留期限如何界定？

---

> **本文档所有"现状"结论均来自对 `v2-arch` 分支（`a285b65`）源码的实测，函数名/配置项名/接口签名与源码一致。**
