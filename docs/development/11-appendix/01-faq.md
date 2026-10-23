# 常见问题（FAQ）

## 启动与配置

**Q1. 启动时报找不到配置文件，但我明明放在项目里了？**
`NewApp` 对不含路径分隔符的文件名会拼到**可执行文件目录**。`go run .` 的二进制在临时目录，因此找不到。用绝对路径，或先 `go build` 再运行。详见 [配置模型](../04-configuration/01-model.md#2-定位规则)。

**Q2. 配置里的 `used: "mysql"` 写了，但日志说未使用？**
检查拼写与空格；`used` 是**子串匹配**，大小写敏感。也可能读到了另一个环境的配置（检查启动日志中的配置路径与 `env`）。

**Q3. 改了配置不生效？**
运行期只读一次。改端口/TLS/路由/中间件需重启；数据源与 `used` 可用 `PUT /cfg/reload` 重载（需 `go.config.reload.token`）。

**Q4. 重载返回 404？**
未配置 `go.config.reload.token`，路由未注册。配置后需重启一次。

**Q5. 重载返回 403？**
来源 IP 不是 loopback/私网，或 token 不匹配。重载接口不信任 `X-Forwarded-For`。

## 数据源

**Q6. `GetConnection()` 报 `multi get connection must specify a database name`？**
多库模式（`go.data.mysql.multidb: true`）必须传库名。Casbin 内部不传库名，因此多库模式下不可用。

**Q7. MySQL 连接成功但查询报错？**
检查 DSN 的 `parseTime=True&loc=Local`、库名、字符集；临时开 `mysql_debug: true` 看 SQL。

**Q8. SQLite 报 `SQLite not opened` 或文件不存在？**
未启用 `used: "sqlite"`，或路径相对可执行目录不存在。用绝对路径并确保目录可写。

**Q9. Redis 挂了，`/health/ready` 还是 200？**
`db.Redis.Check()` 当前是空实现，恒返回 nil。需要监控 Redis 时用业务探针或外部监控。

**Q10. DAO 缓存不生效/脏数据？**
- `CacheConfig.Enabled` 是否为 true、`Store` 是否非 nil；
- `TTL=0` 表示永不过期（不是禁用）；
- 写操作后是否使用了同一 `Store` 与 `Prefix`（否则删不到）。

## 鉴权与安全

**Q11. JWT 全部 401？**
`go.jwt.secret` 为空、算法不是 HMAC、或头不是 `Authorization: Bearer <token>`。401 没有响应体，需看日志。

**Q12. Casbin 启动报找不到模型文件？**
默认 `rbac_model.conf`，仓库只提供 `casbin.conf`。配置 `go.casbin.model_file: casbin.conf`。

**Q13. Casbin 返回 500「权限服务不可用」？**
Enforcer 初始化失败，通常是没有 MySQL 或模型语法错误。检查 `used` 与模型文件。

**Q14. 明明传了 token，Casbin 还是 403？**
策略未配置，或路径/方法与策略不一致（`keyMatch2`）；确认策略表 `casbin_rule` 有对应 `p` 规则。

**Q15. XSS 过滤把富文本标签删了？**
默认 `StrictPolicy`。对富文本字段用 `UGCPolicy` 或加入 `FieldsToSkip`；不要全局放宽。

## 日志与可观测

**Q16. 接口日志没有落库？**
需同时满足：`go.log.req` 非空、`used` 含 `mongodb`、多库时配 `go.log.dbName`。

**Q17. 日志里没有 request_id？**
异步协程未恢复上下文；用 `trace.CopyPreHeaderToCurRoutine`。或请求未经过 trace 中间件。

**Q18. 日志丢了？**
`logsink` 队列满会淘汰最旧。看 `logsink.Stats()` 的 `Dropped`，调大 `go.log.sink.queue` 或降低日志量。

**Q19. `/metrics` 404 或没有 HTTP 指标？**
404：未 `EnableMetrics`/`go.metrics.enabled`；只有构建指标：未挂 `metrics.Middleware()`。

**Q20. VictoriaLogs 启动 panic？**
`InitVictoriaLogs()` 失败会 panic（URL/token/网络）。先 `curl` 验证地址，或移除该输出目标。

## 任务与调用

**Q21. 任务不执行？**
`go.job.enabled`、数据库可用、`handler_name` 已注册、任务状态为运行中、cron 表达式合法。多副本会重复执行，建议单实例开启。

**Q22. 调用其他服务报「服务注册中心未初始化」？**
配置 `go.discovery.registry` 并在 `used` 中启用对应组件；`NewApp` 未初始化时调用直接失败。

**Q23. 调用超时无法控制？**
`client` 没有内置超时。用 goroutine + `time.After` 兜底，或改用业务 HTTP 客户端并手工带 trace 头。

## 构建与测试

**Q24. `go build` 报版本不匹配 / 找不到 `/v2`？**
`jh` 分支模块是 `github.com/maczh/mgin`；`/v2` 是 `master` 线。不要混用。

**Q25. `go vet ./...` 失败？**
有 3 处历史告警（xss/rsautil 的 unreachable code、i18n 测试的 copylocks），不阻断构建；新增告警必须修复。

**Q26. `go test ./...` 很慢或卡住？**
注册中心用例依赖网络/服务。按包运行，见 [测试策略](../10-contributing/03-testing-strategy.md)。

## 脚手架

**Q27. 生成的项目编译失败？**
`--db postgres|clickhouse` 会生成不存在的 DAO；改用 `mysql`。其他编译错误先看 `go mod tidy`。

**Q28. 生成的工程启动连不上 MySQL？**
默认 `used: "mysql"`。本机没有 MySQL 时把 `used` 改为 `""`。

## 其它

**Q29. 一个进程能跑两个 App 吗？**
不能。配置、数据源、注册中心都是全局单例。

**Q30. 支持 PostgreSQL / ClickHouse / ElasticSearch / Kafka / S3 吗？**
当前分支**不支持**，配置会被忽略。完整边界见 [README §2.4](../../../README.md#24-当前分支未实现的能力)。
