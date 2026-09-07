# 微服务注册/发现/注销 优化与调用稳定性提升

> 范围：`pkg/registry`（nacos / etcd / consul / polaris 及统一接口）、`pkg/client`、`pkg/config`
> 目标：优化注册、发现、注销业务逻辑，提升**服务发现性能**与**微服务调用稳定性**

---

## 一、分析阶段发现的问题

### 1. 服务发现性能（最严重）
原实现中，每次微服务调用 `CallCtx → selectHostByLB → registry.Registry.GetServices` 都会**同步打一次注册中心**
（Nacos `/list`、etcd `Get`、Consul `Health().Service`、Polaris `GET /instances`）。
高 QPS 下：
- 单次调用被注册中心 RTT 拖慢；
- 注册中心承受与业务调用量成正比的查询压力；
- 注册中心瞬时抖动会直接穿透到业务调用，无降级。

### 2. 注册稳定性（连接/租约丢失）
- **etcd**：续约（KeepAlive）通道一旦因连接丢失而关闭，原协程**静默退出**，租约过期后实例从发现列表消失，进程活着却“假死”；`DeRegister` 用 `fmt.Printf` 且**不停止续约协程（goroutine 泄漏）**，注销 key 依赖运行时 `c.instanceId`（注册中途失败则为空）。
- **nacos**：心跳失败仅打日志、无重注册；`DeRegister` 直接访问 `n.worker.quit`，**注册失败时 `worker==nil` 触发空指针 panic**；且只停心跳、不主动 DELETE，依赖心跳超时摘除。
- **consul**：注册**无 TTL 健康检查**，进程崩溃后实例可能长期滞留；`DeRegister` 重新计算 IP 生成 ID，与注册时不一致会**反注册失败**；无续活协程。
- **polaris**：`DeRegister` 依赖缓存中的 `serviceId`，缓存丢失即降级到 host/port。

### 3. 调用稳定性
- `selectHostByLB` 在**全部实例熔断**时返回 `ErrAllInstancesCircuitOpen`，但 `callInternal` 之前会**忽略该错误并回退到 v1 随机选实例**，导致对已熔断下游继续施压。

---

## 二、已实施的改动

### A. 新增 `pkg/registry/cache.go` —— 本地发现缓存装饰器（核心性能/稳定性改动）
`CachedRegistry` 实现 `RegistryClient`，包裹真实后端，对 `GetServices` 结果做内存缓存：
- **TTL 缓存**：未过期直接返回，零网络开销（默认 3s，可调）。
- **后台预热**：定时（默认 1s）刷新已知 key，保持缓存温热、及时感知上下线，避免过期瞬间的集中回源（惊群）。
- **并发去重（in-flight）**：同一 key 并发冷命中只回源一次，其余等待结果。
- **降级容错**：回源失败但存在旧数据时**复用旧实例列表**，注册中心抖动不再导致全部调用失败；彻底无数据才返回错误。
- **优雅关闭**：`DeRegister` 停止后台协程，避免泄漏。
- 通过 `registry.NewRegistry()` 默认启用，对调用方与后端**完全透明**。

### B. 配置项扩展（`pkg/config/configure.go`）
`discovery` 新增（单位：秒，≤0 用默认值）：
- `go.discovery.cacheTTL`：发现结果缓存 TTL（默认 3）
- `go.discovery.cacheRefresh`：后台刷新间隔（默认 1，自动收敛为 < TTL）

### C. etcd 注册/续约/注销加固（`pkg/registry/etcd/etcd.go`）
- 新增 `keepAliveLoop`：续约 RPC 失败或通道关闭时**自动重新申请租约并重注册**（使用固定 `instanceId`，幂等），指数退避。
- 记录 `registeredKey / apiUrl`，`DeRegister` 用 `context` 取消续约协程并删除精准 key，改用日志替代 `fmt.Printf`。
- 注册中途失败也能安全注销（兜底拼接 key）。

### D. Nacos 心跳/重注册/注销修复（`pkg/registry/nacos/nacos.go`）
- `DeRegister` 增加 `worker==nil` 保护，**消除空指针 panic**；先停心跳再 DELETE，主动从 Nacos 摘除实例。
- 心跳连续失败 ≥3 次触发 `reRegister()` 主动重新注册，避免“假死”。

### E. Consul 健康检查 + 反注册修复（`pkg/registry/consul/consul.go`）
- 注册时附加 **TTL 健康检查**（`30s` TTL，`1m` 后自动反注册），并启动续活协程每 10s 上报 passing。
  配合 `Health().Service(passingOnly=true)`，实例异常会自动退出发现池。
- 记录 `instanceID`，`DeRegister` 用它**精准反注册**（不再重新计算 IP），并先取消续活协程。

### F. Polaris 健壮性（`pkg/registry/polaris/polaris.go`）
- 注册时记录 `serviceID` 到结构体，`DeRegister` 优先使用，缓存仅作兜底。

### G. 调用稳定性（`pkg/client/client.go`）
- `selectHostByLB` 返回 `ErrAllInstancesCircuitOpen` 时**快速失败**，不再回退 v1 随机选实例，避免对已熔断下游穿透。

---

## 三、验证结果
- `go build ./...`：通过。
- `go vet ./pkg/registry/ ./pkg/client/ ./pkg/config/`：通过。
- `go test ./pkg/registry/`（新增缓存装饰器单测：缓存命中、降级复用旧数据、空数据报错、关闭幂等）：**全部通过**。
- `go test ./pkg/client/...`：通过。
- etcd/consul/polaris 子包既有测试需连真实注册中心（环境无 server），属预存网络依赖，非本次改动引入。

---

## 四、可调参数与上线建议
1. **缓存 TTL/刷新**：默认 3s/1s 适合大多数场景；实例上下线感知要求更高时可调小 `cacheTTL`（如 1s），但会略增注册中心压力。
2. **etcd 重注册退避**：当前上限 30s，可按集群规模调整 `keepAliveLoop` 中的 `backoff`。
3. **Consul TTL**：`30s` TTL + `1m` 自动反注册，若进程 GC 停顿较长可适当放大 TTL 避免误摘除。
4. **建议**：存量项目无需改代码即可享受缓存与降级收益；etcd/consul 的“重注册/续活”仅在进程存活但连接抖动时触发，不影响正常注销路径。

---

## 五、改动文件清单
| 文件 | 改动 |
|---|---|
| `pkg/registry/cache.go` | **新增** 发现缓存装饰器 `CachedRegistry` |
| `pkg/registry/cache_test.go` | **新增** 缓存装饰器单测 |
| `pkg/registry/registry.go` | `NewRegistry` 默认包裹缓存层 |
| `pkg/config/configure.go` | `discovery` 增加 `cacheTTL`/`cacheRefresh` |
| `pkg/registry/etcd/etcd.go` | 重注册续约、精准注销、日志化 |
| `pkg/registry/nacos/nacos.go` | 修复 DeRegister 空指针、主动 DELETE、心跳重注册 |
| `pkg/registry/consul/consul.go` | TTL 健康检查、续活协程、精准反注册 |
| `pkg/registry/polaris/polaris.go` | 本地记录 serviceID |
| `pkg/client/client.go` | 全部实例熔断时快速失败 |
