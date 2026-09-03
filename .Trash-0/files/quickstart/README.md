# quickstart

基于 [MGin](https://github.com/maczh/mgin) 微服务框架生成的项目骨架。

## 目录结构

```
main.go            程序入口
conf/              配置文件 (application.yml + 各组件 <prefix>-<env>.yml)
model/             数据模型 (GORM)
service/           业务逻辑
controller/        HTTP 处理函数
router/            路由与中间件注册
```

## 生成时选择的配置

- 模块路径: `github.com/maczh/mgin/examples/quickstart`
- 端口: `18096`
- 数据库(used): `sqlite`
- 消息队列: `none`
- 注册中心: `none`
- 配置中心: `none`
- JWT: `false`  Casbin: `false`  i18n: `false`

## 运行

```bash
go mod tidy
go build -o quickstart .
./quickstart              # 读取 conf/application.yml
./quickstart -f conf/application.yml
./quickstart -v           # 查看版本
```

> 提示: 启用数据库/消息队列/注册中心等组件后, 需先启动对应中间件, 否则框架会在启动时打印连接错误(不影响进程启动)。
