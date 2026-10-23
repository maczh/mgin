# 日志

MGin 的日志分三类：

| 类型 | 产生者 | 出口 | 用途 |
|------|--------|------|------|
| 运行日志 | `pkg/logs` | 控制台/文件/Kafka/VictoriaLogs/Mongo | 排障、审计 |
| 访问日志 | `middleware/postlog` | `logsink`（默认 Mongo） | 接口调用记录 |
| 调用日志 | `pkg/client` | VictoriaLogs（启用时） | 下游调用耗时与错误 |

---

## 1. 运行日志

```go
import "github.com/maczh/mgin/v2/pkg/logs"

logs.Debug("缓存未命中 key={}", key)
logs.Info("订单创建成功 id={} user={}", id, userId)
logs.Warn("下游响应慢 cost={}ms", cost)
logs.Error("创建订单失败: {}", err.Error())
```

### 1.1 占位符

**只支持 `{}`，按参数顺序替换，不是 `%s`。**

```go
logs.Info("user={} role={}", "tom", 2)   // user=tom role=2
logs.Info("user=%s", "tom")              // 原样输出 user=%s
```

多出的 `{}` 保留原文；多余参数被忽略。

### 1.2 级别

| 级别 | 配置 | 行为 |
|------|------|------|
| debug | `go.logger.level: debug` | 全部输出 |
| info | 默认 | 不输出 debug |
| warn | | 不输出 info/debug |
| error | | 只输出 error |

`go.application.debug: true` 会强制 debug 级别（每次输出实时判断）。
级别可热更新——修改配置文件后无需重启（其他配置不保证）。

### 1.3 输出目标

单目标：

```yaml
go:
  logger:
    level: info
    out: console           # console|file，可 "console,file"
    file: logs/order       # 生成 logs/order.2026-10-23.log
```

多目标（推荐生产）：

```yaml
go:
  logger:
    outputs:
      - target: console
        format: text
      - target: file
        format: json
        file: logs/order
        fields: [time, level, requestId, message]
      - target: victorialogs
```

`logsink` 支持的 target：`console`、`file`、`kafka`、`mongo`、`victorialogs`。
未知 target 会被忽略（队列中丢弃）。

### 1.4 自动字段

每条日志自动带：

| 字段 | 来源 |
|------|------|
| time | 当前时间（RFC3339Nano） |
| level | 级别 |
| message | 格式化后的文本 |
| app | `go.application.name` |
| requestId | `trace.GetRequestId()` |
| goroutineId | 当前 goroutine id |

---

## 2. 访问日志（postlog）

默认挂载，无需配置。记录：

- 方法、URI、状态码、耗时、客户端 IP、User-Agent；
- 请求头、query、请求体（受 `go.log.get` 控制）；
- 响应体：`full` / `line` / `off`；
- 文件响应（zip/pdf/xlsx…）只记录“输出 xx 格式文件，大小 xxMB”，不记录二进制。

### 2.1 配置

```yaml
go:
  log:
    get: line          # full|line|off
    req: orderReqLog   # Mongo 集合名；为空则不写 Mongo
    db: mongodb
    dbName: ""
    sink:
      queue: 100       # 1–100，超出按 100
      shutdown: 3000   # 关闭排空超时（ms）
```

### 2.2 跳过规则

以下路径不记录：`/`、`/docs/`、`/swagger/`、`go.sys.swagger.uri`。

### 2.3 自定义 Handler

```go
postlog.RegisterHandler(myHandler)   // 与默认 Mongo Handler 并存
```

`Handler` 接口：

```go
type Handler interface {
    Name() string
    Write(*models.PostLog) error
}
```

写入失败不会阻塞请求，但会丢失该条日志。

---

## 3. 日志投递（logsink）

`logsink` 是异步队列 + 多 Handler：

```
logs.Info / postlog
   └─ EmitRecord → 每个 Handler 一个 channel（容量可配）
        ├─ 队列满 → 丢弃最旧（计数）
        └─ worker（固定 1，保证 FIFO）→ Handler.Write
```

### 3.1 参数

| 参数 | 默认 | 说明 |
|------|------|------|
| `WithQueueSize(n)` | 1024 | 直接调用时的队列容量 |
| `WithWorkers(n)` | 1 | **恒为 1**（保证顺序） |

通过配置 `go.log.sink.queue` 时有效范围 1–100，超出按 100。

### 3.2 监控

```go
stats := logsink.Stats("mongo")
// Enqueued / Dropped / Written / Failed / QueueLen
```

队列满时 `Dropped` 增长，需要扩容或降低日志量。

### 3.3 关闭

`app.Run` 收到信号后会先 `postlog.Close()` 排空，再关闭插件。
异步写 Mongo 的日志可能仍有少量在途，`sink.shutdown` 控制等待上限。

---

## 4. VictoriaLogs

```yaml
go:
  log:
    victorialogs:
      client: true
      url: "http://127.0.0.1:9428"
      # 其余字段见 08 配置参考
```

- 配置非法（URL 缺失/证书错误）会在 `InitVictoriaLogs` 时 **panic**；
- `client` 为 true 时，`pkg/client` 会额外投递调用日志；
- 采集端协议与字段见 [victorialogs.md](../../victorialogs.md)。

---

## 5. 敏感信息

必须避免：

| 禁止 | 原因 |
|------|------|
| 打印密码、Token、Cookie | 泄露凭证 |
| 打印完整身份证/手机号 | 合规风险 |
| `go.log.get: full` 记录登录/支付响应 | 响应含 token/金额 |
| 把整个请求体结构体打进日志 | 体积大且可能含敏感字段 |

推荐：

```yaml
go:
  log:
    get: line
```

并在业务中脱敏：

```go
logs.Info("login user={} ip={}", maskMobile(mobile), ip)
```

---

## 6. 日志规范

| 场景 | 级别 | 示例 |
|------|------|------|
| 启动/关闭/关键状态变更 | info | `服务启动完成 port={}` |
| 可恢复异常 | warn | `下游超时，准备重试 attempt={}` |
| 需要人工处理 | error | `数据库连接失败: {}` |
| 调试细节 | debug | `缓存命中 key={}` |

- 一条日志只表达一件事；
- 错误日志必须含上下文（服务名、ID、地址）；
- 同一错误只在一层记录，避免重复刷屏；
- 不要用日志做审计（访问日志应入 Mongo/VL 并可检索）。

---

## 7. 排障

| 现象 | 原因 | 处理 |
|------|------|------|
| 日志没有 requestId | 非 HTTP 协程 / 未复制 header | `CopyPreHeaderToCurRoutine` |
| 日志文件不生成 | `out` 未含 `file` 或目录不可写 | 检查 `go.logger.file` 与权限 |
| 级别改了不生效 | 改的是子配置文件/拼错键 | 确认 `go.logger.level` 在主配置 |
| 接口日志缺失 | `go.log.get=off`、路径被跳过、集合名为空 | 检查配置与 `go.log.req` |
| `Dropped` 持续增长 | 队列满/下游慢 | 扩容、降低日志量、异步批量化 |
| 启动 panic | VictoriaLogs 配置非法 | 检查 URL/证书，或关闭 `client` |
