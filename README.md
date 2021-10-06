# MGin 微服务框架范例

    MGin微服务框架范例，用于快速创建基于MGin微服务框架的RESTful微服务程序，本范例程序演示了
    如何使用Mysql/MongoDB/Redis进行增改查操作，作为RESTful API接口提供给其他服务使用。
    在本范例中，采用Nacos作为配置中心和注册服务与发现中心，Mysql、MongoDB和Redis的配置都
    放在Nacos中，都采用了连接池配置，接口请求日志保存到MongoDB中，采用在Http header中
    X-RequestId作为链路跟踪。
    
## 创建MGin新工程的方式

+ 首先采用Git命令行下载这个范例项目，假设自己的新工程名称为 test
    
```shell script
    git clone https://github.com/maczh/mgin.git test
```
    
+ 修改 mgin.yml 文件名为 test.yml
    
```shell script
    cd test
    mv mgin.yml test.yml
```
    
+ 修改 main.go 中指定的配置文件名
    
```go
    const config_file = "test.yml"
```
    
+ 修改 test.yml 配置中的项目名称、端口和nacos服务器的IP和端口
    
```yaml
    go:
      application:
        name: test
        port: 8080
      config:
        server: http://192.168.1.2:8848/
```
    
+ 修改自己项目中要使用到的数据库，比如只用到MySQL,那就去掉go.config.used中的mongodb和redis，如果要使用接口日志，即 go.log.req的值不为空，那么必须加上mongodb的配置，如:
    
```yaml
    go:
      config:
        used: mysql,nacos,mongodb
```
    
+ 在Nacos的配置中心中要有以相应配置，如MySQL的配置 
    mysql-go-test.yml
```yaml
    go:
      data:
        mysql: testuser:**********@tcp(192.168.1.3:3306)/test?charset=utf8&parseTime=True&loc=Local
        mysql_pool:
          min: 5
          max: 20
          idle: 10
          timeout: 300
```

mongodb-go-test.yml
```yaml
    go:
      data:
        mongodb:
          uri: mongodb://testuser:**********@192.168.1.4:27017/test
          db: test
        mongo_pool:
          min: 2
          max: 20
          idle: 10
          timeout: 300 
```

redis-go-test.yml
```yaml
    go:
      data:
        redis:
          host: 192.168.1.5
          port: 6379
          password: *************
          database: 1
          timeout: 1000
        redis_pool:
          min: 3
          max: 20
          idle: 10
          timeout: 300
```

nacos-go-test.yml
```yaml
    go:
      nacos:
        server: 192.168.1.2
        port: 8848
        clusterName: DEFAULT
        weight: 1
```
    
## MGin工程的层次结构为标准MVC结构

+ model 
  数据模型，存放各种对象结构体，如表对象、接口入参对象、出参对象等
+ mysql
  mysql的表操作DAO层，负责实现mysql表的增删改查，采用的是gorm v1.0
+ mongo
  mongodb表的操作DAO层，负责实现mongodb表的境删改查，采用的是Mgo v2
+ redis
  redis操作层
+ service
  业务实现层，从controller层传入参数，调用mysql/mongodb/redis等数据库层进行业务逻辑实现
+ controller
  api接口控制层，负责从http reguest传入的map参数转换成service层调用的参数，调用service层函数实现api接口逻辑
+ Router.go
  http 路由配置，将http的url请求路由指向调用controller层中的函数,
  如:
  ```go
	engine.Any("/user/mysql/save", func(c *gin.Context) {
		result = controller.SaveUserMysql(utils.GinParamMap(c))
		c.JSON(http.StatusOK, result)
	})
  ``` 
