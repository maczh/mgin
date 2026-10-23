# 服务调用、消息与定时任务

## 服务发现调用

两套客户端不能混淆：

- 框架 `client.Call(service, uri, *client.Options) (string, error)`，`client.CallT[T]` 返回 `models.Result[T]`（status）。
- 订单等项目的 `ms.Call/CallT/CallPageT` 是业务本地封装，Options 来自该项目 `model/postlog.go`，返回 common 的 `vo.Response[T]`（code）。

先读目标 `ms/client.go` 以及同一个下游服务文件。服务名和 URI 放在现有 `ms` 常量中，通过 registry 解析，不写死实例地址。`Query` 是 URL 参数，`Json` 是 JSON body，`Data` 用于表单，`Header` 用于必要透传；具体默认协议由 `config.Config.Discovery.CallType` 决定。

保留链路头和所需用户凭据，避免给错误服务透传敏感头。检查调用错误及业务 Code/Status，不把 HTTP 200 等同成功。涉及付款、下单等非幂等写入时，不能因超时无条件重试；先确认业务键、下游去重及重试边界。

`ms/client.go` 的历史日志会输出 Header/Body，示例只能借鉴调用形状，不复制这种敏感信息记录方式。修改封装时覆盖空响应、无效 JSON、业务失败等路径，避免切片或类型断言 panic。

## 消息插件和业务链

| 消息类型 | 真实入口范例 | 关注点 |
|---|---|---|
| NATS | 订单 `main.go`、`mq/enter.go`、`mq/order.go` | UsePlugin("nats", nats.NATS) 后注册 SubscribeQueue；队列组是消费语义的一部分 |
| MQTT | 桌台 `main.go`、`mq/open_table.go` 等 | UsePlugin("mqtt", mqtt.MQTT) 后订阅；topic 常用 `/` 分级 |
| Kafka | 订单 `mq/enter.go`、预订 `kafka/kafka.go` | 核对分区/消费者约定与实际主题；不是所有入口都同版本 |
| RabbitMQ | jh-mq-handler `main.go`、`mqhandler/handler.go` | UsePlugin("rabbitmq", mgrabbit.Rabbit)，RabbitMessageListener；检查延时重试队列与上限 |

订单的 `mq.PublishMsg` 实际先发 MQTT，在配置为 kafka 时再转换 `/` 为 `.` 并发 Kafka；不能概括成二选一。修改前确认订阅方、推送对象和发送错误处理。

RIS 消息常复用 common `model.Message[T]`，带业务消息 ID、时间、门店、订单与 Data；字段与 JSON tag 以实际定义为准。落库和消息发布有失败间隙，普通 Publish 不代表数据库和消息原子提交。需要可靠一致性时沿已有设计实现或明确讨论事务消息/outbox，而不是宣称现有代码保证 exactly-once。

消费者通常先解码、校验并委托业务 service；重复与乱序是需要验证的情形。保留消息标识用于去重，错误可定位但不泄露完整敏感 payload。副作用成功后按实际客户端语义确认；不要臆造 ACK API。

jh-mq-handler 范例使用 `MqMsgId`、`handleRetries` 与配置重试上限/间隔、失败记录。它判断业务 `code == 1`，不是框架 `status == 1`。沿配置设置有限重试，不复制忽略错误或不安全类型转换。

## 定时任务

业务定时器与 mgin `job` 调度器是两条独立路线：订单 `schedule/report.go`、支付 `schedule/handler.go`、预订 `schedule/cron.go` 应按各自调度库维护。多副本部署要核对重复执行、锁、时区及取消机制，不默认为单实例。

本地框架提供 `job.Register(name string, h HandlerFunc)`，其中 `HandlerFunc` 为 `func(ctx *job.Context) error`；源码在 `job/handler.go`。调度器启动涉及配置 used 中的 job；管理路由是否挂载还看 `go.job.enabled`。具体存储、任务创建和鉴权依据当前 `job/` 源码与配置，不只注册 handler 就认为定时任务已经持久化。

## 按症状定位

- 插件未连接：先核对 used、prefix、GetConfigData 的来源与插件注册顺序，再查插件 Check 错误。
- 服务发现失败：核对 registry 类型、目标服务名、group/project 和实际注册信息，避免改成固定 IP 掩盖问题。
- 接口调用成功但业务认为失败：检查 code/status 协议、响应泛型、Method 与 Json/Data。
- DAO 结果异常：检查错误与 nil、零值条件、实际 TableName/CollectionName、连接 tag、隔离条件。
- 消息不达或重复：沿发布者 → topic/队列组 → 消费注册 → 业务状态/幂等检查链路定位。
- 启动找不到配置：核对 NewApp 的路径解析、`-f`、可执行目录及组件 `<prefix>-<env>.yml`。

