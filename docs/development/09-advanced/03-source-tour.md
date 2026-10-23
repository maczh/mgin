# 源码导读

面向需要修改框架的开发者。建议按「启动 → 请求 → 数据 → 周边」的顺序读。

## 1. 入口与生命周期

| 文件 | 关键内容 |
|------|----------|
| `mgin.go` | `Init`、`initResources`、`checkAll`、`SafeExit`、`MGin.Use/UsePlugin`、`Reload` |
| `app.go` | `App` 结构、`NewApp`、`baseRouter`、`mountHealth`、`Run`、`EnableHealth/EnableMetrics`、`/cfg/reload` |
| `cmd/*.go` | 脚手架（独立 module） |

阅读顺序：

1. `NewApp`（app.go）：参数解析 → `Init` → i18n → metrics 标记 → `baseRouter`。
2. `Init`（mgin.go）：配置 → VictoriaLogs → `initResources`。
3. `initResources`：注册中心 → 数据源 → 插件 → job。
4. `baseRouter`：确认中间件顺序与探针挂载。
5. `Run`：端口/TLS、信号、5s 关闭、`SafeExit`。

## 2. 请求链路

| 文件 | 作用 |
|------|------|
| `middleware/trace/trace.go` | 生成/透传 ID，goroutine 级上下文 |
| `middleware/postlog/logger.go` | 请求/响应采集，交给 `logsink` |
| `middleware/cors/cors.go` | 预检与响应头 |
| `middleware/ratelimit/*` | 规则匹配、三种算法、维度 |
| `app.go` 的 `NoRoute`/`recoveryHandler` | 404 与 panic 统一响应 |

调试入口：在 `postlog` 打日志确认链是否按预期执行；注意探针路由不经过业务中间件。

## 3. 数据层

| 文件 | 作用 |
|------|------|
| `db/db.go` | 门面变量 |
| `db/mysql/mysql.go` | 单库/多库、池、重连 |
| `db/dao/mysql.go` | 泛型 CRUD、`Where*`、缓存 |
| `db/dao/mysql_cache.go` | 缓存键与失效 |
| `db/dao/mysql_transaction.go` | 事务绑定 |
| `db/redis/redis.go` | 多库、订阅、池 |
| `db/mongo/mongo.go` | 多库、会话归还 |
| `db/sqlite/sqlite.go` | 纯 Go SQLite |

常见修改点：

- 新增 DAO 方法：同时考虑缓存与事务分支（`daoCacheKey`/`deleteCacheKey`）。
- 修改 `WithContext`：注意当前实现会修改接收者，改动前评估并发影响。

## 4. 配置与远程

| 文件 | 作用 |
|------|------|
| `config/configure.go` | 结构体、`Init`、`GetConfig*`、`GetConfigUrl`、`Reload` 锁 |
| `config/logger.go` | 日志目标解析与 `WorkDir` |

新增配置键的顺序：结构体字段 → `Init` 读取（含默认值）→ 示例 YAML → 文档。

## 5. 日志与管道

| 文件 | 作用 |
|------|------|
| `logs/log.go` | 门面、`{}` 格式化、级别 |
| `logs/output.go` | console/file 输出、按天切分 |
| `logs/victorialogs.go` | VL 上报 |
| `logsink/sink.go` | 队列、单协程、注册/统计 |
| `logsink/record.go` | 运行日志 Record 与 handler |

修改风险：`Write` 中回调日志会导致递归；关闭顺序错误会丢日志。

## 6. 注册发现与调用

| 文件 | 作用 |
|------|------|
| `registry/registry.go` | 工厂与 noop |
| `registry/cache.go` | 缓存装饰器、singleflight、热刷新 |
| `registry/{nacos,consul,etcd,polaris}` | 各实现 |
| `client/client.go` | 四种协议、重试、trace 头 |

注意：Polaris 未接入自动注册/注销；`client` 无超时。

## 7. 可观测与任务

| 文件 | 作用 |
|------|------|
| `health/health.go` | 探针、依赖检查、MarkStarted |
| `metrics/*.go` | 注册指标、中间件、`/metrics` |
| `job/scheduler.go` | 调度循环、阻塞策略 |
| `job/handler.go` | 执行器注册与 `Context` |
| `job/api.go` | `/job` REST 接口 |
| `job/model.go` | 表模型 |

## 8. 修改框架的最小验证集

```bash
go build ./...
(cd cmd && go build ./...)
go test ./health/ ./metrics/ ./logs/ ./logsink/ ./utils/ ./middleware/cors/ ./middleware/limit/ ./middleware/ratelimit/
gofmt -l $(git ls-files '*.go' | grep -v '^.Trash-0/')
```

启动冒烟（配置仅开探针与指标）：

```bash
./app -f ./conf/application.yml
curl -s localhost:8080/health/live
curl -s localhost:8080/metrics | head
```

## 9. 不要做的事

- 不要新增全局可变状态来传请求上下文（用 context/gin.Context）。
- 不要在 `init()` 里读配置或连库。
- 不要在框架内部 `panic` 作为控制流。
- 不要改变默认中间件顺序而不更新文档与测试。
- 不要删除错误码/配置键；走废弃流程。
