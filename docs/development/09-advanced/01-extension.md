# 扩展机制

框架提供四类扩展点：外部组件、中间件、日志落地、任务执行器。所有扩展都必须遵守「开关关闭时无副作用」的原则。

## 1. 外部组件（`mgin.MGin.Use`）

```go
mgin.MGin.Use("sts",
	func(cnf []byte) { sts.Init(cnf) },   // init
	func()            { sts.Close() },    // close
	func() error      { return sts.Check() },
)
```

行为：

- 仅当 `used` 包含 `sts` 时调用 `init`；
- `close` 在 `SafeExit`/重载时调用；
- `check` 被 `checkAll`（每 5 分钟）与重载流程使用。

`UsePlugin` 是对象化版本：

```go
type MginPlugin interface {
	Init(config []byte)
	Close()
	Check() error
}
mgin.MGin.UsePlugin("sts", stsPlugin{})
```

### 新增内置组件的完整清单

1. 新建包（如 `db/postgres`），实现 `Init([]byte)/Close()/Check() error`。
2. 在 `db/db.go` 暴露门面（可选）。
3. 在 `mgin.go` 登记四处：`initResources`、`closeResources`、`checkAll`、`SafeExit`。
4. 在 `config` 的 `Prefix` 结构体与 `Init` 中登记前缀键。
5. 更新 `README §2/§5`、`08-reference/variables.md`、`docs/development/04-configuration/`。
6. 加单元测试与「未启用时零开销」的验证。

> 只做 1–3 而漏掉 4，会出现「`used` 写了但不加载」；漏掉 `SafeExit` 会出现连接泄漏。

## 2. 中间件

```go
func Tenant() gin.HandlerFunc {
	return func(c *gin.Context) {
		if v := c.GetHeader("X-Tenant-Id"); v != "" {
			c.Set("tenant_id", v)
		}
		c.Next()
	}
}

app.Router.Use(Tenant())
```

规范：

- 放在 `middleware/<name>/`，导出构造函数；
- 失败时明确 HTTP 状态与响应体；
- 依赖其他中间件时在注释中写明（如 casbin 依赖 jwt 的 `claims`）；
- 提供 `httptest` 单元测试；
- 在 [中间件表](../08-reference/api/README.md#7-中间件middleware) 登记。

## 3. 日志落地

### 3.1 运行日志 handler

```go
type myHandler struct{ ch chan *logsink.Record }

func (h *myHandler) Name() string { return "mylog" }
func (h *myHandler) Write(r *logsink.Record) error {
	// 不要在 Write 里再写日志，避免递归
	return sendToBackend(r)
}

logsink.RegisterRecord(&myHandler{}, logsink.WithQueueSize(256))
```

注册后可在配置中使用：

```yaml
go:
  logger:
    outputs:
      - target: mylog
        format: json
```

### 3.2 接口日志 handler

```go
type postHandler struct{}

func (postHandler) Name() string                { return "post-mine" }
func (postHandler) Write(e *logsink.Entry) error { return store(e) }

logsink.Register(postHandler{}, logsink.WithQueueSize(512))
```

### 3.3 约束

- 每个 handler 只有 1 个消费协程：`Write` 必须足够快或自行异步。
- 队列有界，满时丢最旧；用 `logsink.Stats()` 监控 `Dropped`。
- `Write` 必须并发安全，且不能回调 `logs.*`。

## 4. 任务执行器

```go
job.Register("syncUserJob", func(ctx *job.Context) error {
	ctx.Log("开始处理，参数=%v", ctx.Param)
	if err := doSync(ctx.Param); err != nil {
		ctx.Log("失败: %v", err)
		return err      // 触发重试
	}
	return nil
})
```

- 注册名必须与数据库 `handler_name` 一致；
- 返回 error 触发重试（`retry_count`/`retry_interval`）；
- 执行器应幂等：单机调度也可能因重启重复执行；
- 长任务用 `ctx` 的上下文处理取消（框架支持超时）。

## 5. 响应与错误码扩展

```go
const (
	BIZ_ORDER_LOCKED = 2001
)

// messageId 走 i18n 字典
c.JSON(200, i18n.Error(BIZ_ORDER_LOCKED, "order_locked"))
```

- 业务码从 2000 起，避免与框架 1000–1014 冲突；
- messageId 必须是字典中存在的键，否则原样返回；
- 不要修改 `errcode` 既有常量的值。

## 6. 指标扩展

```go
var orderCounter = prometheus.NewCounterVec(
	prometheus.CounterOpts{Name: "biz_orders_total", Help: "订单数"},
	[]string{"status"},
)

func init() { prometheus.MustRegister(orderCounter) }
```

- 指标名加业务前缀，避免与 `mgin_*` 冲突；
- 标签基数必须可控（不要用用户 ID、原始 URL）；
- 注册需在 `EnableMetrics` 之前或之后均可（Prometheus 默认注册器）。

## 7. 扩展检查清单

- [ ] 未启用时不建立连接、不启动协程
- [ ] `Close` 幂等、无 panic
- [ ] `Check` 有超时，失败不阻塞主流程
- [ ] 配置键已登记（结构体 + 示例 + 文档）
- [ ] 有测试；外部依赖可 stub
- [ ] 文档登记（README 能力表 + 参考手册）
