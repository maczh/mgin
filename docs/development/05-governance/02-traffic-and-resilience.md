# 流量治理：超时、重试、熔断、限流

## 1. 一次调用经过的治理决策链

```
业务 handler
  │ ① 是否被限流？           middleware/ratelimit（HTTP 入口）
  ▼
client.CallResilientCtx
  │ ② 熔断器是否允许？       breaker.Allow()
  │ ③ 发现实例 + 选择        registry.GetServices → loadbalancer.Pick
  │ ④ 实例级熔断过滤         全部打开 → ErrAllInstancesCircuitOpen
  │ ⑤ 发起 HTTP
  │ ⑥ 超时？                goroutine + timer
  │ ⑦ 失败且幂等？          指数退避重试
  ▼
下游服务
```

治理能力**互相影响**：重试会放大流量，熔断会减少可用实例，缓存会影响实例下线的感知速度。
不要孤立地调某一个参数。

---

## 2. 超时

| 层 | 配置 | 行为 |
|----|------|------|
| 客户端调用 | `ResilienceOptions.Timeout` | 到点返回 `ErrCallTimeout`；**不取消底层 HTTP** |
| HTTP server | 无内置读写超时 | 需要时自定义 `http.Server`（框架未暴露） |
| 数据库 | DSN/驱动 | 用 DSN 参数或 `context.WithTimeout` |
| 注册中心 | 各客户端内部超时 | 不可配 |

建议：

| 下游类型 | 超时 |
|----------|------|
| 内部查询 | 300ms–1s |
| 内部写操作 | 1–3s |
| 第三方 API | 3–10s（并配熔断） |
| 文件/报表 | 30s+，不要重试 |

> 超时是**调用方视角**的止损。下游可能仍在执行，因此写操作必须幂等或可对账。

---

## 3. 重试

### 3.1 两个重试入口

| 入口 | 行为 | 建议 |
|------|------|------|
| `Options.Retry = true` | 失败后**再试一次**（递归） | 不推荐：无退避、可能放大 |
| `CallResilient*` + `MaxRetries` | 指数退避 + 全抖动 | 推荐 |

退避公式：

```
delay = min(BaseDelay * 2^attempt, MaxDelay)
实际等待 = random[delay/2, delay)
```

默认 `BaseDelay=100ms`、`MaxDelay=2s`。

### 3.2 幂等保护

只有 `GET`/`HEAD`/`OPTIONS`/`TRACE` 默认重试。
`POST`/`PUT`/`DELETE`/`PATCH` 默认不重试；`Method` 为空等同 `POST`。

需要重试写操作时：

1. 业务提供幂等键（订单号、请求 ID），下游去重；
2. 显式设置 `MaxRetries`，并接受重复请求的可能；
3. 不要把“重试”当“事务补偿”。

---

## 4. 熔断

### 4.1 两层熔断

| 层 | 粒度 | 触发 | 结果 |
|----|------|------|------|
| 实例级 | `service@host` | 连续失败达阈值 | 该实例被跳过；全部跳过 → `ErrAllInstancesCircuitOpen` |
| 调用级 | `ResilienceOptions.Breaker` | 由业务构造并传入 | 打开后直接 `ErrCircuitOpen` |

### 4.2 状态机

```
Closed ──失败数≥5──► Open ──等待30s──► HalfOpen
  ▲                                      │
  └────────── 成功 ──────────────────────┘
                     │ 失败
                     ▼
                   Open
```

默认参数：

| 参数 | 默认 | 含义 |
|------|------|------|
| `maxFailures` | 5 | 窗口内失败数阈值 |
| `interval` | 60s | 统计窗口 |
| `timeout` | 30s | Open → HalfOpen 等待 |
| `maxHalfOpen` | 1 | 半开允许的并发探测数 |

### 4.3 使用

```go
breaker := client.GetBreaker("order")   // 按名字复用
body, err := client.CallResilientCtx(ctx, "order", "/api/v1/orders/1",
    &client.Options{Method: "GET"},
    &client.ResilienceOptions{Timeout: time.Second, Breaker: breaker})
```

应急重置：

```go
client.ResetBreakers()   // 仅测试或人工止血，会清空全部实例状态
```

### 4.4 注意

- 熔断器状态在**进程内**，多副本各自独立；
- 实例级熔断不会自动恢复实例选择，需等窗口滚动；
- 熔断打开不等于降级：框架不提供 fallback，需要业务返回兜底数据。

---

## 5. 限流

### 5.1 三种算法

| 算法 | 特点 | 参数 |
|------|------|------|
| `token_bucket` | 允许突发，按速率补充 | `rate`（令牌/秒）、`burst` |
| `sliding_window` | 窗口内精确计数，无临界突刺 | `rate`（窗口内上限）、`window`/`windowMs` |
| `concurrency` | 信号量控制同时处理数 | `maxConcurrent`、`wait`、`waitTimeoutMs` |

### 5.2 五种维度

`global`、`ip`、`path`、`ip_path`、`header`（`headerKey` 指定头）。

### 5.3 匹配优先级

1. `whitelist` 路径前缀命中 → 跳过；
2. `whiteIps` 命中 → 跳过；
3. `rules` 按声明顺序匹配（精确路径 / `/*` 前缀 / `:id` 路由模板）；
4. 命中规则后**不再叠加**全局规则；
5. 都未命中 → 使用 `go.ratelimit` 同层的全局规则。

### 5.4 被限流时的响应

```http
HTTP/1.1 200 OK            ← 默认 HttpStatus=200；建议配置为 429
X-RateLimit-Limit: 5
X-RateLimit-Remaining: 0
X-RateLimit-Policy: sliding_window
Retry-After: 12

{"status":1011,"msg":"请求过于频繁，请稍后再试"}
```

- `headers: true` 时才输出 `X-RateLimit-*` / `Retry-After`；
- `wait: true` 时请求会排队，超时后才拒绝；
- 计数在**进程内**，多副本会成倍放大放行量。

### 5.5 推荐配置

```yaml
go:
  ratelimit:
    enabled: true
    headers: true
    whitelist: ["/health", "/metrics"]
    rules:
      - name: login
        path: /api/v1/login
        methods: [POST]
        dimension: ip
        algorithm: sliding_window
        window: 60
        rate: 5
        httpStatus: 429
      - name: write
        path: /api/v1/orders
        methods: [POST]
        dimension: ip
        algorithm: token_bucket
        rate: 20
        burst: 40
        httpStatus: 429
```

---

## 6. 并发与 IP 防护

### 6.1 并发信号量

```go
app.Router.Use(limit.MaxAllowed(200))
```

- 达到上限时请求**阻塞等待**，不是立即拒绝；
- `n <= 0` 时直接 503。

适合保护重资源接口；不适合做全局限流（会耗尽 server 并发）。
需要“超时即拒绝”时用 `ratelimit` 的 `concurrency` 算法。

### 6.2 IP 白名单

```go
app.Router.Use(iplimit.CIDR("10.0.0.0/8,192.168.0.0/16"))
```

- 只允许列表内 IP，其余 403；
- 默认取 `RemoteAddr`；在代理后需设置 `iplimit.TrustedHeaderField`（如 `X-Real-IP`）；
- CIDR 解析失败返回 500。

---

## 7. 灰度与流量路由（现状）

框架**没有**内置的条件路由、权重分流、标签路由。可用的近似方案：

| 方案 | 做法 | 代价 |
|------|------|------|
| 分组隔离 | 灰度实例注册到不同 `group`，调用方按用户/租户选择 `Options.Group` | 需要在业务层决定分组 |
| 双服务名 | 新版本用 `order-v2` 注册，按开关调用 | 需要改名与额外部署 |
| 网关/Ingress | 由网关按 header/权重分流 | 依赖外部组件 |
| Mesh | VirtualService/DestinationRule | 依赖基础设施 |

如果只想要“10% 流量到新版本”，不要自己写随机数；用网关或 Mesh，避免框架层与基础设施层重复治理。

---

## 8. 组合效应对照表

| 组合 | 效果 | 风险 |
|------|------|------|
| 重试 × 熔断 | 失败快速熔断，减少无效重试 | 熔断阈值太低会被瞬时抖动触发 |
| 重试 × 限流 | 重试可能触发自身限流 | 重试次数计入限流配额 |
| 超时 × 重试 | 总耗时 ≈ `(Timeout+Backoff) × (Retries+1)` | 上游超时必须大于该值 |
| 缓存 × 熔断 | 缓存命中时不访问下游 | 缓存穿透时压力集中 |
| 并发限流 × 慢下游 | 排队导致 RT 升高 | 设置排队超时或改用 concurrency 限流 |

**经验值**：上游超时 = 下游 P99 × 1.5；重试最多 1–2 次；熔断阈值 ≥ 5 次且窗口 ≥ 30s。

---

## 9. 反模式

| 反模式 | 后果 |
|--------|------|
| 网关已重试，客户端再重试 | 请求量指数放大 |
| 对所有 POST 开重试 | 重复下单/扣款 |
| 只配超时不配熔断 | 下游恢复前持续打满 |
| 限流 `httpStatus` 保持 200 | 客户端/网关无法识别限流 |
| 用 `limit.MaxAllowed` 做全局限流 | 请求堆积，内存上涨 |
| 依赖发现缓存做秒级下线 | 最长 60s 仍可能打到旧实例 |
