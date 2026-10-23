# 注册与发现

> 本文件由脚本从源码 AST 生成，签名与字段与当前代码一致；新增导出符号后请重新生成。
>
> 注册中心接口、缓存装饰器与四种后端。

## `github.com/maczh/mgin/v2/pkg/registry`

```go
import "github.com/maczh/mgin/v2/pkg/registry"
```

### 变量

| 名称 | 类型/值 | 说明 |
|------|---------|------|
| `Registry` | RegistryClient = |  |

### 类型

#### `CachedRegistry`

```go
type CachedRegistry struct
```

CachedRegistry 是 RegistryClient 的装饰器（decorator）。

解决的核心问题：原实现每次微服务调用（client.CallCtx → selectHostByLB → GetServices）
都会同步打一次注册中心（Nacos /list、etcd Get、Consul Health、Polaris GET），
在高 QPS 下既拖慢单次调用、又给注册中心造成巨大压力，且注册中心抖动会直接击穿到业务调用。

本装饰器在内存中缓存 GetServices 结果：
- 读取命中缓存（未过期）直接返回，零网络开销；
- 后台定时预热，保持缓存温热、及时感知实例上下线；
- 回源失败时优先复用旧实例列表（降级），避免注册中心瞬时故障导致全部调用失败；
- 并发冷命中通过 inFlight 去重，只回源一次；
- 进程关闭（DeRegister）时停止后台刷新协程，避免泄漏。

该层对调用方与注册中心后端完全透明，不改变 RegistryClient 接口语义。

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|

**方法**

```go
func (c *CachedRegistry) Register(data []byte)
```

Register / DeRegister 直接透传到内层客户端；DeRegister 额外停止后台刷新。

```go
func (c *CachedRegistry) DeRegister()
```

DeRegister 停止后台刷新协程，再透传注销。

```go
func (c *CachedRegistry) GetServiceURL(servicename string, groupName ...string) (string, string)
```

GetServiceURL v1 单实例入口：复用缓存的实例列表，随机选 1 个（保持原语义）。
缓存未命中或回源失败时返回 ("", "")，与原 v1 行为一致。

```go
func (c *CachedRegistry) GetServices(servicename string, groupName ...string) ([]string, error)
```

GetServices v2 多实例入口：优先返回未过期的缓存；
未命中或已过期时回源，并通过 inFlight 抑制并发惊群；
回源出错但存在旧数据则降级返回旧数据，彻底无数据才返回错误。

#### `RegistryClient`

```go
type RegistryClient interface
```

RegistryClient 是 mgin v2 扩展后的统一注册中心客户端接口。

v1 接口：Register / GetServiceURL / DeRegister（保持不变，存量项目无需改动）。
v2 新增：GetServices 多实例列表，供 client.CallCtx 的负载均衡使用。

| 方法/内嵌 | 签名 |
|-----------|------|
| `Register` | func(registryConfigData []byte) |
| `GetServiceURL` | func(servicename string, groupName ...string) (string, string) |
| `GetServices` | func(servicename string, groupName ...string) ([]string, error) |
| `DeRegister` | func() |

### 函数

#### `NewCachedRegistry`

```go
func NewCachedRegistry(inner RegistryClient) *CachedRegistry
```

NewCachedRegistry 用缓存层包裹一个真实的 RegistryClient。
inner 为 nil 时直接返回 nil（与 NewRegistry 的兜底逻辑保持一致）。

#### `NewRegistry`

```go
func NewRegistry() RegistryClient
```

## `github.com/maczh/mgin/v2/pkg/registry/nacos`

```go
import "github.com/maczh/mgin/v2/pkg/registry/nacos"
```

### 类型

#### `NacosClient`

```go
type NacosClient struct
```

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|

**方法**

```go
func (n *NacosClient) Register(nacosConfigData []byte)
```

```go
func (n *NacosClient) GetServiceURL(servicename string, groupName ...string) (string, string)
```

```go
func (n *NacosClient) GetServices(servicename string, groupName ...string) ([]string, error)
```

GetServices v2 新增：返回该服务的所有可用实例 URL 列表（健康检查通过 + enabled）。
直接复用 GetServiceURL 已有的 /list 调用，去掉"随机选 1 个"这一步。

```go
func (n *NacosClient) DeRegister()
```

#### `InstanceResp`

```go
type InstanceResp struct
```

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `Name` | string | json:"name" |  |
| `Clusters` | string | json:"clusters" |  |
| `CacheMillis` | int | json:"cacheMillis" |  |
| `Hosts` | []struct { Service string `json:"service"` IP string `json:"ip"` Port int `json:"port"` ClusterName string `json:"clusterName"` Weight int `json:"weight"` Healthy bool `json:"healthy"` InstanceID string `json:"instanceId"` Metadata map[string]string `json:"metadata"` Marked bool `json:"marked"` Enabled bool `json:"enabled"` ServiceName string `json:"serviceName"` Ephemeral bool `json:"ephemeral"` } | json:"hosts" |  |
| `LastRefTime` | int64 | json:"lastRefTime" |  |
| `Checksum` | string | json:"checksum" |  |
| `UseSpecifiedURL` | bool | json:"useSpecifiedURL" |  |
| `Env` | string | json:"env" |  |
| `ProtectThreshold` | interface{} | json:"protectThreshold" |  |
| `ReachLocalSiteCallThreshold` | interface{} | json:"reachLocalSiteCallThreshold" |  |
| `Dom` | string | json:"dom" |  |
| `Metadata` | map[string]string | json:"metadata" |  |

## `github.com/maczh/mgin/v2/pkg/registry/etcd`

```go
import "github.com/maczh/mgin/v2/pkg/registry/etcd"
```

### 类型

#### `EtcdClient`

```go
type EtcdClient struct
```

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|

**方法**

```go
func (c *EtcdClient) Register(etcdConfigData []byte)
```

```go
func (c *EtcdClient) GetServiceURL(servicename string, groupName ...string) (string, string)
```

```go
func (c *EtcdClient) GetServices(servicename string, groupName ...string) ([]string, error)
```

GetServices v2 新增：返回该服务在 etcd 上注册的全部实例 URL 列表。
与 GetServiceURL 走同一条 prefix 扫描路径，只是把"随机选一个"改为"全部返回"。

```go
func (c *EtcdClient) DeRegister()
```

## `github.com/maczh/mgin/v2/pkg/registry/consul`

```go
import "github.com/maczh/mgin/v2/pkg/registry/consul"
```

### 类型

#### `ConsulClient`

```go
type ConsulClient struct
```

ConsulClient 是实现 RegistryClient 接口的结构体

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|

**方法**

```go
func (c *ConsulClient) Register(etcdConfigData []byte)
```

Register 方法用于向 Consul 注册服务
etcdConfigData 是包含 Consul 配置的字节切片

```go
func (c *ConsulClient) GetServiceURL(servicename string, groupName ...string) (string, string)
```

GetServiceURL 方法用于从 Consul 查询服务地址
servicename 是要查询的服务名称
groupName 是可选的服务组名称

```go
func (c *ConsulClient) GetServices(servicename string, groupName ...string) ([]string, error)
```

GetServices v2 新增：返回该服务在 Consul 上的全部健康实例 URL 列表。
与 GetServiceURL 走同一条 Health().Service 路径，只是不再随机选 1 个。

```go
func (c *ConsulClient) DeRegister()
```

DeRegister 方法用于从 Consul 注销服务

## `github.com/maczh/mgin/v2/pkg/registry/polaris`

```go
import "github.com/maczh/mgin/v2/pkg/registry/polaris"
```

### 常量

| 名称 | 类型/值 | 说明 |
|------|---------|------|
| `SUCCESS` | = 200000 |  |
| `EXISTS` | = 400201 |  |

### 类型

#### `ServiceCreateRequest`

```go
type ServiceCreateRequest struct
```

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `Name` | string | json:"name" |  |
| `Namespace` | string | json:"namespace" |  |

#### `InstanceRegisterRequest`

```go
type InstanceRegisterRequest struct
```

InstanceRegisterRequest 注册服务请求

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `Service` | string | json:"service" | 必选，服务名 |
| `Namespace` | string | json:"namespace" | 必选，命名空间 |
| `Host` | string | json:"host" | 必选，服务监听host，支持IPv4/IPv6地址或主机域名 |
| `Port` | int | json:"port" | 必选，服务实例监听port |
| `Protocol` | *string | json:"protocol" | 服务协议 |
| `Weight` | *int | json:"weight" | 服务权重，默认100，范围0-10000 |
| `Priority` | *int | json:"priority" | 实例优先级，默认为0，数值越小，优先级越高 |
| `Version` | *string | json:"version" | 实例提供服务版本号 |
| `Metadata` | map[string]string | json:"metadata" | 用户自定义metadata信息 |
| `Healthy` | *bool | json:"healthy" | 该服务实例是否健康，默认健康 |

**方法**

```go
func (r *InstanceRegisterRequest) SetProtocol(proto string)
```

```go
func (r *InstanceRegisterRequest) SetWeight(weight int)
```

```go
func (r *InstanceRegisterRequest) SetHealthy(healthy bool)
```

#### `ServiceCreateResponse`

```go
type ServiceCreateResponse struct
```

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `Code` | int | json:"code" |  |
| `Info` | string | json:"info" |  |
| `Size` | int | json:"size" |  |
| `Responses` | []struct { Code int `json:"code"` Info string `json:"info"` Client interface{} `json:"client"` Namespace interface{} `json:"namespace"` Service struct { Name string `json:"name"` Namespace string `json:"namespace"` Metadata map[string]string `json:"metadata"` Ports interface{} `json:"ports"` Business interface{} `json:"business"` Department interface{} `json:"department"` CmdbMod1 interface{} `json:"cmdb_mod1"` CmdbMod2 interface{} `json:"cmdb_mod2"` CmdbMod3 interface{} `json:"cmdb_mod3"` Comment interface{} `json:"comment"` Owners interface{} `json:"owners"` Token string `json:"token"` Ctime interface{} `json:"ctime"` Mtime interface{} `json:"mtime"` Revision interface{} `json:"revision"` PlatformId interface{} `json:"platform_id"` TotalInstanceCount interface{} `json:"total_instance_count"` HealthyInstanceCount interface{} `json:"healthy_instance_count"` UserIds []interface{} `json:"user_ids"` GroupIds []interface{} `json:"group_ids"` RemoveUserIds []interface{} `json:"remove_user_ids"` RemoveGroupIds []interface{} `json:"remove_group_ids"` Id string `json:"id"` Editable interface{} `json:"editable"` ExportTo []interface{} `json:"export_to"` } `json:"service"` Instance interface{} `json:"instance"` Routing interface{} `json:"routing"` Alias interface{} `json:"alias"` RateLimit interface{} `json:"rateLimit"` CircuitBreaker interface{} `json:"circuitBreaker"` ConfigRelease interface{} `json:"configRelease"` User interface{} `json:"user"` UserGroup interface{} `json:"userGroup"` AuthStrategy interface{} `json:"authStrategy"` Relation interface{} `json:"relation"` LoginResponse interface{} `json:"loginResponse"` ModifyAuthStrategy interface{} `json:"modifyAuthStrategy"` ModifyUserGroup interface{} `json:"modifyUserGroup"` Resources interface{} `json:"resources"` OptionSwitch interface{} `json:"optionSwitch"` InstanceLabels interface{} `json:"instanceLabels"` Data interface{} `json:"data"` ServiceContract interface{} `json:"serviceContract"` } | json:"responses" |  |

#### `InstanceRegisterResponse`

```go
type InstanceRegisterResponse struct
```

InstanceRegisterResponse 注册服务应答

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `Code` | int | json:"code" |  |
| `Info` | string | json:"info" |  |
| `Size` | int | json:"size" |  |
| `Responses` | []struct { Code int `json:"code"` Info string `json:"info"` Client interface{} `json:"client"` Namespace interface{} `json:"namespace"` Service interface{} `json:"service"` Instance struct { Id string `json:"id"` Service string `json:"service"` Namespace string `json:"namespace"` VpcId interface{} `json:"vpc_id"` Host string `json:"host"` Port int `json:"port"` Protocol *string `json:"protocol"` Version interface{} `json:"version"` Priority interface{} `json:"priority"` Weight interface{} `json:"weight"` EnableHealthCheck interface{} `json:"enableHealthCheck"` HealthCheck interface{} `json:"healthCheck"` Healthy interface{} `json:"healthy"` Isolate interface{} `json:"isolate"` Location interface{} `json:"location"` Metadata map[string]string `json:"metadata"` LogicSet interface{} `json:"logic_set"` Ctime interface{} `json:"ctime"` Mtime interface{} `json:"mtime"` Revision interface{} `json:"revision"` ServiceToken interface{} `json:"service_token"` } `json:"instance"` Routing interface{} `json:"routing"` Alias interface{} `json:"alias"` RateLimit interface{} `json:"rateLimit"` CircuitBreaker interface{} `json:"circuitBreaker"` ConfigRelease interface{} `json:"configRelease"` User interface{} `json:"user"` UserGroup interface{} `json:"userGroup"` AuthStrategy interface{} `json:"authStrategy"` Relation interface{} `json:"relation"` LoginResponse interface{} `json:"loginResponse"` ModifyAuthStrategy interface{} `json:"modifyAuthStrategy"` ModifyUserGroup interface{} `json:"modifyUserGroup"` Resources interface{} `json:"resources"` OptionSwitch interface{} `json:"optionSwitch"` InstanceLabels interface{} `json:"instanceLabels"` Data interface{} `json:"data"` ServiceContract interface{} `json:"serviceContract"` } | json:"responses" |  |

#### `QueryInstanceResponse`

```go
type QueryInstanceResponse struct
```

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `Code` | int | json:"code" |  |
| `Info` | string | json:"info" |  |
| `Amount` | int | json:"amount" |  |
| `Size` | int | json:"size" |  |
| `Namespaces` | []interface{} | json:"namespaces" |  |
| `Services` | []interface{} | json:"services" |  |
| `Instances` | []struct { Id string `json:"id"` Service string `json:"service"` Namespace string `json:"namespace"` VpcId interface{} `json:"vpc_id"` Host string `json:"host"` Port int `json:"port"` Protocol string `json:"protocol"` Version interface{} `json:"version"` Priority interface{} `json:"priority"` Weight int `json:"weight"` EnableHealthCheck interface{} `json:"enableHealthCheck"` HealthCheck interface{} `json:"healthCheck"` Healthy bool `json:"healthy"` Isolate bool `json:"isolate"` Location interface{} `json:"location"` Metadata map[string]string `json:"metadata"` LogicSet interface{} `json:"logic_set"` Ctime string `json:"ctime"` Mtime string `json:"mtime"` Revision string `json:"revision"` ServiceToken string `json:"service_token"` } | json:"instances" |  |
| `Routings` | []interface{} | json:"routings" |  |
| `Aliases` | []interface{} | json:"aliases" |  |
| `RateLimits` | []interface{} | json:"rateLimits" |  |
| `ConfigWithServices` | []interface{} | json:"configWithServices" |  |
| `Users` | []interface{} | json:"users" |  |
| `UserGroups` | []interface{} | json:"userGroups" |  |
| `AuthStrategies` | []interface{} | json:"authStrategies" |  |
| `Clients` | []interface{} | json:"clients" |  |
| `Data` | []interface{} | json:"data" |  |
| `Summary` | interface{} | json:"summary" |  |

#### `DeregisterInstanceRequest`

```go
type DeregisterInstanceRequest struct
```

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `ID` | string | json:"id,omitempty" | 实例ID，如果不设置 service、namespace、host、port，则必须设置 ID 字段 |
| `Host` | string | json:"host,omitempty" | 实例IP |
| `Namespace` | string | json:"namespace,omitempty" | 命名空间 |
| `Port` | int64 | json:"port,omitempty" | 实例端口 |
| `Service` | string | json:"service,omitempty" | 服务名称 |

#### `DeregisterInstanceResponse`

```go
type DeregisterInstanceResponse struct
```

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `Code` | int | json:"code" |  |
| `Info` | string | json:"info" |  |
| `Size` | int | json:"size" |  |
| `Responses` | []struct { Code int `json:"code"` Info string `json:"info"` Client interface{} `json:"client"` Namespace interface{} `json:"namespace"` Service interface{} `json:"service"` Instance struct { Id *string `json:"id"` Service string `json:"service"` Namespace string `json:"namespace"` VpcId interface{} `json:"vpc_id"` Host string `json:"host"` Port int `json:"port"` Protocol interface{} `json:"protocol"` Version interface{} `json:"version"` Priority interface{} `json:"priority"` Weight interface{} `json:"weight"` EnableHealthCheck interface{} `json:"enableHealthCheck"` HealthCheck interface{} `json:"healthCheck"` Healthy interface{} `json:"healthy"` Isolate interface{} `json:"isolate"` Location interface{} `json:"location"` Metadata struct { } `json:"metadata"` LogicSet interface{} `json:"logic_set"` Ctime interface{} `json:"ctime"` Mtime interface{} `json:"mtime"` Revision interface{} `json:"revision"` ServiceToken interface{} `json:"service_token"` } `json:"instance"` Routing interface{} `json:"routing"` Alias interface{} `json:"alias"` RateLimit interface{} `json:"rateLimit"` CircuitBreaker interface{} `json:"circuitBreaker"` ConfigRelease interface{} `json:"configRelease"` User interface{} `json:"user"` UserGroup interface{} `json:"userGroup"` AuthStrategy interface{} `json:"authStrategy"` Relation interface{} `json:"relation"` LoginResponse interface{} `json:"loginResponse"` ModifyAuthStrategy interface{} `json:"modifyAuthStrategy"` ModifyUserGroup interface{} `json:"modifyUserGroup"` Resources interface{} `json:"resources"` OptionSwitch interface{} `json:"optionSwitch"` InstanceLabels interface{} `json:"instanceLabels"` Data interface{} `json:"data"` ServiceContract interface{} `json:"serviceContract"` } | json:"responses" |  |

#### `DeleteServiceResponse`

```go
type DeleteServiceResponse struct
```

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `Code` | int | json:"code" |  |
| `Info` | string | json:"info" |  |
| `Size` | int | json:"size" |  |
| `Responses` | []struct { Code int `json:"code"` Info string `json:"info"` Client interface{} `json:"client"` Namespace interface{} `json:"namespace"` Service struct { Name string `json:"name"` Namespace string `json:"namespace"` Metadata map[string]string `json:"metadata"` Ports interface{} `json:"ports"` Business interface{} `json:"business"` Department interface{} `json:"department"` CmdbMod1 interface{} `json:"cmdb_mod1"` CmdbMod2 interface{} `json:"cmdb_mod2"` CmdbMod3 interface{} `json:"cmdb_mod3"` Comment interface{} `json:"comment"` Owners interface{} `json:"owners"` Token interface{} `json:"token"` Ctime interface{} `json:"ctime"` Mtime interface{} `json:"mtime"` Revision interface{} `json:"revision"` PlatformId interface{} `json:"platform_id"` TotalInstanceCount interface{} `json:"total_instance_count"` HealthyInstanceCount interface{} `json:"healthy_instance_count"` UserIds []interface{} `json:"user_ids"` GroupIds []interface{} `json:"group_ids"` RemoveUserIds []interface{} `json:"remove_user_ids"` RemoveGroupIds []interface{} `json:"remove_group_ids"` Id interface{} `json:"id"` Editable interface{} `json:"editable"` ExportTo []interface{} `json:"export_to"` } `json:"service"` Instance interface{} `json:"instance"` Routing interface{} `json:"routing"` Alias interface{} `json:"alias"` RateLimit interface{} `json:"rateLimit"` CircuitBreaker interface{} `json:"circuitBreaker"` ConfigRelease interface{} `json:"configRelease"` User interface{} `json:"user"` UserGroup interface{} `json:"userGroup"` AuthStrategy interface{} `json:"authStrategy"` Relation interface{} `json:"relation"` LoginResponse interface{} `json:"loginResponse"` ModifyAuthStrategy interface{} `json:"modifyAuthStrategy"` ModifyUserGroup interface{} `json:"modifyUserGroup"` Resources interface{} `json:"resources"` OptionSwitch interface{} `json:"optionSwitch"` InstanceLabels interface{} `json:"instanceLabels"` Data interface{} `json:"data"` ServiceContract interface{} `json:"serviceContract"` } | json:"responses" |  |

#### `PolarisClient`

```go
type PolarisClient struct
```

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|

**方法**

```go
func (c *PolarisClient) Register(registryConfigData []byte)
```

```go
func (c *PolarisClient) GetServiceURL(servicename string, namespaces ...string) (string, string)
```

```go
func (c *PolarisClient) GetServices(servicename string, namespaces ...string) ([]string, error)
```

GetServices v2 新增：返回该服务在 Polaris 上的全部健康实例 URL 列表。
与 GetServiceURL 走同一条 /naming/v1/instances 路径。

```go
func (c *PolarisClient) DeRegister()
```

