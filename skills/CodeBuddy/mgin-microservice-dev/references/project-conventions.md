# jihai 业务项目实际惯例

参考源：`/home/macro/Work/go/src/jihaihotpot.com/jihai/`

## 1. 项目清单与分类

| 项目 | 框架 | 备注 |
|---|---|---|
| jh-ris-order | mgin v1 `v1.25.22-jh` | 最完整，可作 v1 范本 |
| jh-ris-booking | **mgin v2** `v2.0.7` | **脚手架生成**，可作 v2 范本 |
| jh-ris-common | mgin v1 | 公共库（model/vo），被各项目 `replace => ../jh-ris-common` |
| jh-ris-dish / jh-ris-dg / jh-ris-print / jh-ris-setting | mgin v1 | — |
| store-order / store-table / store-sync | mgin v1 | store-sync 无 router/controller，纯后台同步服务 |
| jh-pay / jh-pay-chinaums | mgin v1 | module 域名是老 `jihai.cn` |
| jh-async-mail | mgin v1 | RabbitMQ；module 与 import 路径不一致，当前无法编译 |
| **jh-food-order / sea-admin** | ❌ 不用 MGin | gin-vue-admin 骨架，别拿它们当参考 |

## 2. jh-ris-order main.go（v1 完整范本）

```go
func main() {
	app := mgin.NewApp("", "寄海海鲜火锅RIS订单模块", Version, false)
	if app == nil {                 // 历史写法：NewApp 现已改为 -v 时 os.Exit(0)，不会再返回 nil
		fmt.Printf("Build time: %s\n", BuildTime)
		return
	}
	mysql.MySQLInit()                                        // AutoMigrate + 缓存

	if strings.Contains(config.Config.Config.Used, "mqtt") { // ★ 按配置条件注册
		app.MGin.UsePlugin("mqtt", mqtt.MQTT)
	}
	if config.Config.GetConfigString("go.push.protocol") == "kafka" {
		app.MGin.UsePlugin("kafka", mgkafka.Kafka)
	}
	app.MGin.UsePlugin("nats", nats.NATS)

	mq.Register()                       // 集中注册 MQ 订阅
	app.Router.Use(mw.XTokenAuth())     // 业务鉴权中间件
	router.Register(app.Router)         // v1 传 *gin.Engine
	app.Router.GET("/api/jhris/order/v1/docs/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	app.Run()
}
```

## 3. jh-ris-booking main.go（v2 范本，脚手架风格）

```go
func main() {
	app := mgin.NewApp("conf/application.yml", "寄海海鲜火锅预订服务模块", Version, false)
	if app == nil {
		return
	}
	app.EnableHealth()      // jh 分支同样支持；也可改用 go.health.enabled / go.metrics.enabled
	app.EnableMetrics()

	app.MGin.UsePlugin("mqtt", mqttplugin.MQTT)
	app.MGin.UsePlugin("kafka", kafkaplugin.Kafka)

	mysql.MySQLInit()
	bookingservice.InitBookingSettings()
	go bookingservice.InitStoreTableTemplates()   // 依赖下游的初始化放 goroutine

	mqttsub.InitMqttSubscribers()
	kafka.InitKafkaSubscribers()

	router.RegisterRoutes(app)
	health.MarkStarted()                          // K8s startup 探针切换
	app.Router.GET("/api/booking/v1/docs/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	schedule.Default.StartCron()
	app.Run()
}
```

## 4. 统一响应：业务项目自己 copy `vo.Response`

**这是最重要的一条差异**：框架提供 `models.Result[T]{Status,...}`，但 jihai 全系用 `vo.Response[T]{Code, Data, Msg, SrvTime}`，且**每个项目复制一份**（jh-ris-common 里有一份，jh-ris-order/model/vo 里也有一份，内容逐字相同）。

- 成功判断：`if result.Code != 1`（等价于 `!= vo.SUCCESS`）
- 跨服务调用失败统一用 `-1`：`vo.ErrorT[T](-1, err.Error())`
- **几乎不定义业务错误码常量**，只有 `ERROR=500` / `SUCCESS=1` / `UnLogin=999`，其余直接写字面量 `400`/`500`
- 业务状态常量（订单/支付/预订状态机）另放 `consts/` 或 `constants/`

完整代码见 `code-templates.md` §1。

## 5. 目录命名实测对照

| 目录 | order | booking | 说明 |
|---|---|---|---|
| DAO | `mysql/` | `mysql/` | 都叫 mysql，不叫 dao |
| 常量 | `constants/` | `consts/` | 不统一 |
| MQ | `mq/` | `kafka/` + `mqttsub/` | 不统一 |
| 配置 | 根目录 `<项目名>.yml` | `conf/application.yml` | booking 更规范 |
| 路由 | `router/` (传 engine) | `router/` (传 app) | — |
| 定时任务 | `go.job.*`（框架 job 模块） | `schedule/`（自管 cron） | booking 更常用自管 cron |

## 6. 鉴权

**全部自定义 `mw/` 中间件 + JWT，没有项目用 mgin 的 casbin**（casbin 只出现在 gin-vue-admin 系的 jh-food-order / sea-admin 里）。

- header 名：**`x-token`**（小写）
- 按 URL 前缀分流：`/admin/` 放行、`/app/` 本地 JWT 校验、`/mp/` 调会员服务换会员
- JWT 库 `github.com/dgrijalva/jwt-go`，校验用 `utils.ValidateToken(token)`（密钥 = `go.jwt.secret`）

## 7. MQ 使用场景

| MQ | 谁在用 | 用途 |
|---|---|---|
| NATS | jh-ris-order、jh-ris-dg | 服务内部异步解耦（订单摘要生成、缓存更新） |
| Kafka | jh-ris-booking、jh-ris-order、store-sync | 门店端↔云端、占桌结果回调 |
| MQTT | jh-ris-order、booking、jh-ris-dish、store-* | 推给前端/门店端实时消息 |
| RabbitMQ | jh-async-mail | 异步邮件队列 |

`jh-ris-order` 有个按配置双推的模式值得抄：
```go
func PublishMsg(tag, topic string, msg string) error {
	err := mqtt.MQTT.Publish(tag, topic, 0, false, []byte(msg))   // 前端
	if config.Config.GetConfigString("go.push.protocol") == "kafka" {
		return mgkafka.Kafka.Send(strings.ReplaceAll(topic, "/", "."), msg)  // 门店端
	}
	return err
}
```

## 8. 定时任务三种做法并存

1. **自管 `robfig/cron/v3`**（booking、jh-pay）——最常用，`schedule/` 目录，cron 表达式可从配置读
2. **框架 job 模块**（jh-ris-order）——DB 持久化 + `/job` 管理接口，需要 `go.job.enabled: true`
3. `go.schedule.cron` 配置 —— 简单场景

## 9. Swagger

- 工具 `github.com/swaggo/swag v1.16.6`，产物 `docs/{docs.go,swagger.json,swagger.yaml}`
- UI 挂载路径规律：`/api/<服务>/v1/docs/*any`
- 老项目用大写注解（`@Title` / `@Version`），新项目用标准小写（`@title` / `@version`）——**用小写**
- `@Router` 写完整路径（含 `/api/...` 前缀）
- `@Tags` 按端分：`订单管理` / `后台-时段管理` / `小程序订单管理`
- 泛型被渲染成 `vo.Response-any`
- jh-ris-order 的 docs/ 里还混了手写 md 文档

## 10. 值得抄的工程细节

- **避免循环依赖**：`kafka` 包用 `init()` 把下发函数注入 service 的函数变量（`bookingservice.SendOccupyFn = SendOccupy`），而不是让 service import kafka
- **幂等初始化**：启动时不覆盖已存在的默认设置
- **重复订阅防护**：用 `sync.Map` 记录已订阅 topic
- **并发安全**：service 里的 map 缓存配 `sync.RWMutex`（order 服务踩过 fatal error）
- **中文文件名下载**：`Content-Disposition: attachment; filename*=UTF-8''<url.QueryEscape(name)>`
- **MQ 解析失败返回 nil** 而不是 error，避免死信循环
