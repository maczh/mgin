# 测试

目标：业务逻辑可脱离 HTTP 与真实中间件测试；框架行为有回归保障。
框架自身的测试策略见 [测试策略](../../testing-strategy.md)。

---

## 1. 分层

| 层 | 测什么 | 依赖 | 工具 |
|----|--------|------|------|
| 单元 | service 业务规则、校验、金额计算 | 无 | `testing` + 表驱动 |
| 组件 | handler 状态码/响应体、中间件 | 无 | `httptest` + `gin.TestMode` |
| 数据访问 | SQL、事务、缓存键 | SQLite/MySQL | `dao.WithTx(测试库)` |
| 契约 | 与下游的字段/错误码约定 | 桩 HTTP server | `httptest.Server` |
| 集成 | 注册发现、真实调用 | 注册中心 | docker compose |

原则：**能用接口桩就不用真实中间件**；必须用真实中间件时加 `t.Skip` 守卫。

---

## 2. handler 测试

```go
package controller

import (
    "net/http"
    "net/http/httptest"
    "strings"
    "testing"

    "github.com/gin-gonic/gin"
)

func TestGetOrder_InvalidID(t *testing.T) {
    gin.SetMode(gin.TestMode)
    r := gin.New()
    r.GET("/orders/:id", GetOrder)

    w := httptest.NewRecorder()
    req := httptest.NewRequest(http.MethodGet, "/orders/abc", nil)
    r.ServeHTTP(w, req)

    if w.Code != http.StatusBadRequest {
        t.Fatalf("code = %d, want 400", w.Code)
    }
    if !strings.Contains(w.Body.String(), `"status":1003`) {
        t.Fatalf("unexpected body: %s", w.Body.String())
    }
}
```

要点：

- `gin.SetMode(gin.TestMode)` 避免调试日志刷屏；
- 断言**状态码 + 关键字段**，不要整段字符串比对；
- 每个用例独立建 `gin.New()`，避免路由叠加。

---

## 3. service 测试

把外部依赖抽成接口，测试时用桩：

```go
type OrderRepo interface {
    Create(ctx context.Context, o *model.Order) error
}

type OrderService struct{ repo OrderRepo }

func TestCreateOrder_Duplicate(t *testing.T) {
    svc := &OrderService{repo: stubRepo{err: ErrDuplicate}}
    _, err := svc.Create(context.Background(), CreateOrderReq{OrderNo: "A1", Amount: 1})
    if !errors.Is(err, ErrDuplicate) {
        t.Fatalf("err = %v, want duplicate", err)
    }
}
```

表驱动：

```go
tests := []struct {
    name    string
    req     CreateOrderReq
    wantErr bool
}{
    {"amount zero", CreateOrderReq{OrderNo: "A", Amount: 0}, true},
    {"ok", CreateOrderReq{OrderNo: "B", Amount: 100}, false},
}
for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) { /* ... */ })
}
```

---

## 4. DAO 测试（SQLite 内存库）

`MySQLDao` 支持 `WithTx` 注入任意 `*gorm.DB`，因此可以用 SQLite 做快速测试：

```go
package dao

import (
    "testing"

    "gorm.io/driver/sqlite"
    "gorm.io/gorm"
    "example.com/hello/model"
)

func newTestDAO(t *testing.T) *dao.MySQLDao[model.Order] {
    t.Helper()
    conn, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
    if err != nil {
        t.Fatalf("open sqlite: %v", err)
    }
    if err := conn.AutoMigrate(&model.Order{}); err != nil {
        t.Fatalf("migrate: %v", err)
    }
    return OrderDao.WithTx(conn)
}

func TestCreateAndGet(t *testing.T) {
    d := newTestDAO(t)
    o := &model.Order{OrderNo: "T1", UserId: 1, Amount: 100}
    if err := d.Create(o); err != nil {
        t.Fatalf("create: %v", err)
    }
    got, err := d.One(model.Order{Id: o.Id})
    if err != nil || got.OrderNo != "T1" {
        t.Fatalf("got=%+v err=%v", got, err)
    }
}
```

注意：

- `gorm.io/driver/sqlite` 依赖 CGO；CI 需 `CGO_ENABLED=1` 与 gcc；
- 内存库用 `file::memory:?cache=shared` 才能在多连接间共享；
- 生产方言差异（`ON DUPLICATE KEY`、`FOR UPDATE`）不能用 SQLite 覆盖，需 MySQL 集成测试；
- 不要给测试 DAO 开 `WithCache`，除非专门测缓存。

---

## 5. 调用下游：契约测试

用 `httptest.Server` 模拟下游，验证请求方法、路径、头与解析逻辑：

```go
func TestClientContract(t *testing.T) {
    var gotPath, gotTrace string
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        gotPath = r.URL.Path
        gotTrace = r.Header.Get("X-Trace-Id")
        w.Header().Set("Content-Type", "application/json")
        io.WriteString(w, `{"code":1,"msg":"ok","data":{"id":7}}`)
    }))
    defer srv.Close()

    // 仅测试解析：直接构造 vo 响应
    var resp vo.Response[struct{ Id int `json:"id"` }]
    if err := json.Unmarshal([]byte(`{"code":1,"data":{"id":7}}`), &resp); err != nil {
        t.Fatal(err)
    }
    if resp.Data.Id != 7 {
        t.Fatalf("id = %d", resp.Data.Id)
    }
    _ = gotPath
    _ = gotTrace
}
```

> `client.Call*` 依赖全局 `registry.Registry`，单元测试中不便替换。
> 若要测完整调用链，可在测试中实现一个 `registry.RegistryClient` 桩并赋值给
> `registry.Registry`（测试结束恢复），或改用真实注册中心做集成测试。

---

## 6. 中间件与安全测试

```go
r := gin.New()
r.Use(jwt.JwtAuthorize())
r.GET("/private", func(c *gin.Context) { c.JSON(200, gin.H{"ok": true}) })

w := httptest.NewRecorder()
r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/private", nil))
if w.Code != http.StatusUnauthorized {
    t.Fatalf("code = %d, want 401", w.Code)
}
```

必测场景：

| 中间件 | 用例 |
|--------|------|
| JWT | 无头、格式错、过期、签名错 |
| Casbin | 无策略、路径通配命中、方法不匹配 |
| 限流 | 阈值内/外、白名单、不同维度 |
| XSS | script 标签、事件属性、正常文本不被破坏 |
| trace | 无头生成、有头透传、响应头回写 |

---

## 7. 并发与竞态

```bash
go test -race ./... -run TestConcurrent -count=1
```

编写并发测试：

```go
var wg sync.WaitGroup
for i := 0; i < 50; i++ {
    wg.Add(1)
    go func() { defer wg.Done(); _ = svc.Do(ctx) }()
}
wg.Wait()
```

全局单例（`registry.Registry`、`loadbalancer` 注册表、`client` 熔断器）在测试间会串味，
用例结束需恢复：

```go
t.Cleanup(func() {
    registry.Registry = old
    client.ResetBreakers()
    loadbalancer.SetDefault(loadbalancer.RoundRobinLB)
})
```

---

## 8. 集成测试（可选）

前置条件写进测试文件头：

```go
// 依赖：本地 MySQL 127.0.0.1:3306，库 demo。
// 缺失时 SKIP: go test -short ./...
func TestOrderRepo_MySQL(t *testing.T) {
    if testing.Short() {
        t.Skip("requires mysql")
    }
    // ...
}
```

运行：

```bash
go test ./... -short -count=1          # 跳过集成
go test ./... -count=1                 # 全量
```

---

## 9. 测试检查清单

- [ ] 每个导出的业务函数至少有 1 个成功 + 1 个失败用例
- [ ] handler 断言 HTTP 状态码与业务码，而非仅 body
- [ ] 测试不依赖执行顺序，不写死时间/随机数
- [ ] 外部依赖可跳过，且跳过原因写在文件头
- [ ] 涉及全局单例的测试有 `t.Cleanup` 恢复
- [ ] 并发代码跑过 `-race`
- [ ] 修复缺陷时先补失败用例
