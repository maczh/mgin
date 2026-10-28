# mgin 命令行脚手架使用说明

`mgin` 是 MGIN `jh` 分支的 Go 微服务项目脚手架，源码位于 `cmd/`。

## 安装与基本语法

```bash
cd cmd
go build -o bin/mgin .
./bin/mgin help
./bin/mgin version
```

```text
mgin new <工程名> [选项]
```

工程名必填。终端环境省略选项时会进入交互问答；非交互环境使用默认值。

## 参数

| 参数 | 默认值 | 说明 |
|---|---|---|
| `--module <path>` | `github.com/maczh/<工程名>` | Go module 路径 |
| `--port <n>` | `8080` | HTTP 服务端口 |
| `--db <list>` | `mysql` | `mysql,postgres,sqlite,mongodb,redis,clickhouse,elasticsearch` |
| `--mq <list>` | 空 | `nats,kafka,mqtt,rabbit`，逗号分隔 |
| `--plugins <list>` | 空 | MGIN 外部插件，见下表 |
| `--registry <type>` | `none` | 单注册中心：`nacos/consul/etcd/none` |
| `--registries <list>` | 空 | 命名多注册中心，格式见下文 |
| `--default-registry <name>` | 第一个实例 | 指定默认注册中心实例 |
| `--config-center <type>` | `none` | `file/nacos/consul/etcd/polaris/springconfig/none` |
| `--nacos-api-version <v>` | `v1` | Nacos 注册中心和配置中心 API：`v1/v2/v3` |
| `--i18n` / `--jwt` / `--casbin` | `false` | 启用对应骨架或中间件 |
| `--mgin-version <v>` | 在线最新版本 | MGIN 依赖版本；离线回退 `v1.25.30-jh` |
| `--output <dir>` | `.` | 生成项目父目录 |
| `--force` | `false` | 覆盖已存在目录 |
| `--interactive`, `-i` | `false` | 强制交互问答 |

## 外部插件

| 选择名 | Go module | 注册表达式 |
|---|---|---|
| `opensearch` | `github.com/maczh/opensearch` | `opensearch.OpenSearch` |
| `hwobs` | `github.com/maczh/hwobs` | `hwobs.NewOBS()` |
| `alioss` | `github.com/maczh/alioss` | `alioss.NewOSS()` |
| `kodo` | `github.com/maczh/kodo` | `kodo.NewKodo()` |
| `hive` | `github.com/maczh/hive` | `hive.Hive` |
| `elasticsearch` | `github.com/maczh/elasticsearch` | `elasticsearch.Elastic` |
| `hbase` | `github.com/maczh/hbase` | `hbase.Hbase` |
| `mssql` | `github.com/maczh/mssql` | `mssql.MsSQL` |
| `postgres` | `github.com/maczh/postgres` | `postgres.Postgres` |
| `clickhouse` | `github.com/maczh/clickhouse` | `clickhouse.ClickHouse` |
| `s3` | `github.com/maczh/s3` | `s3.NewS3()` |
| `influxdb` | `github.com/maczh/influxdb` | `influxdb.InfluxDB` |
| `tdengine` | `github.com/maczh/mgtdengine` | `mgtdengine.TDengine` |

> `github.com/maczh/posegres` 仓库不存在，脚手架使用实际仓库 `github.com/maczh/postgres`。

插件或消息队列被选择后，脚手架会生成 `plugins.go`、依赖和对应的 `conf/<plugin>-test.yml`。

## 常用示例

最小项目：

```bash
./bin/mgin new demo --output /tmp
cd /tmp/demo
go mod tidy
go build -o demo .
```

Nacos v3、MySQL、Redis 和 JWT：

```bash
./bin/mgin new order --module example.com/order --db mysql,redis --registry nacos --config-center nacos --nacos-api-version v3 --jwt --mgin-version v1.25.30-jh --output /tmp
```

外部插件：

```bash
./bin/mgin new data-api --db redis --plugins opensearch,alioss,s3 --output /tmp
```

消息队列与数据库插件：

```bash
./bin/mgin new analytics --db redis --mq nats,kafka --plugins postgres,clickhouse,influxdb --output /tmp
```

## 命名多注册中心

`--registries` 格式为 `name:type[:prefix[:namespace[:group[:cluster]]]]`，空字段必须保留冒号：

```bash
./bin/mgin new gateway --registry none --registries 'jhris:nacos:nacos::jhris:DEFAULT,iot:nacos:nacos::iot:DEFAULT,finance-bank:nacos:nacos:finance:bank_account:DEFAULT' --default-registry jhris --output /tmp
```

相同 `prefix` 表示复用同一物理注册中心配置；不同 `namespace/group/clusterName` 由框架覆盖。不同 `prefix` 会生成额外配置文件。

## 生成目录与运行

生成项目包含 `main.go`、`version.go`、`Makefile`、`go.mod`、`conf/application.yml`、组件配置、路由、控制器、服务和模型；选择外部插件或 MQ 时还会生成 `plugins.go`。选择 `mysql/postgres/clickhouse` 作为 `--db` 时生成 `dao/dao.go`。

```bash
cd /tmp/demo
go mod tidy
go build -o demo .
go test ./...
./demo
```

## 注意事项与排障

- `opensearch` 要求 Go ≥ `1.25.9`，选择后生成项目会使用更高的 Go 版本。
- `tdengine` 使用 CGO，需要安装 TDengine C SDK，并能找到 `taos.h`。
- 对象存储插件配置包含密钥占位符，运行前必须填写 endpoint、访问密钥和 bucket。
- `--registry` 与 `--registries` 不能同时使用非 `none` 的配置。
- `--default-registry` 必须出现在 `--registries` 的名称列表中。
- 插件必须出现在 `go.config.used` 和 `go.config.prefix` 中；脚手架会自动生成这两处。
- 目标目录存在时默认失败，确认内容后才使用 `--force`。
- `go mod tidy` 需要访问 Go module proxy；离线时可显式指定 `--mgin-version`，但第三方插件依赖仍需缓存。

