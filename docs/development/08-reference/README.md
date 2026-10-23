# 参考手册

本目录只回答事实性问题：有哪些符号、签名是什么、默认值多少、协议长什么样。设计动机与最佳实践见 [02-concepts](../02-concepts/) 与 [04-configuration](../04-configuration/)。

## 索引

| 文档 | 内容 | 适合 |
|------|------|------|
| [api/README.md](api/README.md) | 各包职责、全部导出 API、签名、参数与约束 | 集成时查函数 |
| [variables.md](variables.md) | 全局变量、常量、配置键（类型/默认值/作用/约束） | 查配置与状态 |
| [types.md](types.md) | 导出类型定义、字段、嵌套关系 | 查结构体 |
| [protocols.md](protocols.md) | HTTP 响应信封、错误码、Header、各端点协议、调用协议 | 对接前后端/跨服务 |
| [commands.md](commands.md) | 脚手架命令、Makefile、常用运维命令 | 构建与运维 |

## 覆盖范围与约定

- 覆盖包：根包 `mgin`、`config`、`db`（含 `db/dao`）、`cache`、`client`、`registry`、`middleware/*`、`models`/`models/vo`、`i18n`、`errcode`、`logs`、`logsink`、`job`、`health`、`metrics`、`casbin`、`utils`。
- 签名以源码为准；行为中的「当前实现」表示已读代码确认，标注「未实测」的表示未做运行时验证。
- `utils` 是工具函数集合（46 个文件、数百个函数），按类别列出代表签名与通用示例；逐个签名可用 `go doc github.com/maczh/mgin/utils` 查阅。
- 本分支能力边界见 [README §2.4](../../../README.md#24-当前分支未实现的能力)：PostgreSQL、ClickHouse、ElasticSearch、Kafka、S3、系统管理 HTTP 接口均未实现。
- 所有示例的模块路径为 `github.com/maczh/mgin`（`jh` 线，不带 `/v2`）。

## 版本与兼容性矩阵

| 项 | 版本/范围 | 来源/说明 |
|----|-----------|-----------|
| 分支 | `jh`（tag 基线 `v1.25.27-jh`） | 本文档适用 |
| Go | 1.25.7 | `go.mod` 声明；低于该版本会触发 toolchain 下载 |
| gin | v1.11.0 | `go.mod` 锁定 |
| GORM / MySQL 驱动 | v1.31.0 / v1.6.0 | `go.mod` 锁定 |
| PostgreSQL / ClickHouse | — | **不支持** |
| MongoDB 驱动 | v1.17.4 | 建议 MongoDB 4.4+；示例用 6.x |
| go-redis | v7.4.1 | 建议 Redis 6/7；`database=-1` 强制 DB0 |
| SQLite 驱动 | `glebarez/sqlite` v1.7.0 | 纯 Go，无需 CGO |
| Casbin / adapter | v2.105.0 / gorm-adapter v3.32.0 | 需 MySQL |
| Prometheus client | v1.19.1 | `/metrics` 文本格式 |
| koanf | v1.4.3 | YAML 配置 |
| cron | v3.0.1 | 支持 5/6 段与 `@every` 等 |
| JWT | `dgrijalva/jwt-go` v3.2.0 | 已停止维护，仅 HMAC |
| 注册中心 | Nacos 2.x / Consul 1.x / etcd 3.5 / Polaris | 按对应客户端 API；未逐版本做兼容测试 |
| 容器/K8s | 任意支持 HTTP 探针的版本 | 部署示例见 [部署](../07-deployment/01-deployment.md) |

「未逐版本验证」表示代码未做跨版本兼容测试，升级外部系统前应在预发回归探针、注册发现与调用协议。
