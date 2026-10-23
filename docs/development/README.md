# MGin 开发者文档

本目录是 MGin（`jh` 分支，模块 `github.com/maczh/mgin`，Go 1.25.7）的开发者文档入口。所有内容以当前代码为准；与 `README_CN.md`/`README_en.md` 冲突时，以本目录与代码为准。

## 目录结构

| 目录 | 内容 | 适合谁 |
|------|------|--------|
| [01-getting-started](01-getting-started/) | 框架简介、环境准备、第一个服务、阅读路径 | 所有人（第一次接触） |
| [02-concepts](02-concepts/) | 架构、核心抽象、设计决策、需求与术语 | 需要理解「为什么」的人 |
| [03-development](03-development/) | 项目结构、服务实现、服务调用、数据访问、测试 | 业务开发者 |
| [04-configuration](04-configuration/) | 配置模型、分域配置指南、加载/刷新/诊断 | 业务开发者、运维 |
| [05-governance](05-governance/) | 注册发现与负载均衡、流量与容错、安全 | 架构师、服务负责人 |
| [06-observability](06-observability/) | 指标、追踪、日志、诊断 | 运维、SRE |
| [07-deployment](07-deployment/) | 部署形态、生产检查、Runbook | 运维、SRE |
| [08-reference](08-reference/) | API、变量、类型、协议、命令（查阅型） | 所有人（集成时查阅） |
| [09-advanced](09-advanced/) | 扩展机制、源码导读、性能 | 框架贡献者、资深开发者 |
| [10-contributing](10-contributing/) | 工作流、编码规范、测试、发布、文档规范 | 贡献者 |
| [11-appendix](11-appendix/) | FAQ、术语表、版本迁移与已知问题 | 所有人 |

## 推荐阅读路径

**业务开发者（最快上线）**
`01-getting-started/03-quickstart` → `03-development/01-project-layout` → `03-development/02-service-implementation` → `08-reference/api/README`（按需查签名）

**架构师 / 技术负责人**
`02-concepts/01-architecture` → `02-concepts/03-design-decisions` → `05-governance/*` → `09-advanced/03-source-tour`

**运维 / SRE**
`04-configuration/01-model` → `06-observability/*` → `07-deployment/*`

**框架贡献者**
`09-advanced/*` → `10-contributing/*` → `03-development/05-testing`

## 文档约定

- **分支限定**：所有内容只描述 `jh` 分支。`master` 分支使用 `github.com/maczh/mgin/v2`，目录与 API 不同。
- **能力边界**：PostgreSQL、ClickHouse、ElasticSearch、Kafka、S3、内置系统管理 HTTP 接口在当前分支**未实现**；完整清单见 [README §2.4](../../README.md#24-当前分支未实现的能力)。
- **术语**：统一使用 [术语表](../development/11-appendix/02-glossary.md) 中的定义（「组件」「子配置」「执行器」「探针」等）。
- **可信度标注**：涉及行为的地方给出代码位置或验证命令；未验证的内容会明确写「未实测」。
- **示例**：除特别说明外，示例模块路径为 `github.com/maczh/mgin`；复制到业务工程时替换为实际 module。

## 快速索引

| 我要…… | 看这里 |
|---------|--------|
| 5 分钟跑起来 | [快速上手](01-getting-started/03-quickstart.md) |
| 理解 App 启动做了什么 | [架构 · 启动时序](02-concepts/01-architecture.md#2-启动时序) |
| 查某个函数签名 | [API 参考](08-reference/api/README.md) |
| 查配置键默认值 | [变量与配置](08-reference/variables.md) |
| 查类型字段 | [类型定义](08-reference/types.md) |
| 排查启动/连接问题 | [FAQ](11-appendix/01-faq.md)、[Runbook](07-deployment/02-runbook.md) |
| 提 PR | [协作流程](10-contributing/01-workflow.md) |
| 看未来几个版本要建什么能力 | [演进路线图](09-advanced/04-evolution-roadmap.md) |
