# 故障排查 Runbook

按症状检索。每条给出：现象 → 可能原因 → 确认命令/日志 → 处置。

---

## R1. 进程起不来 / 立即退出

**确认**

```bash
journalctl -u order -n 100 --no-pager
./app -f /abs/conf/application.yml
```

**分支判断**

| 日志特征 | 原因 | 处置 |
|----------|------|------|
| `open ...application.yml: no such file` | 路径错误（常见于 `go run`） | 用绝对路径或先 build |
| `yaml: line N: ...` | YAML 缩进/制表符 | 修正 YAML（禁止 Tab） |
| `panic: ... i18n` | i18n 初始化失败 | 检查应用名正则与字典库 |
| `panic: ... victorialogs` | 日志后端配置非法 | 修正 `go.log.victorialogs.*` 或关闭该目标 |
| `bind: address already in use` | 端口占用 | `ss -lntp \| grep :8080`，改端口或释放 |
| `permission denied`（SQLite/日志） | 目录权限 | 调整目录属主 |

**回滚**：恢复上一版配置/镜像。

---

## R2. `/health/ready` 返回 503

**确认**

```bash
curl -s localhost:8080/health/ready | jq .
```

**处置**

| `dependencies` 失败项 | 原因 | 处置 |
|-----------------------|------|------|
| `mysql` | DSN/网络/认证/库名 | `mysql -h... -u... -p` 验证；检查多库是否传库名 |
| `mongodb` | URI/认证/网络 | `mongosh <uri>` 验证 |
| `redis`（若实现） | 当前 `Check()` 恒成功，不会失败 | — |

注意：SQLite 不参与就绪检查；Redis 故障不会体现在探针上。

---

## R3. 调用其他服务失败

**确认**

```bash
# 应用日志
grep -i "微服务获取\|服务注册中心未初始化" logs/app-*.log | tail
```

**处置**

| 错误 | 原因 | 处置 |
|------|------|------|
| `服务注册中心未初始化` | `go.discovery.registry` 为空或组件未启用 | 配 `used` + `discovery.registry`，重启 |
| `获取...主机IP端口失败` | 服务名/分组不一致，实例未注册 | 控制台确认实例与 group |
| 非 2xx / `Code=0` | 下游业务错误或路径错 | 看下游日志与网关 |
| 连接拒绝 | 注册的 IP/端口不可达 | 修正 `go.application.ip` 或网络策略 |

---

## R4. 请求变慢 / 5xx 上升

**确认**

```bash
curl -s localhost:8080/metrics | grep -E 'mgin_http_(requests_total|request_duration)'
awk -F'"' '/"status":5/ {print}' logs/app-$(date +%F).log | head
```

**排查顺序**

1. 是否集中在某个 `path`（指标标签）→ 查该接口的下游调用（调用日志）。
2. 下游是否变慢 → 调用的 `cost` 字段。
3. 数据库是否慢 → 开启 `mysql_debug` 临时观察（生产慎用），或看慢查询。
4. 连接池是否耗尽 → 数据库 `SHOW PROCESSLIST`，对比 `mysql_pool.max`。
5. 是否触发限流 → `status=1011` 的占比。

**处置**：扩容/调池/加缓存/限流保护；必要时回滚最近变更。

---

## R5. 探针或 `/metrics` 返回 401/403

**原因**：探针被挂到了受保护的路由组，或注册顺序晚于鉴权中间件。

**确认**

```bash
curl -i localhost:8080/health/live | head -5
```

**处置**：使用 `app.EnableHealth()` / `go.health.enabled: true` 让框架在最前注册；不要手工把 `/health` 加到业务组。

---

## R6. `/metrics` 404 或没有 HTTP 指标

| 现象 | 原因 | 处置 |
|------|------|------|
| 404 | 未启用 | `app.EnableMetrics()` 或 `go.metrics.enabled: true`（需重启） |
| 只有 build/plugin 指标 | 未挂 `metrics.Middleware()` | `app.Router.Use(metrics.Middleware())` |

---

## R7. 接口日志没有落库

**确认顺序**

1. `go.log.req` 是否非空；
2. `used` 是否包含 `mongodb`；
3. 多库模式下 `go.log.dbName` 是否配置；
4. 日志中是否有 postlog handler 错误；
5. Mongo 集合是否可写（权限/磁盘）。

**处置**：补齐配置并重启；无法使用 Mongo 时改用 `logsink` 自定义 handler。

---

## R8. 配置重载失败

```bash
curl -s -X PUT localhost:8080/cfg/reload -H "X-MGin-Reload-Token: $TOKEN"
```

| 返回 | 原因 | 处置 |
|------|------|------|
| 401 | token 缺失/不匹配 | 检查头名与值 |
| 403 | 来源 IP 不是内网 | 从内网发起，或经跳板机 |
| 500 | 新配置解析失败/资源重建失败 | 修正配置；若连接已断，重启恢复 |
| 404 | 未配置 `go.config.reload.token` | 配置后重启一次 |

**注意**：重载不更新端口/路由/中间件；这些必须重启。

---

## R9. 定时任务不执行

**确认**

```bash
curl -s localhost:8080/job/list
curl -s "localhost:8080/job/log?jobId=1"
```

| 原因 | 确认 | 处置 |
|------|------|------|
| `go.job.enabled: false` | 配置 | 开启并重启 |
| 无数据库 | 启动日志 | 启用 MySQL 或 SQLite |
| `handler_name` 无对应执行器 | `/job/handlers` | 注册执行器或修正名称 |
| 任务被停止 | `/job/:id` 的 status | 启动任务 |
| 多副本同时调度 | 部署副本数 | 仅单实例开启，或业务幂等 |
| cron 表达式非法 | 任务日志 | 修正表达式 |

---

## R10. 日志丢字段 / VictoriaLogs 无数据

| 现象 | 原因 | 处置 |
|------|------|------|
| 队列满丢日志 | 下游慢/量大 | 调大 `go.log.sink.queue`，降低日志量 |
| 无 request_id | 异步协程未恢复上下文 | `trace.CopyPreHeaderToCurRoutine` |
| 字段缺失 | 不在白名单 | 调整 `outputs[].fields` |
| VL 无数据 | URL/token 错或网络 | `curl {url}/insert/elasticsearch/` 验证 |

---

## R11. 注册中心整体不可用

**表现**：启动时注册失败、调用大量失败。

**处置**

1. 已建立的调用在缓存 TTL 内仍可用，避免同时重启所有实例。
2. 恢复注册中心后，实例会自动重新注册（Nacos/etcd 有心跳重试；Consul TTL 失败后重新注册）。
3. 需要紧急恢复时，切到备用注册中心或直接使用固定地址（临时改代码/网关）。

---

## R12. 通用信息收集（提工单时附上）

```bash
# 版本
curl -s localhost:8080/health/live | jq .data

# 配置（脱敏）
grep -v -i "password\|secret\|token" conf/application.yml

# 依赖
curl -s -w '\n%{http_code}\n' localhost:8080/health/ready

# 指标
curl -s localhost:8080/metrics > metrics.txt

# 最近日志
tail -n 200 logs/app-$(date +%F).log

# 进程与端口
ss -lntp | grep :8080
```
