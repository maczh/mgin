# MGin 微服务框架（v2）

> 模块路径：`github.com/maczh/mgin/v2` · Go 1.25.7 · MIT License
>
> 一个**单进程、单 `gin.Engine`、HTTP/HTTPS 同启**的 Go 微服务基础框架：
> 把配置中心、注册发现、服务间调用、数据访问、缓存、日志、鉴权、限流、定时任务、
> 健康探针、Prometheus 指标与 OpenTelemetry 接入收拢为可复用组件，并提供 `mgin new` 脚手架。

本文档面向**第一次接触本项目的开发者**，内容与当前代码一一对应。
发现文档与代码不一致时，以代码为准，并请按 [贡献指南](#贡献指南) 提交修正。

> 近期（2026-10）新增能力：命名**多注册中心**（`go.discovery.registries` + `client.Options.Registry`）、
> **Nacos v1/v2/v3**（配置中心与注册中心各自 `apiversion`，支持 token 或用户名密码鉴权）、
> etcd/Consul 的 **namespace / group / cluster** 作用域隔离。详见 [§2.3](#23-服务发现与服务调用)、
> [§5.4](#54-服务间调用含负载均衡与韧性)、[§6.1](#61-主配置-applicationyml)、[§8](#8-常见问题与故障排查)。

---

## 目录

- [1. 项目概述与目标](#1-项目概述与目标)
- [2. 核心功能清单](#2-核心功能清单)
- [3. 技术架构与依赖说明](#3-技术架构与依赖说明)
- [4. 目录结构解析](#4-目录结构解析)
- [5. 快速开始与使用示例](#5-快速开始与使用示例)
- [6. 配置项说明](#6-配置项说明)
- [7. 测试、构建与静态检查](#7-测试构建与静态检查)
- [8. 常见问题与故障排查](#8-常见问题与故障排查)
- [9. 已知限制与未实现能力](#9-已知限制与未实现能力)
- [10. 贡献指南](#10-贡献指南)
- [11. 许可证](#11-许可证)
- [12. 文档导航](#12-文档导航)

---

## 1. 项目概述与目标

### 1.1 一句话定位

mgin 不是"Go 版 Spring Cloud"，而是**面向单进程服务的框架 + 基础设施胶水层**：
一个进程内同时具备配置、发现、调用、数据、日志、鉴权、任务与可观测性，业务代码只写
`controller → service → dao`。

### 1.2 设计目标

| 目标 | 落地方式 |
|------|----------|
| 组件生命周期可插拔 | `pkg/plugin` 统一 `Plugin` 接口 + 注册表驱动 `InitAll/CloseAll/HealthAll` |
| 对存量项目零破坏 | v1 的 `mgin.NewApp`、`mgin.Use`、`MginPlugin`、`client.Call` 等签名保留 |
| 配置与业务解耦 | `go.application.*`（业务）/ `go.config.*`（数据源）/ `go.framework.*`（框架开关）/ `go.runtime.*`（运行期元数据） |
| 生产可上线 | K8s 三探针、优雅关闭、Prometheus 指标、W3C `traceparent` 透传、调用熔断与重试 |
| 快速起步 | `mgin new` 一次生成可编译、可运行、带 Makefile 的工程骨架 |

### 1.3 适用与不适用场景

**适用**：单体或"小规模微服务 + HTTP/JSON 通信"的 Go 服务；需要开箱即用的配置中心 / 注册发现 /
定时任务 / 多数据源 / 鉴权 / 日志采集的后台系统。

**不适用**：需要 gRPC 服务端、API 网关、OAuth2/OIDC 授权服务器、跨语言服务网格、强一致分布式事务的场景——
这些能力当前代码中**未实现**（详见 [第 9 节](#9-已知限制与未实现能力)）。

### 1.4 版本与分支

| 项 | 值 |
|----|----|
| Go module | `github.com/maczh/mgin/v2` |
| Go 版本 | `go 1.25.7`（`go.mod`） |
| 脚手架 module | `github.com/maczh/mgin/v2/cmd/mgin`（**独立 module**，位于 `cmd/`） |
| 脚手架版本 | `2.1.0`（`cmd/templates.go` 中 `toolVersion`） |
| 许可证 | MIT（2022 maczh） |

---

## 2. 核心功能清单

图例：✅ 已实现并可用 · ⚠️ 可用但有明确限制 · ❌ 仅有模型/注释，未实现

### 2.1 应用与生命周期

| 能力 | 入口 | 状态 |
|------|------|------|
| 应用创建、版本号 `-v`、配置文件 `-f` | `mgin.NewApp(configFile, appName, version, xlang)` | ✅ |
| 统一初始化（配置 → 日志 → 注册中心 → 插件） | `mgin.Init(configFile)` | ✅ |
| 启动 HTTP/HTTPS 双服务 | `App.Run()` | ✅ |
| 优雅关闭（逐个 `Shutdown`，默认 5s） | `App.Run()` + `go.application.shutdownTimeout` | ✅ |
| 旧版组件适配 | `mgin.Use` / `mgin.UsePlugin` / `MginPlugin` | ✅ 兼容保留 |
| 统一插件注册 | `plugin.Register/InitAll/CloseAll/HealthAll` | ✅ 6 个内置插件 |

内置插件（`pkg/plugin/builtins.go`）：`mysql`(10)、`sqlite`(10)、`mongodb`(10)、`redis`(15)、`registry`(30)、`job`(90)。

### 2.2 数据访问与存储

| 能力 | 包 | 状态 |
|------|----|------|
| MySQL 单库/多库、连接池、断线重建 | `pkg/db/mysql` | ✅ |
| SQLite 单文件库 | `pkg/db/sqlite` | ✅ |
| MongoDB 单库/多库、连接 Copy/Return | `pkg/db/mongo` | ✅ |
| Redis 单库/多库、`PSubscribe` 自动重连 | `pkg/db/redis` | ✅ |
| 泛型 DAO（MySQL） | `pkg/db/dao.MySQLDao[E]` | ⚠️ 方法集与 `Dao[E]` 接口不一致 |
| 泛型 DAO（Mongo） | `pkg/db/dao.MgoDao[E]` | ⚠️ 同上 |
| 泛型事务 | `db.MySQLTx` / `db.TransactionOn` | ✅ |
| DAO 查询结果缓存（Redis） | `dao.CacheConfig` + `RedisCache` | ⚠️ 写操作按前缀失效，事务内不清理 |
| 进程内缓存 / 磁盘缓存 | `pkg/cache`（memcache / bitcask） | ✅ |
| PostgreSQL / ClickHouse / Elasticsearch / Kafka | — | ❌ `pkg/db` 无对应实现 |

### 2.3 服务发现与服务调用

支持命名多注册中心。多个条目可以共享同一个 `prefix`（同一 Nacos/etcd/Consul
集群），并分别覆盖 `namespace`、`group`、`clusterName`。调用方通过
`client.Options.Registry` 选择目标；留空使用 `go.discovery.defaultRegistry`。
每个命名实例拥有独立的发现缓存，v2 的 `GetServices` 负载均衡主路径和
`GetServiceURL` 兼容路径都会使用同一个目标注册中心。

Nacos 作为配置中心和注册中心时，`apiversion` 支持 `v1`、`v2`、`v3`；
框架会使用对应 Client OpenAPI 路径、响应结构和心跳方式。鉴权支持直接配置
`accessToken`，或通过 `username/password` 登录获取 token。

| 能力 | 包 / API | 状态 |
|------|----------|------|
| Nacos / etcd / Consul / Polaris 注册与发现 | `pkg/registry` | ✅ Polaris 只发现不注册（插件未接入） |
| 单实例获取（v1 兼容） | `RegistryClient.GetServiceURL` | ✅ |
| 多实例获取 | `RegistryClient.GetServices` | ✅ |
| 命名多注册中心 | `go.discovery.registries` + `client.Options.Registry` | ✅ 每实例独立缓存；未知名不回退默认实例 |
| 作用域隔离 | `namespace` / `group` / `clusterName` 覆盖 | ✅ etcd 编入 key，Consul 走原生 namespace + 标签过滤 |
| Nacos 版本 | `apiversion`：`v1` / `v2` / `v3` | ✅ 配置中心与注册中心分别配置 |
| 注册结果本地缓存 | `NewCachedRegistry`（TTL + 后台刷新 + 防击穿） | ✅ |
| HTTP 调用（`x-form`/`json`/`restful`/`file`） | `client.Call` / `client.CallCtx` | ✅ |
| 超时 + 指数退避 + 抖动重试 + 熔断 | `client.CallResilient` / `CallResilientCtx` | ⚠️ 超时为 goroutine 兜底，底层 HTTP 不可取消 |
| 客户端负载均衡 | `pkg/loadbalancer` | ✅ 4 策略；策略短名不生效（见 FAQ） |
| per-instance 熔断 | `client.GetBreaker(service+"@"+host)` | ✅ |

### 2.4 可观测性与运维

| 能力 | 端点 / API | 状态 |
|------|-----------|------|
| 存活/就绪/启动探针 | `GET /health/live`、`/health/ready`、`/health/startup`、`/healthz` | ✅ 默认关闭 |
| 依赖健康检查 | 读 `go.config.used`，仅 `mysql`/`mongodb`/`redis` | ⚠️ SQLite 不参与 |
| Prometheus 指标 | `GET /metrics` | ✅ 需显式 `App.EnableMetrics()` |
| HTTP 指标采集中间件 | `metrics.Middleware()` | ⚠️ 需业务自行 `Use` |
| 链路头透传 | `middleware/trace`（`traceparent` + `X-Trace-Id` + `X-Request-ID`） | ✅ |
| OpenTelemetry 抽象 | `otel.SetTracerProvider/StartSpan/IsEnabled` | ⚠️ 不内置 SDK/Exporter，业务自接 |
| 访问日志异步落库/采集 | `middleware/postlog` + `pkg/logsink` | ✅ |

### 2.5 安全、限流与任务

| 能力 | 包 | 状态 |
|------|----|------|
| JWT 鉴权 | `middleware/jwt` | ✅ |
| Casbin 接口级鉴权 | `pkg/casbin` + `middleware/casbin` | ✅ 基于 `casbin_rule` 表 |
| Session | `middleware/session` | ✅ |
| XSS 净化 | `middleware/xss` | ✅ |
| IP/CIDR 白名单 | `middleware/iplimit` | ✅ |
| 最大并发信号量 | `middleware/limit` | ✅ |
| 单机限流（令牌桶/滑动窗口/并发数） | `middleware/ratelimit` | ⚠️ 单实例内存计数，非集群限流 |
| 定时任务（类 xxl-job） | `pkg/job` | ⚠️ 单机调度，多实例会重复触发 |
| 国际化字典 | `pkg/i18n`（数据库表 `<appName>_i18n`） | ✅ 需 MySQL/SQLite |
| 统一错误码三向映射 | `pkg/errcode` + `i18n.ErrorDef` | ✅ |

### 2.6 工程化

| 能力 | 入口 | 状态 |
|------|------|------|
| 脚手架生成 | `mgin new <name> [flags]` | ✅ 支持交互/非交互 |
| 生成工程 Makefile | `tidy/build/linux/build-multi-os/test/test-race/cover/lint/clean/run` | ✅ |
| 生成工程版本注入 | `-ldflags -X main.Version/BuildTime/GitHash` | ✅ |
| 框架自身构建脚本 / CI | — | ❌ 仓库无根 Makefile、无 CI 配置 |

---

## 3. 技术架构与依赖说明

### 3.1 分层与进程模型

```
                        ┌──────────────────────────────────────────┐
   HTTP/HTTPS  :port    │              gin.Engine                  │
   ──────────────────►  │  /health/*  /metrics  /healthz  (最先挂) │
                        │  trace → postlog → cors → xlang →        │
                        │  ratelimit → recovery → 业务路由         │
                        └───────────────┬──────────────────────────┘
                                        │ controller
                        ┌───────────────▼──────────────────────────┐
                        │  service（ctx 优先）                     │
                        │  client.Call / CallResilient ──► 远程服务│
                        └───────────────┬──────────────────────────┘
                                        │
                ┌───────────────────────┼───────────────────────┐
                ▼                       ▼                       ▼
        pkg/db (mysql/sqlite/     pkg/cache (mem/disk)     pkg/job / pkg/casbin
         mongo/redis) + dao                                 / pkg/i18n
                │                       │                       │
                └───────────────────────┴───────────────────────┘
                                        │
                              pkg/plugin 注册表（Order 升序 Init / 逆序 Close）
```

关键约束（与代码一致）：

1. **单进程、单 `gin.Engine`**，HTTP 与 HTTPS 共用同一个 `Router`。
2. `/health/*`、`/healthz`、`/metrics` 必须在任何 `Router.Use(...)` **之前**注册。
   原因是 Gin 在路由注册时已把当前中间件链快照进节点，后注册的中间件不会回溯生效；
   提前注册可保证探针不被 JWT/Casbin 拦截返回 401。
3. 组件初始化顺序由 `Plugin.Order` 决定：DB(10) → Redis(15) → Registry(30) → Job(90)；
   关闭时逆序，且单个插件 panic 被 `recover` 隔离。

### 3.2 启动与关闭时序

```
NewApp
 ├─ 解析 -f / -v
 ├─ Init: config.Load(application.yml)
 │        └─ 按 go.config.server_type 拉取 <prefix>-<env>.yml（nacos/consul/etcd/polaris/springconfig/file）
 ├─ 填 go.runtime.*（StartedAt/Pid/GoVersion）
 ├─ plugin.RegisterBuiltins() → plugin.InitAll()  （按 Order，失败只记录不中断）
 ├─ health.SetVersion(version)
 ├─ 每 5 分钟 checkAll 巡检（ticker）
 ├─ gin.SetMode(debug/release)
 ├─ i18n.Init()（可选）
 └─ baseRouter: /health → /metrics → 中间件 → NoRoute → /i18n → /job

Run
 ├─ 监听 :port（HTTP）与 :port_ssl（HTTPS，Cert 非空时）
 ├─ go.health.autoStarted=true 时自动 MarkStarted
 ├─ 阻塞等待 SIGINT/SIGHUP/SIGTERM/SIGQUIT
 └─ 停 ticker → SafeExit（排空日志 → CloseAll 逆序）→ 逐个 server.Shutdown(超时)
```

### 3.3 目录分层规则

| 层 | 路径 | 可被谁依赖 |
|----|------|------------|
| 框架入口 | `mgin.go`、`app.go` | 业务 `main` |
| 公开契约 | `pkg/**` | 业务与第三方扩展 |
| 框架私有 | `internal/**` | 仅本 module（当前仅占位） |
| 兼容壳 | `logs/**` | 存量项目（计划删除） |
| 工具 | `cmd/**` | 独立 module，不参与框架构建 |

### 3.4 依赖说明（直接依赖）

| 依赖 | 版本 | 用途 |
|------|------|------|
| `github.com/gin-gonic/gin` | v1.11.0 | HTTP 框架 |
| `gorm.io/gorm` | v1.31.0 | ORM |
| `gorm.io/driver/mysql` / `driver/sqlite` | v1.6.0 / v1.6.0 | MySQL / SQLite 驱动 |
| `github.com/glebarez/sqlite` | v1.7.0 | 纯 Go SQLite（i18n 等） |
| `go.mongodb.org/mongo-driver` + `github.com/maczh/mgo` | v1.17.4 / v1.0.4 | MongoDB |
| `github.com/go-redis/redis/v7` | v7.4.1 | Redis 客户端 |
| `github.com/casbin/casbin/v2` + `gorm-adapter/v3` | v2.105.0 / v3.32.0 | 接口级鉴权 |
| `github.com/dgrijalva/jwt-go` | v3.2.0+incompatible | JWT（历史选型，注意已停止维护） |
| `github.com/knadh/koanf` | v1.4.3 | 多源配置解析（`.` 分隔） |
| `github.com/hashicorp/consul/api` | v1.13.0 | Consul 注册/发现 |
| `go.etcd.io/etcd/client/v3` | v3.5.4 | etcd 注册/发现/配置 |
| `github.com/prometheus/client_golang` | v1.19.1 | 指标 |
| `go.opentelemetry.io/otel` + `otel/trace` | v1.26.0 | 链路抽象（**不含 SDK/Exporter**） |
| `github.com/robfig/cron/v3` | v3.0.1 | 定时任务 |
| `github.com/levigross/grequests` | 2025-06-06 | 服务间 HTTP 调用 |
| `github.com/json-iterator/go` | v1.1.12 | 高性能 JSON |
| `github.com/microcosm-cc/bluemonday` | v1.0.27 | XSS 净化 |
| `git.mills.io/prologic/bitcask` | v1.0.2 | 磁盘 KV 缓存 |
| `github.com/pkg/sftp` + `golang.org/x/crypto` | v1.13.5 / v0.40.0 | SFTP 工具 |
| `github.com/go-session/session` | v3.1.2 | Session 中间件 |

> `go.mod` 的 indirect 段包含 `gorm.io/driver/postgres`、`driver/sqlserver`、`pgx` 等，
> 但 **`pkg/db` 不引用它们**，属传递依赖，不代表框架支持对应数据库。

---

## 4. 目录结构解析

```
mgin/                                  # 框架仓库根（github.com/maczh/mgin/v2）
├── mgin.go                            # 框架初始化入口：Init / Use / UsePlugin / MginPlugin
├── app.go                             # 应用生命周期：NewApp / 中间件装配 / Run / 优雅关闭
├── application.example.yml            # 配置样例（含部分尚未被代码消费的节点，见第 6 节）
├── casbin.conf                        # Casbin 模型（keyMatch2 的平铺 ACL）
├── go.mod / go.sum
├── LICENSE                            # MIT
├── README.md / README_CN.md / README_en.md / sdk_document.md
│
├── cmd/                               # 【独立 module】脚手架 CLI
│   ├── go.mod                         # module github.com/maczh/mgin/v2/cmd/mgin
│   ├── mgin.go                        # 子命令分发：new / version / help
│   ├── new.go                         # flag 解析、交互问答、mgin 版本探测
│   ├── scaffold.go                    # 目录创建、YML/README/Makefile 落盘
│   └── templates.go                   # 代码模板（main/router/controller/service/dao/Makefile）
│
├── docs/                              # 设计、需求、迁移与运维文档（见第 12 节）
│
├── internal/
│   └── doc.go                         # 预留：框架私有实现（当前未使用）
│
├── logs/
│   └── alias.go                       # pkg/logs 的兼容别名壳（计划下个 release 删除）
│
└── pkg/                               # 【对外可导入】所有公开 API
    ├── cache/                         # ICache 接口 + memcache + diskcache(bitcask)
    ├── casbin/                        # SyncedCachedEnforcer 封装、策略增删、casbin_rule
    ├── client/                        # 服务间 HTTP 调用 + 熔断 + 超时/重试 + LB 选点
    ├── config/                        # koanf 配置加载、GetConfig*、运行时元数据
    ├── db/
    │   ├── db.go                      # 门面：db.Mysql / db.Mongo / db.Redis / db.Sqlite
    │   ├── transaction.go             # 泛型事务 MySQLTx / TransactionOn
    │   ├── mysql/                     # 单库/多库、连接池、断线重建
    │   ├── mongo/                     # 单库/多库、连接 Copy/Return
    │   ├── redis/                     # 单库/多库、Pub/Sub、连接池
    │   ├── sqlite/                    # 单文件库
    │   └── dao/                       # MySQLDao / MgoDao / RedisCache / Dao[E] 接口
    ├── errcode/                       # Definition + 15 个内置错误码 + 模块常量
    ├── health/                        # /health/{live,ready,startup} + /healthz
    ├── i18n/                          # 数据库字典（表 <appName>_i18n）+ CRUD API
    ├── job/                           # 定时任务：模型、Store、调度器、HTTP API
    ├── loadbalancer/                  # 4 种策略 + 全局注册表
    ├── logs/                          # 日志门面（{} 占位符）+ 多目标输出 + VictoriaLogs
    ├── logsink/                       # 异步日志队列（Record / PostLog 两套）
    ├── metrics/                       # 6 个 Prometheus 指标 + /metrics Handler
    ├── middleware/
    │   ├── trace/                     # traceparent / X-Trace-Id / X-Request-ID
    │   ├── postlog/                   # 接口访问日志采集与投递
    │   ├── cors/ xlang/ jwt/ casbin/ session/
    │   ├── limit/ iplimit/ ratelimit/ # 并发、IP 白名单、三算法限流
    │   └── xss/                       # bluemonday 净化
    ├── models/                        # Result[T] / PostLog / Timestamp
    │   └── sys/                       # 13 张内置系统表模型 + request DTO + vo
    ├── otel/                          # 链路抽象（SetTracerProvider/StartSpan/IsEnabled）
    ├── plugin/                        # Plugin 接口 + 包级注册表 + 6 个内置插件
    ├── registry/                      # RegistryClient + 缓存装饰器 + 多注册中心集合 + 4 个后端
    │   └── nacos/ etcd/ consul/ polaris/
    ├── utils/                         # 46 个工具文件（加解密/JWT/SFTP/条件构造/IP/…）
    └── vo/                            # Response[T] / ResponsePage[T] 统一出参
```

---

## 5. 快速开始与使用示例

### 5.1 环境要求

- Go **1.25.7 及以上**（`go.mod` 指定）
- 可选：MySQL 5.7+/8.0、Redis、MongoDB、Nacos/Consul/etcd（按启用组件准备）
- 无需 Node.js；仓库无前端构建

### 5.2 方式一：脚手架生成工程（推荐）

```bash
# 在仓库内构建 CLI（独立 module）
cd cmd && go build -o "$(go env GOPATH)/bin/mgin" .
# 或安装已发布版本
# go install github.com/maczh/mgin/v2/cmd/mgin@latest

mgin version            # 输出：mgin scaffold 2.1.0
mgin help               # 查看全部参数
```

非交互式生成：

```bash
mgin new order \
  --module github.com/acme/order \
  --output /home/acme/work \
  --port 8088 \
  --db mysql \
  --registry nacos \
  --config-center none \
  --jwt \
  --health --metrics \
  --mgin-version v2.0.0

cd /home/acme/work/order
make tidy build
./order              # Makefile 的 BINARY 为工程名，产物在工程根目录
```

交互式：`mgin new order -i`（TTY 下直接 `mgin new order` 也会进入问答）。

> 说明：`--loadbalancer` 的取值 `round/random/least/consistent` 会写进生成工程的 YAML，
> 但运行期 `client.Options.LoadBalancer` 只认 `round_robin/random/least_connections/consistent_hash`。
> 需要负载均衡时，请在调用处显式传注册名（见 5.4）。

### 5.3 方式二：在已有项目中直接引入

```go
package main

import (
    "github.com/gin-gonic/gin"
    "github.com/maczh/mgin/v2"
    "github.com/maczh/mgin/v2/pkg/metrics"
)

func main() {
    app := mgin.NewApp("conf/application.yml", "order", "1.0.0", false)

    // 探针与指标必须在业务中间件之前启用
    app.EnableHealth()
    app.EnableMetrics()
    app.Router.Use(metrics.Middleware()) // 采集 HTTP 指标

    app.Router.GET("/api/v1/ping", func(c *gin.Context) {
        c.JSON(200, gin.H{"msg": "pong"})
    })

    app.MarkHealthStarted() // 初始化完成后调用，使 /health/startup 返回 200
    app.Run()
}
```

```bash
curl -i http://localhost:8080/health/live     # 200
curl -i http://localhost:8080/health/ready    # 200 / 503（依赖异常）
curl    http://localhost:8080/metrics | head
```

### 5.4 服务间调用（含负载均衡与韧性）

```go
import (
    "time"
    "github.com/maczh/mgin/v2/pkg/client"
)

// 1) 最简调用（POST 缺省，协议取 go.discovery.callType）
body, err := client.Call("user-service", "/api/v1/users/1", &client.Options{Method: "GET"})

// 2) 带 context + 指定负载均衡策略（注册名，非短名）
body, err = client.CallCtx(ctx, "user-service", "/api/v1/users/{id}", &client.Options{
    Method:       "GET",
    Protocol:     client.CONTENT_TYPE_JSON,
    Path:         map[string]string{"id": "1"},
    LoadBalancer: "consistent_hash",
})

// 3) 超时 + 重试 + 熔断
body, err = client.CallResilientCtx(ctx, "user-service", "/api/v1/users/1", &client.Options{
    Method: "GET",
}, &client.ResilienceOptions{
    Timeout:    800 * time.Millisecond,
    MaxRetries: 2,
    BaseDelay:  100 * time.Millisecond,
    MaxDelay:   2 * time.Second,
    Breaker:    client.GetBreaker("user-service"),
})
```

可用的负载均衡注册名：`round_robin`（默认）、`random`、`least_connections`、`consistent_hash`。
`least_connections` 需业务在请求前后调用 `loadbalancer.LeastConnectionsLB.Inc(host)` 才有真实计数，
否则恒选第一个实例。

多注册中心场景用 `Options.Registry` 指定目标实例（取值来自 `go.discovery.registries[].name`），留空使用 `defaultRegistry`：

```go
body, err = client.CallCtx(ctx, "user-service", "/api/v1/users/1", &client.Options{
    Method:       "GET",
    Registry:     "iot",          // 留空 = go.discovery.defaultRegistry
    Group:        "iot",          // 仍覆盖该实例配置的 group
    LoadBalancer: "round_robin",
})
```

`Registry` 同时作用于负载均衡主路径（`GetServicesFrom`）与单实例兼容路径（`GetServiceURLFrom`）；
一致性哈希 key 与 per-instance 熔断 key（`breaker(registry@service@host)`）都带注册中心名，
不同注册中心的同名服务互不影响。名称拼错时不会回退默认实例：`GetServicesFrom` 返回错误，
框架回退兼容路径后仍取不到 host，最终报「微服务获取 X 服务主机IP端口失败」。

### 5.5 数据访问与事务

```go
import (
    "context"
    "github.com/maczh/mgin/v2/pkg/db"
    "github.com/maczh/mgin/v2/pkg/db/dao"
    "gorm.io/gorm"
)

type Product struct {
    ID    uint   `gorm:"primaryKey"`
    Name  string `gorm:"size:100"`
}

func (Product) TableName() string { return "product" }

var productDao = &dao.MySQLDao[Product]{}

func ListProducts(ctx context.Context) ([]Product, error) {
    return productDao.WithContext(&ctx).All(Product{})
}

func CreateWithTx(ctx context.Context, p *Product) error {
    _, err := db.MySQLTx(ctx, func(tx *gorm.DB) (any, error) {
        return nil, (&dao.MySQLDao[Product]{}).WithTx(tx).Create(p)
    })
    return err
}
```

多数据源时：`go.data.mysql.multidb=true` + `dbNames`，DAO 需设置 `Tag` 返回库名：
`&dao.MySQLDao[Product]{Tag: func() string { return "order_db" }}`。

### 5.6 统一错误码 + i18n

```go
var ErrStockShortage = errcode.New("product", 4001, 409, "product.stock.shortage")

func Buy(c *gin.Context) {
    def := ErrStockShortage
    c.JSON(def.HTTPStatus, i18n.ErrorDef(def, "sku-1")) // 409 + {"status":4001,"msg":"..."}
}
```

i18n 未启用或字典缺失时，`Format` 会回退为 **messageKey 原文**（例如 `product.stock.shortage`），
不会 panic。要让文案生效：启用 i18n 并在 `<appName>_i18n` 表插入 `message_id='product.stock.shortage'` 的记录。

### 5.7 定时任务

```go
import "github.com/maczh/mgin/v2/pkg/job"

job.Register("syncOrder", func(ctx *job.Context) error {
    ctx.Log("开始同步，参数=%v", ctx.Param)
    return nil
})

// 配置 go.job.enabled=true 后：
//   POST /job               新建任务（handlerName=syncOrder）
//   POST /job/:id/start     启动
//   POST /job/:id/trigger   手动触发
```

### 5.8 限流

```yaml
go:
  ratelimit:
    enabled: true
    whitelist: ["/health"]
    rules:
      - name: "登录按 IP"
        path: "/api/login"
        methods: ["POST"]
        algorithm: "sliding_window"
        dimension: "ip"
        rate: 5
        window: 60
        httpStatus: 429
```

框架在 `go.ratelimit.enabled=true` 时自动挂载中间件；也可编程式使用 `ratelimit.Allow(key, rule)`。

---

## 6. 配置项说明

配置解析入口：`config.Config.Init()`（`pkg/config/configure.go`），使用 koanf，**分隔符为 `.`**。
`GetConfigXxx("go.application.port")` 即读取 YAML 的 `go → application → port`。

### 6.1 主配置 `application.yml`

#### `go.application.*`（业务信息）

| 键 | 类型 | 默认 | 说明 |
|----|------|------|------|
| `name` | string | — | 应用名（注册中心服务名、日志字段、i18n 表名前缀） |
| `project` | string | `DEFAULT` | 项目/分组名，注册中心与配置中心使用 |
| `port` | int | 0 | HTTP 端口；`<=0` 不启动 HTTP |
| `port_ssl` | int | 0 | HTTPS 端口（`cert` 非空时生效） |
| `cert` / `key` | string | — | 证书与私钥文件名（相对可执行文件目录） |
| `debug` | bool | false | `true` 时 gin 为 debug 模式，且日志强制 debug 级别 |
| `ip` | string | — | 本机对外 IP（注册使用） |
| `shutdownTimeout` | int | 5 | 每个 server 的优雅关闭超时（秒），`<=0` 回退 5 |

#### `go.config.*`（配置中心与组件白名单）

| 键 | 类型 | 说明 |
|----|------|------|
| `server` | string | 配置中心地址 |
| `server_type` | string | `nacos` / `consul` / `etcd` / `polaris` / `springconfig` / `file` / 其他 |
| `token` | string | Polaris Token |
| `apiversion` | string | Nacos 配置中心接口版本：`v1`/`v2`/`v3`，默认 `v1`（写 `1`/`2`/`3` 等价） |
| `namespace` | string | Nacos 配置中心 namespaceId（v1 以 `tenant` 传参） |
| `username` / `password` | string | Nacos 鉴权账号；`accessToken` 为空时才用于登录换取 token |
| `accessToken` | string | 直接使用已有 token，优先级高于用户名密码 |
| `path` | string | `file` 模式下的配置目录 |
| `env` | string | 环境名，决定子配置文件 `{prefix}-{env}.yml` |
| `used` | string | 启用组件，逗号分隔（**子串匹配**，见 FAQ） |
| `prefix.mysql` / `.postgres` / `.mongodb` / `.redis` / `.clickhouse` / `.elasticsearch` / `.kafka` / `.sqlite` / `.nacos` / `.etcd` / `.consul` | string | 各组件子配置文件名前缀 |

> `prefix.postgres` 虽被解析，但内置插件不读取；`prefix` 下没有 `polaris` 字段。

#### `go.logger.*`（运行日志）

| 键 | 类型 | 默认 | 说明 |
|----|------|------|------|
| `level` | string | `error` | `debug`/`info`/`warn`/`error`，每次输出时实时读取（支持热更新） |
| `out` | string | — | 逗号分隔：`console`、`file` |
| `file` | string | — | 文件输出目录（按天切分 `<file>.YYYY-MM-DD.log`） |
| `outputs[]` | list | — | 非空时替代 `out`/`file`；元素：`target`、`format`(json/text)、`fields`、`logtext`、`file` |

#### `go.log.*`（接口日志与采集）

| 键 | 说明 |
|----|------|
| `db` | 接口日志使用的 Mongo 连接名 |
| `req` | 请求日志集合名（**同时**被读入 `RequestTableName` 与 `Req`） |
| `call` | 调用日志集合名 |
| `dbName` | 多库模式下日志库名 |
| `get` | GET 响应输出策略：`full` / `line` / `off` |
| `api` | API 日志集合名 |
| `kafka.use` / `kafka.topic` | Kafka 日志输出（topic 缺省 = 应用名） |
| `sink.queue` | 每个 handler 队列长度，默认 100（上限 100） |
| `sink.workers` | 恒为 1（配置其他值会被重置） |
| `sink.shutdown` | 退出排空等待毫秒，默认 3000 |
| `victorialogs.url` | VictoriaLogs 采集地址（必须 HTTPS 且应用名合法） |
| `victorialogs.api_token` | 采集 token |
| `victorialogs.api_crt` | 采集服务证书 PEM |
| `victorialogs.request` | 是否上报接口日志 |
| `victorialogs.client` | 是否上报 `client.Call` 调用结果 |
| `victorialogs.timeout` | 单次请求超时（毫秒） |

#### `go.discovery.*`

| 键 | 默认 | 说明 |
|----|------|------|
| `registry` | — | `nacos`/`consul`/`etcd`/`polaris`；`registries` 非空时被忽略 |
| `registries[]` | — | 命名注册中心列表：`name`（必填，唯一）、`type`、`prefix`（子配置前缀，默认同 `type`）、`namespace`、`group`、`clusterName` |
| `defaultRegistry` | 列表首项 | `client.Options.Registry` 留空时使用的实例名 |
| `callType` | `x-form` | `client.Options.Protocol` 缺省值 |
| `cacheTTL` | 60 | 服务发现缓存 TTL（秒） |
| `cacheRefresh` | 30 | 后台刷新间隔（秒，必须小于 TTL，否则取 TTL/2） |

#### 鉴权与安全

| 键 | 默认 | 说明 |
|----|------|------|
| `go.jwt.secret` | — | JWT 签名密钥 |
| `go.casbin.enabled` | false | 是否启用 Casbin 中间件 |
| `go.casbin.model_file` | `rbac_model.conf` | 模型文件；不含路径分隔符时拼配置目录 |
| `go.sys.enabled` / `initdb` | false | 内置 sys 模块开关（模型存在，业务需自行建表） |
| `go.sys.baseUri` | `/api/v1` | sys 基础路径 |
| `go.sys.swagger.enabled` / `uri` | false / `/swagger/sys` | Swagger 路径（白名单使用） |
| `go.sys.casbin` | false | sys 模块内是否启用 Casbin |
| `go.cors.headers` | — | 逗号分隔的额外允许头（直接由 `GetConfigString` 读取） |

#### `go.i18n.*`

| 键 | 默认 | 说明 |
|----|------|------|
| `enabled` | false | 启用字典与语言中间件 |
| `default` | `zh-cn` | 默认语言（`X-Lang` 缺省值） |
| `dbName` | — | MySQL 多库连接名（留空取第一个） |
| `sqlite` | `<appName>_i18n.db` | 无 MySQL 时的独立 SQLite 文件 |
| `api.enabled` | false | 自动挂载 `/i18n/{list,get,save,update,delete}` |

#### 可观测性

| 键 | 默认 | 说明 |
|----|------|------|
| `go.health.enabled` | false | 挂载 `/health/live`、`/health/ready`、`/health/startup` 与 `/healthz` |
| `go.health.autoStarted` | false | 监听启动后自动 `MarkStarted()` |
| `go.metrics.enabled` | false | **仅当**已调用 `App.EnableMetrics()` 时才参与判断 |

#### `go.ratelimit.*`

| 键 | 默认 | 说明 |
|----|------|------|
| `enabled` | false | 自动挂载限流中间件 |
| `idleTimeout` | 600 | 空闲限流器回收（秒） |
| `whitelist[]` | — | 路径前缀白名单 |
| `whiteIps[]` | — | IP 白名单 |
| `headers` | false | 是否输出 `X-RateLimit-*` / `Retry-After` |
| `rules[].name/path/methods` | — | 规则名、精确/`/*` 前缀/Gin `:id` 模板、方法列表 |
| `rules[].algorithm` | `token_bucket` | `token_bucket` / `sliding_window` / `concurrency` |
| `rules[].dimension` | `global` | `global` / `ip` / `path` / `ip_path` / `header` |
| `rules[].headerKey` | `Authorization` | `dimension=header` 时的键 |
| `rules[].rate/burst/window/maxConcurrent/wait/waitTimeoutMs` | — | 各算法参数 |
| `rules[].code/message/httpStatus` | 1011 / — / 200 | 触发时返回的业务码、文案、HTTP 状态 |

#### `go.job.*`

| 键 | 默认 | 说明 |
|----|------|------|
| `enabled` | false | 启动调度器 |
| `dbName` | — | 多库模式指定库 |
| `initdb` | **true** | 自动 `AutoMigrate` 两张表 |
| `tablePrefix` | `mgin_` | 表名前缀 |
| `scanInterval` | 1 | 调度扫描间隔（秒） |
| `refreshInterval` | 30 | 从库同步任务配置间隔（秒） |
| `logRetainDays` | 30 | 日志保留天数，0 不清理 |
| `maxConcurrent` | 50 | 全局最大并发 |
| `maxSerialQueue` | 10 | `serial` 阻塞策略排队上限 |
| `timezone` | Local | 例如 `Asia/Shanghai` |

#### `go.framework.*`（框架元配置）

| 键 | 默认 | 实际是否生效 |
|----|------|--------------|
| `healthEnabled` | false | ❌ 未被消费（请用 `go.health.enabled`） |
| `i18nEnabled` | false | ✅ 与 `go.i18n.enabled`、`NewApp(..., true)` 等效 |
| `ratelimitEnabled` | false | ❌ 未被消费（请用 `go.ratelimit.enabled`） |
| `recoverEnabled` | false | ❌ 未被消费（Recovery 恒挂载） |
| `logRequests` | false | ❌ 未被消费（postlog 恒挂载） |

#### `go.runtime.*`

不读取 YAML，由框架与构建脚本填充：`startedAt`、`pid`、`goVersion`（`mgin.Init` 写入），
`commitHash`、`buildTime`（建议由 `-ldflags` 注入到业务变量后再赋值）。

### 6.2 子配置 `{prefix}-{env}.yml`

由 `go.config.prefix.<组件>` + `go.config.env` 拼出文件名，再按 `go.config.server_type` 从
本地/配置中心加载。内部键为：

```yaml
go:
  data:
    mysql:
      multidb: false
      dbNames: "db1,db2"
      db1: "user:pass@tcp(127.0.0.1:3306)/db1?charset=utf8mb4&parseTime=True&loc=Local"
      mysql: "user:pass@tcp(127.0.0.1:3306)/db?charset=utf8mb4&parseTime=True&loc=Local"
    mysql_debug: false
    mysql_pool: { max: 100, total: 50, timeout: 30, life: 3600 }

    mongodb:
      multidb: false
      dbNames: "db1"
      db1: { uri: "mongodb://127.0.0.1:27017", db: "demo" }
      uri: "mongodb://127.0.0.1:27017"
      db: "demo"
    mongo_pool: { max: 100 }

    redis:
      multidb: false
      dbNames: "0"
      uri: "redis://127.0.0.1:6379/0"   # 或 host + port + password + database
    redis_pool: { max: 100, min: 10, idleTimeout: 300, timeout: 5 }

  nacos:                    # 注册中心 / 配置中心共用
    server: 127.0.0.1
    port: 8848
    apiversion: v1          # v1 / v2 / v3，需与服务端版本一致
    namespace: public       # 注册中心 namespaceId
    group: DEFAULT_GROUP
    clusterName: DEFAULT
    username: ""            # 开启鉴权时填写
    password: ""
    accessToken: ""         # 已有 token 时直接使用，优先于用户名密码登录
    lan: ""                 # true 时优先用内网 IP 注册
    lanNet: ""

  etcd:
    server: 127.0.0.1
    port: 2379
    prefix: /mgin
    namespace: ""           # 编入 key，具体位置由 keyTemplate 决定
    group: ""
    clusterName: DEFAULT
    keyTemplate: "{prefix}/{group}/{service}/{instance}"   # 支持 {namespace}/{prefix}/{group}/{cluster}/{service}/{instance}
    lan: ""
    lanNet: ""

  consul:
    server: 127.0.0.1
    port: 8500
    token: ""               # Consul ACL token
    namespace: ""           # Consul Enterprise namespace，社区版留空
    group: DEFAULT_GROUP
    clusterName: DEFAULT    # 以服务标签形式过滤实例
    lan: ""
    lanNet: ""

  polaris:{ token: "", lan: "", lanNet: "", server: "127.0.0.1", port: 8091, namespace: "default" }
```

> `sqlite` 不走子配置文件：`go.config.prefix.sqlite` 的值**就是文件路径**（相对路径拼可执行文件目录）。

---

## 7. 测试、构建与静态检查

### 7.1 框架仓库

```bash
go build ./...                 # 编译全部包（当前通过）

# 核心包测试（不依赖外部中间件）
go test ./pkg/health/... ./pkg/client/... ./pkg/loadbalancer/... \
        ./pkg/metrics/... ./pkg/otel/... ./pkg/errcode/... \
        ./pkg/logs/... ./pkg/cache/... -count=1

# 全量（需要本地 MySQL/Redis/etcd/consul/nacos 的项目会失败）
go test ./... -count=1
```

**基线状态（写入文档时实测）**：

| 检查 | 结果 |
|------|------|
| `go build ./...` | ✅ 通过 |
| `cd cmd && go build ./...` | ✅ 通过 |
| 核心包测试 | ✅ `health` / `client` / `loadbalancer` / `metrics` / `otel` / `errcode` / `logs` / `cache` / `i18n` / `job` / `db/dao` / `registry` / `utils` 通过 |
| `pkg/logsink` | ⚠️ `TestQueueKeepsNewestAndFIFO` 失败：测试断言队列上限 100，代码已改为 1024，用例未同步 |
| `pkg/config` | ⚠️ 依赖远程配置中心，无服务时约 90s 超时失败 |
| `go vet ./...` | ⚠️ 3 处历史告警：`pkg/middleware/xss/xss.go:513`、`pkg/utils/rsautil.go:369`、`pkg/utils/rsautil.go:406`（unreachable code） |
| `gofmt -l .` | ⚠️ 14 个历史文件未格式化（如 `pkg/client/client.go`、`pkg/config/configure.go`） |

因此**不要**把全仓库 `go vet` / `gofmt -l .` 作为硬性门禁；新增/修改的文件必须自身干净：

```bash
gofmt -l $(git diff --name-only -- '*.go')   # 应无输出
go vet $(git diff --name-only -- '*/...' | xargs -r -n1 dirname | sort -u | sed 's|^|./|')
```

已知需外部依赖的测试：`pkg/config`、`pkg/registry/consul`、`pkg/registry/etcd`、
`pkg/registry/polaris`、`pkg/i18n/mysql_test.go`、`pkg/db/dao`（缓存用例依赖 Redis）。
这些测试在无对应服务时失败/panic 属预期，不是框架构建问题。

### 7.2 脚手架

```bash
cd cmd
go build -o "$(go env GOPATH)/bin/mgin" .   # 已实测通过
go vet ./...                                # cmd 模块自身干净
mgin version
```

### 7.3 生成工程

生成工程自带 Makefile：

```bash
make tidy          # GOPROXY=https://goproxy.cn go mod tidy
make build         # 注入 Version/BuildTime/GitHash
make test          # go test -count=1 ./...
make test-race     # -race
make cover         # coverage.html
make lint          # go vet + 可选 golangci-lint
make build-multi-os# linux/darwin/windows × amd64/arm64 → dist/
make linux         # 交叉编译 + upx
make clean
```

---

## 8. 常见问题与故障排查

### Q1. `/health/live` 返回 404

- 探针默认**关闭**。二选一：配置 `go.health.enabled: true`，或在 `NewApp` 之后立即
  `app.EnableHealth()`。
- 若在 `app.Run()` 之后才调用，路由已注册完毕，无效。

### Q2. 配了 `go.metrics.enabled: true`，但 `/metrics` 仍 404

`app.baseRouter()` 的挂载条件是 `app.metricsEnabled && go.metrics.enabled`，
而 `metricsEnabled` 只能由 `App.EnableMetrics()` 置位。**请显式调用 `app.EnableMetrics()`**，
或两者同时配置。`EnableMetrics()` 本身不检查配置，是最可靠的方式。

### Q3. `/health/ready` 一直 200，但数据库其实挂了

`/health/ready` 只检查 `go.config.used` 中出现的 `mysql`、`mongodb`、`redis`。
SQLite、PostgreSQL、Kafka 等不参与；且 `db.Redis.Check()`、`db.Sqlite.Check()` 目前恒返回 nil。
另外，`/health/ready` **不读取** `plugin.HealthAll()`，插件健康与就绪探针是两条独立链路。

### Q4. 配了 `loadBalancer: consistent` 但感觉仍是轮询

框架注册名是 `round_robin` / `random` / `least_connections` / `consistent_hash`。
短名（`round`/`least`/`consistent`）会让 `loadbalancer.Get()` 返回 nil，`client` **静默回退**默认轮询。
同时，`go.framework.loadBalancer` 这个 YAML 键**没有任何代码读取**；请在
`client.Options.LoadBalancer` 中显式传注册名。

### Q5. `go.config.used` 明明没写 `mysql`，却尝试连接 MySQL

`plugin.enabled()` 使用 `strings.Contains` 子串匹配。`used: "mysqlx"`、`"notmysql"`
都会误判为启用。请用逗号分隔的精确名称：`used: "mysql,redis"`。

### Q6. 多库模式下 MySQL 连接池/调试配置不生效

`pkg/db/mysql` 在多库分支后仍保留了 `if m.mysql == nil { ... return }`，
导致 `mysql_debug` 与连接池设置被跳过。当前建议：多库场景显式确认连接可用，
或在业务侧自行设置 `conn.DB().SetMaxOpenConns` 等参数。

### Q7. 启动时报 `读取配置文件错误`

- `NewApp` 会把**非绝对路径**拼到 `filepath.Dir(os.Args[0])` 之后：
  以 `./bin/order` 启动时读 `./bin/conf/application.yml`；以 PATH 方式启动（`os.Args[0]` 为程序名）时
  等价于当前工作目录。
- 未传 `-f` 且 `configFile` 为空时，默认文件名是 `<os.Args[0] 去 .exe>.yml`，
  例如 `./bin/order` → `./bin/order.yml`。
- `go run .` 的 `os.Args[0]` 在临时构建目录，相对路径会指向那里；本地调试请传**绝对路径**。

### Q8. 配置中心拉不到 `xxx-test.yml`

文件名规则固定为 `{prefix}-{env}.yml`：
`go.config.prefix.mysql` 的值是 `mysql`、`go.config.env` 是 `test`，则文件名为 `mysql-test.yml`。

`server_type: nacos` 时 URL 由 `apiversion` 决定，`server` 末尾有无 `/` 都可以（框架会去掉尾部斜杠）：

| apiversion | 请求路径 |
|---|---|
| v1 | `{server}/nacos/v1/cs/configs?group=&dataId=&tenant=` |
| v2 | `{server}/nacos/v2/cs/config?group=&dataId=&namespaceId=` |
| v3 | `{server}/nacos/v3/client/cs/config?groupName=&dataId=&namespaceId=`，响应取 `data.content` |

常见原因：`apiversion` 与服务端不一致、`go.config.namespace` 与实际命名空间不一致、
开启鉴权却没配 `accessToken` 或 `username/password`（v3 登录路径为 `/nacos/v3/auth/user/login`）。
其它来源（consul/etcd/polaris/springconfig）以及 `server_type` 未识别的默认分支不会自动补 `/`，
此时 `server` 需自带斜杠。

### Q9. 使用 Polaris 时服务能发现、但注册不上去

`plugin.Registry()` 的 `enabledFn` 只识别 `nacos`/`etcd`/`consul`，未包含 `polaris`，
因此 Polaris 的 `Register` 不会被调用（`NewRegistry()` 仍会创建客户端用于发现）。
需要注册时，业务侧手动调用 `registry.Registry.Register(configData)`。

### Q10. 定时任务在多副本部署时重复执行

调度器是**单机内存调度 + DB 轮询**，没有分布式锁/选主。多副本请只在一个副本开启
`go.job.enabled`，或自行加分布式锁。

### Q11. i18n 错误响应里显示的是 `errcode.xxx` 而不是中文

未启用 i18n 或字典缺记录时，`i18n.Format/String` 回退为 messageKey 原文。
需配置 `go.i18n.enabled: true` 并在 `<appName>_i18n` 表插入对应 `message_id`。
注意：`ErrorDef` 需要业务侧自己 `c.JSON(def.HTTPStatus, ...)`，框架不会替你设置状态码。

### Q12. 404/500 返回体里的 `status` 与 HTTP 状态码不一致

这是设计如此：`status` 是**业务错误码**（如 1000），HTTP 状态码由 `Definition.HTTPStatus` 决定
（404）。NoRoute 与 Recovery 已按真实状态码返回，i18n CRUD 接口仍保持 HTTP 200 包装。

### Q13. 日志级别改了要重启吗

不需要。`go.logger.level` 在**每次输出时**重新读取；配置中心热更新后立即生效。
但 `go.application.debug: true` 会强制 debug 级别，优先级更高。

### Q14. `go test ./...` 大面积失败

先确认是否缺少 MySQL/Redis/etcd/consul/polaris。框架大量测试是集成型测试，
无外部依赖时请按 7.1 的命令只跑核心包。

### Q15. 配了 `go.discovery.registries`，调用却仍走旧注册中心 / 报「微服务获取 X 服务主机IP端口失败」

- `registries` 非空时 `go.discovery.registry` 与 `used` 里的 `nacos`/`etcd`/`consul` 都不再决定注册：
  `plugin.Registry()` 直接启用，`RegisterConfigured()` 按每个条目的 `prefix` 读子配置并逐一注册。
- `client.Options.Registry` 必须与某个 `name` 完全一致。未知名**不会**回退 `defaultRegistry`：
  `GetServicesFrom` 返回错误后框架回落到兼容路径，仍取不到 host，最终只报「微服务获取 X 服务主机IP端口失败」。
- `type` 只支持 `nacos`/`etcd`/`consul`/`polaris`，其它值会被跳过（不报错，等于该实例不存在）；
  `name` 重复时只保留第一个。
- 每个实例有独立缓存，改完 `group`/`namespace` 后需等 `cacheTTL` 过期或重启。

---

## 9. 已知限制与未实现能力

以下结论来自对当前代码的核对，**不要在方案中假设其存在**：

1. **无 S3/对象存储**：`pkg/storage` 目录不存在，`go.mod` 无 AWS SDK，`RegisterBuiltins` 未注册 S3。
   `application.example.yml` 中的 `go.s3.*` 与 `used: "...s3"` 不会被消费。
2. **无 PostgreSQL / ClickHouse / Elasticsearch / Kafka 数据源实现**：`pkg/db` 只有
   `mysql`、`sqlite`、`mongo`、`redis` 四个子包。
3. **`Dao[E]` 接口未被实现**：`dao/define.go` 的 7 方法接口与 `MySQLDao`/`MgoDao`
   方法集不匹配，仓库内无 `var _ Dao[E]` 断言。业务请直接使用 `MySQLDao[E]`。
4. **`go.framework.*` 多数开关未接线**：仅 `i18nEnabled` 被使用。
5. **`go.framework.loadBalancer` / `go.framework.metrics` / `go.framework.otel` 不存在于配置结构体**。
6. **`client.CallCtx` 的 ctx 不能取消底层 HTTP**：只有 `CallResilientCtx` 用 goroutine + timer
   做超时兜底，底层连接可能继续占用。
7. **`otel` 包不提供 `Init`/`Shutdown`**：需业务自行构建 `TracerProvider` 后
   `otel.SetTracerProvider(tp)`。
8. **sys 模块只有模型**：13 张 `sys_*` 表无 DDL、无 `AutoMigrate`、无 CRUD 实现，
   建表需业务侧完成。
9. **无集群限流/分布式任务调度/网关/gRPC 服务端/OAuth2**。
10. **`logs/` 顶层别名壳计划删除**，新代码请直接 import `pkg/logs`。

---

## 10. 贡献指南

### 10.1 分支与提交

- 从 `v2`（或当前开发分支）拉出特性分支：`feat/<topic>`、`fix/<topic>`、`docs/<topic>`。
- 提交信息建议遵循仓库既有风格，推荐 Conventional Commits：
  `feat(client): 支持 per-instance 熔断`、`fix(app): HTTPS server 未 Shutdown`、
  `docs(readme): 修正负载均衡策略名`。
- 一次提交只做一件事；格式化、重命名与功能改动分开。

### 10.2 改动前必读

1. 新 API 放 `pkg/`，**不要**新增顶层包（`logs/` 兼容壳除外）。
2. 保持 v1 公开签名不变；破坏性变更必须走大版本并更新
   `docs/migration-v1-to-v2.md` 同类迁移文档。
3. 新增组件优先实现 `plugin.Plugin` 并用 `plugin.Register` 注册，不要在 `mgin.Init` 里写 if 链。
4. 新增配置项统一放在 `go.*` 命名空间，并在 `pkg/config` 中显式读取；
   同步更新 README 第 6 节。
5. 探针/指标端点必须保持"最先注册"，不要移动到业务中间件之后。

### 10.3 提交前检查

```bash
# 只检查本次改动的 Go 文件（仓库存在历史未格式化文件与 3 处 vet 告警）
gofmt -l $(git diff --name-only -- '*.go')
go build ./...             # 通过
go test ./pkg/health/... ./pkg/client/... ./pkg/loadbalancer/... \
        ./pkg/metrics/... ./pkg/otel/... ./pkg/errcode/... \
        ./pkg/logs/... ./pkg/cache/... -count=1
cd cmd && go build ./... && go vet ./...   # 脚手架模块干净
```

`pkg/logsink` 当前有一个与代码不同步的失败用例（见 7.1），修复前不要把它加入门禁。
新增代码不得引入新的 `go vet` 告警；不要为了清零历史告警而夹带无关重构。

### 10.4 PR 要求

- 描述：变更动机、行为影响、兼容性说明（是否破坏 import/签名/配置）。
- 文档：涉及配置、API、目录结构时同步更新 `README.md` 与对应 `docs/*.md`。
- 测试：新增/修改逻辑需带单测；依赖外部中间件的测试请用 `t.Skip` 或本地桩隔离。
- 评审关注点：并发安全、错误处理、资源释放（连接/协程/ticker）、日志占位符是否成对。

### 10.5 文档规范

- 术语统一：**插件（Plugin）**、**注册中心（Registry）**、**负载均衡器（LoadBalancer）**、
  **探针（Probe）**、**业务错误码（Code）**、**HTTP 状态码（HTTPStatus）**。
- 文档中的 API 签名必须与代码一致；不确定时先跑 `go doc`。
- 不写入未实现的能力；计划中的能力放"已知限制/路线图"。

---

## 11. 许可证

本项目采用 **MIT License**，版权归 `maczh`（2022）所有。允许商用、修改、分发，
需保留版权声明与许可声明。完整文本见 [`LICENSE`](LICENSE)。

---

## 12. 文档导航

| 文档 | 内容 |
|------|------|
| [docs/requirements.md](docs/requirements.md) | 需求文档：角色、场景、功能/非功能需求、验收标准、需求追踪矩阵 |
| [docs/architecture.md](docs/architecture.md) | 系统架构设计：进程模型、启动时序、插件生命周期、调用链、架构决策与债务 |
| [docs/modules-and-api.md](docs/modules-and-api.md) | 模块与接口说明：逐包职责、公开签名、中间件顺序、错误码表 |
| [docs/data-design.md](docs/data-design.md) | 数据结构设计：`sys_*`、`casbin_rule`、`mgin_job_*`、i18n 表、缓存键、多数据源 |
| [docs/development-setup.md](docs/development-setup.md) | 开发环境搭建：Go/中间件准备、构建、调试、IDE、常见安装问题 |
| [docs/coding-standards.md](docs/coding-standards.md) | 编码规范与协作流程：命名、错误处理、并发、日志、评审、分支策略 |
| [docs/testing-strategy.md](docs/testing-strategy.md) | 测试策略：分层、现有测试清单、编写方式、覆盖率与 CI 建议 |
| [docs/release-process.md](docs/release-process.md) | 版本发布与变更记录：语义化版本、发布清单、CHANGELOG 规范、回滚 |
| [docs/migration-v1-to-v2.md](docs/migration-v1-to-v2.md) | 从 v1 升级到 v2 的 import 路径迁移指引 |
| [docs/i18n.md](docs/i18n.md) | i18n 使用说明 |
| [docs/victorialogs.md](docs/victorialogs.md) | VictoriaLogs 接入说明 |
| [docs/v2-prd.md](docs/v2-prd.md) / [docs/v2-design.md](docs/v2-design.md) / [docs/v2-tasks.md](docs/v2-tasks.md) | v2 重构的历史 PRD、设计与任务清单（部分内容为规划态） |
| [sdk_document.md](sdk_document.md) | 旧版 SDK 详细用法（部分章节对应 v1，请以代码为准） |
| [README_CN.md](README_CN.md) / [README_en.md](README_en.md) | v1 时代完整手册（含已不存在的能力描述，阅读前请看文首说明） |
