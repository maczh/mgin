# 流量管控与容错

治理能力不是孤立开关：一次请求会依次经过「来源限制 → 限流 → 并发限制 → 鉴权 → 业务」，调用下游时再叠加「发现 → 重试 → 超时」。本文按这条链路说明组合效应。

## 1. 一次请求的治理决策链

```
客户端
  │
  ├─ CORS（预检直接 204）
  ├─ iplimit.CIDR        ← 不在白名单网段：403
  ├─ ratelimit           ← 超限：默认 200 + status=1011
  ├─ limit.MaxAllowed    ← 并发满：请求排队/499
  ├─ jwt                 ← 无/非法 token：401（无 body）
  ├─ casbin              ← 无策略：403
  ├─ XSS 过滤            ← 策略错误：500
  ├─ 业务 handler
  │     └─ client.Call → registry（缓存） → 下游
  │           └─ Retry=true：失败重试 1 次（无超时）
  └─ 响应
```

顺序由 `baseRouter` 与业务 `Use` 的先后决定。框架默认只挂 CORS、trace、postlog、recovery；其余需显式挂载。

## 2. 限流（ratelimit）

三种算法：

| 算法 | 适用 | 关键参数 |
|------|------|----------|
| `token_bucket` | 允许突发 | `rate`（每秒补充）、`burst` |
| `sliding_window` | 严格窗口配额 | `rate`（窗口内次数）、`window`（秒） |
| `concurrency` | 保护慢资源 | `maxConcurrent` |

五种维度：`global`、`ip`、`path`、`ip_path`、`header`（需指定 `headerKey`）。

```yaml
go:
  ratelimit:
    enabled: true
    whitelist: ["/health", "/actuator"]
    whiteIps: ["127.0.0.1"]
    rules:
      - name: "全局"
        algorithm: "token_bucket"
        dimension: "global"
        rate: 100
        burst: 20
      - name: "登录按 IP"
        path: "/api/login/*"
        methods: ["POST"]
        algorithm: "sliding_window"
        dimension: "ip"
        rate: 5
        window: 60
        httpStatus: 429
```

规则按声明顺序匹配，未命中回落第一条全局规则。限流器按 key 懒创建，`idleTimeout` 后回收。

**约束**：计数在进程内，多实例部署时每个实例独立计数，总配额 = 单实例配额 × 副本数。需要全局限流时用网关/Redis 方案。

## 3. 并发限制（limit）

```go
app.Router.Use(limit.MaxAllowed(100)) // 同时最多 100 个请求
```

- 纯信号量，`n<=0` 不限制。
- 等待期间客户端断开返回 499。
- 与 ratelimit 的区别：`limit` 只看「同时处理数」，不关心速率；`ratelimit` 关心「单位时间请求数」。

## 4. 超时与重试

| 环节 | 现状 | 建议 |
|------|------|------|
| HTTP 服务端 | `http.Server` 未设置读写超时 | 在网关/Ingress 设超时；慢接口自行用 context |
| `client` 调用 | **无超时**，`Retry` 仅 1 次 | 关键调用用 goroutine + `time.After` 兜底 |
| 数据库 | 由 GORM/驱动与池控制 | 用 `WithContext` 传递超时 |
| 事务 | `TransactionOn` 使用传入 ctx | 给 ctx 设置 deadline |

```go
ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
defer cancel()
user, err := userService.Get(ctx, id)
```

## 5. 熔断与降级

框架**没有内置熔断器**。可用的降级手段：

1. **服务发现降级**：`CachedRegistry` 在注册中心不可用时复用旧实例。
2. **缓存降级**：读失败时回退缓存/默认值（业务实现）。
3. **限流保护**：用 `concurrency` 规则保护下游，避免雪崩。
4. **超时兜底**：见上。

需要熔断时，在 `client` 外层包一层（如 `sony/gobreaker`），按服务名维护状态；不要修改框架内部。

## 6. 组合效应（重要）

| 组合 | 效果 | 风险 |
|------|------|------|
| 限流 + 重试 | 重试会消耗配额，可能放大限流 | 重试只对幂等请求开启 |
| 限流 + 多实例 | 配额按实例放大 | 需要网关限流 |
| JWT + Casbin | Casbin 依赖 JWT 的 `claims` | 顺序反了会全部 401 |
| CORS + 鉴权 | 预检不携带 Authorization | 预检必须直接 204，不能拦 |
| 探针 + 鉴权 | 探针先注册，天然绕过 | 不要手工把探针挂到受保护组 |
| 缓存 + 限流 | 限流拒绝的请求不会读缓存 | 热点 key 仍可能击穿 |

## 7. 灰度与流量分配

框架不提供标签路由/权重分流；可实现的方式：

- **注册中心侧**：给实例打元数据/权重，按分组（Nacos group）隔离灰度。
- **网关侧**：按 header/用户比例分流到不同服务名。
- **业务侧**：读取配置开关，在 handler 内部分流。

不建议在 `client` 内做随机权重——每次调用都重新选实例，无法保证会话粘性。

## 8. 配置检查清单

- [ ] 限流是否设置了 `httpStatus`（否则客户端只看到 200）
- [ ] 白名单是否覆盖 `/health` 与 `/metrics`
- [ ] 重试是否只用于幂等接口
- [ ] 调用下游是否有超时兜底
- [ ] 多副本时 job 是否只在一个实例启用
- [ ] 并发限制是否大于数据库连接池上限（否则排队在 DB）
