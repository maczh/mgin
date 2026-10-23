# Runbook：按症状排障

> 用法：搜症状 → 按“确认”逐项排除 → 执行“处置”。
> 所有命令以 Linux + K8s 为例，按实际环境替换。

---

## 1. 启动失败

### 1.1 进程立即退出

| 可能原因 | 确认 | 处置 |
|----------|------|------|
| VictoriaLogs 配置非法（唯一会 panic 的配置） | 日志含 `victorialogs` panic 栈 | 修正 URL/证书，或 `go.log.victorialogs.client=false` |
| 端口被占用 | `ss -lntp \| grep 8080` | 换端口或停占用进程 |
| 业务初始化主动退出 | 日志含 `bootstrap`/`os.Exit` | 修复依赖或临时跳过 |
| 配置路径错误 | `读取配置文件错误: open ...` | 用 `-f` 绝对路径 |

### 1.2 进程在但无响应

```bash
curl -v localhost:8080/health/live
ss -lntp | grep 8080
```

- `port`/`port_ssl` 都为 0 → 只等信号，不监听；
- 只配了 `port_ssl` 但证书缺失 → HTTPS 启动失败，HTTP 可能也没开；
- 容器端口未暴露/Service 选择器错误。

### 1.3 启动很慢

- `/health/ready` 的依赖检查在重连（MySQL 网络不通时可能数十秒）；
- 注册中心地址不可达，注册阻塞；
- 日志中查 `正在连接` / `连接失败`。

---

## 2. 配置不生效

| 症状 | 确认 | 处置 |
|------|------|------|
| 改了 YAML 无变化 | 是否热更新项？ | 除日志级别外都需重启 |
| 组件未初始化 | 日志无 `xxx 初始化` | 查 `go.config.used` 是否含名称 |
| 子配置读不到 | `GetConfigString` 返回空 | 子配置不在 `Config.Cnf`，改用组件配置 |
| 配置中心拉不到 | 日志有 `读取配置错误` | 核对 `{prefix}-{env}.yml` 与 group/project |
| `go.framework.*` 配了无效 | — | 该节点多数未接线，改用 `go.health.*` 等 |

---

## 3. 探针异常

### 3.1 `/health/live` 404

- `go.health.enabled` 未开，且未调用 `EnableHealth()`；
- 请求打到了错误端口/路径（`/healthz` 是别名）；
- 网关改写了路径。

### 3.2 `/health/ready` 503

```bash
curl -s localhost:8080/health/ready
# {"status":"fail","dependencies":{"mysql":"fail"}}
```

| 依赖 fail | 排查 |
|-----------|------|
| mysql | DSN、网络、库是否存在、账号权限；看启动日志 |
| redis | 地址/密码/DB 号；`PING` |
| mongodb | URI、认证库；`mongosh` 连接 |

注意 `used` 启用了但连接失败时，ready 会一直 503 —— 要么修复依赖，要么从 `used` 移除。

### 3.3 `/health/startup` 一直 503

- 未调用 `MarkStarted()`；
- `go.health.autoStarted=false`（默认）且业务忘了标记；
- 业务初始化卡住，没走到标记代码。

---

## 4. 调用失败

### 4.1 “获取服务主机IP端口失败”

按顺序确认：

```bash
# 1) 服务是否注册
curl "http://nacos:8848/nacos/v1/ns/instance/list?serviceName=order&groupName=DEFAULT_GROUP"

# 2) 分组是否一致（provider 注册分组 vs Options.Group）
# 3) 本进程 used 是否含注册中心
# 4) 注册中心地址/网络
```

### 4.2 全部实例熔断

错误：`client: all instances circuit-open`

- 下游确实不可用 → 修复下游；
- 阈值过低被抖动触发 → 调整 `maxFailures/interval`；
- 应急：`client.ResetBreakers()`（会清空全部状态）。

### 4.3 超时

错误：`client.ErrCallTimeout`

- 下游慢：查下游 P99；
- 超时设置过短；
- 连接池耗尽：查本进程连接数与下游连接数；
- 注意超时不会取消底层请求，下游可能仍在处理。

### 4.4 返回 4xx/5xx 但 err 为 nil

`client.Call` 不把非 2xx 当错误。必须解析响应体/状态：

```go
resp := client.CallT[Order](...)
if resp.Code != vo.SUCCESS { ... }
```

### 4.5 响应解析为空

- 下游用了 `models.Result`（`status`）而客户端用 `CallT`（期望 `code`）；
- 下游返回非 JSON（HTML 错误页）；
- 打印原始 `body` 确认。

---

## 5. 认证与授权

| 现象 | 原因 | 处置 |
|------|------|------|
| 所有接口 401 | `go.jwt.secret` 为空；token 过期；签名算法非 HMAC | 检查配置与 token |
| 401 但 token 正常 | `Authorization` 未带 `Bearer ` 或格式错 | 按中间件规则传 |
| Casbin 403 | 策略缺失 / 路径与方法不匹配 | 查 `casbin_rule`，用 `keyMatch2` 规则核对 |
| Casbin 401 | 没有 `claims`（JWT 中间件不写 claims） | 增加写入 claims 的中间件 |
| 登录接口被限流 | 规则过严 | 调整 `rate/window` 或加白名单 |
| 管理接口被 IP 拦截 | `iplimit.CIDR` 是白名单 | 加 IP 或设置可信头 |

---

## 6. 限流

| 现象 | 确认 | 处置 |
|------|------|------|
| 限流不生效 | `go.ratelimit.enabled`；规则路径匹配；`used` 不需要 ratelimit | 修正配置并重启 |
| 误伤健康检查 | `whitelist` 未包含 `/health` | 加白名单 |
| 所有请求共用一个桶 | `dimension: global` | 改 `ip`/`ip_path` |
| 多副本放行量翻倍 | 进程内计数 | 接受，或改用网关/Redis 限流 |
| 响应 200 但被限 | 默认 `httpStatus=200` | 配 `httpStatus: 429` |

---

## 7. 定时任务

| 现象 | 原因 | 处置 |
|------|------|------|
| 任务未执行 | `used` 不含 `job` 或 `enabled=false` | 修正配置 |
| 多副本重复执行 | 每个副本都启用了 job | 只保留一个副本启用 |
| 任务一直“执行中” | handler 阻塞/超时未设 | 设置 `timeout`，检查 handler |
| 日志表暴涨 | `logRetainDays=0` 或清理未跑 | 设保留天数 |
| cron 不触发 | 表达式时区/格式错 | 用 5/6 段或 `@every`；配 `timezone` |
| 建表失败 | 无 DDL 权限 | 预建表或 `initdb=false` |

---

## 8. 日志与指标

| 现象 | 原因 | 处置 |
|------|------|------|
| `/metrics` 404 | 未 `EnableMetrics()` | 加调用 |
| 无 `mgin_http_*` | 未挂 `metrics.Middleware()` | `app.Router.Use(...)` |
| 访问日志缺失 | `go.log.get=off`、路径被跳过、`go.log.req` 为空 | 检查配置 |
| 日志无 requestId | 异步 goroutine | `CopyPreHeaderToCurRoutine` |
| 日志队列丢弃 | 下游写入慢 | 扩容/降低日志量；看 `Dropped` |
| `mgin_build_info` 缺失 | 未 `SetBuildInfo` | 启动时调用 |

---

## 9. 性能问题

### 9.1 CPU 高

```bash
go tool pprof http://localhost:6060/debug/pprof/profile   # 若已开启 pprof
```

常见：JSON 序列化大对象、正则、日志格式化、锁竞争、GC（大对象分配）。
框架未内置 pprof 路由，需要临时加：

```go
import _ "net/http/pprof"
go http.ListenAndServe("127.0.0.1:6060", nil)
```

### 9.2 内存高

- 发现缓存条目过多（服务数 × 分组）；
- 日志队列积压（配置上限 100，可接受）；
- 大响应体被 postlog 缓存（`go.log.get=full`）；
- goroutine 泄漏（超时未取消、ticker 未停）。

```bash
go tool pprof http://localhost:6060/debug/pprof/heap
```

### 9.3 延迟高

按链路排查：

1. `/metrics` 看哪个 path 慢；
2. 日志找 requestId 的耗时；
3. 下游调用耗时（VictoriaLogs client 日志）；
4. DB 慢查询；
5. 连接池等待（`sql.DBStats.WaitCount`）。

---

## 10. 注册中心

| 现象 | 原因 | 处置 |
|------|------|------|
| 容器内注册了 127.0.0.1 | 未设 `go.application.ip` | 设为 Pod IP 或让注册中心使用可达地址 |
| 实例重复/互相覆盖 | 实例 ID 不含 IP:Port | 检查注册中心实例 ID 规则 |
| 下线后仍有流量 | 发现缓存 TTL | 等 TTL 或缩短 `cacheTTL` |
| Polaris 调不到 | 不会自动注册 | 手动 `Register` 或改用其他后端 |

---

## 11. 升级/回滚后异常

- 检查 `go.config.used` 是否被新版本解析出不同结果；
- 检查错误响应结构是否变化（v2 起 404/500 不再返回 200）；
- 检查废弃配置是否仍在用（`logs/` 兼容壳只保留一个 release）；
- 回滚后确认探针与注册状态，再逐步恢复流量。
