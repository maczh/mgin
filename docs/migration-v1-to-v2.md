# mgin v1 → v2 导入路径迁移指引

mgin v2.0 把全部对外框架包从仓库顶层收拢到 `pkg/` 目录，形成清晰的 `pkg/`（对外可导入）边界。
本文件给出「旧包 → 新包」的完整映射，以及一条可复制执行的 sed 迁移命令。

> 只有 `logs` 包额外保留了顶层别名壳 `github.com/maczh/mgin/logs`（内部 re-export `pkg/logs`），
> 因此**存量代码不修改 logs 的 import 也能编译**，新代码建议直接写 `pkg/logs`。

---

## 1. 顶层旧包 → pkg/ 新包 映射表

| 旧导入路径 | 新导入路径 | 说明 |
|------------|------------|------|
| `github.com/maczh/mgin/client` | `github.com/maczh/mgin/pkg/client` | 服务调用 Call/CallCtx/CallResilient 等 |
| `github.com/maczh/mgin/config` | `github.com/maczh/mgin/pkg/config` | 配置中心 Configure / GetConfig* |
| `github.com/maczh/mgin/errcode` | `github.com/maczh/mgin/pkg/errcode` | 错误码常量 |
| `github.com/maczh/mgin/i18n` | `github.com/maczh/mgin/pkg/i18n` | 国际化 Error/String/Format/Success |
| `github.com/maczh/mgin/health` | `github.com/maczh/mgin/pkg/health` | 健康探针 Router/MarkStarted |
| `github.com/maczh/mgin/models` | `github.com/maczh/mgin/pkg/models` | Result 等通用模型 |
| `github.com/maczh/mgin/registry` | `github.com/maczh/mgin/pkg/registry` | 注册中心 RegistryClient/NewRegistry |
| `github.com/maczh/mgin/middleware` | `github.com/maczh/mgin/pkg/middleware` | 含 jwt/casbin/cors/ratelimit/trace/… 子包 |
| `github.com/maczh/mgin/casbin` | `github.com/maczh/mgin/pkg/casbin` | RBAC 辅助 |
| `github.com/maczh/mgin/utils` | `github.com/maczh/mgin/pkg/utils` | 通用工具 |
| `github.com/maczh/mgin/logs` | `github.com/maczh/mgin/pkg/logs` | 顶层别名壳仍可用，建议改用 pkg/logs |
| `github.com/maczh/mgin/job` | `github.com/maczh/mgin/pkg/job` | 定时任务调度 |
| `github.com/maczh/mgin/db` | `github.com/maczh/mgin/pkg/db` | 数据源门面 + db/dao、db/mysql 等子包 |
| `github.com/maczh/mgin/storage` | `github.com/maczh/mgin/pkg/storage` | storage/s3 等 |
| `github.com/maczh/mgin/cache` | `github.com/maczh/mgin/pkg/cache` | 缓存 |

> 子包路径一并跟随上移，例如：
> - `github.com/maczh/mgin/middleware/jwt` → `github.com/maczh/mgin/pkg/middleware/jwt`
> - `github.com/maczh/mgin/db/dao` → `github.com/maczh/mgin/pkg/db/dao`
> - `github.com/maczh/mgin/registry/nacos` → `github.com/maczh/mgin/pkg/registry/nacos`
> - `github.com/maczh/mgin/storage/s3` → `github.com/maczh/mgin/pkg/storage/s3`
> - `github.com/maczh/mgin/models/sys` → `github.com/maczh/mgin/pkg/models/sys`

---

## 2. 一键迁移命令

在**你的业务工程根目录**（go.mod 所在目录）执行以下脚本，批量改写所有 `.go` 文件中的
旧导入路径为 `pkg/` 新路径：

```bash
#!/usr/bin/env bash
set -e
MODULE="github.com/maczh/mgin"
for p in client config errcode i18n health models registry middleware casbin utils logs job db storage cache; do
  grep -rl "${MODULE}/${p}" --include='*.go' . 2>/dev/null | while read -r f; do
    # 注意：仅替换以 ${MODULE}/${p} 开头的导入前缀，避免误伤
    sed -i.bak -E "s#(github.com/maczh/mgin)/${p}([/\"]|$)#\1/pkg/${p}\2#g" "$f"
  done
done
# 确认无误后删除备份
# find . -name '*.go.bak' -delete
```

> 说明：正则中的 `([/\"]|$)` 保证只匹配完整包路径边界，不会把
> `github.com/maczh/mgin/clientx` 之类的非法路径误改。
> 如果只需覆盖受影响的精确子串，也可直接用下方更简单的逐包替换（已在 mgin 仓库自身验证）：
>
> ```bash
> for p in client config errcode i18n health models registry middleware casbin utils logs job db storage cache; do
>   grep -rl "github.com/maczh/mgin/$p" --include='*.go' . | xargs -r sed -i '' "s|github.com/maczh/mgin/$p|github.com/maczh/mgin/pkg/$p|g"
> done
> ```

### logs 别名壳（向后兼容）

`github.com/maczh/mgin/logs` 仍可作为导入路径使用（内部 re-export 了 `pkg/logs` 的全部
公开符号），因此 **logs 这一项可以保持不变**。计划于下一个 release 移除该别名壳，
新工程请直接使用 `github.com/maczh/mgin/pkg/logs`。

---

## 3. 不受影响的部分

以下路径在 v2.0 **未变更**，无需改动：

- 根入口包 `github.com/maczh/mgin`（`mgin.NewApp` / `mgin.Init` / `app.Run` 等签名与行为不变）。
- `cmd/`（`mgin new` 脚手架生成器）保持在顶层；其生成的工程模板已将导入改为 `pkg/...`。
- `go.mod` / `go.sum`：**未引入任何新的第三方依赖**。
- 配置键：`go.application.*` / `go.config.*` / `go.discovery.*` / `go.jwt.*` / `go.sys.*` / `go.casbin.*` 全部保留可读。
- `client.Call` 签名不变（新增 `CallCtx` 重载，老调用方行为一致）。

---

## 4. 新增的 v2 配置命名空间（见 docs/v2-package-map.md 与 application.example.yml）

- `go.framework.*`：框架自身元配置（healthEnabled / i18nEnabled / ratelimitEnabled / recoverEnabled / logRequests）。
- `go.runtime.*`：运行时元数据（commitHash / buildTime / goVersion / startedAt / pid），由构建期 `-ldflags` 注入 `CommitHash` / `BuildTime`，运行期由框架在 `Init()` 时填入 `startedAt` / `pid` / `goVersion`。

旧配置键可继续读取，`go.framework.*` 与 `go.runtime.*` 均为新增，**不替代旧键**。

### 4.1 框架开关的取值方式

业务代码可通过 `Config.Framework.HealthEnabled` 等字段直接读取。框架内部组件在 `Init()` 时按以下优先级判定：
1. 业务代码显式调用（如 `App.EnableHealth()`）优先；
2. 否则读 `Config.Framework.*`；
3. 全部未配置时保持各组件的零值（多数为关闭，业务无感）。

### 4.2 运行时元数据的注入方式

```bash
go build -ldflags "-X main.CommitHash=$(git rev-parse --short HEAD) \
                  -X main.BuildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
          -o myapp .
```

> `CommitHash` / `BuildTime` 须在工程入口的 `version.go` 中以 `var` 声明（参考 `cmd/scaffold.go` 中的 `tmplVersion` 模板）。

## 5. 新增的 v2 API（既有调用方行为不变）

| 新 API | 等价的旧 API | 备注 |
|--------|--------------|------|
| `client.CallCtx(ctx, service, uri, op)` | `client.Call(service, uri, op)` | 老 `Call` 内部委托 `CallCtx(context.Background(), ...)`，签名与行为不变 |
| `client.CallResilientCtx(ctx, service, uri, op, ro)` | `client.CallResilient(service, uri, op, ro)` | 同上 |
| `plugin.Plugin` 接口 + `plugin.Register` + `plugin.InitAll/CloseAll/HealthAll` | 直接调用组件 Init 函数 | 新代码推荐用 Plugin 系统；老 `mgin.Use / UsePlugin` 仍保留 |
| `Config.Framework.*` | 无（直接读 `config.Config.GetConfigBool("go.framework.*")`） | 零值兜底，未配置时所有开关关闭 |
| `Config.Runtime.*` | 无 | `startedAt` / `pid` / `goVersion` 由框架在 `Init()` 填入；`commitHash` / `buildTime` 由 `-ldflags` 注入 |

## 5. v2.1 新增 API 索引（P1 精选 4 项）

### 5.1 客户端负载均衡
- `pkg/loadbalancer.RoundRobinLB` / `RandomLB` / `LeastConnectionsLB` / `ConsistentHashLB`
- `pkg/loadbalancer.Default()` 取全局默认策略（默认 RoundRobin）
- `pkg/loadbalancer.Register(name, lb)` 注册自定义策略
- `client.Options.LoadBalancer` 字段指定调用方策略（为空走默认）
- `registry.RegistryClient.GetServices(service, group...)` 新接口返多实例 URL 列表
- per-instance 熔断：所有被熔断的实例会被 LB 跳过，重选不到时返 `client.ErrAllInstancesCircuitOpen`

### 5.2 Prometheus 指标
- 启用方式：配置 `go.metrics.enabled: true` 或调用 `app.EnableMetrics()`
- 端点：`GET /metrics`（与 `/health` 同样策略：baseRouter 最早注册）
- 关键指标：`mgin_http_requests_total` / `mgin_http_request_duration_seconds` / `mgin_http_requests_in_flight` / `mgin_build_info` / `mgin_plugin_health` / `mgin_dependency_up`
- 中间件：`r.Use(metrics.Middleware())` 自动记录所有路由的 HTTP 指标
- 上报 plugin/依赖健康：`metrics.SetPluginHealth(name, true)` / `metrics.SetDependencyUp(name, true)`

### 5.3 统一错误码增强
- `errcode.Definition{Code, HTTPStatus, MessageKey, Module}` 结构
- `errcode.New(module, code, httpStatus, msgKey)` 构造器（自动修正非法 HTTPStatus）
- `errcode.LookupDef(code)` 查预置 Definition
- `errcode.RegisterDef(d)` 业务覆盖预置或注册自定义
- `i18n.ErrorDef(def, args...)` / `i18n.ErrorDefT[T](def, args...)` 三向映射（i18n 文案 + 业务码）
- 框架 NoRoute 与 Recovery 已改用 ErrorDef，正确返 404/500

### 5.4 OpenTelemetry
- 框架 trace header 自动加 W3C `traceparent`（兼容既有 `X-Trace-Id`）
- `pkg/otel.SetTracerProvider(tp)` 注入业务自建 TracerProvider
- `pkg/otel.StartSpan(ctx, name, ...)` 便捷开 span
- `pkg/otel.IsEnabled()` 判断是否已启用（未启用时所有 span 走 noop）
- `client.callInternal` 在 OTel 启用时自动开 `mgin.client.call` span
