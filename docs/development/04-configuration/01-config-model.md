# 配置模型

## 1. 两类配置，两种加载方式

这是 MGin 配置体系最容易误解的地方：

```
application.yml（唯一被 koanf 加载进 Config.Cnf 的文件）
   │
   ├─ go.application.*      进程参数
   ├─ go.config.*           数据源编排（used / env / server_type / prefix）
   ├─ go.logger.* go.log.*  日志
   ├─ go.discovery.*        发现与调用
   └─ go.health / metrics / job / i18n / ratelimit / casbin / sys / jwt
          │
          │  按 prefix + env 计算文件名
          ▼
   mysql-test.yml / redis-test.yml / nacos-test.yml ...（组件子配置）
          │
          └─ 由各组件 Init(cnfData []byte) 自行 koanf 解析
```

关键结论：

| 事实 | 影响 |
|------|------|
| `Config.Cnf` **只包含 application.yml** | `GetConfigString("go.data.mysql")` 读不到 `mysql-test.yml` 里的键 |
| 子配置按文件整块传给组件 | 不存在“全局深度合并”；同名键不会互相覆盖 |
| 每个组件只认自己的节点 | 把 `redis` 配置写进 `mysql-test.yml` 不会生效 |
| 没有配置也能启动 | `Cnf == nil` 时所有 `GetConfig*` 返回零值，进程继续运行 |

---

## 2. 命名空间

| 命名空间 | 归属 | 是否被框架读取 |
|----------|------|----------------|
| `go.application.*` | 进程/HTTP | ✅ |
| `go.config.*` | 数据源编排 | ✅ |
| `go.logger.*`、`go.log.*` | 日志 | ✅ |
| `go.discovery.*` | 注册发现与调用 | ✅ |
| `go.data.*`（在子配置文件中） | 数据源 | ✅（组件内解析） |
| `go.nacos/etcd/consul/polaris.*`（子配置） | 注册/配置中心 | ✅（组件内解析） |
| `go.health.*`、`go.metrics.*` | 探针与指标 | ✅ |
| `go.ratelimit.*` | 限流 | ✅ |
| `go.job.*` | 定时任务 | ✅ |
| `go.i18n.*` | 国际化 | ✅ |
| `go.jwt`、`go.casbin`、`go.sys` | 安全 | ✅ |
| `go.framework.*` | 框架元配置 | ⚠️ 仅 `i18nEnabled` 被消费 |
| `go.runtime.*` | 运行期元数据 | 只读，不来自 YAML |

完整键表（类型/默认值/生效条件）见 [08 · 配置项参考](../08-reference/configuration.md)。

---

## 3. 前缀与环境

```yaml
go:
  config:
    env: test          # 环境名
    server_type: file  # 配置来源：file|nacos|consul|etcd|polaris|springconfig
    server: ""         # 配置中心地址（file 时可为空）
    path: conf         # file 模式的目录
    prefix:
      mysql: mysql     # 子配置文件名前缀
      redis: redis
      sqlite: ./data/app.db   # 注意：sqlite 的值是文件路径
```

子配置文件名固定：

```
{prefix}-{env}.yml
```

例：`prefix.mysql = mysql`、`env = test` → `mysql-test.yml`。

### 3.1 各来源的取数地址

| `server_type` | 地址规则 | 备注 |
|---------------|----------|------|
| `file` | `{WorkDir 或 path}/{prefix}-{env}.yml` | 默认；`WorkDir` = application.yml 所在目录 |
| `nacos` | `{server}nacos/v1/cs/configs?group={project}&dataId={prefix}-{env}.yml` | `server` 需以 `/` 结尾或按实现拼接 |
| `consul` | `{server}/v1/kv/{project}/{prefix}-{env}.yml?dc=dc1&raw=true` | dc 固定 dc1 |
| `etcd` | clientv3 key `/config/{project}/{prefix}-{env}.yml` | 需要 `go.etcd.server/port` |
| `polaris` | `GET /config/v1/configfiles/release?...` | 需要 `X-Polaris-Token` |
| `springconfig` | `{server}/{project}/{prefix}-{env}.yml` | — |
| 其他 | `{server}{prefix}-{env}.yml` | 兜底拼接 |

`project` = `go.application.project`，未配置时为 `DEFAULT`。

---

## 4. 生效条件（Enable 语义）

配置写了不等于生效，要看组件是否被启用：

| 组件 | 启用条件 | 说明 |
|------|----------|------|
| MySQL | `go.config.used` 含 `mysql` | 子串匹配 |
| SQLite | `used` 含 `sqlite` | `prefix.sqlite` 作为文件名 |
| MongoDB | `used` 含 `mongodb` | |
| Redis | `used` 含 `redis` | |
| 注册中心 | `used` 含 `nacos`/`etcd`/`consul` | `go.discovery.registry` 决定客户端类型 |
| 定时任务 | `used` 含 `job` **且** `go.job.enabled=true` | 两个条件都要 |
| i18n | `NewApp(..., true)` 或 `go.i18n.enabled=true` 或 `go.framework.i18nEnabled=true` | |
| 健康探针 | `go.health.enabled=true` 或 `app.EnableHealth()` | 后者立即挂载 |
| 指标 | **必须** `app.EnableMetrics()` | 仅 `go.metrics.enabled` 不挂载 |
| 限流 | `go.ratelimit.enabled=true` | 且 `go.ratelimit` 节点存在 |
| Casbin | `go.casbin.enabled=true` | 仍需业务挂载中间件 |
| JWT | 业务挂载 `jwt.JwtAuthorize()` | 无全局开关 |
| postlog / recovery / cors | 恒启用 | 无开关 |

> `used` 的匹配是 `strings.Contains`，不是精确匹配。写 `mysqlx` 会误启用 `mysql`。

---

## 5. 优先级与覆盖

MGin **没有**多来源深度合并，因此“优先级”只体现在三个层面：

1. **进程参数 vs 配置文件**：`-f` 只决定读哪个文件，不覆盖单键。
2. **主配置 vs 子配置**：互不覆盖，各读各的。
3. **配置文件 vs 代码**：
   - `EnableHealth()` 会无视 `go.health.enabled` 直接挂载；
   - `EnableMetrics()` 会无视 `go.metrics.enabled` 直接挂载；
   - 业务可在 `NewApp` 之后修改 `config.Config` 字段，但**不保证**已初始化的组件感知。

如果需要“环境变量覆盖”，在部署层渲染配置（如 Helm/Kustomize/envsubst），
框架不读取环境变量。

---

## 6. 动态刷新

| 配置 | 是否热更新 | 说明 |
|------|-----------|------|
| `go.logger.level` | ✅ 实时 | 每次输出都重新读 |
| `go.application.debug` | ✅ 实时 | 强制 debug 级别 |
| 限流规则 | ⚠️ 构造时 | 中间件创建时读取；改配置需重启 |
| 服务发现 TTL | ⚠️ 启动时 | 注册中心实例创建时读取 |
| 数据源/插件 | ❌ | 仅启动时初始化 |
| 业务配置 | ❌ | 用 `GetConfig*` 每次读的是启动时快照 |

没有配置中心推送监听（除远端拉取发生在启动阶段）。需要动态业务开关时，
用数据库/Redis 配置，而不是 application.yml。

---

## 7. 配置缺失与错误

| 情况 | 行为 |
|------|------|
| application.yml 不存在 | 记录 `读取配置文件错误`，`Cnf=nil`，进程继续 |
| 子配置文件不存在 | `GetConfigData` 返回 nil，组件 Init 报错但不中断 |
| 单个 key 不存在 | `GetConfig*` 返回零值（`""`/`0`/`false`/`nil`） |
| 类型不匹配 | koanf 尝试转换，失败返回零值 |
| `go.config.used` 为空 | 所有数据源/注册/任务插件都不启用 |
| VictoriaLogs 配置非法 | `InitVictoriaLogs` **panic**（唯一会中断启动的配置） |

---

## 8. 最小配置模板

```yaml
go:
  application:
    name: order
    project: shop
    port: 8080
    debug: false
    shutdownTimeout: 10
  config:
    env: prod
    server_type: file
    path: conf
    used: "mysql,redis,nacos"
    prefix:
      mysql: mysql
      redis: redis
      nacos: nacos
  logger:
    level: info
    out: console
  discovery:
    registry: nacos
    callType: json
  health:
    enabled: true
    autoStarted: true
  metrics:
    enabled: true
```

> 注意：`metrics.enabled` 只是“允许”，还需要 `app.EnableMetrics()`；
> 若希望完全由配置驱动，请在 `main` 中显式调用一次。

下一步：[配置方式与核心配置语义](02-config-guide.md)。
