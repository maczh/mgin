# MGin v2 需求文档

| 项 | 内容 |
|----|------|
| 文档版本 | v2.0 |
| 适用范围 | `github.com/maczh/mgin/v2`（Go 1.25.7） |
| 文档性质 | **现状基线需求**：以当前代码可验证的行为为准，同时标注部分实现/未实现项 |
| 相关文档 | [架构设计](architecture.md) · [模块与接口](modules-and-api.md) · [数据设计](data-design.md) · [测试策略](testing-strategy.md) |
| 维护规则 | 新增/变更能力时同步更新本文的 FR/NFR 条目与验收标准 |

> 阅读提示：本文不是"愿望清单"。每条需求都标注了实现状态与代码位置；
> 标为 ❌ 的条目属于**明确不在当前版本范围内**，不得在方案中假设其可用。

---

## 1. 背景与定位

### 1.1 问题背景

Go 服务在落地时通常需要重复解决同一批工程问题：配置文件与环境隔离、数据库/Redis/Mongo
连接与多数据源、服务注册与发现、服务间 HTTP 调用与容错、日志采集、鉴权、限流、
定时任务、健康探针与监控指标。

每个项目各自实现会导致：

- 组件初始化散落、生命周期不一致，退出时连接/协程泄漏；
- 错误响应格式与 HTTP 状态码不统一，网关与前端难以处理；
- 可观测性缺失，线上排障只能靠日志文本搜索；
- 新项目起步慢，脚手架质量参差。

### 1.2 产品定位

mgin 是**单进程内运行**的 Go 微服务基础框架（framework + infrastructure glue），
提供统一的组件契约、配置体系与工程脚手架，让业务只关注
`controller → service → dao` 三层。

**不是**：API 网关、服务网格、gRPC 框架、OAuth2 授权服务器、分布式事务框架。

### 1.3 版本目标（v2）

| 编号 | 目标 | 状态 |
|------|------|------|
| G-1 | 组件生命周期统一为 `Plugin` 契约，新增数据源不改框架入口 | ✅ |
| G-2 | 补齐生产必需能力：三探针、优雅关闭、调用熔断/重试、Prometheus、traceparent | ✅ |
| G-3 | 提供统一错误码 → HTTP 状态码 → i18n 文案的三向映射 | ✅ |
| G-4 | 包结构收敛到 `pkg/`，明确"公开契约 / 框架私有"边界 | ✅ |
| G-5 | 脚手架生成 v2 兼容工程，支持 health/metrics/otel/LB 开关 | ✅（LB/配置开关存在偏差，见 FR-19） |
| G-6 | 保持 v1 公开 API 向后兼容 | ✅（import 路径除外） |

---

## 2. 角色与使用场景

### 2.1 角色

| 角色 | 关注点 | 主要接触面 |
|------|--------|-----------|
| 业务开发者 | 快速接入数据源、调用其他服务、统一响应 | `mgin.NewApp`、`pkg/client`、`pkg/db/dao`、`pkg/errcode` |
| 服务负责人 | 上线安全、可观测、容量与限流 | `pkg/health`、`pkg/metrics`、`middleware/ratelimit`、`pkg/job` |
| 平台/运维 | 注册发现、配置中心、日志采集、优雅发布 | `pkg/registry`、`pkg/config`、`pkg/logs`、`pkg/logsink` |
| 框架维护者 | 契约稳定、向后兼容、扩展点 | `pkg/plugin`、根 `mgin.go`/`app.go`、`cmd/` |
| 新加入的开发者 | 能在一小时内跑起来并改第一个接口 | README、`docs/development-setup.md` |

### 2.2 典型场景

| 场景 | 描述 | 覆盖需求 |
|------|------|----------|
| S-1 新服务起步 | 用脚手架生成工程，本地起 HTTP 服务并验证探针 | FR-01、FR-02、FR-16 |
| S-2 接入数据库 | 单库/多库 MySQL，事务写入，分页查询 | FR-05、FR-06 |
| S-3 跨服务调用 | 通过注册中心发现下游，带超时/重试/熔断调用 | FR-08、FR-09 |
| S-4 统一错误响应 | 业务错误码映射 4xx/5xx，并按语言返回文案 | FR-12、FR-13 |
| S-5 上线发布 | 探针接入 K8s，指标接入 Prometheus，优雅退出不丢请求 | FR-14、FR-15、NFR-04 |
| S-6 定时任务 | 在管理端配置 cron 任务，查看执行日志 | FR-17 |
| S-7 接口防护 | 登录接口按 IP 限流，管理接口 JWT + Casbin 鉴权 | FR-11、FR-18 |
| S-8 排障 | 用 trace id 串起访问日志与下游调用 | FR-15、FR-20 |

---

## 3. 功能需求

状态说明：✅ 已实现 · ⚠️ 已实现但有明确限制 · ❌ 未实现（不在本版本范围）

### 3.1 应用生命周期

| 编号 | 需求 | 验收标准 | 状态 | 实现位置 |
|------|------|----------|------|----------|
| FR-01 | 创建应用并加载配置 | `NewApp(configFile, name, version, xlang)` 返回可用 `*App`；配置缺失时记录错误但不 panic | ✅ | `app.go` |
| FR-02 | 命令行约定 | `-f` 指定配置文件（缺省为可执行文件同名 `.yml`）；`-v` 打印版本后退出 | ✅ | `app.go` |
| FR-03 | 统一初始化 | `mgin.Init` 依次完成配置、VictoriaLogs、注册中心实例、Runtime 元数据、内置插件注册与初始化 | ✅ | `mgin.go` |
| FR-04 | 优雅关闭 | 收到 SIGINT/SIGHUP/SIGTERM/SIGQUIT 后：停巡检 → 排空日志 → 插件逆序关闭 → 每个 server 独立超时 Shutdown | ✅ | `app.go`、`mgin.go` |
| FR-04a | 关闭超时可配 | `go.application.shutdownTimeout`（秒），非法值回退 5s | ✅ | `pkg/config` |
| FR-04b | HTTP/HTTPS 同时关闭 | 两个 server 都纳入 `[]*http.Server` 管理 | ✅ | `app.go` |

### 3.2 配置管理

| 编号 | 需求 | 验收标准 | 状态 | 实现位置 |
|------|------|----------|------|----------|
| FR-05 | 本地 YAML 配置 | 支持点分路径读取 `GetConfigString/Int/Bool/StringArray`，不存在时返回零值不报错 | ✅ | `pkg/config/configure.go` |
| FR-06 | 远程配置中心 | 支持 nacos / consul / etcd / polaris / springconfig / file；子文件名 `{prefix}-{env}.yml` | ✅ | `pkg/config` |
| FR-07 | 配置分层 | 业务 `go.application.*`、数据源 `go.config.*`、框架 `go.framework.*`、运行期 `go.runtime.*` | ⚠️ `go.framework.*` 仅 `i18nEnabled` 生效 | `pkg/config`、`app.go` |
| FR-08 | 热更新 | 日志级别每次输出实时读取；限流配置在中间件构造时读取 | ⚠️ 非全量热更新 | `pkg/logs/output.go`、`pkg/middleware/ratelimit` |

### 3.3 数据访问

| 编号 | 需求 | 验收标准 | 状态 | 实现位置 |
|------|------|----------|------|----------|
| FR-09 | 多数据源接入 | mysql / sqlite / mongodb / redis 四类，按 `go.config.used` 启用 | ✅ | `pkg/plugin/builtins.go`、`pkg/db` |
| FR-10 | 多库（multidb） | `multidb=true` + `dbNames`；MySQL/Redis 取连接必须显式指定库名 | ✅ | `pkg/db/mysql`、`pkg/db/redis` |
| FR-11 | 泛型 DAO | `MySQLDao[E]` 提供 Create/MultiCreate/Updates/Save/Delete/All/One/Exists/Count/Pager/Where* | ✅（`Dao[E]` 接口未对齐 ❌） | `pkg/db/dao/mysql.go` |
| FR-12 | 事务 | `MySQLTx(ctx, fn, dbName...)`、`TransactionOn(ctx, conn, fn)`；DAO 通过 `WithTx` 绑定 | ✅ | `pkg/db/transaction.go` |
| FR-13 | 查询缓存 | `WithCache(CacheConfig)` 对 All/One/Exists/Count/Where* 生效；写操作按前缀失效 | ⚠️ 事务内不清理；`Clear()` = FLUSHDB | `pkg/db/dao/mysql.go`、`redis_cache.go` |
| FR-14 | 内存/磁盘缓存 | `cache.OnMemCache` / `cache.OnDiskCache`，支持 TTL 与遍历 | ✅ | `pkg/cache` |

### 3.4 注册发现与调用

| 编号 | 需求 | 验收标准 | 状态 | 实现位置 |
|------|------|----------|------|----------|
| FR-15 | 服务注册 | nacos / etcd / consul 按 `go.config.used` 自动注册；退出时反注册 | ✅ ｜ Polaris ❌ | `pkg/plugin/builtins.go`、`pkg/registry` |
| FR-16 | 服务发现 | `GetServiceURL` 返回单实例；`GetServices` 返回全部实例；命名多注册中心按 namespace/group/cluster 隔离 | ✅ | `pkg/registry/registry.go` |
| FR-17 | 发现缓存 | TTL + 后台刷新 + 并发回源抑制（防击穿） | ✅ | `pkg/registry/cache.go` |
| FR-18 | 服务间调用 | 支持 GET/POST/PUT/DELETE 与 x-form/json/restful/file 四种协议 | ✅ | `pkg/client/client.go` |
| FR-19 | 客户端负载均衡 | 4 策略：`round_robin` / `random` / `least_connections` / `consistent_hash`，可按调用指定 | ⚠️ 最少连接需业务维护计数；一致性哈希无虚拟节点 | `pkg/loadbalancer`、`pkg/client` |
| FR-20 | 调用韧性 | 超时、指数退避 + 抖动、幂等方法重试、熔断（三态，per-instance） | ⚠️ 超时为 goroutine 兜底，底层 HTTP 不可取消 | `pkg/client/resilience.go`、`breaker.go` |

### 3.5 可观测性

| 编号 | 需求 | 验收标准 | 状态 | 实现位置 |
|------|------|----------|------|----------|
| FR-21 | 健康探针 | `GET /health/live`（不查依赖）、`/health/ready`（查依赖，失败 503）、`/health/startup`（未标记 503）、`/healthz` 别名 | ✅ 默认关闭 | `pkg/health`、`app.go` |
| FR-22 | 探针不被鉴权拦截 | 探针路由在一切 `Use()` 之前注册 | ✅ | `app.go` |
| FR-23 | Prometheus 指标 | `/metrics` 暴露 6 个指标；HTTP 指标由 `metrics.Middleware()` 采集 | ⚠️ 需显式 `EnableMetrics()` 且中间件需业务挂载 | `pkg/metrics` |
| FR-24 | 链路透传 | 入站透传/生成 `traceparent`，同步 `X-Trace-Id`、`X-Request-ID`；出站自动注入 | ✅ | `pkg/middleware/trace` |
| FR-25 | OTel 抽象 | `SetTracerProvider/StartSpan/IsEnabled`；`client` 调用自动开 span | ✅ 不含 SDK/Exporter | `pkg/otel` |
| FR-26 | 访问日志 | 记录请求/响应、耗时、客户端 IP；支持控制台/文件/Kafka/VictoriaLogs/Mongo | ✅ | `pkg/middleware/postlog`、`pkg/logsink` |

### 3.6 安全与流量治理

| 编号 | 需求 | 验收标准 | 状态 | 实现位置 |
|------|------|----------|------|----------|
| FR-27 | JWT 鉴权 | 校验 `Authorization: Bearer`，失败 401 | ✅ | `pkg/middleware/jwt` |
| FR-28 | 接口级鉴权 | Casbin `sub,obj,act` 模型，支持按角色查询/更新策略；`casbin_rule` 自动建表 | ✅ | `pkg/casbin`、`pkg/middleware/casbin` |
| FR-29 | 会话 | `session.New/FromContext/Destroy/Refresh` | ✅ | `pkg/middleware/session` |
| FR-30 | XSS 净化 | 对 JSON/form/multipart/query 做 bluemonday 净化 | ✅ | `pkg/middleware/xss` |
| FR-31 | 限流 | 三种算法、五种维度、路径白名单与 IP 白名单、响应头开关 | ⚠️ 单实例内存，非集群 | `pkg/middleware/ratelimit` |
| FR-32 | 并发/IP 防护 | `limit.MaxAllowed(n)` 信号量；`iplimit.CIDR` 白名单 403 | ✅ | `pkg/middleware/limit`、`iplimit` |

### 3.7 定时任务与国际化

| 编号 | 需求 | 验收标准 | 状态 | 实现位置 |
|------|------|----------|------|----------|
| FR-33 | 任务调度 | cron（5/6 段、`@every`）/fixed_rate/fixed_delay/once；阻塞策略 serial/discard/concurrent/cover；重试与超时 | ✅ | `pkg/job` |
| FR-34 | 任务管理 API | `/job/list`、`/:id`、增删改、start/stop/trigger、handlers、log | ✅ | `pkg/job/api.go` |
| FR-35 | 任务持久化 | `AutoMigrate` 建 `<prefix>job_info` / `<prefix>job_log`；日志按天清理 | ✅ | `pkg/job/store.go` |
| FR-36 | 国际化 | 数据库字典表 `<appName>_i18n`，`X-Lang` 选择语言，缺省回退默认语言 | ✅ | `pkg/i18n` |
| FR-37 | 统一错误响应 | `Definition{Code, HTTPStatus, MessageKey, Module}`；15 个内置错误码；NoRoute 404 / Recovery 500 | ✅ | `pkg/errcode`、`pkg/i18n` |

### 3.8 脚手架

| 编号 | 需求 | 验收标准 | 状态 | 实现位置 |
|------|------|----------|------|----------|
| FR-38 | 生成工程 | `mgin new <name>` 生成 main/router/controller/service/dao/model/conf/Makefile/go.mod/README | ✅ | `cmd/scaffold.go`、`templates.go` |
| FR-39 | 交互与非交互 | TTY 下逐项问答；CI 下用 flag 或 `-i`；非法值回落默认 | ✅ | `cmd/new.go` |
| FR-40 | 组件选择 | db（7 选，逗号分隔）、mq（4 选）、registry、config-center、jwt/casbin/i18n/sys、health/metrics/otel/LB | ⚠️ `--db postgres/clickhouse` 会生成不可编译的 dao | `cmd/new.go`、`cmd/templates.go` |
| FR-41 | 版本解析 | 显式 `--mgin-version` 优先；否则从 GOPROXY 探测最新 semver，失败回退 `v2.0.0` | ✅ | `cmd/new.go` |
| FR-42 | 覆盖保护 | 目录已存在且无 `--force` 时报错；`--force` 仅覆盖同名文件不删旧文件 | ✅ | `cmd/scaffold.go` |

### 3.9 明确不做（Out of Scope）

| 编号 | 需求 | 原因 |
|------|------|------|
| OOS-01 | S3/对象存储 | 当前无 `pkg/storage`、无 SDK 依赖；示例 YAML 中的 `go.s3` 为未落地设计 |
| OOS-02 | PostgreSQL / ClickHouse / Elasticsearch / Kafka 数据源 | `pkg/db` 无实现，仅脚手架模板提及 |
| OOS-03 | gRPC 服务端/客户端 | 无 proto 与 server 实现 |
| OOS-04 | API 网关、灰度路由、聚合 | 超出框架边界 |
| OOS-05 | OAuth2 / OIDC 授权服务器 | 仅 JWT/Casbin |
| OOS-06 | 分布式锁、集群限流、分布式任务调度 | 需要额外协调组件，当前为单机语义 |
| OOS-07 | 分布式事务（Saga/TCC） | 仅本地事务 |
| OOS-08 | 自动生成 sys 模块 CRUD 与建表 | 仅提供模型 |

---

## 4. 非功能需求

| 编号 | 类别 | 要求 | 现状 |
|------|------|------|------|
| NFR-01 | 兼容性 | v1 公开函数签名不变；仅 import 路径迁移到 `pkg/` | ✅ `logs/` 兼容壳保留一个 release |
| NFR-02 | 可构建性 | `go build ./...` 通过；新增/修改代码 `gofmt` 干净且不引入新的 `go vet` 告警 | ⚠️ 全仓库 `go vet` 有 3 处历史 `unreachable code`，`gofmt -l .` 列出 14 个历史文件 |
| NFR-03 | 启动健壮性 | 单个组件初始化失败不阻断进程（记录首个错误后继续） | ✅ `plugin.InitAll` |
| NFR-04 | 退出安全 | 关闭顺序：日志排空 → 插件逆序 → HTTP Shutdown；单插件 panic 不影响其他插件 | ✅ `plugin.CloseAll` 使用 recover |
| NFR-05 | 并发安全 | 全局注册表（plugin/loadbalancer/breaker/metrics）使用锁或 atomic；请求上下文用协程局部缓存 | ✅ |
| NFR-06 | 性能 | 探针与指标端点无鉴权链路；限流未启用时零额外开销；服务发现结果本地缓存 | ✅ |
| NFR-07 | 可观测性 | 请求可凭 `X-Request-ID`/`traceparent` 串联访问日志与下游调用 | ✅ |
| NFR-08 | 可测试性 | 核心包（health/client/loadbalancer/metrics/otel/errcode/logsink）无外部依赖即可单测 | ✅ 28 个 `_test.go` |
| NFR-09 | 依赖克制 | OTel 不强制引入 SDK/Exporter；熔断器纯标准库 | ✅ |
| NFR-10 | 配置健壮性 | 缺失配置返回零值不 panic；非法 HTTP 状态码修正为 500 | ✅ `errcode.New` |
| NFR-11 | 文档一致性 | README 与 `docs/` 的 API/配置键以代码为准 | 本版已按代码重写 |

---

## 5. 关键业务规则（必须与实现一致）

1. **探针挂载时机**：`/health/*`、`/healthz`、`/metrics` 必须在任何 `Router.Use` 之前注册。
   违反会导致探针被 JWT/Casbin 拦截或限流误伤。
2. **错误码三向映射**：`Code` 是业务码，`HTTPStatus` 是 HTTP 状态；`NoRoute` 返 404、
   `Recovery` 返 500；业务 handler 需自行 `c.JSON(def.HTTPStatus, i18n.ErrorDef(def, ...))`。
3. **i18n 回退**：字典缺失时返回 messageKey 原文，不 panic、不返回空串。
4. **`go.config.used` 匹配**：插件用 `strings.Contains` 子串匹配；值必须是逗号分隔的精确名称。
5. **数据库选择**：`go.job` / `i18n` / `postlog` 均按 `go.config.used` 的优先级选择数据源；
   多库模式必须指定库名（Mongo 例外，默认第一个连接）。
6. **插件顺序**：DB(10) → Redis(15) → Registry(30) → Job(90)；关闭逆序。
7. **负载均衡策略名**：只认注册名；未命中静默回落默认轮询。

---

## 6. 验收标准（发布前必须通过）

### 6.1 框架级

- [ ] `go build ./...` 通过
- [ ] 本次改动文件 `gofmt -l` 无输出（历史文件不强制）
- [ ] 本次改动包 `go vet` 无新增告警（全仓库基线 3 处）
- [ ] 核心包测试通过：`pkg/{health,client,loadbalancer,metrics,otel,errcode,logs,cache,i18n,job,db/dao,registry,utils}`
- [ ] `pkg/logsink` 的 `TestQueueKeepsNewestAndFIFO` 已知失败（断言上限 100，代码为 1024），修复前不计入门禁
- [ ] 最小可运行示例：`NewApp` + `EnableHealth` + `EnableMetrics` + `Run`，三个探针与 `/metrics` 返回预期状态
- [ ] 收到 SIGTERM 后进程在 `shutdownTimeout` 内退出，无 panic
- [ ] 404 返回 HTTP 404，panic 返回 HTTP 500

### 6.2 脚手架级

- [ ] `cd cmd && go build ./...` 通过
- [ ] `mgin new demo --db sqlite` 生成工程后可 `make build` 并启动
- [ ] `--help`、`version`、缺参报错文案正确
- [ ] `--force` 覆盖行为符合 FR-42

### 6.3 文档级

- [ ] README 的配置键与 `pkg/config` 读取的键一致
- [ ] `docs/modules-and-api.md` 的签名与 `go doc` 一致
- [ ] 每张表的字段与 `pkg/models` / `pkg/job/model.go` 一致
- [ ] 未实现能力在 README 第 9 节与本文 OOS 中均有记录

---

## 7. 需求追踪矩阵

| 需求组 | 代码 | 文档 |
|--------|------|------|
| FR-01 ~ FR-04b | `app.go`、`mgin.go` | [architecture.md](architecture.md) §2–3 |
| FR-05 ~ FR-08 | `pkg/config` | [modules-and-api.md](modules-and-api.md) §3 |
| FR-09 ~ FR-14 | `pkg/db`、`pkg/cache`、`pkg/db/dao` | [data-design.md](data-design.md)、[modules-and-api.md](modules-and-api.md) §4 |
| FR-15 ~ FR-20 | `pkg/registry`、`pkg/client`、`pkg/loadbalancer` | [modules-and-api.md](modules-and-api.md) §5–6 |
| FR-21 ~ FR-26 | `pkg/health`、`pkg/metrics`、`pkg/otel`、`pkg/middleware/trace`、`pkg/logsink` | [architecture.md](architecture.md) §6 |
| FR-27 ~ FR-32 | `pkg/middleware/*`、`pkg/casbin` | [modules-and-api.md](modules-and-api.md) §8 |
| FR-33 ~ FR-37 | `pkg/job`、`pkg/i18n`、`pkg/errcode` | [data-design.md](data-design.md) §4–6 |
| FR-38 ~ FR-42 | `cmd/` | [development-setup.md](development-setup.md) §4 |
| NFR-01 ~ NFR-11 | 全仓库 | [testing-strategy.md](testing-strategy.md)、[release-process.md](release-process.md) |

---

## 8. 风险与待决事项

| 编号 | 风险/问题 | 影响 | 建议 |
|------|-----------|------|------|
| R-01 | 文档曾声称 S3、Postgres、ClickHouse、11 类插件 | 使用方按文档接入会失败 | 已在 README/本文标注未实现；后续要么实现，要么清理示例 YAML |
| R-02 | `go.framework.*` 多数开关未接线 | 用户配置后无效果且无提示 | 统一由 `Framework` 驱动，或在启动日志打印"该配置未生效" |
| R-03 | 负载均衡短名静默回落 | 线上实际策略与预期不符 | `Get` 未命中时回退并打印 Warn；脚手架改用注册名 |
| R-04 | `Dao[E]` 接口悬空 | 依赖接口编程的业务无法编译 | 对齐 `MySQLDao` 方法集，或删除接口避免误导 |
| R-05 | `client.CallCtx` 不能取消底层 HTTP | 上游超时后下游连接仍占用 | 升级 `grequests`/改用可取消 client，或明确文档化 |
| R-06 | 多库 MySQL 连接池/调试配置被跳过 | 容量与排障能力下降 | 修正 `m.mysql == nil` 分支 |
| R-07 | `go.config.used` 子串匹配 | 组件名包含关系导致误启用 | 改为切分后精确比较 |
| R-08 | sys 13 张表无迁移 | 业务接入需手写 DDL | 提供迁移命令或 AutoMigrate 示例 |
| R-09 | 定时任务/限流为单机语义 | 多副本重复执行/限流失效 | 文档明确，或引入选主/Redis 计数 |
| R-10 | Polaris 只发现不注册 | 使用 Polaris 的服务无法被调用 | 将 polaris 纳入插件 enabledFn 与 prefix |
