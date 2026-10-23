# 配置模型

MGin 的配置只有两级：**主配置**（`application.yml`）与**子配置**（`<prefix>-<env>.yml`）。理解这两级与定位规则，能避免 80% 的启动问题。

## 1. 层级与职责

```
application.yml
├── go.application.*      应用名、端口、项目、debug
├── go.logger.*           运行日志
├── go.log.*              访问/调用日志、VictoriaLogs、队列
├── go.config.*           组件开关（used）、子配置前缀、配置中心
├── go.discovery.*        注册中心与调用默认协议
├── go.jwt / go.casbin / go.cors / go.sys
├── go.health / go.metrics
├── go.ratelimit          限流规则（内联）
├── go.job                调度器（内联）
└── go.i18n               字典（内联）

<prefix>-<env>.yml
└── go.data.*             数据源连接（由组件自行解析）
```

## 2. 定位规则

`mgin.NewApp(configFile, ...)` 的路径处理：

| 输入 | 行为 |
|------|------|
| 绝对路径 | 直接使用 |
| 含 `/` 或 `\` 的相对路径 | 相对**当前工作目录** |
| 不含分隔符（如 `application.yml`） | 拼到**可执行文件目录** |
| 空 | 使用 `-f` 参数，默认 `<可执行文件名>.yml` |

配置文件所在目录记为 `WorkDir`，它同时是以下相对路径的基准：

- 子配置（`file` 模式）；
- `casbin.model_file`；
- SQLite 数据库文件、i18n 独立 SQLite 文件；
- 磁盘缓存（未指定绝对路径时）。

`go.config.path` 可覆盖本地子配置目录（不改变 `WorkDir` 的其他用途）。

## 3. 组件与子配置

```yaml
go:
  config:
    used: "mysql,redis,mongodb,job"
    env: test
    prefix:
      mysql: mysql
      redis: redis
      mongodb: mongodb
```

- `used`：逗号分隔，**子串匹配**。`mysql` 会命中任何包含该子串的值。
- `env`：子配置后缀，切换环境只改这一项。
- `prefix.<组件>`：子配置文件名前缀；`sqlite` 例外，值就是文件名。

组件初始化完全由 `used` 驱动；未声明时不会建立连接、不会启动协程、不会出现在 `/health/ready`。

## 4. 配置来源与优先级

单一来源，不存在多层覆盖。来源由 `go.config.server_type` 决定：

| 来源 | 定位方式 | 备注 |
|------|----------|------|
| `file`（默认） | `<path>/<prefix>-<env>.yml` | 本地文件，缺失时返回 nil |
| Nacos | `{server}/nacos/v1/cs/configs?group={project}&dataId={prefix}-{env}.yml` | 需要 `go.config.server` |
| Consul | `{server}/v1/kv/{project}/{prefix}-{env}.yml?raw=true` | — |
| etcd | `/config/{project}/{prefix}-{env}.yml` | 5s 超时 |
| Polaris | `.../configfiles/release?...&name={prefix}-{env}.yml` | 需要 `go.config.token`，支持 `prefix@v版本` |
| Spring Config | `{server}/{project}/{prefix}-{env}.yml` | — |

优先级（从高到低）：

1. 代码中显式调用的值（如 `health.Router(group)`）；
2. 命令行 `-f` 指定的主配置；
3. 配置中心/本地子配置。

**没有**环境变量覆盖机制。容器化部署时建议用 ConfigMap/Secret 挂载文件，或由启动脚本渲染配置。

## 5. 默认值来源

默认值只在 `config.Init` 中设置一次，之后不会因配置缺失而回退（除少数组件内部兜底）。关键默认值：

| 键 | 默认 |
|----|------|
| `go.application.project` | `DEFAULT` |
| `go.discovery.callType` | `x-form` |
| `go.discovery.cacheTTL` / `cacheRefresh` | 60 / 60 秒 |
| `go.logger.level` | `error`（`debug=true` 时强制 `debug`） |
| `go.log.sink.queue` / `workers` / `shutdown` | 100 / 1 / 3000ms |
| `go.casbin.model_file` | `rbac_model.conf`（仓库只提供 `casbin.conf`） |
| `go.job.tablePrefix` | `mgin_` |
| `go.job.scanInterval` / `refreshInterval` | 1 / 30 秒 |
| `go.job.logRetainDays` / `maxConcurrent` / `maxSerialQueue` | 30 / 50 / 10 |
| `go.i18n.default` | `zh-cn` |

完整键表见 [变量与配置](../08-reference/variables.md)。

## 6. 配置的最小可用集合

```yaml
go:
  application:
    name: myapp
    port: 8080
    project: myapp
  config:
    used: ""
    env: test
```

其余全部可省略。先用最小集启动，再按需逐个启用组件，是排查配置问题最有效的方法。

## 7. 约束

1. **一个进程一份配置**：不支持多 App 或多租户配置隔离。
2. **`used` 是子串匹配**：自定义组件名不要与内置名互为子串。
3. **子配置只在启动/重载时读取**：运行期修改文件不会生效。
4. **`WorkDir` 依赖可执行文件位置**：容器内请使用绝对路径或固定工作目录。
5. **未实现的键不报错**：`go.s3`、`go.data.postgres` 等会被静默忽略，容易误判「配置已生效」。
