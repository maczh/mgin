# 实际源码导航

以下记录来自 2026-10-15 本机源码检查，是定位线索。版本会变，使用时重新核对目标 `go.mod`、imports 和 replace；源码优先于旧 README 或 AGENTS 中的版本描述。

框架根：`/home/Macro/work/go/src/github.com/maczh/mgin`（module `github.com/maczh/mgin`，Go 1.25.7）。
范例根：`/home/Macro/work/go/src/jihaihotpot.com/jihai`。下表路径均相对于范例根。

| 项目 | 检查时版本 | 适用场景与优先文件 |
|---|---|---|
| `jh-ris-order` | mgin v1.25.22-jh | 多端 MVC、NATS、服务调用；先读 `AGENTS.md`、`main.go`、`router/enter.go`、`controller/app/order.go`、`mysql/enter.go`、`ms/client.go`、`mq/enter.go` |
| `store-table` | v1.25.19-jh | 较简单的接口到 DAO、MQTT 桌台事件；`router/enter.go`、`controller/table.go`、`service/table.go`、`mysql/enter.go`、`model/table.go`、`vo/result.go` |
| `jh-pay` | v1.25.17-jh | 支付业务及列表分页；`router/order.go`、`controller/order.go`、`service/order.go`、`model/request/req.go`、`model/vo/result.go` |
| `jh-mq-handler` | v1.21.17-jh | RabbitMQ 消费、业务转发及有限重试；`main.go`、`initialize/init_config.go`、`mqhandler/handler.go`；旧版本样例，不整体照搬实现 |
| `jh-async-mail` | v1.25.2-jh | 异步邮件服务；`main.go`、`service/rabbit.go`、`controller/template.go` |
| `jh-ris-booking` | 同时 require v1.25.6-jh 与 v2.0.8 | `main.go` 实际导入 v2；`router/router.go`、`mysql/enter.go`、`mqttsub/subscriber.go`、`schedule/cron.go`；仅在 v2 任务中参考其启动和探针 API |
| `jh-ris-common` | v1.25.6-jh | 跨服务共享 `model/`、`vo/`，部分服务通过 `replace` 指向 `../jh-ris-common` |

`sea-admin`、`sea-admin-1`、`jh-food-order` 检查时没有直接 mgin 依赖，不能仅因位于同一目录就作为 mgin 标准结构。

## 框架定位

- 生命周期和基础中间件：`app.go`、`mgin.go`。
- 配置：`config/configure.go`、`application.example.yml`。
- 结果协议：`models/result.go`；请求客户端：`client/client.go`；服务发现：`registry/registry.go`。
- MySQL/Mongo DAO：`db/dao/mysql.go`、`db/dao/mgo.go`；连接包装：`db/`。
- 日志与链路：`logs/`、`middleware/trace/`、`middleware/postlog/`。
- 任务：`job/handler.go`、`job/scheduler.go`、`job/api.go`。
- 脚手架是独立模块：`cmd/go.mod`、`cmd/new.go`、`cmd/scaffold.go`、`cmd/templates.go`。生成前检查模板实际导入的主版本。

检索首选 `rg`；若未安装，用 `find`、`grep` 或 Python 定向搜索。排除 `.git`、二进制、日志、vendor 和生成文档，避免把无关历史副本当实际实现。

