# MGin 微服务框架

MGin微服务框架，用于快速创建基于MGin微服务框架的RESTful微服务程序

## MGin框架功能

### Web服务框架

- Gin

### 支持统一的配置中心

- Nacos
- Consul
- SpringCloud Config

### 支持的服务发现与注册中心

- Nacos
- Consul (计划)
- Etcd (计划)

### 内置支持自动连接的数据库

- MySQL (GORM v2)
- MongoDB (mgo v2)
- Redis (go-redis)
- ElasitcSearch (olivere/elastic)
- Kafka  
- 其他各类的数据库、消息队列等计划与中间件模式实现自动加载
- Mysql/mongodb/redis已经支持多库连接

### 支持通过插件自动加载外部数据库、消息队列模块

```go
func (m *mgin) Use(dbConfigName string, dbInit dbInitFunc, dbClose dbCloseFunc, dbCheck dbCheckFunc)
// 范例
import "github.com/maczh/mgrabbit"
...
//加载RabbitMQ消息队列
mgin.MGin.Use("rabbitmq", mgrabbit.Rabbit.Init, mgrabbit.Rabbit.Close, nil)
```

### 支持的接口协议

- http
- https
- gRPC (计划)

### 支持的接口参数规范

- x-form  (x-form-urlencoded)
- json (url query string + json body, POST + GET)
- restful

## 安装
```shell script
go get -u github.com/maczh/mgin
```

## 使用方法

### 本地配置文件

+ 默认文件名为`模块名.yml`，可自定义名称，配置内容如下
```yaml
go:
  application:
    name: myapp         #应用名称,用于自动注册微服务时的服务名
    port: 8080          #端口号
    project: myproj     #所属项目名称
    port_ssl:           #https端口号
    cert:               #ssl证书文件地址
    key:                #ssl证书私钥文件地址
    debug:              #本地调试模式，可注册到nacos，可调用其他微服务，调试实例不可被其他实例调用
    ip: xxx.xxx.xxx.xxx  #微服务注册时登记的本地IP，不配可自动获取，如需指定外网IP或Docker之外的IP时配置
  discovery:                      
    registry: nacos                    #微服务的服务发现与注册中心类型 nacos,consul,默认是 nacos
    callType: json                     #微服务调用参数模式 x-form,json,restful 三种模式可选
  config:                               #统一配置服务器相关
    server: http://192.168.1.5:8848/    #配置服务器地址
    server_type: nacos                  #配置服务器类型 nacos,consul,springconfig
    env: test                           #配置环境 一般常用test/prod/dev等，跟相应配置文件匹配
    used: nacos,mysql,mongodb,redis,kafka     #当前应用启用的配置
    prefix:                             #配置文件名前缀定义
      mysql: mysql                      #mysql对应的配置文件名前缀，如当前配置中对应的配置文件名为 mysql-test.yml
      mongodb: mongodb
      redis: redis
      nacos: nacos
      elasticsearch: elasticsearch
      kafka: kafka
  logger:                 #控制台日志与文件日志输出，logs包的输出
    level: debug
    out: console,file          #日志输出到控制台与文件
    file: /opt/logs/myapp      #日志文件路径与前缀，后面自动加上.yyyy-MM-dd.log，目录必须已创建
  log:                    #controller接口访问日志与微服务调用请求日志
    db: mongodb           #日志库，支持mongodb与elasticsearch
    req: MyappRequestLog  #接口访问日志表名称，在es中使用工程名称${go.application.project}_${go.log.req}作为索引名
    call: MyappCallLog    #微服务调用日志表，表名规则同上
    kafka:
      use: true           #接口日志是否发送到kafka
      topic: myapp        #kafka消息主题,支持多个topic，以逗号分隔
```
+ mysql配置范例 mysql-test.yml
```yaml
go:
  data:
    mysql: user:pwd@tcp(xxx.xxx.xxx.xxx:3306)/dbname?charset=utf8&parseTime=True&loc=Local
    mysql_debug: true   #打开调试模式
    mysql_pool:     #连接池设置,若无此项则使用单一长连接
      max: 200      #实际最大连接数
      total: 1000   #最大并发数,不填默认为最大连接数5倍
      timeout: 30   #空闲连接超时，秒，默认60秒
      life: 5       #连接生命周期，分钟，默认60分钟
```
+ mysql多库连接配置范例 mysql-multidb-test.yml
```yaml
go:
  data:
    mysql: 
      multidb: true
      dbNames: test1,test2
      test1: user1:pwd1@tcp(xxx.xxx.xxx.xxx:3306)/dbname1?charset=utf8&parseTime=True&loc=Local
      test2: user2:pwd2@tcp(xxx.xxx.xxx.xxx:3306)/dbname2?charset=utf8&parseTime=True&loc=Local
    mysql_debug: true   #打开调试模式
    mysql_pool:     #连接池设置,若无此项则使用单一长连接
      max: 200      #实际最大连接数
      total: 1000   #最大并发数,不填默认为最大连接数5倍
      timeout: 30   #空闲连接超时，秒，默认60秒
      life: 5       #连接生命周期，分钟，默认60分钟
```


+ mongodb配置范例 mongodb-test.yml
```yaml
go:
  data:
    mongodb:
      uri: mongodb://user:pwd@xxx.xxx.xxx.xxx:port/dbname #当使用复制集时 mongodb://user:pwd@192.168..3.5:27017,192.168.3.6:27017/dbname?replicaSet=replsetname
      db: dbname
      debug: true   #打开调试模式
    mongo_pool:     #连接池设置,若无此项则使用单一长连接
      max: 20       #最大连接数
```

+ mongodb多库连接配置范例 mongodb-multidb-test.yml
```yaml
go:
  data:
    mongodb:
      multidb: true
      dbNames: test1,test2
      test1:
          uri: mongodb://user1:pwd1@xxx.xxx.xxx.xxx:port/dbname1 #当使用复制集时 mongodb://user:pwd@192.168..3.5:27017,192.168.3.6:27017/dbname?replicaSet=replsetname
          db: dbname1
      test2:
        uri: mongodb://user2:pwd2@xxx.xxx.xxx.xxx:port/dbname2 #当使用复制集时 mongodb://user:pwd@192.168..3.5:27017,192.168.3.6:27017/dbname?replicaSet=replsetname
        db: dbname2
      debug: true   #打开调试模式
    mongo_pool:     #连接池设置,若无此项则使用单一长连接
      max: 20       #最大连接数
```


+ redis配置范例 redis-test.yml
```yaml
go:
  data:
    redis:
      host: xxx.xxx.xxx.xxx
      port: 6379
      password: password
      database: 1
      timeout: 1000
    redis_pool:
      min: 3        #最小空闲连接数,默认2
      max: 200      #连接池大小，最小默认10
      idle: 10      #空闲超时，分钟,默认5分钟
      timeout: 300  #连接超时，秒，默认60秒
```

+ redis多库连接配置范例 redis-multidb-test.yml
```yaml
go:
  data:
    redis:
      multidb: true
      dbNames: test1,test2
      test1:
          host: xxx.xxx.xxx.xxx
          port: 6379
          password: password
          database: 1
      test2:
        host: xxx.xxx.xxx.xxx
        port: 6379
        password: password
        database: 2
    redis_pool:
      min: 3        #最小空闲连接数,默认2
      max: 200      #连接池大小，最小默认10
      idle: 10      #空闲超时，分钟,默认5分钟
      timeout: 300  #连接超时，秒，默认60秒
```

+ nacos配置范例 nacos-test.yml
```yaml
go:
  nacos:
    server: xxx.xxx.xxx   #nacos服务IP
    port: 8848            #nacos端口
    clusterName: DEFAULT
    group: OpenApi    #根据项目不同配置不同分组
    weight: 1
    lan: true   #以内网地址注册，否则以公网地址注册
    lanNet: 192.168.3.    #网段前缀
```

+ Elasticsearch配置范例 elasticsearch-test.yml
```yaml
go:
  elasticsearch:
    uri: http://xxx.xxx.xxx.xxx:9200
    user: elastic
    password: ***********
```

+ Kafka连接配置范例 Kafka-test.yml
```yaml
go:
  data:
    kafka:
      servers: "xxx.xxx.xxx.xxx:9092,xxx.xxx.xxx.xxx:9092"   #集群多个服务器之间用逗号分隔
      ack: all    #ack模式 no,local,all
      auto_commit: true   #是否自动提交
      partitioner: hash   #分区选择模式 hash,random,round-robin
      version: 2.8.1    #kafka版本
```

#### kafka发送消息
```go
    db.Kafka.Send("my_topic", "测试消息")
```

#### kafka侦听主题消息并处理

- 定义消息处理函数
```go
func handleMsg(msg string) error {
	logs.Debug("收到Kafka消息:{}",msg)
	return nil
}

```

- 在main.go中添加侦听代码
```go
	//侦听kafka消息，说明，一个topic对应一个groupId
	err := db.Kafka.MessageListener("my_group_id","my_topic",handleMsg)
	if err != nil {
		logs.Error("侦听kafka消息失败")
	}
```

### 微服务工程范例

* 服务端参见 examples/mgin-server项目
* 客户端参见 examples/mgin-client项目

### 版本更新
- v1.19.1 Result实现any与泛型T互转函数
- v1.19.0 支持go 1.19，Result改用泛型,重构client.Call函数，支持泛型返回
