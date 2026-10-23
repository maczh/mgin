# 注册、发现与负载均衡

## 1. 注册：服务如何被看见

```
plugin.InitAll
  └─ Registry 插件（Order=30）
       ├─ used 含 nacos  → registry.Registry.Register(conf/nacos-<env>.yml)
       ├─ used 含 etcd   → ...
       └─ used 含 consul → ...
             │
             ▼
       注册中心（实例：ip:port + 分组 + 元数据）
```

注册什么：

| 字段 | 来源 |
|------|------|
| 服务名 | `go.application.name` |
| 地址 | `go.application.ip`，为空时探测本机地址 |
| 端口 | `go.application.port` |
| 分组 | `go.nacos.group` 等子配置 |
| 集群 | `clusterName` |

退出时 `DeRegister`：Nacos 取消实例、etcd 撤销租约、Consul 注销服务。

### 1.1 后端差异

| 能力 | Nacos | etcd | Consul | Polaris |
|------|-------|------|--------|---------|
| 自动注册 | ✅ | ✅ | ✅ | ❌ 仅发现 |
| 健康过滤 | ✅ `/list` 的 healthy | ❌ | ✅ passingOnly | ❌ |
| 心跳/租约 | 心跳失败 3 次重注册 | Lease + KeepAlive | TTL 续活（30s） | 由控制面负责 |
| 多实例 | ✅ | ✅ | ✅ | ✅ |
| 配置中心 | ✅ | ✅ | ✅ | ✅ |

> Polaris 的 `Register` 存在实现，但 registry 插件未把 polaris 列入启用条件，
> 因此**不会自动注册**。需要时手动调用 `registry.Registry.Register(...)`。

---

## 2. 发现：一次调用如何拿到地址

```go
urls, err := registry.Registry.GetServices("order", "prod")
// urls = ["10.0.0.8:8080", "10.0.0.9:8080"]
```

### 2.1 缓存装饰器

`NewRegistry()` 返回的是 `CachedRegistry`（装饰真实客户端）：

| 机制 | 行为 |
|------|------|
| key | `service + "\x00" + join(groups, ",")` |
| TTL | `go.discovery.cacheTTL`（默认 60s） |
| 后台刷新 | `go.discovery.cacheRefresh`（默认 30s） |
| 并发回源 | 同一 key 的并发请求只回源一次（single-flight） |
| 回源失败 | 返回旧值（降级），不直接报错 |
| 预热 | 刷新 goroutine 定期更新 |

影响：

- 实例下线后最长 TTL 内仍可能被调用到 → 客户端必须有超时/重试/熔断；
- 新实例上线后最长 refresh 周期才可见；
- 修改 TTL 需重启。

### 2.2 两个 API 的差异

| API | 返回 | 是否过滤不健康 | 用途 |
|-----|------|----------------|------|
| `GetServiceURL(service, groups...)` | 单个地址 + 命中分组 | 否（Nacos） | v1 兼容、兜底 |
| `GetServices(service, groups...)` | 全部地址 | Nacos/Consul 是 | 负载均衡 |

---

## 3. 负载均衡

```go
type LoadBalancer interface {
    Pick(instances []string, key string) (string, error)
}
```

### 3.1 四种策略

| 注册名 | 行为 | 适用 | 注意 |
|--------|------|------|------|
| `round_robin` | 原子计数轮询 | 默认；实例同质 | 实例列表变化时计数不重置 |
| `random` | 随机 | 无状态、同质 | 分布有波动 |
| `least_connections` | 选当前连接数最少 | 长耗时、异构 | **必须**业务调用 `Inc(host)` 维护计数，否则恒选第一个 |
| `consistent_hash` | FNV-1a 取模 | 需要同一 key 命中同一实例 | 无虚拟节点，实例变化会大范围漂移；key 为空退化为轮询 |

### 3.2 使用

```go
body, err := client.CallCtx(ctx, "order", "/api/v1/orders/1", &client.Options{
    Method:       "GET",
    LoadBalancer: "consistent_hash",  // 用注册名，不是短名
})
```

默认策略：`loadbalancer.Default()`，初始为 `RoundRobinLB`，可用
`SetDefault` 替换。

### 3.3 least_connections 的正确用法

```go
lb := loadbalancer.LeastConnectionsLB
host, _ := lb.Pick(instances, "")
dec := lb.Inc(host)
defer dec()          // 请求结束归还
```

不调用 `Inc` 时所有实例计数为 0，`Pick` 永远返回第一个。

### 3.4 自定义策略

```go
type IPHash struct{}

func (IPHash) Pick(instances []string, key string) (string, error) {
    if len(instances) == 0 {
        return "", loadbalancer.ErrEmptyInstances
    }
    h := fnv.New32a()
    _, _ = h.Write([]byte(key))
    return instances[int(h.Sum32())%len(instances)], nil
}

func init() { loadbalancer.Register("ip_hash", IPHash{}) }
```

见 [09 · 扩展点](../09-advanced/01-extension-points.md)。

### 3.5 常见坑

| 现象 | 原因 |
|------|------|
| 配了 `round` 但没生效 | 只认注册名 `round_robin`；未命中静默回落默认 |
| 所有请求打到一个实例 | 使用了 `least_connections` 但未 `Inc`；或 `GetServices` 只返回一个 |
| 灰度实例也分到流量 | 当前无权重/标签路由；需用分组或网关实现 |
| 全部实例熔断后不降级 | 设计如此：`ErrAllInstancesCircuitOpen` 快速失败 |

---

## 4. 多注册中心、多环境与单元化

### 4.1 一个进程一个注册中心

`registry.Registry` 是全局单例，`go.discovery.registry` 只能选一个。
不支持同时向多个注册中心注册。

### 4.2 用分组隔离环境

- Nacos：`group`
- etcd：`prefix` + `group`
- Consul：`group`
- Polaris：`namespace`

调用时通过 `Options.Group` 指定，未指定则用当前配置分组。
建议 `group` 与环境强绑定（`dev`/`test`/`prod`），避免跨环境误调。

### 4.3 单元化（同城多活）

当前能力可做到：

1. 每个单元部署独立注册中心 + 独立分组；
2. 本单元服务只注册到本单元；
3. 跨单元调用显式指定 `Group`。

不提供：就近路由、故障切流、全局配额。这些需要网关/Mesh/注册中心自身能力。

---

## 5. 排障命令

```bash
# Nacos：列出实例
curl "http://127.0.0.1:8848/nacos/v1/ns/instance/list?serviceName=order&groupName=DEFAULT_GROUP"

# Consul
curl "http://127.0.0.1:8500/v1/health/service/order?passing=true"

# etcd
ETCDCTL_API=3 etcdctl get --prefix /mgin/

# 应用侧：打开 debug 日志可看到
# "微服务{}请求: ..." 与选中的 host
```

| 症状 | 排查顺序 |
|------|----------|
| 调用报“获取主机IP端口失败” | 服务是否注册 → 分组是否一致 → `GetServices` 是否返回 → 网络 |
| 注册成功但调用不到 | Nacos 健康状态 → 实例 IP 是否可达（容器内网 IP 常见） |
| 实例下线仍有流量 | 缓存 TTL → 下游超时配置 |
| 分组切换后调不通 | `Options.Group` 与 provider 注册分组是否一致 |
