# logs

MGIN 日志输出，使用 `{}` 替换参数：

```go
logs.Info("用户 {} 登录", userID)
```

级别由 `go.logger.level`（debug/info/warn/error）控制。
`go.application.debug=true` 强制 debug。旧的 `go.logger.out/file` 配置保持兼容。
多目标 text/JSON、字段选择、文本模板、协程 ID、外部 logsink handler 和 GORM 日志配置
参见 [README_CN.md](../README_CN.md#logs-运行日志的多目标输出)。
