# 日志

框架有三类日志，配置与落点不同，不要混为一谈。

| 类别 | 产生方 | 内容 | 默认去向 |
|------|--------|------|----------|
| 运行日志 | `logs.Info/Warn/Error/Debug` | 启动、连接、业务关键事件 | console/file/VictoriaLogs |
| 接口日志（PostLog） | `middleware/postlog` | 一次 HTTP 请求/响应 | MongoDB（条件满足时） |
| 调用日志 | `client` | 下游调用的耗时与结果 | VictoriaLogs（`client: true`） |

## 1. 运行日志

```go
logs.Debug("缓存命中 key={}", key)
logs.Info("加载{}成功，耗时{}ms", "user", cost)
logs.Warn("下游超时，使用旧缓存")
logs.Error("查询失败: {}", err)
```

规则：

- 占位符固定 `{}`，按参数顺序替换；不要用 `%s`。
- 每条日志自动带 `[requestId] ` 前缀（无请求时为空）。
- 级别：`go.logger.level`（默认 `error`）；`go.application.debug: true` 强制 `debug`。
- 字段白名单（`outputs[].fields`）：`time, level, message, app, request_id, goroutine_id, package, file, function, line`。
- `logtext` 模板示例：`"[{time}] [{level}] [{request_id}] {message}"`。

### 输出目标

```yaml
go:
  logger:
    level: info
    outputs:
      - target: console
        format: text
      - target: file
        file: ./logs/order
        format: json
      - target: victorialogs
        format: json
  log:
    victorialogs:
      url: https://vl.example.com:9428
      api_token: ""
      timeout: 5000
```

- `file`：写到 `<file>.YYYY-MM-DD.log`，**不自动清理**。
- `victorialogs`：`InitVictoriaLogs()` 失败会 panic，启动前先验证连通性。
- 兼容旧配置：`out: console,file` + `file: ./logs/app`。
- 自定义目标：`logs.RegisterHandler(h logsink.RecordHandler, opts ...logsink.Option)`。

## 2. 日志管道（logsink）

```go
logsink.Register(handler, logsink.Option{QueueSize: 100, ShutdownTimeout: 3 * time.Second})
```

- 每个 handler 一条队列、**1 个消费协程**，保证 FIFO。
- 队列满时**淘汰最旧**并计数，不阻塞业务。
- `Close()` 先停止接收，再排空；`SafeExit` 会调用（超时 3s）。
- `logsink.Stats()` 可查看投递/丢弃/队列长度，建议接入指标。

配置：`go.log.sink.queue`（默认 100，代码默认 1024）、`workers`（强制 1）、`shutdown`（毫秒）。

## 3. 接口日志（PostLog）

启用条件（**同时**满足）：

1. `go.log.req` 非空（集合名）；
2. `go.config.used` 包含 `mongodb`；
3. 多库模式下 `go.log.dbName` 已配置（否则 handler 返回错误并丢弃）。

```yaml
go:
  log:
    req: orderRequestLog
    dbName: order
    get: line      # full/line/off：GET 响应体记录策略
```

字段：`time/requestId/responseTime/method/contentType/uri/clientIP/requestHeader/requestParam/requestBody/responseStr/responseMap/appName/apiName`。

- 请求体/响应体超长会截断；`get: line` 只记录 120 字符。
- 代码不创建索引，建议给 `time` 建 TTL 索引、`requestId` 建普通索引。
- 敏感信息（密码、token）不会被自动脱敏，业务应在日志层过滤或配置 `go.log.get: off`。

## 4. 调用日志

`go.log.victorialogs.client: true` 时，`client.Call*` 异步上报：

| 字段 | 内容 |
|------|------|
| `request_id` | 当前链路 ID |
| `service` / `uri` / `method` | 目标与动作 |
| `code` / `msg` | 下游返回 |
| `cost` | 耗时（ms） |

它与运行日志走同一个 VictoriaLogs 客户端，但独立字段，便于按服务聚合。

## 5. 查询与关联

```bash
# 按 request_id 串起一次请求
grep 'abc123' logs/order-2026-10-23.log

# MongoDB 查询接口日志
mongosh --eval 'db.orderRequestLog.find({requestId:"abc123"}).sort({time:1})'

# 统计 5xx
grep '"status":5' logs/order-2026-10-23.log | wc -l
```

VictoriaLogs 中用 `request_id` 字段关联运行日志与调用日志。

## 6. 约束与陷阱

1. **不要**在业务里直接 `fmt.Println`/标准库 `log`；级别与目标不受控。
2. `logs.Error(err.Error())` 这类调用可用，但推荐 `logs.Error("{}", err)` 保留占位符一致性。
3. 队列满会丢日志：高流量场景调大 `queue` 或降低日志量。
4. 文件日志不会自动轮转删除，需外部清理。
5. 接口日志只支持 MongoDB；未启用 mongodb 时即使配置了 `go.log.req` 也不会落库。
6. 日志中的 `request_id` 依赖 trace 中间件；异步协程需按 [链路追踪](02-tracing.md#3-跨协程传播) 恢复上下文。
