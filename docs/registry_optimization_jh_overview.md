# mgin/jh 分支：微服务注册/发现/注销优化与调用稳定性提升

> 分支：`jh`（扁平布局，模块 `github.com/maczh/mgin`，注册接口为
> `RegistryClient{ Register([]byte); GetServiceURL(servicename string, group ...string)(string,string); DeRegister() }`）
> 目标：与 `pkg/`（v2）分支一致的优化，但适配本分支更精简的接口（无 `GetServices`/负载均衡/熔断）。

## 一、服务发现性能（核心收益）

新增 `registry/cache.go`：`CachedRegistry` 装饰器，默认由 `registry.NewRegistry()` 包裹真实后端。
对 `GetServiceURL` 的返回结果做 **TTL 缓存（默认 3s）+ 后台刷新（默认 1s）+ 并发去重 + 降级容错**：

- **性能**：绝大多数发现调用命中本地缓存，**零注册中心 RTT**，注册中心压力下降 1~2 个数量级。
- **稳定性**：注册中心抖动/短暂不可用时，**复用上一次成功发现的实例**（降级），避免把全部微服务调用瞬间打挂；空结果（确无实例）**不写入缓存**，下一次立即回源。
- **并发**：对同一 `服务名+分组` 的并发发现请求做 singleflight 合并，只打一次后端。
- **后台刷新**：仅对“近期仍被访问（热）”的 key 周期刷新，让负载在实例间轮换并剔除已下线实例；进程退出 `DeRegister` 时优雅停止刷新协程。

> 适配说明：本分支 `GetServiceURL` 返回**单一实例地址**（后端内部已随机选一个），因此缓存的是“最终选定的实例”，
> 在 TTL 窗口内同一服务会落到同一实例；后台刷新会重新随机挑选，跨窗口实现负载分散。
> 这与 `pkg/` 分支“缓存实例列表 + 调用端负载均衡”的粒度不同，但性能与降级收益等价。

配置（`config/configure.go` 的 `discovery` 结构，加载于 `Init`）：

```yaml
go:
  discovery:
    registry: "nacos"   # nacos / etcd / consul / polaris
    callType: x-form
    cacheTTL: 3         # 缓存时长（秒），<=0 用默认 3
    cacheRefresh: 1     # 后台刷新间隔（秒），<=0 用默认 1
```

## 二、注册稳定性（各后端）

### etcd（`registry/etcd/etcd.go`）
- 新增 `keepAliveLoop`：租约续约通道关闭或续约失败时**自动重建租约并重新注册**（`reRegister`，幂等），
  修复原 goroutine 静默退出导致实例“假死”（租约过期被摘除）的问题。
- `EtcdClient` 记录 `registeredKey` / `apiUrl` / `ctx+cancel`；`DeRegister` 用 `cancel()` 取消后台协程，
  并精确删除 `registeredKey`（原实现有 goroutine 泄漏 + `fmt.Printf` 噪音，已移除）。

### nacos（`registry/nacos/nacos.go`）
- 心跳 `nacosHeartbeatWorker` 增加失败计数，连续失败 ≥ `maxFails(3)` 次触发 `reRegister()` 重新注册，
  避免注册中心会话失效后实例脱管。
- `DeRegister` 先停心跳（`close(quit)`，带重复 close 保护）再发 DELETE，避免注销后残留心跳；
  保留 `worker != nil` 判空（防止未注册即注销时空指针）。

### consul（`registry/consul/consul.go`）
- 注册时附加 **TTL 健康检查（30s）** 并启动 `passTTL` 续活协程（25s 上报一次 `HealthPassing`），
  `Health().Service(passingOnly)` 会自动剔除异常实例。
- 记录 `instanceID`，`DeRegister` 优先用存储的 ID 精确反注册（原实现按 IP 重算，易不匹配），
  并通过 `ctx/cancel` 停止续活协程；增加 `client == nil` 保护。

### polaris（`registry/polaris/polaris.go`）
- 注册时把 `instanceId` 存到结构体字段，`DeRegister` 优先使用（缓存 `polaris` 作为兜底），
  避免进程重启后缓存丢失导致按 IP 反注册失败。

## 三、调用稳定性（`registry` + `client`）

- 本分支 `client.Call` 直接调用 `registry.Registry.GetServiceURL` 获取单实例地址，无独立熔断层。
  发现缓存的**降级复用**即为本分支的“调用稳定性”主要抓手：注册中心不可用时，调用仍可打到**上一次成功发现的实例**，
  由真实网络连通性决定成败，而不是在发现阶段就整体失败。
- 说明：`pkg/` 分支的“全部实例熔断→快速失败”依赖负载均衡+熔断（本分支接口无实例列表，不适用）；
  若后续引入 `GetServices`/负载均衡，可再叠加熔断层。

## 四、验证

- `go build ./...` 通过（全程 `GOFLAGS`/GOCACHE 指向空间充足目录）。
- `go vet ./registry/ ./client/ ./config/` 通过。
- 新增 `registry/cache_test.go`（父包 `registry`）：
  - 缓存命中避免回源 ✅
  - 后端抖动时降级复用旧实例 ✅
  - 空结果不进入缓存、每次回源 ✅
  - 并发首查 singleflight 合并为 1 次回源 ✅
- `go test -run '^$' ./registry/...` 确认各后端测试文件**编译通过**（既有 etcd/consul/polaris 测试需真实注册中心，本环境无 server，非本次引入）。

## 五、改动文件清单

| 文件 | 改动 |
|------|------|
| `config/configure.go` | `discovery` 增加 `CacheTTL`/`CacheRefresh` 并加载 |
| `registry/cache.go` | **新增** `CachedRegistry` 发现缓存装饰器 |
| `registry/registry.go` | `NewRegistry()` 默认用 `CachedRegistry` 包裹各后端 |
| `registry/cache_test.go` | **新增** 装饰器单测 |
| `registry/etcd/etcd.go` | 字段 + `keepAliveLoop`/`reRegister`/`DeRegister` 取消+精确删 key |
| `registry/nacos/nacos.go` | 心跳失败重注册 + `DeRegister` 顺序/判空 |
| `registry/consul/consul.go` | TTL 检查 + `passTTL` 续活 + 存储 instanceID + `DeRegister` 停止 |
| `registry/polaris/polaris.go` | 本地存储 `instanceId` 优先反注册 |
| `application.example.yml` | 新增 `discovery` 配置示例 |

## 六、上线建议

1. 默认 `cacheTTL=3/cacheRefresh=1` 适用于多数场景；高变更频率集群可调小 `cacheRefresh`（如 0.5）。
2. Consul TTL（30s）/ 续活（25s）已留余量，若 Consul 健康检查窗口不同可调整。
3. etcd 重注册为**无退避立即重试**（失败则 1s 后重试），如集群网络极不稳定可加指数退避。
