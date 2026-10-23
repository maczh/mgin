---
name: mgin-dev-skill
description: 基于 github.com/maczh/mgin jh/v1 源码与项目文档，开发、审查、排障和扩展 MGIN 微服务，包括启动配置、MVC/DAO、服务发现调用、治理容错、可观测性、消息和脚手架。用户明确指定 MGIN 或维护其依赖时使用；不要把 v2 API 或业务服务约定泛化到 v1。
---

# MGIN 微服务开发

以本地框架源码为 API 依据，以 `docs/development/` 的适用章节为行为与配置契约，再以目标项目及最接近的寄海服务为工程范例。完成可编译、保持既有接口契约的实现；不要把某一个服务的业务规则推广为框架规则。文档描述可能滞后于源码，冲突时以当前目标 `go.mod`、源码、测试和实际依赖解析结果为准，并在交付中指出冲突。

## 先确定上下文

- 框架基准：`/home/Macro/work/go/src/github.com/maczh/mgin`。
- 实际范例根目录：`/home/Macro/work/go/src/jihaihotpot.com/jihai`。
- 先读目标目录及其上级适用的 `AGENTS.md`，再读 `go.mod`、`go.work`（若有）、`main.go` 和相关构建目标。核对 module、Go 版本、依赖版本、replace 与实际 import。
- 按任务读取 [references/project-docs.md](references/project-docs.md) 的文档路由；不要默认把 `docs/` 全量塞入上下文。架构、配置、治理、服务调用、测试、部署/Runbook、迁移章节分别按需核对。
- 以上路径是本机默认位置；若不存在，先在当前工作区定位，不要凭空认定 API 不存在。
- 区分 `github.com/maczh/mgin` 与 `github.com/maczh/mgin/v2`。本地框架根目录是前者，不能据此推断 v2 API。目标锁定版本优先；本地源码与依赖不同，以实际解析的依赖源码确认签名，不能静默升级或添加本地 replace。
- 只阅读相关代码及配置键；不把真实凭据、客户数据或完整运行配置复制进技能、模板、日志或输出。

按需阅读，不要默认加载全部参考：

| 当前任务 | 参考 |
|---|---|
| 选择范例、确认版本、寻找真实实现 | [references/source-map.md](references/source-map.md) |
| 启动、配置、插件、框架 API、v1/v2 边界 | [references/framework.md](references/framework.md) |
| 接口、请求响应、DAO、分页、模型与验证 | [references/service-patterns.md](references/service-patterns.md) |
| 微服务调用、MQ、定时任务及故障排查 | [references/integrations.md](references/integrations.md) |
| 发现/负载均衡、配置键、演进和文档/迁移边界 | [references/project-docs.md](references/project-docs.md) |

## 开发约定

1. 沿目标项目完成调用链：`router → controller → service → DAO / ms / mq`。路由绑定与输出；controller 绑定、校验、提取鉴权上下文；service 负责业务。遵循目标项目现有的单例和 `enter.go` 汇总方式，不为小改动重构全项目。
2. 复用目标的 `ro` / `request`、`vo` 和公共模型。`vo.Response[T]` 的 `code` 与框架 `models.Result[T]` 的 `status` 是不同协议，禁止直接替换。HTTP 状态与业务码分别检查；多数范例 HTTP 200 承载业务错误。
3. 输入字段、路径、鉴权、业务状态、分页和输出 JSON 与现有调用方兼容。不得把订单服务的端前缀、鉴权放行规则或成功码当成所有服务的固定值。
4. 使用项目 DAO 和服务调用封装；SQL 条件参数化，检查错误、未命中及零值语义。新增模型同步 DAO 声明及迁移入口。涉及事务时确认所有操作真正使用同一事务连接。
5. 金额与状态沿用领域模型；订单项目使用整数分和状态常量。消息复用既有信封、主题及消费者约定；对新增重试与副作用确认幂等性和终止条件。
6. 用 `logs` 的 `{}` 占位符，避免照搬旧代码中的完整 token、请求体日志或忽略错误。业务错误返回明确且不泄露内部细节的信息。
7. v1 全局单例（`config.Config`、`db.*`、`registry.Registry`）意味着一个进程一个 App；热重载串行且只重建资源，不重建路由、中间件或端口。测试必须隔离/恢复全局状态。
8. 服务调用必须区分传输成功与业务成功：HTTP 200 不等于 `status/code` 成功；`Retry` 只执行一次且只对幂等操作启用；超时、熔断、fallback、发现缓存会叠加，需说明组合语义。
9. 服务发现保留旧 `RegistryClient` 必需接口，扩展能力用类型断言；缓存、watcher、熔断器是进程内能力，不宣称跨副本一致。真实 watch 只有后端支持时才可使用，轮询要说明延迟与取消。

## 新建服务

先确定 module、端口、mgin 主版本及确实需要的存储、注册中心和消息插件；可从邻近同版本服务裁剪或核对 `cmd/` 脚手架。保留项目构建方式，使用占位配置；不要复制生产配置。最小结果应包含入口、选定配置、完整接口调用链及所需依赖。无需的数据库、MQ、内置管理接口不默认启用。

## 验证与交付

- `gofmt` 修改的 Go 文件；针对业务变化验证正常、非法输入、未命中及相关失败路径，状态或金额修改补充有意义的边界测试。
- 先读测试初始化：`go test` 也可能连接数据库、发消息或付款。确认隔离后执行相关测试，再按项目要求构建。不要为了检查编译直接启动服务；`NewApp` 就可能连库、注册和启动任务。
- 阅读 Makefile 后选择构建命令：某些 `make build` 会先 `go mod tidy`，`make linux` 还依赖 upx。仅在依赖确实变化时整理模块，不通过全局 `go env -w` 改开发环境。
- 对框架/治理改动至少覆盖：正常、空结果、超时、依赖不可用、重复调用、并发、关闭/重载和旧接口兼容路径；涉及 goroutine、ticker、watch 或连接池时检查取消、幂等关闭和 race。
- Swagger 源注释与实际路由一致；按项目命令生成文档，避免手改生成文件。保留 `version.go` 的 ldflags 注入机制。
- 交付时说明实现、验证结果及具体未验证依赖；不能把未执行的联调或编译称为通过。

