# mgin v2 包归属表（v2-package-map）

本文件按"对外可导入 / 框架内部 / 根入口"三档列出 v2 全部包及其用途。设计依据见 `docs/v2-design.md` §2.2。

## 1. 根入口（不进入 pkg/，保持向后兼容）

| 路径 | 用途 | 维护承诺 |
|------|------|----------|
| `mgin.go` | 框架统一初始化入口，导出 `Init / Use / UsePlugin / MginPlugin / Mgin` | 长期稳定 |
| `app.go` | 应用生命周期，导出 `NewApp / App.Run / App.MarkHealthStarted / App.EnableHealth` | 长期稳定 |

## 2. pkg/ — 对外可导入

业务代码与第三方扩展**唯一**可依赖的路径。任何对外能力的扩展均在此目录下。

| 包 | 用途 | 关键导出 |
|----|------|----------|
| `pkg/cache` | 缓存抽象（memcache / diskcache / bitcask） | `icache`、`memcache`、`diskcache` |
| `pkg/casbin` | RBAC 适配 | `casbin.Engine` |
| `pkg/client` | 服务间 HTTP 调用 | `Call` / `CallCtx` / `CallResilient` / `CallResilientCtx` / `GetBreaker` |
| `pkg/config` | 配置中心 | `Config` / `GetConfig*` / `GetShutdownTimeout` |
| `pkg/db` | 数据源门面 + 各子包 | `db.Mysql/Pg/...` |
| `pkg/db/dao` | 泛型 DAO | `Dao[E]` / `MySQLDao[E]` / `PostgresDao[E]` / `ClickhouseDao[E]` / `MgoDao[E]` |
| `pkg/errcode` | 错误码常量 | `URI_NOT_FOUND` / `SystemError` / … |
| `pkg/health` | 健康探针 | `Router` / `MarkStarted` / `IsStarted` |
| `pkg/i18n` | 国际化 | `Error` / `String` / `Format` / `Success` |
| `pkg/job` | 定时任务（类 xxl-job） | `Start` / `Stop` / `Register` |
| `pkg/logs` | 日志门面 | `Info` / `Debug` / `Warn` / `Error` |
| `pkg/middleware` | HTTP 中间件 | `cors / iplimit / jwt / limit / postlog / ratelimit / session / trace / xlang / xss` |
| `pkg/models` | 通用响应模型 | `Result` / `ResultPage` |
| `pkg/models/sys` | 内置系统管理 | 11 张表 + request/vo |
| `pkg/plugin` | **v2 新增**统一插件契约 | `Plugin` / `Register` / `InitAll` / `CloseAll` / `HealthAll` |
| `pkg/registry` | 注册中心 | `RegistryClient` / `NewRegistry` |
| `pkg/storage/s3` | S3 对象存储 | `NewS3` / `GetS3` |
| `pkg/utils` | 通用工具 | 47 个工具函数 |

## 3. internal/ — 框架内部（暂为空）

v2.0 暂未使用，但保留为未来"框架实现细节"的存放位置。**业务代码禁止 import internal/ 下的任何包**（Go 工具链会强制拒绝）。

## 4. cmd/ — 脚手架生成器（顶层，与框架入口同级）

`cmd/scaffold.go` / `cmd/templates.go` / `cmd/new.go` 是 `mgin new` 命令的入口。
v2 生成的工程模板 import 已全部改为 `pkg/...`。

## 5. docs/ — 文档

| 文件 | 用途 |
|------|------|
| `mgin-architecture-options.md` | 6 套架构方案对比、决策流程图、演进路线图 |
| `v2-prd.md` | v2 重构增量 PRD |
| `v2-design.md` | v2 重构架构设计 |
| `v2-tasks.md` | 27 个实施任务 |
| `v2-package-map.md` | 本文件，包归属表 |
| `migration-v1-to-v2.md` | 旧 import 路径迁移指引 |

## 6. logs/ — 顶层别名壳（**下个 release 删除**）

`logs/alias.go` 是 `pkg/logs` 的 re-export 别名壳，仅为存量项目平滑升级。
新代码请直接 `import "github.com/maczh/mgin/v2/pkg/logs"`。
