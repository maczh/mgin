# MGin 插件生态与接入五步法

兄弟仓库在 `/home/macro/Work/go/src/github.com/maczh/`：`mgkafka`、`mgrabbit`、`mqtt`、`mssql`、`nats`、`opensearch`、`hbase`、`hive`、`influxdb`。

## 0. 核心事实：约定式插件，编译期零耦合

**这 9 个仓库没有一个在 go.mod 里 require mgin。** 它们只靠结构化约定对接——实现三方法 + 用 koanf 解析 `go.data.<name>.*`。

v1 契约（`mgin/mgin.go`）：
```go
type MginPlugin interface {
	Init(configData []byte)
	Close()
	Check() error           // 每 5 分钟被 app.MGin.checkAll() 回调，用于断线重连
}
func (m *mgin) UsePlugin(dbConfigName string, p MginPlugin)
func (m *mgin) Use(dbConfigName string, init func([]byte), close func(), check func() error)
```

> ⚠️ 只能 `app.MGin.UsePlugin(...)` / `app.MGin.Use(...)`。`docs/development/09-advanced/01-extension.md` §1 写的包级 `mgin.MGin.Use(...)` 编译不过（`mgin.go` 里 `var MGin = &mgin{}` 已注释）。另外 `Use`/`UsePlugin` 会**立即执行 init**，不是延迟到 `Run()`；name 不在 `used` 里或 `prefix.<name>` 缺失时只打日志并跳过。

v2 契约（`pkg/plugin/plugin.go`）：
```go
type Plugin interface {
	Name() string
	Order() int             // 启动顺序，小者先 Init，Close 逆序
	Init(ctx context.Context) error
	Close(ctx context.Context) error
	Health() error          // 供 /health/ready 自报
	Enabled() bool          // 通常读 go.config.used
}
plugin.Register(p Plugin)  // 注册表
```
v2 保留 `UsePlugin` 兼容 v1 的老插件，所以老代码不用改。

---

## 1. 接入五步法（所有插件通用）

### 第 1 步：`conf/application.yml` 声明

```yaml
go:
  config:
    used: "kafka,nats,mssql"        # ← ① 组件名必须出现在这里
    env: test
    prefix:                          # ← ② 每个组件一个前缀
      kafka: kafka
      nats: nats
      mssql: mssql
```

### 第 2 步：放 `conf/<prefix>-<env>.yml`，根节点 `go.data.<name>`

例 `conf/kafka-test.yml`：
```yaml
go:
  data:
    kafka:
      servers: "127.0.0.1:9092"      # 集群用逗号分隔
      ack: all                       # no|local|all
      auto_commit: true
      partitioner: hash              # hash|random|round-robin
      version: 2.8.1
```

### 第 3 步：main.go 里注册一行

```go
app := mgin.NewApp("conf/application.yml", "myservice", Version, false)
app.MGin.UsePlugin("kafka", mgkafka.Kafka)
```
**没有 `init()` 自动注册，没有 `mgin.RegisterPlugin`。** 必须手动调。

### 第 4 步：直接用包级单例

```go
mgkafka.Kafka.Send("topic", "msg")
```

### 第 5 步：多连接时第一参数是 `tag`，单连接传 `""`

```go
mqtt.MQTT.Publish("", "topic", 0, false, payload)   // 单连接
nats.NATS.Publish("c1", "subject", data)            // 多连接的 c1
```

---

## 2. 配置节点命名规律（★ 最容易写错）

| 插件 | 配置根节点 |
|---|---|
| mgkafka | `go.data.kafka.*` |
| mqtt | `go.data.mqtt.*` |
| nats | `go.data.nats.*` |
| mssql | `go.data.mssql.*` |
| opensearch | `go.data.opensearch.*` |
| hbase | `go.data.hbase.*` |
| hive | `go.data.hive.*` |
| influxdb | `go.data.influxdb.*` |
| **mgrabbit** | **`go.rabbitmq.*`（没有 `data` 这一层，唯一例外）** |

**多连接分两派**：

| 派别 | 开关 | 名字列表 | 子块 | 适用 |
|---|---|---|---|---|
| A | `<节点>.multi: true` | `<节点>.conns: c1,c2` | `<节点>.c1.*` | mqtt / nats / hbase / hive / mgrabbit |
| B | `<节点>.multidb: true` | `<节点>.dbNames: db1,db2` | `<节点>.db1.*` | mssql / opensearch / influxdb（与内置 `go.data.mysql` 同源） |

**新写插件用派别 B**（和框架内置一致）。派别 A 示例：
```yaml
go:
  data:
    nats:
      multi: true
      conns: c1,c2
      c1: { uri: "nats://host1:4222" }
      c2: { uri: "nats://host2:4222" }
```

---

## 3. 插件速查

### 3.1 mgkafka —— `mgkafka.Kafka`

```go
Kafka.Send(topic, data string) error
Kafka.SendMsgs(topic string, data []string) error
Kafka.Subscribe(groupID, topic string, listener func(topic, msg string) error) error
Kafka.MessageListener(groupID, topic string, listener func(topic, msg string) error) error  // Subscribe 别名
Kafka.CreateTopic(topic) error
Kafka.GetProducer() / GetConsumer() / GetConsumerGroup(id) / GetAdminClient()
```
- ⚠️ **唯一没有 `tag` 参数的插件**
- 回调签名是 `func(topic, msg string) error`（字符串，不是字节）
- `Send`/`Subscribe` 会先 `ensureTopic()` 自动建 topic
- 一个 topic 对应一个 groupID

### 3.2 mqtt —— `mqtt.MQTT`

```go
MQTT.Publish(tag, topic string, qos byte, retained bool, payload interface{}) error
MQTT.Subscribe(tag, topic string, qos byte, handler paho.MessageHandler) error
MQTT.SubscribeMultiple(tag string, filters map[string]byte, cb paho.MessageHandler) error
MQTT.UnSubscribe(tag string, topics ...string) error
MQTT.GetConnection(tag ...string)
```
handler 是 paho 风格，需要 import `paho "github.com/eclipse/paho.mqtt.golang"`：
```go
func onOpen(client paho.Client, msg paho.Message) {
	// msg.Topic() / msg.Payload()
}
mqtt.MQTT.Subscribe("", "biz/table/open/#", 1, onOpen)
```
⚠️ yml 里的 `subscribe` / `handlerFuncName` 配置项**代码中无实现**，别用。

### 3.3 nats —— `nats.NATS`

```go
NATS.Publish(tag, topic string, payload []byte) error
NATS.Subscribe(tag, topic string, handler nats.MsgHandler) error          // 广播
NATS.SubscribeQueue(tag, topic, queueName string, handler nats.MsgHandler) error  // 队列，一条只被消费一次
NATS.UnSubscribe(tag string, topics ...string) error
// JetStream
NATS.CreateStream(tag, stream string, subjects []string)
NATS.PublishStream(tag, stream string, data []byte, subject string)
NATS.SubscribeStream(tag, stream, consumer string, handler jetstream.MessageHandler)
```
认证四选一：`user+password` / `token` / `nkeys_seed` / `jwt_creds`。

### 3.4 mgrabbit —— `mgrabbit.Rabbit`

```go
conn, _ := Rabbit.GetConnection(tag ...string)   // ★ 没有包级 Publish
conn.RabbitSendMessage(queueName string, msg string)     // 不返回 error，失败只打日志
conn.RabbitCreateNewQueue(queueName)
conn.RabbitCreateDeadLetterQueue(queueName, toQueueName string, ttl int)
Rabbit.RabbitMessageListener(tag, queueName string, listener func(msg []byte))
```

### 3.5 mssql —— `mssql.MsSQL`

```go
db, err := mssql.MsSQL.GetConnection(dbName ...string)   // 返回 *gorm.DB
// 之后标准 GORM：db.Raw(...).Scan(&x) / db.Where(...).First(&x) / db.Create(&x)
```
配置：`go.data.mssql.dsn` + `.multidb` + `.dbNames` + `.<name>.dsn` + `.debug` + `.pool.{max,total,timeout,lifeTime}`

### 3.6 opensearch —— `opensearch.OpenSearch`

```go
OpenSearch.Index(tag, index, docID string, doc any, opts ...DocOption)
OpenSearch.Get(tag, index, docID string, out any) (bool, error)
OpenSearch.Update(tag, index, docID string, doc any, opts ...)
OpenSearch.Delete(tag, index, docID string, opts ...)
OpenSearch.Search(tag string, indices []string, query any, opts ...SearchOption)
OpenSearch.SearchInto(tag string, indices []string, query any, out any, opts ...) (int64, error)
OpenSearch.Bulk / DeleteByQuery / Count / CreateIndex / DeleteIndex / IndexExists / Refresh / Health / Ping
// 选项: WithRefresh / WithRouting / WithTimeout / WithFrom / WithSize / WithSortBy / WithTrackTotalHits
```

### 3.7 hbase —— `hbase.Hbase`

数据统一用 `map[string]map[string][]byte`（列族 → 列限定符 → 值）：
```go
Hbase.Get(tag, table, rowKey string, opts ...QueryOption) (map[string]map[string][]byte, error)
Hbase.Put(tag, table, rowKey string, values map[string]map[string][]byte) error
Hbase.Delete / Scan / Append / Increment / CheckAndPut
Hbase.CreateTable(tag, table string, families map[string]map[string]string)
// 每个方法都有 ...Context 变体；选项 WithFamilies / WithFilter
```

### 3.8 hive —— `hive.Hive`

```go
Hive.Query(tag, sql string) ([]map[string]interface{}, error)
Hive.Exec(tag, sql string) error
Hive.Columns(tag, sql string) ([][]string, error)
Hive.Cursor(tag string) / Ping / QueryContext / ExecContext
```
结果行 key 是 `表名.列名`（gohive RowMap 语义），NULL 为 `nil`。

### 3.9 influxdb —— `influxdb.InfluxDB`

底层：
```go
InfluxDB.Write(tag, measurement string, tags map[string]string, fields map[string]any, ts time.Time) error
InfluxDB.Query(tag, flux string) (*api.QueryTableResult, error)
InfluxDB.WriteRecord(tag string, lines ...string) error   // line protocol
```
仿 GORM 链式会话：
```go
var rows []CpuMetric
err := influxdb.InfluxDB.Table("cpu").
	Tag("host", "s1").TagIn("region", "cn", "us").
	Fields("usage", "mem").TimeRange(t1, t2).
	OrderByTime(false).Limit(100).
	Find(&rows)
```
结构体标签：`influx:"time"` / `influx:"tag:列名"` / `influx:"field:列名"` / `influx:"measurement:名"`。

---

## 4. MQ 插件 API 对照

| 能力 | kafka | mqtt | nats | rabbit |
|---|---|---|---|---|
| 发送 | `Send(topic, data string)` | `Publish(tag,topic,qos,retained,payload)` | `Publish(tag,topic,[]byte)` | `conn.RabbitSendMessage(q, msg)` |
| 订阅 | `Subscribe(groupID, topic, fn)` | `Subscribe(tag,topic,qos,fn)` | `Subscribe/SubscribeQueue` | `RabbitMessageListener(tag,q,fn)` |
| 消费组 | groupID | ✗ | `SubscribeQueue` 的 queueName | 队列名 |
| 回调签名 | `func(topic, msg string) error` | `paho.MessageHandler` | `nats.MsgHandler` | `func(msg []byte)` |
| tag 首参 | ❌ 无 | ✅ | ✅ | ✅ |
| 多连接 | ❌ | ✅ | ✅ | ✅ |

**只有 mqtt 与 nats 的 API 是同源的**（代码几乎逐行对应）。kafka 是异类，rabbit 最弱。

---

## 5. 自己写一个插件

照这个骨架写即可，框架会自动纳入生命周期管理：

```go
package myplugin

import (
	"github.com/knadh/koanf"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/rawbytes"
	"github.com/sadlil/gologger"
)

var MyPlugin = &myPlugin{}

type myPlugin struct {
	configData []byte
	conf       *koanf.Koanf
}

func (p *myPlugin) Init(configData []byte) {
	p.configData = configData
	p.conf = koanf.New(".")
	if err := p.conf.Load(rawbytes.Provider(configData), yaml.Parser()); err != nil {
		gologger.GetLogger().Error(err.Error())
		return
	}
	// 读 go.data.myplugin.xxx，建连接
	dsn := p.conf.String("go.data.myplugin.dsn")
}

func (p *myPlugin) Close() { /* 释放 */ }
func (p *myPlugin) Check() error { /* 断线重连；每 5 分钟被回调 */ return nil }
```

要点：
- `Init` **不返回 error**（v1 契约），失败只能打日志
- 配置根节点用 `go.data.<name>.*`，多连接用 `multidb` + `dbNames`（派别 B）
- 导出一个包级单例变量名，供 `UsePlugin` 传入
