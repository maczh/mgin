# mgin v2 任务分解清单（v2-tasks）

- **基线**：`v2-arch`（`a285b65`）
- **配套**：`docs/v2-design.md`（架构设计）、`docs/v2-prd.md`
- **格式**：每条任务含【依赖】【估算】【包/接口边界】【风险】【回归测试点】。
- **回归底线**：现有 17 个测试文件必须全部通过（`client/breaker_test.go`、`client/resilience_test.go`、`config/configure_test.go`、`health/health_test.go`、`job/job_test.go`、`middleware/ratelimit/ratelimit_test.go`、`registry/{consul,etcd,polaris}_test.go`、`storage/s3/client_test.go`、`utils/util_test.go`）。新增能力另加 ≥5 个测试。
- **三阶段**：v2.0（P0，理顺骨架） / v2.1（P1，本轮能力补齐） / v2.2（P2 + 递延 P1）。

---

## 阶段一 · v2.0 —— 理顺骨架（P0）

### T1 建立 pkg/ + internal/ 骨架并迁移现有包
- **依赖**：无
- **估算**：3 天
- **包/接口边界**：新增 `pkg/`、`internal/` 目录；迁移 `client config errcode i18n health models registry middleware casbin utils logs→logger job db storage` → `pkg/*`；`cache` → `internal/cache`；新增 `internal/plugin internal/bootstrap internal/healthcheck internal/configloader`。
- **风险**：高（触及几乎所有包的 import 路径）
- **回归**：`go build ./...`；17 个存量测试通过；`go vet ./...` 对 v2 包干净。

### T2 编写 v2 import 迁移工具/文档 + 更新脚手架
- **依赖**：T1
- **估算**：2 天
- **包/接口边界**：新增 `cmd/migrate`（旧路径→`pkg/*` 映射）；改 `cmd/scaffold.go` 生成 `internal/router|controller|service|dao|model` 五层。
- **风险**：中
- **回归**：`go run ./cmd mgin new demo` 生成工程可 `go build`；旧平铺工程仍编译通过。

### T3 定义统一 Plugin 接口与注册表
- **依赖**：无
- **估算**：2 天
- **包/接口边界**：新增 `internal/plugin`：`Plugin` 接口（`Name/Order/Init(ctx)/Close(ctx)/Health/Enabled`）、`Register/Unregister/Run/Shutdown/Health()`。
- **风险**：中
- **回归**：新增 `plugin_test.go`——注册顺序、去重、`Run` 失败回滚。

### T4 适配层：数据源 / s3 / job / registry 接入 Plugin
- **依赖**：T3
- **估算**：3 天
- **包/接口边界**：`internal/plugin/adapters`：`dbPlugin`（mysql/postgres/mongodb/redis/clickhouse/elasticsearch/kafka/sqlite）、`s3Plugin`（包装 `s3.S3`）、`jobPlugin`、`registryPlugin`。
- **风险**：高（需逐组件核对 `Init/Close/Check` 签名）
- **回归**：各数据源 `Health()` 经 `plugin.Health()` 汇总；存量配置驱动下 `Enabled()` 与 `Used` 一致。

### T5 重写生命周期：Init/SafeExit/checkAll 由插件注册表驱动
- **依赖**：T3、T4
- **估算**：3 天
- **包/接口边界**：`internal/bootstrap` 取代 `mgin.go` 硬编码 `if Contains(Used,...)` 链；保留 `mgin.Init/Use/UsePlugin`（委托 `plugin.Register`）。
- **风险**：高（框架启动核心路径）
- **回归**：17 个存量测试；新增"注册一个测试插件，验证 Init/Close 顺序"测试。

### T6 client.Call 增加 context（零破坏）
- **依赖**：无
- **估算**：2 天
- **包/接口边界**：`pkg/client` 新增 `CallCtx(ctx, service, uri, op)`、`CallTCtx`；`Call/CallT` 委托 `CallCtx(context.Background(),...)`；评估 grequests `WithContext`/`HTTPClient` 注入。
- **风险**：中
- **回归**：新增 `client/ctx_test.go`——ctx 取消不阻塞调用方；旧 `Call` 单测仍过。

### T7 扩展 db/dao.Dao[E] 接口
- **依赖**：无
- **估算**：3 天
- **包/接口边界**：`db/dao` 接口扩为 `Insert/Find/Where/Update/Delete/Count/Page...`（对齐 `MySQLDao` 实测方法）；`MySQLDao`/`PostgresDao` 全实现；新增 Repository 端口-适配器示例。
- **风险**：高（影响所有 service 层）
- **回归**：新增 `dao_test.go`——用内存桩实现 `Dao` 完成单测，不依赖真实库；存量 `MySQLDao` 行为不变。

### T8 配置分层：framework / application / runtime
- **依赖**：无
- **估算**：2 天
- **包/接口边界**：`pkg/config` 新增 `Framework`/`Runtime` 结构；`GetShutdownTimeout()` 优先 `go.framework.shutdownTimeout`→`go.application.shutdownTimeout`→5s；旧键全部可读。
- **风险**：中
- **回归**：新增 `config/framework_test.go`——新旧键并存读取；`configure_test.go` 仍过。

### T9 配置样例 + 包归属表 + 迁移指引文档
- **依赖**：T1、T8
- **估算**：1 天
- **包/接口边界**：`docs/migration-v2.md`、README `pkg/` vs `internal/` 归属表、`application.yml` v2 样例。
- **风险**：低
- **回归**：文档中的样例可 `go build` 配套示例工程。

---

## 阶段二 · v2.1 —— 能力补齐（本轮 P1）

### T10 RegistryClient 新增 GetServices（多实例）
- **依赖**：T1
- **估算**：2 天
- **包/接口边界**：`pkg/registry` 接口新增 `GetServices(service, group...) []string`；nacos/etcd/consul/polaris 四实现复用各自 list 逻辑（nacos 见 `nacos.go:138`）。
- **风险**：中
- **回归**：`registry/*_test.go` 仍过；新增 `GetServices` 返回多实例测试。

### T11 LoadBalancer 接口与 4 策略
- **依赖**：无
- **估算**：2 天
- **包/接口边界**：`pkg/client/loadbalancer.go`：`LoadBalancer` 接口 + `RoundRobin/Random/LeastConnections/ConsistentHash` + 按名注册表。
- **风险**：中
- **回归**：新增 `loadbalancer_test.go`——4 策略选取正确性（含一致性哈希稳定性）。

### T12 CallCtx 接入负载均衡 + per-instance 熔断
- **依赖**：T6、T10、T11
- **估算**：2 天
- **包/接口边界**：`pkg/client`：`CallCtx` 内 `GetServices`→剔除熔断 Open 实例→`Pick`；`GetBreaker(service+"@"+host)` per-instance。
- **风险**：高
- **回归**：新增 `client/lb_test.go`——实例剔除与选定；`breaker_test.go` 仍过。

### T13 Prometheus 指标包 + /metrics 挂载
- **依赖**：T1
- **估算**：2 天
- **包/接口边界**：新增 `pkg/metrics`：`Init/ Middleware/ Handler`；`app.baseRouter()` 早期挂载 `/metrics`（早于业务中间件），受 `go.framework.metrics.enabled` 控制。
- **风险**：中（[需要新依赖] `prometheus/client_golang`）
- **回归**：新增 `metrics_test.go`——`/metrics` 返回 Prometheus 文本、含 `mgin_http_requests_total`；17 测试仍过。

### T14 统一错误码 + HTTP 状态映射 + i18n 键驱动
- **依赖**：T1
- **估算**：3 天
- **包/接口边界**：`pkg/errcode` 新增 `ErrorCode` 结构 + `Register` + `HTTPStatusOf`；`errcode/constant.go` 硬编码中文改为 i18n 键；`pkg/errcode/error.go` 错误链 `New/Unwrap`。
- **风险**：高（影响所有错误返回路径）
- **回归**：新增 `errcode_test.go`——code→HTTP 状态映射、i18n 键兜底；存量 `i18n.Error` 调用仍编译。

### T15 框架响应改写：按 errcode 返回正确 HTTP 状态
- **依赖**：T14
- **估算**：2 天
- **包/接口边界**：`app.go` `NoRoute`/`recoveryHandler` 改用 `errcode.HTTPStatusOf`；新增 `app.ResultError(c, err)`。
- **风险**：中
- **回归**：新增 `app/response_test.go`——404/401/500 等状态码正确；存量返回体结构 `models.Result` 不变。

### T16 OpenTelemetry 接入：TracerProvider + trace 中间件升级
- **依赖**：T1
- **估算**：3 天
- **包/接口边界**：新增 `pkg/oteltrace`：`Init(opts)` 读 `go.framework.trace.*`，OTLP HTTP 默认 / stdout 可选；升级 `pkg/middleware/trace` 输出 `traceparent/tracestate`。
- **风险**：高（[需要新依赖] `go.opentelemetry.io/otel` + sdk + otlp）
- **回归**：新增 `oteltrace_test.go`——span 创建与导出（stdout）；`trace_test.go` 兼容旧 `GetHeaders`。

### T17 gorm / grequests 插桩
- **依赖**：T16、T6
- **估算**：2 天
- **包/接口边界**：`db` 驱动 `Init` 后注入 otel gorm 插件；`CallCtx` 出站 span + 注入 `traceparent`；保留旧 TraceId 兼容。
- **风险**：中
- **回归**：跨服务调用 trace_id 可串联（集成验证）；`client`/`db` 存量测试仍过。

### T18 P1 集成测试 + 17 测试回归 + README 用法
- **依赖**：T10–T17
- **估算**：2 天
- **包/接口边界**：整体回归与文档（`metrics`/`errcode`/`otel`/`lb` 用法）。
- **风险**：中
- **回归**：17 存量测试 + 本轮新增测试（≥5）全绿；`go build ./...`、`go vet ./...` 干净。

---

## 阶段三 · v2.2 —— 高级特性（含递延 P1）

### T19 API 网关 pkg/gateway
- **依赖**：T12
- **估算**：5 天
- **包/接口边界**：新增 `pkg/gateway`：路由（gin）、聚合（顺序/并行/fork-join）、鉴权（casbin/jwt 透传）、限流、灰度（Header/Cookie/权重）；YAML 配置。
- **风险**：高
- **回归**：新增转发/灰度单测；不影响存量。

### T20 OAuth2 / OIDC 资源服务器 pkg/middleware/oauth2
- **依赖**：T1
- **估算**：3 天
- **包/接口边界**：新增 `pkg/middleware/oauth2`：复用 `middleware/jwt` + 可选 `golang.org/x/oauth2`；最小可用 JWT Bearer 校验。
- **风险**：中
- **回归**：新增校验单测。

### T21 分布式锁 pkg/lock
- **依赖**：T1
- **估算**：3 天
- **包/接口边界**：新增 `pkg/lock`：`Locker` 接口（`Lock/TryLock/Unlock`）+ Redis（首选，复用 go-redis）/etcd（备选）后端，可重入 + 自动续期。
- **风险**：中（零新增依赖）
- **回归**：新增锁语义单测（可用 miniredis，若引入需标注）。

### T22 gRPC 服务端 + 客户端
- **依赖**：无
- **估算**：5 天
- **包/接口边界**：grpc 提升 direct；新增 `pkg/grpc`。
- **风险**：高
- **回归**：ping-pong 集成测试。

### T23 分布式 Session（Redis store）
- **依赖**：T1
- **估算**：3 天
- **包/接口边界**：升级 `middleware/session` 支持 Redis 后端。
- **风险**：中

### T24 多租户（Header → schema）
- **依赖**：T7
- **估算**：3 天
- **包/接口边界**：`pkg/db` 增加 Tag/Header 路由选择器。
- **风险**：中

### T25 API 文档自动生成
- **依赖**：无
- **估算**：3 天
- **包/接口边界**：基于注释生成 OpenAPI；复用 `sys.Swagger`。
- **风险**：中

### T26 集群限流（Redis 中心化）
- **依赖**：T13
- **估算**：3 天
- **包/接口边界**：升级 `middleware/ratelimit` 为 Redis 中心化。
- **风险**：中

### T27 分布式事务 Saga / TCC
- **依赖**：T21
- **估算**：5 天
- **包/接口边界**：新增 `pkg/tx`。
- **风险**：高

---

## 汇总

| 阶段 | 任务数 | 任务 ID | 总估算（人天） |
|------|--------|---------|----------------|
| v2.0（P0） | 9 | T1–T9 | 21 |
| v2.1（P1 本轮） | 9 | T10–T18 | 21 |
| v2.2（P2 + 递延 P1） | 9 | T19–T27 | 34 |
| **合计** | **27** | T1–T27 | **76** |

- **依赖主线**：T1 → T2/T5/T8/T9；T3 → T4 → T5；T6 → T12/T17；T7 → T24；T10+T11 → T12；T14 → T15；T16 → T17；T12 → T19。
- **风险最高**：T1（全量 import 迁移）、T4/T5（生命周期重写）、T7（Dao 扩展）、T12（LB+熔断协同）、T14（错误码全局改造）、T16（OTel 接入）、T19/T22/T27。
- **回归策略贯穿**：每个 P0/P1 任务均要求"17 存量测试 + 至少 1 个新增测试"通过；结构性任务（T1/T5）额外要求 `go build ./...` 与 `go vet ./...` 干净。
