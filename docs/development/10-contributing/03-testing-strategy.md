# 测试策略（jh 分支）

目标：在不依赖外部服务的前提下覆盖核心逻辑；对外部依赖用可控的集成测试或 stub 验证；把每次发布前的验证固定成清单。

---

## 1. 测试分层

| 层 | 范围 | 依赖 | 运行频率 |
|----|------|------|----------|
| 单元测试 | `utils`、`models`、`logs` 格式化、`health`/`metrics` 纯逻辑、中间件分支 | 无 | 每次提交 |
| 组件测试 | `logsink` 队列、`registry` 缓存、`ratelimit` 算法、`cors`、`limit`、`i18n` 存储（SQLite） | SQLite（文件/内存） | 每次提交 |
| 集成测试 | `db/mysql`、`db/mongo`、`db/redis`、`i18n`（MySQL）、`job`（MySQL/SQLite） | 本地或 CI 容器 | 合并前 / 每日 |
| 端到端 | 启动示例服务，验证 `/health/*`、`/metrics`、业务接口、优雅退出 | 完整进程 + 可选中间件 | 发布前 |

---

## 2. 现有用例清单

`find . -name '*_test.go'`（26 个文件）：

| 包 | 测试文件 | 覆盖点 |
|----|----------|--------|
| `client` | `client_test.go` | Options 归一化、协议选择、URL/路径替换 |
| `config` | `configure_test.go`、`logger_test.go` | 配置读取、默认值、日志目标解析 |
| `db` | `transaction_test.go` | 事务提交/回滚、nil 参数 |
| `db/dao` | `mysql_cache_test.go`、`mysql_transaction_test.go` | 缓存键、缓存失效、事务绑定 |
| `health` | `health_test.go` | 探针路径、状态码、依赖检查 |
| `metrics` | `metrics_test.go` | 指标注册、`<nomatch>` 标签 |
| `i18n` | `api_test.go`、`mysql_test.go`、`store_test.go` | CRUD、存储切换、语言回退 |
| `job` | `job_test.go` | 调度类型、阻塞策略、日志 |
| `logs` | `output_test.go`、`victorialogs_test.go` | 字段格式化、text/json、上报 |
| `logsink` | `record_test.go`、`sink_test.go` | FIFO、队列淘汰、关闭排空、并发投递 |
| `middleware/cors` | `cors_test.go` | 允许头/方法、预检 |
| `middleware/limit` | `limit_test.go` | 并发信号量、客户端断开 |
| `middleware/postlog` | `logger_test.go` | 请求/响应采集、GET 策略 |
| `middleware/ratelimit` | `ratelimit_test.go` | 三种算法、维度、白名单 |
| `registry` | `cache_test.go` | TTL、singleflight、降级 |
| `registry/*` | `consul_test.go`、`etcd_test.go`、`polaris_test.go` | 注册/发现（需要服务或 HTTP stub） |
| `utils` | `util_test.go` | 加密、编码、字符串、切片等 |

未覆盖（需要补充）：`cache`（无测试）、`middleware/casbin`、`middleware/jwt`、`middleware/xss`、`middleware/iplimit`、`middleware/session`、`db/mysql`（连接层）、`db/mongo`、`db/redis`、`casbin`、`logsink` 与 VictoriaLogs 的联调。

---

## 3. 运行方式

```bash
# 只跑无需外部依赖的包（推荐提交前）
go test ./health/ ./metrics/ ./logs/ ./logsink/ ./utils/ \
        ./middleware/cors/ ./middleware/limit/ ./middleware/ratelimit/

# 需要 SQLite（无需服务）的包
go test ./i18n/ ./job/

# 需要本地 MySQL/Redis/Mongo 的包
go test ./db/... ./config/

# 注册中心（需要可达服务或网络）
go test ./registry/...

# 覆盖率
go test -coverprofile=cover.out ./health/ ./metrics/ ./logs/ ./logsink/ ./utils/
go tool cover -html=cover.out
```

约定：

- 不要依赖执行顺序；需要全局单例的测试先 `Reset()`/重置配置。
- 涉及网络的测试要能在离线时**快速失败或跳过**，不要无限重试。
- `go test ./...` 目前会拉长执行时间（注册中心用例），本地建议按包运行。

---

## 4. 实测基线（当前代码）

| 包 | 结果 | 备注 |
|----|------|------|
| `health` | ok | — |
| `metrics` | ok | — |
| `logs` | ok | — |
| `logsink` | ok | 修复了 `TestQueueKeepsNewestAndFIFO` 与实现参数不一致的问题 |
| `utils` | ok | — |
| `middleware/cors`、`middleware/limit` | ok | — |
| `client` | ok | 不访问真实注册中心 |
| `config`、`db/...`、`job`、`i18n` | 未在本机完整执行 | 需要 MySQL / 部分需要 SQLite 与本地服务 |
| `registry/...` | 未完整执行 | 依赖注册中心/网络，耗时较长 |

`go vet ./...` 现状：存在 3 处历史告警（`middleware/xss`、`utils/rsautil.go` 的 unreachable code；`i18n/store_test.go` 的 copylocks）。它们不影响构建，但建议逐步清理。

> 结论：**不能**把 `go test ./...` 作为唯一门禁；发布前按 §6 清单执行。

---

## 5. 编写测试的约定

### 5.1 命名与结构

```go
func TestXxx_场景_期望(t *testing.T) {
    cases := []struct{ name string; in ...; want ... }{...}
    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) { ... })
    }
}
```

- 优先表驱动；断言失败信息包含输入与期望。
- 一个测试只验证一个行为；避免共享可变全局状态。
- 使用 `t.Cleanup` 释放临时文件/连接/注册的 handler。

### 5.2 HTTP 与中间件

```go
r := gin.New()
r.Use(cors.Cors())
r.GET("/ping", func(c *gin.Context) { c.String(200, "ok") })

w := httptest.NewRecorder()
req := httptest.NewRequest(http.MethodOptions, "/ping", nil)
req.Header.Set("Origin", "https://example.com")
r.ServeHTTP(w, req)
// 断言 w.Code / w.Header()
```

### 5.3 外部依赖

- MySQL/Redis/Mongo：优先用 Docker 起真实实例；用独立库名/前缀，测试结束清理。
- 注册中心：用 `httptest.Server` 模拟 HTTP API（Nacos/Polaris），或对 `CachedRegistry` 注入假实现。
- 时间相关：注入可替换的 `now` 函数或使用足够大的容差，避免 flaky。

---

## 6. 发布前验证清单

- [ ] `go build ./...` 通过。
- [ ] `cd cmd && go build ./...` 通过。
- [ ] `gofmt -l $(git ls-files '*.go' | grep -v '^.Trash-0/')` 无输出。
- [ ] 单元/组件测试通过（§3 第一、二组）。
- [ ] 目标环境跑通集成测试（`db/...`、`i18n`、`job`）。
- [ ] 手工回归：最小服务 `/ping`、`/health/live`、`/health/ready`、`/health/startup`、`/metrics`。
- [ ] 手工回归：`SIGTERM` 后日志出现排空与退出，无 panic。
- [ ] 变更涉及配置/表结构时，升级一份旧配置/旧库验证兼容。

---

## 7. CI 建议（当前仓库无 CI）

建议流水线阶段：

```yaml
stages:
  - lint:   gofmt 检查 + go vet（允许已知告警白名单）
  - build:  go build ./... && (cd cmd && go build ./...)
  - unit:   go test -race ./health/ ./metrics/ ./logs/ ./logsink/ ./utils/ ./middleware/...
  - integration:
      services: [mysql:8, redis:7, mongo:6]
      script: go test ./db/... ./i18n/ ./job/
  - e2e:    启动示例服务 + curl 探针
```

要求：

- 集成阶段使用独立数据库与前缀，结束后清理。
- `-race` 至少覆盖 `logsink`、`cache`、`utils` 等并发密集包。
- 已知 vet 告警用白名单显式声明，避免「全绿」假象。

---

## 8. 覆盖率目标

| 范围 | 目标 |
|------|------|
| `utils`、`models`、`errcode` | ≥ 70% |
| `health`、`metrics`、`logsink` | ≥ 60% |
| `middleware/*` | ≥ 50% |
| `db/*`、`registry/*` | 关键路径覆盖（连接失败、重连、缓存降级） |

覆盖率是参考指标，不替代场景覆盖：连接失败、超时、空输入、并发与关闭流程优先。
