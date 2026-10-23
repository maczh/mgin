# 接下来读什么：三条阅读路径

MGin 文档不是线性书。按你的角色选一条路径，避免在无关章节浪费时间。

---

## 1. 业务开发者（后端 CRUD / API）

**目标**：一周内独立交付一个带数据库、鉴权、日志的服务。

| 顺序 | 文档 | 用时 | 读完能做 |
|------|------|------|----------|
| 1 | [快速上手](02-quickstart.md) | 15 min | 跑起服务、访问探针 |
| 2 | [项目结构](../03-development/01-project-layout.md) | 10 min | 知道代码放哪 |
| 3 | [服务开发](../03-development/02-service-development.md) | 40 min | 写路由/控制器/DAO、统一错误响应 |
| 4 | [服务调用](../03-development/03-service-calls.md) | 25 min | 调下游、传上下文、传文件 |
| 5 | [配置手册](../04-configuration/02-config-guide.md) | 20 min | 接 MySQL/Redis、改端口与日志 |
| 6 | [日志](../06-observability/03-logging.md) | 15 min | 打出可检索的日志 |
| 7 | [示例库](../08-reference/examples.md) | 按需 | 抄一段能跑的代码 |

**跳过**：02 概念、05 治理细节、07 部署、09 高级。遇到问题再查。

**常见卡点**：

- 配置键写错但无报错 → [配置诊断](../04-configuration/02-config-guide.md#5-配置错误诊断)
- DAO 报“not opened” → 忘开 `go.config.used`
- 探针 404 → 只写了 `go.health.enabled` 但用的是旧版 `NewApp` 装配？

---

## 2. 架构师 / 技术负责人

**目标**：判断 MGin 是否适配团队，划清能力边界，给出接入方案。

| 顺序 | 文档 | 关注点 |
|------|------|--------|
| 1 | [框架简介](01-introduction.md) | 定位与不适用场景 |
| 2 | [架构](../02-concepts/01-architecture.md) | 单进程模型、组件拓扑、生命周期 |
| 3 | [核心抽象](../02-concepts/02-abstractions.md) | 扩展点是否够用 |
| 4 | [设计决策](../02-concepts/03-design-decisions.md) | 技术债与代价 |
| 5 | [治理决策链](../05-governance/02-traffic-and-resilience.md) | 与 Mesh/网关的分工 |
| 6 | [生产约束](../07-deployment/02-operations.md) | 多副本限制、容量 |
| 7 | [兼容矩阵](../08-reference/compatibility.md) | 升级策略 |
| 8 | [已知问题](../11-appendix/03-migration-and-issues.md) | 哪些能力不能承诺 |

**必须明确的边界**：

- 无 gRPC、无分布式事务、无集群限流、无对象存储、无 PostgreSQL/ClickHouse 驱动；
- 定时任务与限流是**单实例语义**，多副本需选主或外部组件；
- `go.framework.*` 多数开关未接线，别在方案里依赖。

---

## 3. 运维 / SRE

**目标**：让服务可部署、可观测、可排障。

| 顺序 | 文档 | 产出 |
|------|------|------|
| 1 | [部署形态](../07-deployment/01-deployment.md) | K8s manifest、探针参数 |
| 2 | [生产清单](../07-deployment/02-operations.md) | 上线前逐项核对 |
| 3 | [指标](../06-observability/01-metrics.md) | Prometheus 抓取与告警规则 |
| 4 | [日志](../06-observability/03-logging.md) | 采集与保留 |
| 5 | [追踪](../06-observability/02-tracing.md) | trace 串联 |
| 6 | [Runbook](../07-deployment/03-runbook.md) | 收藏，故障时按症状查 |
| 7 | [配置项参考](../08-reference/configuration.md) | 核对线上配置 |

**关键约束**：

- `/health/ready` 会真实探活依赖，超时时间要大于最慢一次重连；
- 优雅停机默认 5s，K8s `terminationGracePeriodSeconds` 必须大于它；
- 限流计数在进程内，副本数变化会线性放大实际放行量。

---

## 4. 框架贡献者

```
10-contributing/01-contributing.md
  → 02-concepts/01-architecture.md（生命周期）
  → 09-advanced/02-source-walkthrough.md（源码主线）
  → 09-advanced/01-extension-points.md（扩展方式）
  → 08-reference/api（签名基线）
```

先跑通 `go build ./...` 与核心测试，再读
[编码规范与门禁](../10-contributing/01-contributing.md#4-编码规范)。

---

## 5. 一页速查

| 我想…… | 直接看 |
|---------|--------|
| 加一个接口 | [服务开发 §3](../03-development/02-service-development.md#3-控制器与路由) |
| 返回统一错误 | [错误码与响应](../03-development/02-service-development.md#5-统一响应与错误码) |
| 调另一个服务 | [服务调用](../03-development/03-service-calls.md) |
| 连 MySQL | [配置指南 §3](../04-configuration/02-config-guide.md#3-数据源配置) |
| 配 Nacos | [服务发现](../05-governance/01-discovery-and-lb.md#2-注册中心接入) |
| 加限流 | [容错与限流](../05-governance/02-traffic-and-resilience.md#5-限流) |
| 加鉴权 | [安全](../05-governance/03-security.md) |
| 接 Prometheus | [指标](../06-observability/01-metrics.md) |
| 查配置键 | [配置项参考](../08-reference/configuration.md) |
| 查函数签名 | [API 参考](../08-reference/api/01-app.md) |
| 服务起不来 | [Runbook](../07-deployment/03-runbook.md#1-启动失败) |
