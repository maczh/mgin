# 阅读路径

不同角色需要的上下文不同。下面给出三条最短路径，按顺序读即可，不必从头到尾刷完整套文档。

## 1. 业务开发者：一周内独立交付服务

| 顺序 | 文档 | 目标 |
|------|------|------|
| 1 | [快速上手](03-quickstart.md) | 跑起来，看到探针响应 |
| 2 | [项目结构](../03-development/01-project-layout.md) | 知道文件放哪、module 怎么配 |
| 3 | [服务实现](../03-development/02-service-implementation.md) | 写 controller/service，统一返回与错误码 |
| 4 | [数据访问](../03-development/04-data-access.md) | 连库、DAO、事务、缓存 |
| 5 | [服务调用](../03-development/03-service-call.md) | 调其他服务，处理协议与错误 |
| 6 | [配置模型](../04-configuration/01-model.md) | 按环境切换、配置中心 |
| 7 | [API 参考](../08-reference/api/README.md) | 随用随查签名 |
| 8 | [FAQ](../11-appendix/01-faq.md) | 踩坑时先查这里 |

## 2. 架构师 / 技术负责人：评估与定规范

| 顺序 | 文档 | 关注点 |
|------|------|--------|
| 1 | [架构说明](../02-concepts/01-architecture.md) | 分层、启动时序、请求链、生命周期 |
| 2 | [核心抽象](../02-concepts/02-abstractions.md) | App/组件/门面/DAO/执行器/探针 |
| 3 | [设计决策](../02-concepts/03-design-decisions.md) | 为什么 200 + status、为什么单机 job |
| 4 | [需求与边界](../02-concepts/04-requirements.md) | 已实现/未实现、非功能约束 |
| 5 | [服务治理](../05-governance/) | 发现与 LB、限流与容错、安全 |
| 6 | [已知风险](../02-concepts/01-architecture.md#8-已知风险与技术债) | 上线前必须知道的坑 |
| 7 | [源码导读](../09-advanced/03-source-tour.md) | 需要改框架时的入口 |

## 3. 运维 / SRE：部署与排障

| 顺序 | 文档 | 关注点 |
|------|------|--------|
| 1 | [配置模型](../04-configuration/01-model.md) | 配置来源、环境切换、重载边界 |
| 2 | [指标](../06-observability/01-metrics.md) | 指标名、标签、告警建议 |
| 3 | [日志](../06-observability/03-logging.md) | 目标、级别、落库条件、VictoriaLogs |
| 4 | [部署](../07-deployment/01-deployment.md) | 单机/容器、探针、优雅停机 |
| 5 | [Runbook](../07-deployment/02-runbook.md) | 按症状排查 |
| 6 | [诊断](../06-observability/04-diagnostics.md) | 运行时状态、日志与指标联动 |

## 4. 框架贡献者

[扩展机制](../09-advanced/01-extension.md) → [源码导读](../09-advanced/03-source-tour.md) → [协作流程](../10-contributing/01-workflow.md) → [编码规范](../10-contributing/02-coding-standards.md) → [测试策略](../10-contributing/03-testing-strategy.md) → [发布规范](../10-contributing/04-release-process.md)。

## 5. 阅读约定

- 标注「**未实现**」的能力不要按文档排期，先看 [能力边界](../../../README.md#24-当前分支未实现的能力)。
- 示例默认使用模块路径 `github.com/maczh/mgin`，复制到业务工程时替换。
- 遇到文档与代码不一致，以代码为准，并提 issue/PR 修正文档。
