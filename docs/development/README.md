# MGin v2 开发者文档

> 模块：`github.com/maczh/mgin/v2` · Go 1.25.7 · 文档版本 v2.0
>
> 面向：第一次接触 MGin 的业务开发者、需要集成的架构师、负责上线与排障的运维/SRE。
> 所有示例与签名均以当前仓库代码为准；文档与代码冲突时以代码为准并提 PR 修正。

---

## 1. 这套文档怎么用

文档分为四类，混读容易迷失，请按目的选择：

| 你想做的事 | 去哪读 | 类型 |
|-----------|--------|------|
| 5 分钟跑起来 | [01 · 快速上手](01-getting-started/02-quickstart.md) | Tutorial |
| 写业务接口、调下游服务 | [03 · 开发指南](03-development/02-service-development.md) | How-to |
| 理解框架为什么这样设计 | [02 · 核心概念](02-concepts/01-architecture.md) | Explanation |
| 查某个函数/配置键 | [08 · 参考手册](08-reference/README.md) | Reference |
| 上线、扩容、排障 | [07 · 部署与运维](07-deployment/01-deployment.md) | How-to |
| 参与框架开发 | [10 · 贡献指南](10-contributing/01-contributing.md) | Process |

**不要**从头线性通读：50 篇文档里只有 5 篇是每个人必读的（见下方角色路径）。

---

## 2. 目录

```
docs/development/
├── 01-getting-started/     快速上手：环境、第一个服务、第一个调用方、学习路径
├── 02-concepts/            核心概念：架构、抽象、设计决策
├── 03-development/         开发指南：项目结构、服务实现、调用、协议、上下文、测试
├── 04-configuration/       配置手册：模型、方式、核心配置语义、加载刷新、诊断
├── 05-governance/          服务治理：发现与负载均衡、流量与容错、安全
├── 06-observability/       可观测性：指标、追踪、日志
├── 07-deployment/          部署运维：形态、生产清单与生命周期、Runbook
├── 08-reference/           参考手册：API、变量、配置项、协议、CLI、兼容性、示例
├── 09-advanced/            高级主题：扩展点、源码走读、性能调优
├── 10-contributing/        贡献指南：流程、规范、测试、发布
└── 11-appendix/            附录：FAQ、术语表、迁移与已知问题
```

### 01 · 快速上手

| 文档 | 内容 |
|------|------|
| [01-introduction.md](01-getting-started/01-introduction.md) | 框架定位、适用/不适用场景、与同类框架差异 |
| [02-quickstart.md](01-getting-started/02-quickstart.md) | 环境准备 → 第一个服务 → 验证 → 第一个调用方 |
| [03-learning-paths.md](01-getting-started/03-learning-paths.md) | 业务开发者 / 架构师 / 运维 三条阅读路径 |

### 02 · 核心概念

| 文档 | 内容 |
|------|------|
| [01-architecture.md](02-concepts/01-architecture.md) | 分层、组件拓扑、启动与关闭时序、请求流转 |
| [02-abstractions.md](02-concepts/02-abstractions.md) | Plugin、Registry、LoadBalancer、DAO、Middleware、Definition |
| [03-design-decisions.md](02-concepts/03-design-decisions.md) | 7 个关键决策与代价、与网关/Service Mesh 的边界 |

### 03 · 开发指南

| 文档 | 内容 |
|------|------|
| [01-project-layout.md](03-development/01-project-layout.md) | 标准目录、module 组织、依赖管理 |
| [02-service-development.md](03-development/02-service-development.md) | 路由、控制器、服务层、DAO、错误响应、生命周期 |
| [03-service-calls.md](03-development/03-service-calls.md) | 服务间调用、协议、上下文透传、文件上传 |
| [04-testing.md](03-development/04-testing.md) | 单元/集成/契约测试与可测试性设计 |

### 04 · 配置手册

| 文档 | 内容 |
|------|------|
| [01-config-model.md](04-configuration/01-config-model.md) | 层级、来源、优先级、命名空间 |
| [02-config-guide.md](04-configuration/02-config-guide.md) | 配置方式、核心配置语义、加载/刷新、错误诊断 |

### 05 · 服务治理

| 文档 | 内容 |
|------|------|
| [01-discovery-and-lb.md](05-governance/01-discovery-and-lb.md) | 注册、发现、缓存、四种负载均衡 |
| [02-traffic-and-resilience.md](05-governance/02-traffic-and-resilience.md) | 治理决策链、超时、重试、熔断、限流、灰度现状 |
| [03-security.md](05-governance/03-security.md) | JWT、Casbin、会话、XSS、IP/并发防护 |

### 06 · 可观测性

| 文档 | 内容 |
|------|------|
| [01-metrics.md](06-observability/01-metrics.md) | 6 个指标、采集、告警建议 |
| [02-tracing.md](06-observability/02-tracing.md) | traceparent、trace id、OTel 接入 |
| [03-logging.md](06-observability/03-logging.md) | 运行日志、访问日志、日志投递与脱敏 |

### 07 · 部署与运维

| 文档 | 内容 |
|------|------|
| [01-deployment.md](07-deployment/01-deployment.md) | 单机/Docker/K8s、探针、多副本约束 |
| [02-operations.md](07-deployment/02-operations.md) | 生产就绪清单、优雅停机、容量与调优 |
| [03-runbook.md](07-deployment/03-runbook.md) | 按症状检索的故障处置手册 |

### 08 · 参考手册

| 文档 | 内容 |
|------|------|
| [README.md](08-reference/README.md) | 参考手册索引与阅读约定 |
| [variables.md](08-reference/variables.md) | 全部全局变量/单例：类型、默认值、作用、约束 |
| [configuration.md](08-reference/configuration.md) | 全部配置键：类型、默认值、生效条件 |
| [types.md](08-reference/types.md) | 核心类型关系图与嵌套说明 |
| [protocols.md](08-reference/protocols.md) | 请求/响应、错误体、trace 头、调用协议 |
| [cli.md](08-reference/cli.md) | `mgin` 脚手架命令与参数 |
| [compatibility.md](08-reference/compatibility.md) | 版本矩阵与兼容策略 |
| [examples.md](08-reference/examples.md) | 每个包的可运行示例与异常语义 |
| [api/](08-reference/api/) | 13 份由源码 AST 生成的完整 API 清单 |

### 09 · 高级主题

| 文档 | 内容 |
|------|------|
| [01-extension-points.md](09-advanced/01-extension-points.md) | 自定义插件/负载均衡/缓存/日志/中间件 |
| [02-source-walkthrough.md](09-advanced/02-source-walkthrough.md) | 启动、请求、调用三条主线的源码路径 |
| [03-performance.md](09-advanced/03-performance.md) | 性能杠杆、基准测试、常见瓶颈 |

### 10 · 贡献指南

| 文档 | 内容 |
|------|------|
| [01-contributing.md](10-contributing/01-contributing.md) | 分支、提交、PR、评审、编码规范、测试与发布门禁 |

### 11 · 附录

| 文档 | 内容 |
|------|------|
| [01-faq.md](11-appendix/01-faq.md) | 高频问题与误解 |
| [02-glossary.md](11-appendix/02-glossary.md) | 术语表（全站统一用词） |
| [03-migration-and-issues.md](11-appendix/03-migration-and-issues.md) | v1→v2 迁移、已知问题与限制 |

---

## 3. 角色阅读路径

### 3.1 业务开发者（最常见）

```
01-introduction → 02-quickstart
      → 03-development/01-project-layout
      → 03-development/02-service-development
      → 03-development/03-service-calls
      → 04-configuration/02-config-guide（按需查）
      → 06-observability/03-logging（打日志）
      → 08-reference/examples（抄示例）
```

### 3.2 架构师 / 技术负责人

```
02-concepts/01-architecture → 02-concepts/03-design-decisions
      → 05-governance/02-traffic-and-resilience
      → 07-deployment/02-operations（生产约束）
      → 08-reference/compatibility
      → 11-appendix/03-migration-and-issues（能力边界）
```

### 3.3 运维 / SRE

```
07-deployment/01-deployment → 07-deployment/02-operations
      → 06-observability/*（指标/日志/追踪）
      → 07-deployment/03-runbook（收藏此页）
      → 08-reference/configuration（核对配置）
```

---

## 4. 文档约定

### 4.1 状态标记

文中出现的状态标记含义固定：

| 标记 | 含义 |
|------|------|
| ✅ | 当前代码已实现，可直接使用 |
| ⚠️ | 已实现但有明确限制，使用前必读约束 |
| ❌ | **未实现**，不在当前版本范围；不得按文档描述假设可用 |

### 4.2 术语

统一使用 [术语表](11-appendix/02-glossary.md) 的定义：

- **服务（service）**：一组提供相同能力的实例集合，注册中心里的逻辑名。
- **实例（instance）**：一个可被调用的地址，如 `10.0.0.8:8080`。
- **插件（plugin）**：实现 `plugin.Plugin` 的组件，有 Init/Close/Health 生命周期。
- **组件（component）**：用户口语中的中间件/能力，如 MySQL、Redis；不等同于插件。
- **探针（probe）**：`/health/live`、`/health/ready`、`/health/startup`。
- **业务错误码（code）**：`errcode.Definition.Code`，与 HTTP 状态码分离。

### 4.3 代码示例

- 示例默认放在**独立工程**中运行，不在框架仓库内执行。
- 配置路径注意：`NewApp` 会把相对路径拼到 `filepath.Dir(os.Args[0])`，
  本地 `go run` 请传绝对路径（详见 [FAQ](../development/11-appendix/01-faq.md)）。
- 标注 `// runnable` 的代码块可直接复制为 `main.go` 运行；其余为片段。

### 4.4 API 参考的生成方式

`08-reference/api/*.md` 由仓库脚本解析 Go AST 生成，保证：

- 覆盖**全部导出**常量、变量、类型、字段、方法、函数；
- 签名、字段、tag 与源码逐字一致；
- 代码注释原样保留（参数的语义多数写在注释中）。

生成器不在仓库内（一次性工具）；修改导出 API 后请按
[贡献指南](10-contributing/01-contributing.md) 的“文档更新”要求同步。

参数/返回值/异常的可读说明与可运行示例集中在
[08-reference/examples.md](08-reference/examples.md) 与各主题章节。

---

## 5. 版本与反馈

| 项 | 值 |
|----|----|
| 文档对应代码 | `v2` 分支当前 HEAD |
| Go 版本 | 1.25.7 |
| 文档基线核对 | `go build ./...` 通过；核心包测试除 `pkg/logsink` 一个历史用例外通过 |
| 反馈方式 | 提交 PR 修改对应 Markdown；或提 issue 标注“docs:” |

相关文档（仓库既有，保留深度细节）：

- [架构设计](../architecture.md) · [模块与接口](../modules-and-api.md)
- [数据设计](../data-design.md) · [测试策略](../testing-strategy.md)
- [发布流程](../release-process.md) · [v1→v2 迁移](../migration-v1-to-v2.md)
