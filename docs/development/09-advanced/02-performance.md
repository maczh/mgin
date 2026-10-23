# 性能与容量

## 1. 热点路径

| 路径 | 成本 | 优化手段 |
|------|------|----------|
| JSON 序列化/反序列化 | CPU 主要开销 | 避免超大响应；分页；必要时用 `easyjson`（需改业务） |
| 日志格式化 + 队列 | CPU + 内存 | 生产用 `info`；关闭不需要的字段；`file` 目标异步写 |
| 数据库查询 | 网络 + 锁 | 索引、分页、DAO 缓存、连接池 |
| 服务发现 | 网络（缓存后为内存） | 调大 `cacheTTL`；避免高频变更 |
| Casbin | 每请求 `LoadPolicy` + Enforce | 高并发时改定时刷新或本地缓存 |
| 内存缓存 | `clone.Clone` 深拷贝 | 大对象改存指针/磁盘缓存 |
| `trace` 上下文 | 每次请求 map 写入 + 定时清理 | 无需优化，注意内存上限 |

## 2. 容量计算

### 2.1 连接池

```
需要的最大连接数 ≈ 单实例 QPS × 平均 SQL 耗时(s)
数据库上限        ≈ 实例数 × pool.max × 0.7
```

- `mysql_pool.max <= 1` 不启用池（每次新建连接），生产必须 `> 1`。
- `total`（最大空闲）不要大于 `max`。
- 连接泄漏的典型症状：QPS 不高但 `SHOW PROCESSLIST` 持续增长。

### 2.2 内存

| 项 | 估算 |
|----|------|
| 日志队列 | `queue × 平均日志大小 × handler 数` |
| 内存缓存 | 无上限，按业务估算；建议加容量控制或改用 Redis |
| 请求体 | `并发 × 平均 body`；大文件走流式 |
| Go GC | 默认 GOGC=100；内存受限时设 `GOGC=50` 或 `GOMEMLIMIT` |

### 2.3 协程

- 每请求由 net/http 分配，正常。
- `client` 无超时会让协程滞留下游；必须用 context 兜底。
- `PSubscribe` 每个订阅一条协程 + 重连；不要创建大量订阅。

## 3. 压测方法

```bash
# wrk
wrk -t4 -c200 -d30s --latency http://localhost:8080/api/v1/users/1

# hey
hey -z 30s -c 100 http://localhost:8080/api/v1/users/1

# 观察
curl -s localhost:8080/metrics | grep -E 'mgin_http_(requests_total|request_duration_seconds|requests_in_flight)'
```

对照实验：

1. 先空载（只返回常量），建立基线；
2. 加数据库查询，观察 P99；
3. 加缓存，比较命中前后；
4. 开启日志/指标，确认开销可接受。

## 4. 基准测试（Go）

```go
func BenchmarkParse(b *testing.B) {
	raw := []byte(`{"id":1,"name":"alice"}`)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var u User
		if err := json.Unmarshal(raw, &u); err != nil {
			b.Fatal(err)
		}
	}
}
```

```bash
go test -bench=. -benchmem ./internal/...
```

## 5. 调优清单

- [ ] `mysql_pool.max` 与副本数匹配数据库上限
- [ ],DAO 缓存 TTL 合理（不要 0）
- [ ] 分页接口有 `size` 上限
- [ ] 生产 `logger.level=info`，关闭 `mysql_debug`
- [ ] 日志 `outputs` 不重复写同一目标
- [ ] `go.log.get=line/off` 避免大响应体
- [ ] `job.maxConcurrent` 不超过数据库承载
- [ ] 限流规则保护慢接口
- [ ] 容器 `GOMAXPROCS` 与 CPU limit 对齐（Go 1.25 已支持容器感知）
- [ ] 大文件上传/下载走流式，不加载进内存

## 6. 已知性能限制

1. `logsink` 每 handler 单协程，吞吐上限约「每秒数万条小日志」；更高需自定义 handler。
2. `MemCache` 无容量淘汰，长时间运行会持续增长。
3. `casbin` 每请求加载策略，QPS 高时成为瓶颈。
4. `RedisCache.Range` 全库 SCAN，禁止在请求路径调用。
5. 服务发现缓存是进程级，无法跨实例共享。
