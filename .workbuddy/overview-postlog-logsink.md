# 接口日志异步化改造（postlog → logsink）

## 做了什么

把 `middleware/postlog` 里"产生日志 + 硬编码写 MongoDB"的耦合拆开：中间件只负责产生 `models.PostLog` 并异步抛出，
具体落库/发消息/写 ES 的动作交给注册的 handler 完成，外部插件（mgkafka、elasticsearch 插件等）无需改动框架即可接收全量接口日志。

## 现状问题（改造前）

| 问题 | 说明 |
|---|---|
| 目标硬编码 | 只能写 MongoDB；`go.log.db=elasticsearch`、`go.log.kafka.use` 从未真正生效（无发送代码） |
| 阻塞投递 | `accessChannel <- ...` 队列长度 100，满了直接阻塞请求协程 |
| 单消费者 | 一个 goroutine 串行处理，无扩展点，handler panic 会拖垮整个日志链路 |
| 无优雅关闭 | `SafeExit` 不排空，退出即丢日志 |
| 无意义往返 | 先 `ToJSON` 再 `Unmarshal`，纯属浪费 |
| 隐藏缺陷 | swagger uri 为空时 `strings.Contains(path, "")` 恒为真，**全部接口日志被跳过** |

## 新增/改动文件

- **新增 `logsink/sink.go`**（核心）：Handler 接口、注册中心、每 handler 独立队列 + 独立消费协程、非阻塞投递、
  丢弃/失败计数与 60s 限流告警、panic recover、`Unregister`/`Close` 排空、`Stats()` 观测。
- **新增 `logsink/handler.go`**：`HandlerFunc` 适配器，支持链式 `WithClose`，一行挂接。
- **新增 `middleware/postlog/mongo.go`**：内置 `mongodb` handler，行为与旧消费者一致（含多库 `dbName` 校验）；
  `go.log.req` 非空且 `used` 含 mongodb 时自动注册，保持存量应用零改动。
- **改 `middleware/postlog/logger.go`**：移除 `accessChannel`/`handleAccessChannel`/`Mgo`；
  新增 `RegisterHandler`、`Close`；投递改为 `logsink.Emit(postLog)`；修复 swagger 空路径跳过全部日志的缺陷。
- **改 `config/configure.go`**：新增 `go.log.sink.{queue,workers,shutdown}`，默认 1024 / 1 / 3000ms。
- **改 `mgin.go`**：`SafeExit()` 开头先 `postlog.Close()` 排空日志，再关闭数据库连接。
- **改 `cmd/templates.go`**：脚手架 main 模板增加挂接日志 handler 的注释示例。
- **文档**：README_CN 新增第 25 章 + 11.6 更新 + 目录/版本说明；README_en 同步（11.6 + 第 25 章）。
- **测试**：`logsink/sink_test.go`（注册/去重/丢弃/隔离/排空/并发）、`middleware/postlog/logger_test.go`（端到端冒烟），`-race` 全绿。

## 关键设计决策

1. 接口放独立轻量包 `logsink`（只依赖 `models`），插件 import 它不会被迫拉入 gin / mongo 依赖链。
2. 每个 handler 独立队列 + worker，慢 handler 互不影响，可按 handler 统计丢弃量。
3. 队列满即丢弃并计数告警，绝不让日志写不出去拖慢接口。
4. 关闭时不 close 通道（避免 send panic），改用 `done` 信号 + drain，超时放弃。

## 兼容性

- `postlog.RequestLogger()` 签名与行为不变；内置 mongo handler 自动注册，存量配置无需改动。
- **破坏性**：移除导出变量 `postlog.Mgo`（其 `Set` 的效果每次请求都会被覆盖，本就无效）。自定义落库改用 `logsink.Register`。
- `go.log.kafka.*` 配置保留，语义改为"由 mgkafka 插件以 handler 形式接入"，`topic` 仍可复用。

## 应用侧挂接示例

```go
logsink.MustRegister(logsink.NewHandlerFunc("kafka", func(e *logsink.Entry) error {
    return mgkafka.Kafka.Send(config.Config.Log.Kafka.Topic, utils.ToJSON(e))
}).WithClose(mgkafka.Kafka.Close))
```

## 验证

```
go test ./logsink/ ./middleware/postlog/ -race -count=1   # ok
go build ./...                                             # ok
```
