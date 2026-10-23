# 测试

业务侧测试的重点是：controller/service 的逻辑、DAO 的查询条件、协议的序列化，以及关键错误路径。框架的测试策略与实测基线见 [测试策略](../10-contributing/03-testing-strategy.md)。

## 1. 单元测试（不连数据库）

```go
func TestParseID(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want int64
		ok   bool
	}{
		{"正常", "42", 42, true},
		{"空", "", 0, false},
		{"非数字", "x", 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseID(tc.in)
			if (err == nil) != tc.ok {
				t.Fatalf("ok=%v, err=%v", tc.ok, err)
			}
			if err == nil && got != tc.want {
				t.Fatalf("got %d, want %d", got, tc.want)
			}
		})
	}
}
```

## 2. HTTP / 中间件测试

用 `httptest` 直接驱动路由，不启动真实端口：

```go
func TestGetUser_ParamMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/users/:id", controller.GetUser)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/users/abc", nil)
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("HTTP %d", w.Code)
	}
	var body models.Result[any]
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Status != errcode.PARAM_ERROR {
		t.Fatalf("status=%d", body.Status)
	}
}
```

注意：框架默认返回 HTTP 200，断言应看 `body.Status`。

## 3. DAO 与数据库测试

- 优先使用 SQLite 内存库验证 SQL 形态（`file::memory:?cache=shared`），不要依赖真实 MySQL。
- 需要 MySQL 特性（连接池、多库、事务隔离）时，用测试专用库与专属前缀，结束后清理。
- 测试 DAO 缓存时，使用带 TTL 的 `CacheConfig`，避免 0（永久）导致用例互相污染。

```go
func TestUserDao_WhereAll(t *testing.T) {
	conn, err := db.Sqlite.GetConnection()
	if err != nil {
		t.Skip("sqlite not initialized:", err)
	}
	if err := conn.AutoMigrate(&model.User{}); err != nil {
		t.Fatal(err)
	}
	conn.Create(&model.User{Name: "alice", Status: 1})

	d := &dao.MySQLDao[model.User]{}
	got, err := d.WhereAll("status = ?", 1)
	if err != nil || len(got) != 1 {
		t.Fatalf("got=%v err=%v", got, err)
	}
}
```

## 4. 外部依赖与跳过

```go
if _, err := db.Mysql.GetConnection(); err != nil {
	t.Skip("MySQL 不可用，跳过集成用例")
}
```

- 网络类用例（注册中心）必须能在离线时快速失败或跳过。
- 不要依赖执行顺序；涉及全局单例的测试串行执行（`-p 1`）或先重置状态。

## 5. 命令

```bash
# 业务工程
go test ./...
go test -race ./internal/...
go test -run TestGetUser -v ./controller/

# 框架自身的可独立运行包
go test ./health/ ./metrics/ ./logs/ ./logsink/ ./utils/ ./middleware/cors/ ./middleware/limit/
```

## 6. 覆盖率

```bash
go test -coverprofile=cover.out ./...
go tool cover -html=cover.out
```

建议目标：service/controller ≥ 70%，DAO 关键查询全覆盖，错误分支（参数缺失、记录不存在、DB 错误）必须有断言。

## 7. 发布前自测清单

- [ ] 参数缺失/类型错误返回正确的 `status`
- [ ] 记录不存在返回 `DATA_NOT_FOUND` 而不是 nil panic
- [ ] 事务失败可回滚
- [ ] 鉴权失败返回 401/403，且不影响探针
- [ ] 限流配置生效（默认 200 + 1011）
- [ ] `go vet` 无新增告警
