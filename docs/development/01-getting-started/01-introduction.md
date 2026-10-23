# 框架简介

## 1. 它解决什么问题

写一个 Go 后端服务，业务代码可能只有几百行，但周边工程问题总是一样的：

- 配置从哪来？本地 YAML、Nacos、etcd、Consul 怎么切换？
- MySQL/Redis/Mongo 连接池怎么初始化，进程退出时怎么关？
- 调下游服务时，服务地址从注册中心拿，还是写死？超时、重试、熔断怎么办？
- 日志怎么带 trace id？接口 QPS 和耗时怎么给 Prometheus？
- 健康探针、优雅停机、错误码与多语言文案怎么统一？

MGin 把这些**重复的工程决策**收敛成一套可复用组件与约定。你只需写
`controller → service → dao`，其余能力通过配置和少量装配代码获得。

## 2. 一句话定位

> MGin 是**单进程内运行**的 Go 微服务基础框架（framework + infrastructure glue），
> 不是网关、不是 Service Mesh、不是 RPC 框架。

一个 MGin 应用就是一个 `gin.Engine`：

- 同一进程同时提供 HTTP（`go.application.port`）与可选 HTTPS（`port_ssl`+证书）；
- 组件（MySQL、Redis、注册中心、任务调度）以插件形式初始化与关闭；
- 服务间调用走 HTTP/JSON，通过注册中心发现实例，客户端做负载均衡与熔断。

## 3. 适合与不适合

| 适合 | 不适合 |
|------|--------|
| 中小规模 HTTP 微服务、内部业务系统 | 需要 gRPC/多语言 IDL 的强契约场景 |
| 希望统一配置、日志、错误码、探针的团队 | 需要服务网格（mTLS、细粒度流量镜像） |
| 单仓库多服务、以 MySQL 为主的数据访问 | 需要分布式事务、跨库强一致 |
| 已有 Nacos/Consul/etcd 基础设施 | 只有 K8s Service、无注册中心的纯云原生团队（可不用注册插件） |
| 需要内置定时任务、i18n、Casbin 的中后台 | 需要集群级限流、分布式任务调度 |

## 4. 与常见方案的关系

| 对比对象 | 差异 |
|----------|------|
| 裸 Gin | MGin 在 Gin 之上补配置、数据源、注册发现、调用容错、探针、指标、错误码与脚手架 |
| go-zero / Kratos | 它们自带代码生成与更强约束（IDL、分层模板）；MGin 只提供库与约定，目录与分层由你决定 |
| Spring Cloud | MGin 是库而非平台；没有配置中心/网关/链路后端的自带实现，依赖外部中间件 |
| Service Mesh | MGin 在**进程内**做负载均衡与熔断；Mesh 在 Sidecar 做。二者可共存，但不要重复配置重试 |
| Dubbo | MGin 的调用是 HTTP/JSON，不做多协议、不做泛化调用、不做 IDL |

## 5. 能力全景（一屏）

```
接入      gin 路由 · 中间件链 · HTTP/HTTPS · 优雅关闭
配置      YAML · Nacos/Consul/etcd/Polaris/SpringConfig · 点分键读取
数据      MySQL · SQLite · MongoDB · Redis · 泛型 DAO · 事务 · 查询缓存
治理      注册发现 · 4 种负载均衡 · 超时/重试/熔断 · 限流 · JWT/Casbin
可观测    /health/* · /metrics · traceparent · 控制台/文件/Kafka/Mongo/VictoriaLogs 日志
扩展      插件 · 自定义负载均衡 · 自定义缓存 · 自定义日志 Handler · 中间件
工具      定时任务 · i18n · 统一错误码 · 40+ 工具函数
```

每项的精确签名见 [08 · 参考手册](../08-reference/README.md)；
能力边界与未实现项见 [11 · 已知问题](../11-appendix/03-migration-and-issues.md)。

## 6. 一个最小应用的形状

```go
func main() {
    app := mgin.NewApp("/etc/hello/application.yml", "hello", "1.0.0", false)
    app.EnableHealth()
    app.EnableMetrics()
    app.Router.GET("/api/v1/ping", func(c *gin.Context) {
        c.JSON(200, gin.H{"msg": "pong"})
    })
    app.Run()
}
```

启动后自动拥有：`/health/live`、`/health/ready`、`/health/startup`、`/metrics`、
请求日志、trace id、CORS、panic 恢复、404 统一响应、优雅停机。

下一步：[环境准备与第一个服务](02-quickstart.md)。
