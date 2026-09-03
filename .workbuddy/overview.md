# mgin 脚手架命令行工具（cmd/mgin）

## 目标
在 mgin 项目中新增 `cmd/` 目录，提供一个模仿 beego `bee` 的命令行脚手架 `mgin`，用于一键生成 mgin 微服务工程骨架，并在创建时选择数据库、端口、消息队列、注册中心、配置中心、JWT、Casbin、i18n 等配置。

## 交付物（`cmd/` 目录）
| 文件 | 作用 |
|------|------|
| `mgin.go` | 入口与子命令分发（`new` / `version` / `help`） |
| `new.go` | 解析 `mgin new <工程名> [选项]`，支持交互式与 flag 两种方式；处理位置参数与 flag 混排 |
| `templates.go` | 所有代码/配置模板（main / router / controller / model / dao / service / go.mod 等）与组件元信息 |
| `scaffold.go` | 推导配置（used / prefix / 数据层 / 配置中心），写出整个工程树，并渲染 README；所有写出文件经 UTF-8 合法性校验 |

## 本次新增 / 修改（第二轮）
1. **mgin 版本锁定为 `v1.25.13-jh`**：`templates.go` 中 `tmplGoMod` 的 `require github.com/maczh/mgin v1.25.13-jh`；`main.go` 改用由构建注入的 `Version` 变量（不再硬编码 `"1.0.0"`）。
2. **生成源码强制 UTF-8**：`scaffold.go` 的 `write()` 在落盘前用 `unicode/utf8.ValidString` 校验，含中文注释的文件均保证为合法 UTF-8（杜绝乱码）。
3. **新增 `version.go` 与 `Makefile`**（参考 `jihaihotpot.com/jihai/jh-ris-order`）：
   - `version.go`：`package main` 下声明 `Version / BuildNum / BuildTime / GitHash`，由 `Makefile` 通过 `-ldflags "-X main.Version=..."` 注入。
   - `Makefile`：`BINARY`、`VERSION`（取自 git tag）、`BUILD_TIME`、`GIT_HASH`，`build` / `linux`（含 `upx`）/ `run` 目标，统一 `GOPROXY=https://goproxy.cn`。
4. **配置改为 CLI 交互问答**：`new.go` 重写参数解析 —— 显式 `-i/--interactive` 或检测到终端(TTY)时进入菜单式问答，逐项选择「工程名 / module / 端口 / 数据库(多选) / 消息队列 / 注册中心 / 配置中心 / JWT / Casbin / i18n / 系统模块」，已通过 flag 提供的取值作为默认值展示；管道/CI（非 TTY）仍走 flag + 默认值路径。参数切分逻辑修正了「取值型 flag 的取值被当作工程名」的解析 bug。
5. **main 与 controller 添加 Swagger 风格注释**：
   - `main.go`：`// @title` / `@version` / `@description` / `@contact` / `@license` / `@BasePath` 等。
   - `controller.go`：每个 handler 增加 `// @Summary` / `@Tags` / `@Produce` / `@Param` / `@Success` / `@Router` 等。

## 本次新增 / 修改（第三轮）
1. **消息队列改为多选并接入官方插件**：`--mq` 支持 `nats,kafka,mqtt,rabbit` 逗号分隔多选（交互菜单同样为多选，`none`/直接回车表示不使用）。
   | 选项 | 插件模块 | 初始版本 | 配置键 | 独立配置文件 |
   |------|----------|----------|--------|--------------|
   | `nats` | `github.com/maczh/nats` | v0.0.2 | `go.data.nats.*` | `conf/nats-<env>.yml` |
   | `kafka` | `github.com/maczh/mgkafka` | v1.1.2 | `go.data.kafka.*` | `conf/kafka-<env>.yml` |
   | `mqtt` | `github.com/maczh/mqtt` | v1.1.6 | `go.data.mqtt.*` | `conf/mqtt-<env>.yml` |
   | `rabbit` | `github.com/maczh/mgrabbit` | v0.1.1 | `go.rabbitmq.*` | `conf/rabbitmq-<env>.yml` |
   - 生成 `plugins.go`，通过 `app.MGin.Use(name, pkg.Singleton.Init, pkg.Singleton.Close, nil)` 注册；`main.go` 在启用任意 MQ 时调用 `registerMQPlugins(app)`。
   - 所选插件自动写入 `go.mod` 的 `require`（版本为已知初始值，`go mod tidy` 后自动修正）。
2. **mgin 依赖版本自动获取最新发布版**：`--mgin-version` 可手动覆盖；否则查询 `GOPROXY` 的 `@v/list` 取最高版本，离线/超时回退 `v1.25.13-jh`。注意不能用 `@latest`——`-jh` 后缀在 semver 中是预发布号，`@latest` 会错误返回更低的 `v1.25.4`。
3. 顺带修复：`splitArgs` 的取值型 flag 清单缺失 `-mgin-version`，导致 `--mgin-version <ver>` 的取值被误判为工程名。

## 用法
```bash
go build -o mgin ./cmd
./mgin new <工程名> \
  --module github.com/you/proj \
  --port 8080 \
  --db mysql,redis \
  --mq nats,kafka \
  --registry nacos \
  --config-center nacos \
  --jwt --casbin --i18n
```
省略任意选项时，若为终端则逐项交互询问；非终端（如管道）使用默认值。

## 生成内容
- 源码：`main.go`、`version.go`、`router/router.go`、`controller/controller.go`、`model/model.go`、`service/service.go`（sqlite 等内存层场景无 `dao/`）、`dao/dao.go`（数据库场景）、`plugins.go`（启用消息队列时）
- 配置：`conf/application.yml` + 各组件 `conf/<prefix>-<env>.yml`（mysql/redis/kafka/nacos…）+ `conf/casbin.conf`
- 构建：`Makefile`
- 其它：`go.mod`、`README.md`、`.gitignore`
- 选项映射：`--db` → `go.config.used`/`prefix`；`--mq/--registry/--config-center` → `go.config.used`、`go.config.server_type`、各组件 yml 与 `go.discovery.registry`；`--jwt/--casbin/--i18n` → 对应中间件与配置项；`--port` → `go.application.port`。

## 验证结果（已实测）
1. `go build ./cmd` 编译通过；二进制可运行。
2. 生成工程可 `go build` 通过 —— 关键 API 均已对齐真实 mgin：
   - `mgin.NewApp(cfg, name, ver, xlang)`、`app.Router`、`app.Run()`
   - `models.Success/Error`、`dao.MySQLDao[model.Product]{}`（泛型 DAO，`All`/`One`）
   - `middleware/jwt.JwtAuthorize()`、`middleware/casbin.CasbinHandler()`
3. 启动生成的 app：成功监听所配置端口（如 18097），路由注册正确（`GET /api/v1/products` 等）。
4. 中间件链路实测：无 token → `401`；合法 JWT → 通过 JWT 中间件、被 Casbin 中间件以 `401 用户未登录` 拒绝（证明两层中间件均已正确挂载）；去掉 Casbin 后合法 JWT 可到达 controller→service→dao（DB 未启动返回系统异常，证明整条调用链通畅）。
5. 消息队列多选实测：`--mq nats,kafka,mqtt,rabbit` 生成的工程 `go mod tidy` + `go build` 均通过，四个插件全部解析并编译成功（验证 `nats.NATS.Init/Close`、`mgkafka.Kafka.Init/Close`、`mqtt.MQTT.Init/Close`、`mgrabbit.Rabbit.Init/Close` 的方法值签名与 mgin 的 `dbInitFunc func([]byte)` / `dbCloseFunc func()` 完全匹配）；单选（仅 mqtt）与不选（不生成 `plugins.go`、`main.go` 不调用注册函数）两种场景同样正确。

## 仓库内示例工程 `examples/quickstart`
- 位置：`examples/quickstart/`，module 为 `github.com/maczh/mgin/examples/quickstart`。
- 生成命令：`go run ./cmd new quickstart --module github.com/maczh/mgin/examples/quickstart --db sqlite --port 18096 --output examples --force`，其 `go.mod` 已追加 `replace github.com/maczh/mgin => ../..` 以便直接基于本地源码构建/运行。
- 选用 `sqlite` → 走内存示例层（无需任何外部中间件），配置中心默认 `file`（读取本地 `conf/*.yml`）。
- 已实测：`go build` 通过；启动后 `GET /api/v1/products` 返回 `200` 与示例商品列表，`GET /api/v1/products/2` 返回单条，开箱即用。
- 注意：`examples/quickstart/go.mod` 中的 `replace` 仅用于仓库内演示；作为独立工程发布时应删除该行并执行 `go mod tidy`。

## 注意事项（使用者需知）
- 生成的 `go.mod` 中 mgin 版本由脚手架自动获取最新发布版（离线回退 `v1.25.13-jh`）；消息队列插件版本为已知初始值，执行 `go mod tidy` 后会自动修正。
- **kafka 的特殊之处**：mgin 框架内置了 sarama 版 kafka 客户端，`go.config.used` 含 `kafka` 时框架会先初始化内置客户端，再初始化 `mgkafka` 插件（两者读取同一份 `go.data.kafka` 配置）。若只想用插件，需从 `used` 中去掉 `kafka`——但注意 `Use()` 要求 `used` 含该组件名，故当前实现保留 `kafka` 键，生成代码中的注释已说明。nats/mqtt/rabbit 无内置实现，不存在此问题。
- `version.go` 中的 `Version` 等变量由 `make build` / `make linux` 通过 `-ldflags` 注入；直接 `go run .`（不带 ldflags）时 `Version` 为空字符串，框架可正常处理。
- `Makefile` 的 `linux` 目标会调用 `upx`，若环境无 `upx` 可删除该行；`make build` 首步会执行 `go mod tidy`（需联网）。
- `--i18n` 依赖框架的 x-lang 服务，运行时需该服务可达，否则 i18n 初始化会失败。
- 配置中心为 nacos/consul/etcd 等远端类型时，组件配置从对应配置中心拉取；本地无服务时框架仅打印连接错误、不阻断进程启动。默认日志目录已改为相对 `logs/`，避免 `/opt/logs` 无写权限导致进程退出。
- 交互模式：终端下直接 `./mgin new` 即可进入菜单问答；非交互（管道/CI）时通过 flag 指定，未指定的项使用默认值（工程名必填）。
