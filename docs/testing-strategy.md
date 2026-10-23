# MGin v2 测试策略

| 项 | 内容 |
|----|------|
| 文档版本 | v2.0 |
| 适用范围 | 框架仓库、`cmd/` 脚手架、生成工程 |
| 当前状态 | 28 个 `_test.go`；**无 CI 配置**，测试靠本地/人工执行 |

---

## 1. 目标与原则

### 1.1 目标

1. 保证**契约稳定**：公开 API 的签名与语义不被无意破坏。
2. 保证**并发正确**：熔断器、负载均衡、注册缓存、日志队列在 `-race` 下无竞争。
3. 保证**降级正确**：依赖不可用时行为可预期（探针 503、熔断快速失败、缓存回源降级）。
4. 保证**脚手架可用**：生成的工程能编译、能启动、探针可访问。
5. 保证**文档不漂移**：关键配置键与状态码有测试兜底。

### 1.2 原则

- **测试与实现同包**（`package client` 而非 `client_test`），便于验证未导出逻辑；
  仅当需要验证外部可用性时才用 `_test` 包。
- **默认无外部依赖**：核心能力用桩/内存实现；集成测试显式标注依赖。
- **失败可复现**：不依赖时间随机性；必须依赖时注入可控时钟或用宽阈值。
- **优先测行为，不测实现**：断言返回值/状态码/计数器，不断言私有字段。

---

## 2. 测试分层

| 层级 | 对象 | 依赖 | 执行频率 | 示例 |
|------|------|------|----------|------|
| L1 单元 | 纯函数、状态机、算法 | 无 | 每次提交 | `breaker_test.go`、`loadbalancer_test.go` |
| L2 组件 | 单包 + 内存桩 | 无 | 每次提交 | `client_test.go`（桩注册中心）、`health_test.go`（httptest） |
| L3 集成 | 包 + 本地中间件 | MySQL/Redis/etcd/... | 每日/发布前 | `registry/*_test.go`、`i18n/mysql_test.go` |
| L4 端到端 | 生成工程 | 本地端口 | 发布前 | `mgin new` → `make build` → 启动 → curl |
| L5 兼容性 | v1 API 调用方 | 编译 | 发布前 | 用旧签名编译示例工程 |

---

## 3. 现有测试清单

| 包 | 文件 | 覆盖点 | 外部依赖 |
|----|------|--------|----------|
| 根 | `logs/alias_test.go` | 别名壳 re-export 完整性 | 无 |
| cache | `cache_test.go` | mem/disk 存取与过期 | 无（磁盘临时目录） |
| config | `configure_test.go`、`logger_test.go` | 配置解析、日志配置 | 可能依赖文件/远程 |
| client | `client_test.go` | 调用流程、协议、桩注册中心 | 无 |
| client | `breaker_test.go` | 三态转换、窗口、半开探测 | 无 |
| client | `resilience_test.go` | 退避、重试、超时、幂等判定 | 无 |
| errcode | `definition_test.go` | 错误码注册/查询/非法状态修正 | 无 |
| health | `health_test.go` | 三探针状态码、MarkStarted、版本 | 无（httptest） |
| i18n | `store_test.go`、`api_test.go` | 字典 CRUD、语言回退、HTTP API | SQLite 临时库 |
| i18n | `mysql_test.go` | MySQL 字典 | **MySQL** |
| job | `job_test.go` | 调度、注册、执行日志 | 纯 Go SQLite |
| loadbalancer | `loadbalancer_test.go` | 4 策略、空实例、注册表 | 无 |
| logs | `output_test.go`、`victorialogs_test.go` | 字段选择、格式化、采集校验 | 无（HTTP 桩） |
| logsink | `sink_test.go`、`record_test.go` | 队列淘汰、FIFO、关闭排空 | 无（**当前 `TestQueueKeepsNewestAndFIFO` 失败**，见 §5） |
| metrics | `metrics_test.go` | 指标注册、路径归一化 | 无 |
| otel | `otel_test.go` | 启停、span 创建 | 无 |
| registry | `cache_test.go` | TTL、刷新、防击穿、降级 | 无（fake client） |
| registry/consul | `consul_test.go` | 注册/发现 | **Consul** |
| registry/etcd | `etcd_test.go` | 注册/发现 | **etcd** |
| registry/polaris | `polaris_test.go` | 注册/发现 | **Polaris** |
| db/dao | `mysql_cache_test.go` | 缓存键、失效、事务内禁用 | Redis 或桩 |
| middleware/postlog | `logger_test.go` | 请求/响应采集 | 无 |
| middleware/ratelimit | `ratelimit_test.go` | 三算法、维度、白名单 | 无 |
| utils | `util_test.go` | 工具函数抽样 | 无 |

---

## 4. 执行命令

### 4.1 分层执行

```bash
# L1+L2：核心，无需中间件（推荐作为提交门禁）
go test -count=1 \
  ./pkg/health/... ./pkg/client/... ./pkg/loadbalancer/... \
  ./pkg/metrics/... ./pkg/otel/... ./pkg/errcode/... \
  ./pkg/logs/... ./pkg/cache/... \
  ./pkg/middleware/postlog/... ./pkg/middleware/ratelimit/... \
  ./logs/...
# 注意：pkg/logsink 因用例与实现不同步，暂不纳入门禁（见 §5）

# L3：集成（需服务）
go test -count=1 ./pkg/registry/... ./pkg/i18n/... ./pkg/db/...
```

### 4.2 全量与竞态

```bash
go test ./... -count=1
go test -race ./pkg/client/... ./pkg/loadbalancer/... ./pkg/logsink/... -count=1
```

### 4.3 覆盖率

```bash
go test ./pkg/health/... ./pkg/client/... ./pkg/loadbalancer/... \
        ./pkg/metrics/... ./pkg/otel/... ./pkg/errcode/... \
        -coverprofile=coverage.out
go tool cover -func=coverage.out | tail -1
go tool cover -html=coverage.out
```

> 仓库当前未提交覆盖率基线。建议在 CI 中记录数值，避免回退；不要为数字而写无断言测试。

### 4.4 脚手架与生成工程

```bash
cd cmd && go build -o /tmp/mgin . && go vet ./...

/tmp/mgin new smoke --module github.com/me/smoke --db sqlite --health --metrics --output /tmp
cd /tmp/smoke
make tidy build
# 启动并验证（BINARY 为工程名，产物在工程根目录）
(./smoke &) ; sleep 2
curl -fsS localhost:8080/health/live
curl -fsS localhost:8080/health/ready
curl -fsS localhost:8080/metrics | grep mgin_build_info
kill %1
```

---

## 5. 已知失败与处理

| 包 | 现象 | 原因 | 处理 |
|----|------|------|------|
| `pkg/logsink` | `TestQueueKeepsNewestAndFIFO` 失败：断言 `QueueSize==100`，实际 `1000→1024` | 代码把队列上限改为 1024（默认 1024），测试未同步 | 修正/删除该断言；修复前不纳入门禁 |
| `pkg/config` | 约 90s 后超时失败 | 测试尝试连接远程配置中心（etcd 等） | 提供本地服务；或给该包加 `t.Skip` 守卫后单独跑 |
| `pkg/registry/consul` | 连接拒绝 | 无 Consul | docker 启动或跳过 |
| `pkg/registry/etcd` | 连接超时 | 无 etcd | 同上 |
| `pkg/registry/polaris` | 连接失败 | 无 Polaris | 同上 |
| `pkg/i18n/mysql_test.go` | 连接失败/跳过 | 无 MySQL | 提供 MySQL 或跳过 |

**实测通过（无外部服务）**：`pkg/i18n`、`pkg/job`、`pkg/db/dao`、`pkg/registry`（根包缓存）、
`pkg/middleware/postlog`、`pkg/middleware/ratelimit`、`pkg/utils`、`logs`。

**约定**：新增集成测试必须在文件头注释写明所需服务与启动方式：
`// 依赖：本地 Redis (127.0.0.1:6379)；缺失时 t.Skip`。

---

## 6. 编写指南

### 6.1 表驱动

```go
func TestCircuitBreakerState(t *testing.T) {
    tests := []struct {
        name       string
        failures   int
        wantState  State
    }{
        {"closed", 0, StateClosed},
        {"open-after-5", 5, StateOpen},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            b := NewCircuitBreaker(tt.name, 5, time.Minute, time.Second)
            for i := 0; i < tt.failures; i++ { b.Failure() }
            if got := b.State(); got != tt.wantState {
                t.Fatalf("state = %v, want %v", got, tt.wantState)
            }
        })
    }
}
```

### 6.2 HTTP 测试

```go
func TestLive(t *testing.T) {
    gin.SetMode(gin.TestMode)
    r := gin.New()
    health.Router(r.Group("/health"))

    w := httptest.NewRecorder()
    r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/health/live", nil))

    if w.Code != http.StatusOK {
        t.Fatalf("code = %d, want 200", w.Code)
    }
}
```

### 6.3 接口桩

避免真实注册中心：实现 `registry.RegistryClient` 的最小桩（参考 `client_test.go` 的 `testRegistry`），
只实现被测路径需要的方法，其余返回零值。

```go
type stubRegistry struct{ urls []string }
func (s *stubRegistry) Register([]byte) {}
func (s *stubRegistry) DeRegister() {}
func (s *stubRegistry) GetServiceURL(string, ...string) (string, string) { return s.urls[0], "" }
func (s *stubRegistry) GetServices(string, ...string) ([]string, error) { return s.urls, nil }
```

### 6.4 并发测试

```go
var wg sync.WaitGroup
for i := 0; i < 100; i++ {
    wg.Add(1)
    go func() { defer wg.Done(); _ = lb.Pick(instances, "k") }()
}
wg.Wait()
```

配合 `-race`。若测试涉及全局注册表（plugin/loadbalancer/breaker），用例结束需清理：

```go
t.Cleanup(func() { loadbalancer.SetDefault(loadbalancer.RoundRobinLB) })
```

### 6.5 时间相关

- 不使用 `time.Sleep` 断言"恰好过期"；用短 TTL + 轮询等待，或注入时钟接口。
- 退避/重试测试断言区间（`>= BaseDelay/2 && < MaxDelay*2`），不断言精确值。

### 6.6 日志与输出

测试中可把日志级别设为 `error` 减少噪音；不要断言日志文本（易脆），
改为断言可观察行为（返回值、计数器、指标）。

---

## 7. 覆盖重点

按风险排序，优先保证：

| 优先级 | 模块 | 必测场景 |
|--------|------|----------|
| P0 | `client` 熔断/重试/超时 | 全实例熔断快速失败、幂等重试、超时返回 `ErrCallTimeout` |
| P0 | `health` | live 恒 200、ready 依赖失败 503、startup 未标记 503 |
| P0 | `app` 生命周期 | 重复 `EnableHealth/EnableMetrics` 不 panic、404/500 状态码 |
| P1 | `loadbalancer` | 空实例、单实例、分布均匀性（统计阈值） |
| P1 | `registry` 缓存 | 命中、过期回源、回源失败降级、并发防击穿 |
| P1 | `errcode`/`i18n` | 未注册码回退 500、缺失文案回退 key、`{}` 替换 |
| P1 | `logsink` | 满队列淘汰最旧、关闭排空、handler panic 隔离 |
| P2 | `job` | cron 解析、阻塞策略、超时/重试、日志清理 |
| P2 | `ratelimit` | 三算法边界、白名单、维度组合 |
| P2 | `dao` 缓存 | 键稳定性、写后失效、事务内不缓存 |

---

## 8. 不建议的测试

- 不为 1:1 转发函数写测试（如 `Call` → `CallCtx`）。
- 不测第三方库行为（GORM/gin 本身）。
- 不做脆弱的字符串/HTML 断言。
- 不依赖执行顺序的测试；每个用例独立 setup/teardown。
- 不用真实公网地址；需要 HTTP 时用 `httptest.Server`。

---

## 9. CI 建议（当前未落地）

仓库没有 CI 配置，建议新增 `.github/workflows/ci.yml` 或等价流水线：

```yaml
name: ci
on: [push, pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.25.7' }
      - run: go build ./...
      # 注意：全仓库 go vet 有 3 处历史 unreachable code 告警，
      # 先只对核心包做门禁；待基线修复后再改为 ./...
      - run: >
          go vet
          ./pkg/health/... ./pkg/client/... ./pkg/loadbalancer/...
          ./pkg/metrics/... ./pkg/otel/... ./pkg/errcode/...
      # 只检查改动文件格式（历史有 14 个未格式化文件）
      - run: test -z "$(gofmt -l $(git diff --name-only -- '*.go'))"
      - run: >
          go test -count=1
          ./pkg/health/... ./pkg/client/... ./pkg/loadbalancer/...
          ./pkg/metrics/... ./pkg/otel/... ./pkg/errcode/...
          ./pkg/logs/... ./pkg/cache/...
          ./pkg/middleware/postlog/... ./pkg/middleware/ratelimit/...
      - run: cd cmd && go build ./... && go vet ./...
```

集成测试单独 job，使用 service containers 启动 MySQL/Redis/etcd，并设置
`-tags=integration` 或环境变量守卫。

### 9.1 发布门禁（建议）

| 门禁 | 命令 | 阻断 | 备注 |
|------|------|------|------|
| 编译 | `go build ./...` | 是 | 当前通过 |
| 静态检查 | 改动包 `go vet` + 改动文件 `gofmt -l` | 是 | 全仓库有 3 处历史 vet 告警、14 个未格式化文件 |
| 核心测试 | L1+L2 命令 | 是 | |
| 竞态 | `go test -race` 关键包 | 是 | |
| 集成测试 | L3 | 是（发布前） | 需中间件 |
| 脚手架冒烟 | `mgin new` + `make build` | 是（发布前） | |
| 覆盖率 | 记录数值，不下降 | 否（观察） | 暂无基线 |

> 建议单独安排一次"基线清理"PR 修复 3 处 `unreachable code` 并统一 `gofmt`，
> 之后再把全仓库 `go vet` / `gofmt` 作为硬门禁。

---

## 10. 缺陷回归规则

1. 每个线上缺陷必须有对应回归测试，命名包含问题语义（`TestXxx_WhenYyy`）。
2. 回归测试先失败后修复（红 → 绿）。
3. 若缺陷无法用自动化测试覆盖（如仅多副本复现），在 PR 中记录手工验证步骤与结论。
