# 生命周期、配置与版本边界

以下 API 在本地 v1 module 源码中核实。目标项目锁定较旧版本时，先核对依赖源码是否具备相应能力。

## 启动

`mgin.NewApp(configFile, appName, version string, xlang bool) *App` 会读取配置并初始化启用的内置依赖、服务注册和路由；不是无副作用的构造函数。

典型业务顺序：

```go
app := mgin.NewApp("", "服务名称", Version, false)
// 项目使用的数据库模型迁移/业务初始化
// app.MGin.UsePlugin("mqtt", mqtt.MQTT) 等所需外部插件
// 消费者、业务中间件、router 和 swagger 注册
app.Run()
```

这只是顺序示意，具体导入与初始化按目标项目。插件必须在使用其连接、订阅之前初始化。v1 本地 `NewApp` 已安装 trace、postlog、CORS、可选语言、配置启用的限流及 recovery，通常无需重复挂载。

- 空配置路径从 `-f` 取值，默认是可执行文件同名 `.yml`。只有文件名、没有路径分隔符时，相对可执行文件目录解析；`go run` 的临时可执行路径可能造成配置找不到。调试显式传入含目录的配置路径。
- 本地 v1 `-v` 打印版本后 `os.Exit(0)`，不能依赖旧样例中的 `app == nil` 分支来处理版本参数。
- `Run()` 启动服务、等待信号并调用框架退出流程。不要在单元测试中直接运行。
- 框架内置资源初始化与业务 `mysql.MySQLInit()` 分工不同：前者连接，后者通常 AutoMigrate。

## 配置及插件

读取方式：`config.Config.GetConfigString("go.xxx")`、`GetConfigInt`、`GetConfigBool`；已解析内容还包括 `config.Config.App`、`Config`、`Discovery`。

关键关系：

- `go.application.name/port/project/debug`：服务身份和监听。
- `go.config.used`：逗号分隔的启用项；`go.config.prefix.<组件>`：组件配置前缀。
- `go.config.server_type/server/env`：配置来源，支持类型以当前 `GetConfigUrl/GetConfigData` 实现为准。
- `go.discovery.registry`、`callType`：服务发现及默认调用协议；`json`、`x-form` 等协议影响客户端 Body 的用法。

本地 `UsePlugin(key, plugin)` 要求 plugin 实现：

```go
type MginPlugin interface {
    Init(configData []byte)
    Close()
    Check() error
}
```

`UsePlugin` 检查 used，读取 `go.config.prefix.<key>`，经 `GetConfigData` 获取配置后初始化并纳入健康检查、退出流程。只加 Go 注册语句而不配置 used/prefix 不会成功启用。

内置 MySQL、MongoDB、Redis、SQLite、注册中心、job 等与外部 `mqtt`、`nats`、`kafka`、`rabbitmq` 插件区分。插件 key 以现有入口为准，例如 mgrabbit 使用 `rabbitmq`。配置前缀并不总是内联 YAML 节点：`server_type: file` 会解析 `<prefix>-<env>.yml`，应核对源码而非猜测。

## v2 与新能力

`jh-ris-booking/main.go` 使用 `github.com/maczh/mgin/v2`、`v2/pkg/health`，并调用 `EnableHealth()`、`EnableMetrics()`、`health.MarkStarted()`。这些不能直接移植到本地 v1 的 `App`。遇到混合 v1/v2 imports，要先厘清各包的真实依赖和类型，不能批量替换路径后声称迁移完成。

脚手架 `cmd/new.go` 有 `-module`、`-port`、`-db`、`-mq`、`-registry`、`-config-center`、`-mgin-version`、`-output` 等参数；默认可能在线解析版本，且检测到 TTY 会进入交互。自动化前检查参数和模板，在新目录输出，不对已有服务使用 `-force`。

