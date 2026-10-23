# 版本迁移与已知问题

## 1. 版本线对照

| 项 | `jh`（本文档） | `master` |
|----|----------------|----------|
| 模块路径 | `github.com/maczh/mgin` | `github.com/maczh/mgin/v2` |
| 最新 tag（参考） | `v1.25.27-jh` | `v2.0.x` |
| 目录布局 | 根目录 `db/`、`config/` 等 | `pkg/` 下分层 |
| 脚手架 | `cmd/mgin`（独立 module） | 同名，参数与模板不同 |
| 文档 | `docs/development/` | 各自维护 |

两条线**不能**互相 `replace` 或混用依赖；`go get` 报「module path mismatch」通常是选错了线。

## 2. 从旧代码/旧文档迁移到 jh

### 2.1 依赖与导入

```diff
- require github.com/maczh/mgin/v2 v2.0.11
+ require github.com/maczh/mgin v1.25.27-jh
```

```diff
- "github.com/maczh/mgin/v2/pkg/db"
+ "github.com/maczh/mgin/db"
```

### 2.2 已移除的能力 / API

| 旧写法（README_CN / master） | jh 现状 | 替代 |
|------------------------------|---------|------|
| `db.Pg` / `PostgresDao` | 不存在 | 无（选 MySQL 或自行实现） |
| `db.Clickhouse` / `ClickhouseDao` | 不存在 | 无 |
| `db.ElasticSearch` / `db.Kafka` | 不存在 | 无（日志用 VictoriaLogs/自定义 handler） |
| `storage/s3`、`go.s3` | 不存在 | 业务自行接入 SDK |
| `db.Mysql.UseCache()` | 不存在 | `dao.MySQLDao.WithCache(dao.CacheConfig{...})` |
| 外部 `x-lang` 服务 | 已删除 | 内置 `i18n`（`X-Lang` 头 + 数据库字典） |
| `go.sys` 管理后台接口 | 仅模型，无 HTTP | 业务实现，或使用 master |
| `client` 内加权随机 | 已移除 | 注册中心侧分组/权重 |

### 2.3 配置键变更

| 旧键 | 新键 | 说明 |
|------|------|------|
| `go.config.reloadToken` | `go.config.reload.token` | 嵌套结构；旧键不会生效 |
| `go.application.portSSL` | `go.application.port_ssl` | 代码读取下划线形式 |
| `go.application.ipAddr` | `go.application.ip` | — |
| `go.log.request` | `go.log.req` | 结构体 json 名与读取键不同，按读取键配置 |
| `go.log.logDb` | `go.log.db` | 读取键为 `go.log.db` |

> 判断某个键是否真的生效：用 `config.Config.GetConfigXxx("键")` 打印，而不是只看 YAML 是否有该行。

### 2.4 行为差异

| 项 | 旧行为 | 当前行为 |
|----|--------|----------|
| 404 响应 | 可能 HTTP 404 | HTTP 200 + `status=1000` |
| panic 响应 | HTTP 500 | HTTP 200 + `status=1001` |
| 限流拒绝 | 可能 429 | 默认 200 + `status=1011`（可配 `httpStatus`） |
| 日志占位符 | `%s`/`%v` | `{}` 顺序替换 |
| i18n 初始化失败 | 降级 | `panic` |
| Redis 健康检查 | 可能真实检查 | 空实现，恒成功 |
| `/metrics` 配置 | 可能与代码不一致 | `go.metrics.enabled` 或 `EnableMetrics()` 均可（已验证） |

### 2.5 迁移步骤

1. 改 module 与 import 路径，`go mod tidy`。
2. 全局搜索上表旧符号并替换。
3. 按 §2.3 修正配置键，补齐 `used`/`prefix`。
4. 修正日志占位符与响应构造（统一用 `i18n.*`）。
5. 跑 `go build ./...`、单元测试与探针冒烟。
6. 对照 [能力边界](../../../README.md#24-当前分支未实现的能力) 删除对未实现能力的依赖。

## 3. 已知问题（当前代码确认）

| # | 问题 | 影响 | 规避 |
|---|------|------|------|
| 1 | `db.Redis.Check()` 恒返回 nil | Redis 故障不影响 `/health/ready` 与自检 | 外部监控 Redis |
| 2 | `client` 无超时 | 下游不响应会长期占用协程 | 业务侧 context/超时兜底 |
| 3 | `MySQLDao.WithContext` 修改接收者 | 并发复用 DAO 会串 ctx | 每次新建或复制 DAO |
| 4 | `MySQLDao.Updates(entity)` 不带 WHERE | 可能全表更新/GORM 报错 | 用 `DB()/Where(...).Updates(...)` |
| 5 | `RedisCache.Clear()=FLUSHDB`、`Range` 全库 SCAN | 与业务共用 DB 会误删/变慢 | 独立 DB、不要用 Range |
| 6 | `WithCache` TTL=0 永不过期 | 缓存永不失效 | 显式设置 TTL |
| 7 | `Dao[E]` 接口只声明 `Insert` | 只有 `MgoDao` 满足接口 | 使用具体 DAO 类型 |
| 8 | Polaris 不自动注册/注销 | 需手动 `Register` | 启动时显式调用 |
| 9 | 接口日志仅支持 MongoDB | 无 Mongo 时日志不落库 | 自定义 `logsink` handler |
| 10 | `dgrijalva/jwt-go` 已停止维护 | 安全风险 | 逐步迁移到 `golang-jwt/jwt` |
| 11 | `xss` 不过滤请求头 | 头部注入风险 | 网关/业务层校验 |
| 12 | `utils.CheckSqlValidate` 是黑名单 | 不能防注入 | 参数绑定 |
| 13 | 脚手架 `--db postgres|clickhouse` | 生成不可编译代码 | 用 `mysql` 或自行实现 |
| 14 | `.Trash-0/` 被 git 跟踪 | 干扰 `gofmt -l .` | 提交时排除，建议后续移除 |
| 15 | 无 CI、无 Makefile（框架仓库） | 质量依赖本地执行 | 按 [测试策略](../10-contributing/03-testing-strategy.md) 手动验证 |

## 4. 升级检查清单

- [ ] 阅读本次版本的 CHANGELOG（若有）与破坏性变更
- [ ] `go build ./...` 与 `cd cmd && go build ./...`
- [ ] 配置键对照 §2.3，特别是 `reload.token`
- [ ] 旧 API 全局搜索：`UseCache`、`db.Pg`、`db.Clickhouse`、`x-lang`
- [ ] 探针与 `/metrics` 冒烟
- [ ] 接口返回体兼容（404/panic/限流的状态码）
- [ ] 日志占位符与字段白名单
- [ ] 回滚方案：保留上一版本二进制与配置

## 5. 历史文档说明

`README_CN.md`、`README_en.md`、`sdk_document.md` 包含历史/上游内容，其中 PostgreSQL、ClickHouse、ElasticSearch、Kafka、S3、系统管理模块的章节在本分支无实现。两份 README 顶部已加提示；遇到冲突时以 `docs/development/` 与代码为准，并欢迎提 PR 修正。
