# MGin 微服务框架（v2-arch）

> 模块路径：`github.com/maczh/mgin` · Go 1.25+ · 当前分支：`v2-arch`（基线：`v1.25.14-jh`）
>
> v2-arch 是**对 v1.25.14-jh 的内部架构重构**，**模块路径未升级**（仍为 `github.com/maczh/mgin`）。
> 它在保留 v1 所有对外 API 的同时，引入了统一的插件系统、客户端韧性、四类客户端负载均衡、
> Prometheus 指标、统一错误码与 OpenTelemetry 接入；并把脚手架（`cmd/`）升级为对应的 v2 输出。
>
> 详细 SDK 用法见 [`sdk_document.md`](sdk_document.md)；架构设计见 [`docs/`](docs)；
> 从 v1 升级的迁移指引见 [`docs/migration-v1-to-v2.md`](docs/migration-v1-to-v2.md)。

---

## 目录

- [快速开始](#快速开始)
- [v2 新增能力一览](#v2-新增能力一览)
- [脚手架 `mgin new` 完整使用](#脚手架-mgin-new-完整使用)
- [v2 框架目录结构](#v2-框架目录结构)
- [与 v1.25.14-jh 的功能变迁](#与-v12514-jh-的功能变迁)
- [配置分层 `go.framework.*` / `go.application.*` / `go.runtime.*`](#配置分层)
- [升级到 v2-arch](#升级到-v2-arch)
- [测试 / 构建 / Makefile](#测试--构建--makefile)

---

## 快速开始

### 1. 业务项目（用脚手架生成，1 分钟上手）

```bash
# 安装脚手架 CLI
go install github.com/maczh/mgin/cmd/mgin@v1.25.14-jh
# 或在 v2-arch 分支本地构建：cd cmd && go build -o $(go env GOPATH)/bin/mgin .

# 一步生成一个带健康检查、Prometheus 指标、负载均衡、注册中心的 Go 微服务工程
mgin new order --module github.com/acme/order \
    --db mysql --registry nacos --health --metrics --otel \
    --loadbalancer consistent --port 8080

cd order
make tidy
make build
./order
```

启动后会监听 `http://localhost:8080`，同时暴露：

| 端点 | 来源 | 用途 |
|------|------|------|
| `/health/live` | `pkg/health` | K8s 存活探针，不查依赖 |
| `/health/ready` | `pkg/health` | K8s 就绪探针，按配置数据源实时 Check |
| `/health/startup` | `pkg/health` | K8s 启动探针，需业务侧 `health.MarkStarted()` |
| `/metrics` | `pkg/metrics` | Prometheus 抓取，HTTP / 插件 / 依赖健康 |
| `/api/v1/...` | 业务路由 | 你的 controller + service |

### 2. 框架消费方（在自己的项目里直接 import）

```go
package main

import (
    "github.com/gin-gonic/gin"
    "github.com/maczh/mgin"
    "github.com/maczh/mgin/pkg/client"
    "github.com/maczh/mgin/pkg/health"
    "github.com/maczh/mgin/pkg/metrics"
)

func main() {
    app := mgin.NewApp("conf/application.yml", "order", "1.0.0", true)

    // 显式启用 v2 能力（也可在 application.yml 用 go.framework.* 开关）
    app.EnableHealth()
    app.EnableMetrics()
    health.MarkStarted() // 启动完毕后调用，让 /health/startup 返 200

    app.Router.Use(metrics.Middleware())

    // 业务路由
    app.Router.GET("/api/v1/hello", func(c *gin.Context) {
        // 跨服务调用：v2 的 CallCtx 支持 context + 客户端负载均衡 + per-instance 熔断
        body, err := client.CallCtx(c.Request.Context(), "user-service", "/api/v1/users/1", &client.Options{
            Method: "GET",
        })
        if err != nil {
            // 返 503 由 errcode.Definition 三向映射决定
            c.JSON(503, gin.H{"error": err.Error()})
            return
        }
        c.JSON(200, body)
    })

    app.Run()
}
```

完整 SDK 用法见 [`sdk_document.md`](sdk_document.md)。

---

## v2 新增能力一览

v2-arch 提交链（`v2-arch` 分支，从旧到新）：

| Commit | 一句话 |
|---|---|
| `a285b65` | **健康探针 + client.Call 韧性 + HTTPS 优雅关闭** 三大生产缺口补齐 |
| `d6416a6` | **v2.0 骨架重构**：包迁移到 `pkg/`、统一 `Plugin` 系统、配置分层、`client.CallCtx` 零破坏升级、`Dao[E]` 接口扩展 |
| `e3cfca2` | **v2.1 微服务能力**：客户端负载均衡（4 策略）、Prometheus 指标、统一错误码增强、OpenTelemetry 抽象 |
| `7507f7f` | **脚手架 v2-arch 适配**：`cmd/{mgin,new,scaffold,templates}.go` 输出 v2 兼容代码、新增 `--health/--metrics/--otel/--loadbalancer` 开关 |

### 核心架构层

- **统一插件系统**（`pkg/plugin`）—— `Plugin{Name,Order,Init(ctx),Close(ctx),Health,Enabled}` 接口；用注册表驱动 mgin 生命周期。
  包含 mysql / postgres / sqlite / mongodb / redis / clickhouse / elasticsearch / kafka / s3 / registry / job 11 类内置插件。
- **包结构分层**—— 顶层仅 `cmd docs internal logs pkg`；`logs/` 保留顶层别名壳（re-export `pkg/logs`），1 个 release 后删除。
- **配置三层**—— `go.framework.*`（框架元配置） / `go.application.*`（业务，向后兼容） / `go.runtime.*`（运行时元数据，启动时填充）。
- **DAO 接口扩展**—— `Dao[E any]` 从 1 方法扩为 7（Insert / Where / Find / FindById / Update / Delete / Count）。

### 客户端韧性（`pkg/client`）

- `client.Call(service, uri, *Options)` —— 旧的同步调用签名不变。
- `client.CallCtx(ctx, service, uri, *Options)` —— v2 新增，入参 `context.Context` 实现真正调用方不被阻塞。
- `client.CallResilient(ctx, service, uri, *Options, *ResilienceOptions)` —— 叠加**超时 + 指数退避 jitter 重试 + 熔断**。
- **`pkg/client/breaker.go`** —— 三态熔断器 `Closed / Open / HalfOpen`，含全局 `GetBreaker(name)` 注册表；纯标准库零依赖。
- **`callWithTimeout` 兜底策略** —— 因暂不升级 `grequests`，超时是 goroutine+timer 兜底；调用方不被阻塞，但底层连接残留（doc 中已诚实标注局限）。

### 服务发现 + 负载均衡

- `registry.RegistryClient.GetServices(service, group...)` —— 4 注册中心（nacos / consul / etcd / polaris）全部支持多实例查询；老 `GetServiceURL` 保留。
- `pkg/loadbalancer` —— 提供 4 策略：`RoundRobin` / `Random` / `LeastConnections` / `ConsistentHash`；通过 `RoundRobinLB` 等全局变量与 `loadbalancer.Default()` 访问。
- `client.Options.LoadBalancer` —— 按字段名指定策略（默认 `round`），同时支持 `random` / `least` / `consistent`。
- **per-instance 熔断** —— `client.GetBreaker(service+"@"+host)`，被熔断的实例会被 LB 短时剔除重选；全部熔断返 `ErrAllInstancesCircuitOpen`。

### 可观测性

- **健康检查**（`pkg/health`）—— `/health/live`、`/health/ready`、`/health/startup`；`go.health.enabled=true` 或 `app.EnableHealth()` 启用；`health.MarkStarted()` 让 startup 返 200。
- **Prometheus 指标**（`pkg/metrics`）—— `/metrics`；6 个核心指标：
  - `mgin_http_requests_total{method,path,status}`
  - `mgin_http_request_duration_seconds{method,path}`
  - `mgin_http_requests_in_flight`
  - `mgin_build_info{version,commit,go_version}`（恒为 1）
  - `mgin_plugin_health{name}`
  - `mgin_dependency_up{name}`
- **OpenTelemetry**（`pkg/otel`）—— 轻量接口 + 业务自接 SDK：
  - `otel.SetTracerProvider(tp)` / `otel.StartSpan(ctx, name)` / `otel.IsEnabled()`
  - W3C `traceparent` 由 `middleware/trace` 自动透传（兼容既有 `X-Trace-Id`）
  - `client.callInternal` 在 OTel 启用时自动开 `mgin.client.call` span

### 统一错误码（`pkg/errcode` + `pkg/i18n`）

- `errcode.Definition{Code, HTTPStatus, MessageKey, Module}` —— 错误码 → HTTP 状态码 → i18n 文案 三向映射。
- `errcode.New(module, code, httpStatus, msgKey)` / `RegisterDef(d)` / `LookupDef(code)` / `AllDefs()`。
- `i18n.ErrorDef(def, args...)` / `i18n.ErrorDefT[T](def, args...)` —— 渲染 `models.Result[T]`。
- **NoRoute 现在返 404**（不再 200）；**Recovery 现在返 500**（不再 200）。

### 进程生命周期

- **HTTPS 优雅关闭** —— v2-arch 把 `server` 与 `serverSsl` 收集到切片，退出时逐个 `Shutdown`；超时由 `go.application.shutdownTimeout` 决定（默认 5 秒）。
- 顺带修复了 `signalChan := make(chan os.Signal)` 无缓冲会丢信号的隐患。

---

## 脚手架 `mgin new` 完整使用

`cmd/` 是独立的 Go module（`cmd/go.mod`），可独立 build：

```bash
cd cmd && go build -o $(go env GOPATH)/bin/mgin .
# 或 go install github.com/maczh/mgin/cmd/mgin@<version>
```

### 非交互模式

```bash
mgin new <工程名> [选项]
```

**通用选项：**

| Flag | 类型 | 默认 | 说明 |
|------|------|------|------|
| `--module <path>` | string | `github.com/maczh/<工程名>` | Go module 路径 |
| `--port <port>` | int | `8080` | HTTP 端口 |
| `--db <list>` | csv | `mysql` | `mysql/postgres/sqlite/mongodb/redis/clickhouse/elasticsearch`（逗号分隔） |
| `--mq <list>` | csv | — | `nats/kafka/mqtt/rabbit`（逗号分隔，对应 `maczh/nats`、`maczh/mgkafka`、`maczh/mqtt`、`maczh/mgrabbit` 插件） |
| `--registry <type>` | enum | `none` | `nacos/consul/etcd/none` |
| `--config-center <type>` | enum | `none` | `nacos/consul/etcd/polaris/springconfig/file/none` |
| `--i18n` | bool | `false` | 启用国际化 |
| `--jwt` | bool | `false` | 启用 JWT 中间件 |
| `--casbin` | bool | `false` | 启用 Casbin 接口鉴权 |
| `--sys` | bool | `false` | 启用内置系统管理模块（master 分支；jh 分支已移除） |
| `--output <dir>` | string | `.` | 输出目录 |
| `--mgin-version <ver>` | string | 自动解析（离线回退 `v1.25.14-jh`） | mgin 依赖版本 |
| `--force` | bool | `false` | 覆盖已存在目录 |
| `--interactive` / `-i` | bool | `false` | 强制使用交互式问答模式 |

**v2 新能力开关：**

| Flag | 类型 | 说明 |
|------|------|------|
| `--health` | bool | 启用 `/health/{live,ready,startup}` 探针 |
| `--metrics` | bool | 启用 `/metrics` 端点（Prometheus） |
| `--otel` | bool | 启用 OpenTelemetry（业务侧需自接 `TracerProvider`） |
| `--loadbalancer <name>` | enum | `round/random/least/consistent`（默认 `round`） |

### 交互模式

省略非必填选项直接 `mgin new <工程名>`，CLI 在终端逐项询问（数据库选哪几种、是否启用 JWT / Casbin / i18n / Sys、新能力的 4 个开关、LB 策略等）；非终端环境自动取默认值生成。

### 生成工程结构

```
<工程名>/
├── main.go              # 入口，挂 Health/Metrics/Dao/Plugin
├── version.go           # 由 Makefile -ldflags 注入 Version/BuildTime/GitHash
├── Makefile             # 含 tidy / build / build-multi-os / test / test-race / cover / lint / clean / run / linux
├── go.mod               # 显式指定 mgin 依赖版本
├── conf/
│   └── application.yml  # 含 go.framework.* 节点（v2 新增）
├── model/
│   └── model.go         # GORM 模型
├── dao/                 #（非 memory 模式）泛型 Dao 封装
│   └── dao.go
├── service/
│   └── service.go       # 业务层（ctx context.Context，v2 起强制）
├── controller/
│   └── controller.go    # 用 errcode.Definition + i18n.ErrorDef 三向映射
└── router/
    └── router.go
```

`--sys` 启用时还会多生成 `casbin.conf` 与 `middleware/xss` 等可选文件。

### 生成工程的标准用法

```bash
cd <工程名>
make tidy                                  # go mod tidy（GOPROXY=https://goproxy.cn）
make build                                 # 本地 build，注入版本信息
make test                                  # 跑全部 *_test.go
make test-race                             # 跑 race detector
make cover                                 # 覆盖率
make linux                                 # 交叉编译 + upx 压缩
make build-multi-os                        # 一次性产出 5 OS/ARCH 的 dist/ 目录（linux/darwin/windows × amd64/arm64）
./<工程名>                                 # 启动（读取 conf/application.yml）
./<工程名> -v                              # 输出版本号与构建信息
```

### 一个完整示例

```bash
# 生成 mysql+nacos 全能力工程，依赖 v1.25.14-jh（兼容模块路径）
mgin new order-svc \
    --module github.com/acme/order-svc \
    --port 8088 \
    --db mysql \
    --registry nacos \
    --health \
    --metrics \
    --otel \
    --loadbalancer consistent \
    --jwt \
    --mgin-version v1.25.14-jh \
    --force

cd order-svc
make tidy build
./order-svc -v    # 查看版本与构建信息
./order-svc       # 启动：监听 :8088，自动加载 conf/application.yml
```

启动后：

```bash
curl http://localhost:8088/health/live      # 200 OK
curl http://localhost:8088/health/ready     # 200 OK（依赖 Check 通过）
curl http://localhost:8088/metrics          # Prometheus 文本输出
curl http://localhost:8088/api/v1/...       # 你自己的业务接口
```

---

## v2 框架目录结构

```
mgin/                                   # 框架仓库根 (github.com/maczh/mgin)
├── mgin.go                             # 框架入口：NewApp / Init / SafeExit (Plugin 驱动)
├── app.go                              # 进程生命周期 / HTTP+HTTPS / Health/Metrics 挂载点
├── application.example.yml             # v2 完整配置示例（含 go.framework.* 与 go.runtime.*）
├── casbin.conf                         # Casbin 模型文件
├── cmd/                                # 【独立 module】脚手架 CLI (cmd/go.mod)
│   ├── mgin.go                         # CLI 主入口与 usage
│   ├── new.go                          # new 子命令：参数解析 + 交互问答
│   ├── scaffold.go                     # 工程目录创建 + Makefile/YML/README 写入
│   └── templates.go                    # 代码模板：main.go / router.go / controller.go / service.go / dao.go / Makefile / version.go
├── internal/                           # 框架内部实现预留（v2.0 暂未使用）
├── docs/                               # 设计/任务/迁移文档
├── logs/                               # 顶层别名壳（re-export pkg/logs；1 release 后删除）
├── pkg/                                # 【对外可导入】所有公开 API 都在这
│   ├── client/                         # service 间 HTTP 调用 + 韧性（断路器/超时/重试/LB）
│   ├── config/                         # 多源配置读取（nacos/consul/etcd/polaris/springconfig/file）
│   ├── errcode/                        # 统一错误码（Definition + 三向映射）
│   ├── health/                         # K8s 探针 + 依赖 Check
│   ├── i18n/                           # 国际化文案 + ErrorDef 入口
│   ├── job/                            # xxl-job 风格分布式任务调度
│   ├── loadbalancer/                   # 4 策略负载均衡器
│   ├── metrics/                        # Prometheus 指标 + /metrics
│   ├── middleware/                     # trace/postlog/cors/jwt/casbin/session/limit/xss/ratelimit
│   ├── models/                         # 内置 ORM（GORM 风格）
│   ├── otel/                           # OpenTelemetry 抽象 + W3C traceparent
│   ├── plugin/                         # 【核心】统一 Plugin 接口与注册表
│   ├── registry/                       # nacos / consul / etcd / polaris
│   ├── storage/                        # s3
│   ├── cache/                          # memcache / diskcache / gorm 二级缓存
│   ├── casbin/                         # Casbin gorm-adapter v3
│   ├── db/                             # 多数据库连接（mysql/pg/mgo/clickhouse/...）
│   ├── utils/                          # 通用工具（加解密/JWT/SFTP/限流/条件构造器/...）
│   └── logs/                           # 日志真实实现
└── sdk_document.md                     # 旧 SDK 详细文档（互补）
```

---

## 与 v1.25.14-jh 的功能变迁

> v1.25.14-jh 提交 `5d8f40a`（"新增cmd命令行生成mgin工程框架"）；v2-arch 在其之上重写了 4 个独立 commit。

### 大方向变化

| 维度 | v1.25.14-jh | v2-arch | 增量价值 |
|------|-------------|---------|----------|
| 包结构 | 14 个顶层包平铺（`client/` `config/` `errcode/` ...） | 全部收至 `pkg/`；顶层仅 `cmd docs internal logs pkg` | 内部细节私有化，未来引入 `internal/` |
| 生命周期 | `mgin.go` 手写 `if Contains(used, "mysql")` 等十几次 | 统一 `Plugin` 接口 + 注册表驱动，`Init/Close/Health` 标准化 | 加一个数据源 = 注册一个 plugin，不再改 `mgin.go` |
| 配置命名空间 | 仅 `go.application.*` `go.config.*` 等 | 新增 `go.framework.*` / `go.runtime.*` | 框架元配置与业务配置解耦 |
| 健康检查 | 无（`checkAll` 是每 5 分钟连通性自检） | `/health/{live,ready,startup}` | K8s liveness/readiness/startup 全支持 |
| 客户端韧性 | 无；`Options.Retry` 是朴素递归重试 | `client.CallCtx` + `breaker.go` + `resilience.go` | 真正超时 + 指数退避 + 熔断 |
| HTTPS 优雅关闭 | **bug**：`app.go` 只 Shutdown HTTP server，HTTPS serverSsl 从不关 | 收集到 `[]*http.Server`，逐个 Shutdown | HTTPS 生产可上线 |
| 客户端 LB | `GetServiceURL(service, group)` 只返一个 URL | 新增 `GetServices(...)` + `pkg/loadbalancer` 4 策略 | 多实例横向扩展 |
| Prometheus 指标 | 无 | `/metrics` + 6 个核心指标 + gin 中间件 | 可观测性 |
| 错误码 | 散落 `const`，HTTP 状态码错乱（常返 200） | `errcode.Definition` 三向映射；NoRoute 404 / Recovery 500 | 网关/前端体验更可控 |
| OpenTelemetry | 仅 `X-Trace-Id` 透传 | W3C `traceparent` + `pkg/otel` 抽象 + span 埋点 | 可与 Jaeger/Tempo 集成 |
| DAO | 仅 `Insert(entity *E) error` | 7 个方法完整接口 | 业务层可依赖接口 |

### API 层面的兼容与破坏

| API | v1 | v2 | 备注 |
|-----|----|----|------|
| `mgin.NewApp` | ✓ | ✓ | 签名与行为一致 |
| `mgin.Run / Init / SafeExit` | ✓ | ✓ | 同上 |
| `client.Call(service, uri, op)` | ✓ | ✓ | 签名不变；内部委托 `CallCtx(context.Background(), ...)` |
| `client.CallCtx(ctx, ...)` | ✗ | ✓ 新增 | v2 引入，零破坏 |
| `client.CallResilient(ctx, ...) / CallResilient(...)` | ✗ | ✓ 新增 | 韧性封装 |
| `client.Options.LoadBalancer` | ✗ | ✓ 新增 | 可选字段，缺省走 round-robin |
| `registry.RegistryClient.GetServices` | ✗ | ✓ 新增 | 4 实现同步补齐 |
| `errcode.New / LookupDef / RegisterDef` | ✗ | ✓ 新增 | 旧常量仍可用 |
| `i18n.ErrorDef(def, args...)` | ✗ | ✓ 新增 | 三向映射 |
| `health.Router(group) + MarkStarted/StartedAt` | ✗ | ✓ 新增 | |
| `metrics.Handler()/Middleware()` | ✗ | ✓ 新增 | |
| `otel.SetTracerProvider/StartSpan/IsEnabled` | ✗ | ✓ 新增 | |
| `client.GetBreaker(name)` | ✗ | ✓ 新增 | |
| `loadbalancer.Default/Register/Get` | ✗ | ✓ 新增 | |
| `config.Config.Framework` / `Runtime` | ✗ | ✓ 新增 | 旧 `App/Log/Logger` 100% 向后兼容 |
| **import 路径（破坏）** | `github.com/maczh/mgin/client` 等顶层 | `github.com/maczh/mgin/pkg/client/...` | 14 个旧路径迁到 `pkg/`；`logs/` 保留顶层 1 release 的别名壳 |
| `app.NoRoute` / `recoveryHandler` 返 HTTP 状态 | 恒为 200 | 404 / 500 | 按 `Definition.HTTPStatus` |

完整迁移指引（带 sed 一键替换脚本）见 [`docs/migration-v1-to-v2.md`](docs/migration-v1-to-v2.md)。

### 依赖变化

- **新增 direct 依赖（v2.1）**：
  - `github.com/prometheus/client_golang v1.19.1`
  - `go.opentelemetry.io/otel/trace v1.26.0`
- **OTel SDK / Exporter 不强制引入**——业务自接，最少依赖。

---

## 配置分层

```yaml
go:
  application:           # v1 业务配置，100% 向后兼容
    name: order
    port: 8080
    debug: true

  config:
    used: "mysql"        # 数据源白名单
    env: dev

  framework:             # v2 新增：框架自身元配置
    shutdownTimeout: 5   # 秒；优雅关闭总预算
    engine:              # HTTP server
      readTimeout: 30
      writeTimeout: 30
    health:              # /health 探针
      enabled: true
      autoStarted: false # false 时业务侧需调用 health.MarkStarted()
    metrics:             # /metrics 端点
      enabled: true
    otel:                # OpenTelemetry 抽象
      enabled: true
      # endpoint: ""    # 业务侧 SetTracerProvider 时自带
    loadBalancer: round  # round/random/least/consistent

  runtime:               # v2 新增：运行时元数据，mgin.Init() 启动时填充
    startedAt: 2026-09-04T15:00:00+08:00
    pid: 12345
    goVersion: go1.25.7
    commitHash: e3cfca2  # 由 make build 阶段 -ldflags 注入
    buildTime: 2026-09-04 # 由 make build 阶段 -ldflags 注入
```

完整示例见 [`application.example.yml`](application.example.yml)。

---

## 升级到 v2-arch

老项目拉取 `v2-arch` 后，**主要工作是改 import 路径**：

```bash
# 一键 sed（macOS / BSD）：把 14 个旧顶层包替换为 pkg/
for f in $(grep -rl "github.com/maczh/mgin/\(client\|config\|errcode\|i18n\|health\|models\|registry\|middleware\|casbin\|utils\|logs\|job\|db\|storage\|cache\)" --include="*.go" .); do
    sed -i '' 's|github.com/maczh/mgin/\(client\|config\|errcode\|i18n\|health\|models\|registry\|middleware\|casbin\|utils\|logs\|job\|db\|storage\|cache\)\([^a-zA-Z]\)|github.com/maczh/mgin/pkg/\1\2|g' "$f"
done

# Linux：
# sed -i 's|github.com/maczh/mgin/\(client\|config\|errcode\|...\)\([^a-zA-Z]\)|github.com/maczh/mgin/pkg/\1\2|g'
```

或者用脚手架 `--mgin-version v1.25.14-jh`（默认值）保持兼容，单独开 v2-arch 分支开新工程。

完整迁移文档参见 [`docs/migration-v1-to-v2.md`](docs/migration-v1-to-v2.md)。

---

## 测试 / 构建 / Makefile

### 仓库级

```bash
# 已有测试的包（health / client / loadbalancer / metrics / otel / errcode）
go test ./pkg/health/... ./pkg/client/... ./pkg/loadbalancer/... \
         ./pkg/metrics/... ./pkg/otel/... ./pkg/errcode/... -count=1

# 框架核心 build 验证
go build ./...
go vet ./...
```

> **已知失败**：`pkg/config / pkg/registry/{consul,etcd,polaris}` 的少量测试在 jh 基线上就会 panic（v1 既有 bug，依赖远程 etcd/consul/polaris 进程；与 v2-arch 重构无关，不在本分支责任范围）。

### 脚手架级

```bash
cd cmd && go build -o $(go env GOPATH)/bin/mgin .
mgin version
```

### 生成工程级

生成工程自带 `Makefile`：

```bash
make tidy build           # 默认构建
make build-multi-os       # 5 OS/ARCH 一次性产出 dist/
make test / test-race
make cover
make clean
```

---

## 相关链接

- 旧版详细 SDK 用法：[`sdk_document.md`](sdk_document.md)
- v2 设计文档：[`docs/`](docs)
  - [`docs/v2-prd.md`](docs/v2-prd.md) — v2 增量 PRD
  - [`docs/v2-design.md`](docs/v2-design.md) — 架构设计
  - [`docs/v2-tasks.md`](docs/v2-tasks.md) — 27 任务清单
  - [`docs/v2-package-map.md`](docs/v2-package-map.md) — `pkg/` 包归属表
  - [`docs/migration-v1-to-v2.md`](docs/migration-v1-to-v2.md) — v1 → v2 迁移
  - [`docs/mgin-architecture-options.md`](docs/mgin-architecture-options.md) — 六套架构方案对比（基于源码实测的额外前置文档）
- 版本说明：基线 `v1.25.14-jh`（`5d8f40a`）；v2-arch 在其之上叠加 4 个 commit。
