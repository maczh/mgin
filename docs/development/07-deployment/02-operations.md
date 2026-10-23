# 运维：生产清单、生命周期与容量

## 1. 生产就绪清单

### 1.1 配置

- [ ] `go.application.name`、`project`、`port` 正确且全局唯一
- [ ] `go.config.env` 与部署环境一致
- [ ] `go.application.shutdownTimeout` 已设置（建议 10–30s）
- [ ] K8s `terminationGracePeriodSeconds` > `shutdownTimeout`
- [ ] `go.health.enabled=true`，`go.metrics` 已启用并调用 `EnableMetrics()`
- [ ] `go.application.debug=false`，`go.logger.level=info`
- [ ] 生产关闭 Swagger（`go.sys.swagger.enabled=false`）
- [ ] 密钥（JWT secret、DB 密码）来自 Secret，不在镜像/仓库

### 1.2 资源与连接

- [ ] MySQL `mysql_pool.max` × 副本数 < 实例 `max_connections` × 0.8
- [ ] Redis `redis_pool.max` 合理（默认 100，注意每副本独立）
- [ ] 容器 memory limit ≥ P99 使用量 × 1.5
- [ ] CPU limit 不会导致 GC/调度抖动（建议 requests=limits 或 limits 为 requests 2–4 倍）
- [ ] 日志目录有容量上限与轮转

### 1.3 可靠性

- [ ] 下游调用配置了超时（`ResilienceOptions.Timeout`）
- [ ] 写操作未开启自动重试
- [ ] 关键接口有限流
- [ ] 定时任务只有一个副本启用
- [ ] 探针路径未被网关鉴权拦截
- [ ] 优雅关闭已验证（`kill -TERM` 无 5xx）

### 1.4 可观测

- [ ] Prometheus 能抓到 `mgin_*`
- [ ] 日志可检索（requestId 字段）
- [ ] 有 5xx、P99、依赖 down 告警
- [ ] trace 头已透传到下游（抓包或日志验证）

---

## 2. 生命周期

### 2.1 启动顺序

```
配置 → 日志 → 注册中心实例 → 插件（DB → Redis → Registry → Job）
     → 基础中间件 → 路由 → 监听 → MarkStarted
```

失败语义：

| 阶段 | 失败行为 |
|------|----------|
| 配置文件 | 记录错误，继续（组件可能全部不可用） |
| VictoriaLogs | **panic，启动失败** |
| 单个插件 | 记录错误，继续 |
| 端口占用 | 记录错误；另一协议可能仍启动 |
| 业务初始化 | 由业务决定是否 `os.Exit(1)` |

### 2.2 就绪判定

- `/health/live`：进程活着，恒 200；
- `/health/ready`：依赖可用才 200；
- `/health/startup`：`MarkStarted` 后 200；
- `go.health.autoStarted=true` 时 `Run()` 启动监听后自动标记。

业务应在**所有初始化完成**后调用 `health.MarkStarted()`，不要在 `main` 开头调用。

### 2.3 优雅关闭

信号：`SIGINT`、`SIGTERM`、`SIGHUP`、`SIGQUIT`。

```
停止巡检 → postlog.Close（排空）→ i18n.Close
        → 插件逆序关闭（Job → Registry → Redis → DB）
        → 每个 http.Server.Shutdown（独立超时）
```

注意：

- 关闭期间新请求可能被拒绝（连接已断），客户端需重试；
- 长时间 handler 超过 `shutdownTimeout` 会被强制中断；
- 后台 goroutine 不受框架管理，需业务自行 `cancel`；
- `SIGHUP` 也会触发退出（不是重载配置）。

### 2.4 验证优雅关闭

```bash
# 持续压测
hey -z 30s -c 20 http://127.0.0.1:8080/api/v1/hello &

# 发送 TERM，观察是否有 5xx / 连接错误
kill -TERM $(pgrep -f '/order ')
```

预期：进程在 `shutdownTimeout` 内退出，压测端无 502/connection reset 激增。

---

## 3. 容量规划

### 3.1 连接数

```
MySQL 连接上限 ≈ mysql_pool.max × 副本数
Redis 连接上限 ≈ redis_pool.max × 副本数
```

经验：

| 组件 | 单副本建议 | 说明 |
|------|-----------|------|
| MySQL max | 20–100 | 按 QPS × 平均耗时估算：`并发 ≈ QPS × RT(s)` |
| MySQL idle(total) | 5×max 或更小 | 过大浪费连接 |
| Redis max | 20–100 | 缓存命中耗时低 |
| Mongo | 按驱动默认 | 日志写入异步 |

示例：QPS 500、平均 RT 20ms → 并发约 10；`max=50` 足够，副本 4 → 200 连接。

### 3.2 并发与 goroutine

每个请求至少 1 个 goroutine；`CallResilientCtx` 超时后可能残留 1 个。
估算：

```
goroutine ≈ 并发请求 × (1 + 超时残留比例) + 后台任务
```

内存：每 goroutine 初始 2–8KB，按 10k 并发 ≈ 数十 MB，通常不是瓶颈。
真正的瓶颈是下游连接池与锁。

### 3.3 日志

| 项 | 估算 |
|----|------|
| 单条访问日志 | 0.5–2KB（`get: line`） |
| 100 QPS × 1KB | ≈ 8.6GB/天（未压缩） |
| 队列 | 100（配置上限），满则丢弃最旧 |

生产建议：`go.log.get: line`，Mongo/VL 设置 TTL 或按天分区。

### 3.4 调优杠杆（按收益排序）

1. 减少下游调用次数（批量、缓存）；
2. 合理的连接池与超时；
3. 数据库索引与慢查询；
4. 避免在请求路径写大日志/大 JSON；
5. 提高副本数（无状态服务优先水平扩展）；
6. 最后才考虑调整 GOGC/GOMAXPROCS。

---

## 4. 变更与升级

| 变更 | 步骤 |
|------|------|
| 配置变更 | 改 ConfigMap → 滚动重启（无热加载） |
| 框架升级 | 先看 CHANGELOG/迁移文档 → 本地编译测试 → 灰度一个副本 |
| 数据库变更 | 独立迁移脚本，先兼容后清理；不要依赖框架迁移 |
| 证书更新 | 替换文件后重启（无热更新） |
| 扩容 | 注意连接数与限流阈值同步调整 |

回滚：

```bash
kubectl rollout undo deployment/order
# 或切回上一镜像 tag
```

回滚后确认：注册中心实例数、探针状态、错误率、定时任务未被重复启用。

---

## 5. 日常巡检

| 频率 | 检查 |
|------|------|
| 每日 | 5xx 比例、P99、依赖 down、日志 Dropped |
| 每周 | 连接池使用率、慢查询、磁盘/日志容量 |
| 每月 | 依赖版本与证书有效期、限流阈值是否仍合理、任务执行成功率 |

自动化：把 [指标](../06-observability/01-metrics.md) 的查询做成看板与告警，
不要把巡检寄托在人工。
