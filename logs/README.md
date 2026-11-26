# logs

MGIN 日志输出，使用 `{}` 替换参数：

```go
logs.Info("用户 {} 登录", userID)
```

级别由 `go.logger.level`（debug/info/warn/error）控制；`go.application.debug=true` 强制 debug。旧的 `go.logger.out/file` 配置保持兼容。

- 多目标 text/JSON、字段选择、文本模板、协程 ID、外部 logsink handler 和 GORM 日志配置：见 [日志与可观测性](../docs/development/06-observability/03-logging.md)。
- VictoriaLogs 通过 OTLP/HTTP protobuf 上报（`/insert/opentelemetry/v1/logs`）：见 [VictoriaLogs 接入](../docs/victorialogs.md)。
- 接口日志（postlog）请求/响应缓冲各封顶 1 MiB；chunked 请求体只读取上限+1 字节，剩余流仍交给业务 handler。
