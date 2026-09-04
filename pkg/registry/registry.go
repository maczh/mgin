package registry

import (
	"github.com/maczh/mgin/v2/pkg/config"
	"github.com/maczh/mgin/v2/pkg/registry/consul"
	"github.com/maczh/mgin/v2/pkg/registry/etcd"
	"github.com/maczh/mgin/v2/pkg/registry/nacos"
	"github.com/maczh/mgin/v2/pkg/registry/polaris"
)

var Registry RegistryClient

// RegistryClient 是 mgin v2 扩展后的统一注册中心客户端接口。
//
// v1 接口：Register / GetServiceURL / DeRegister（保持不变，存量项目无需改动）。
// v2 新增：GetServices 多实例列表，供 client.CallCtx 的负载均衡使用。
type RegistryClient interface {
	// Register 把本服务注册到配置中心。
	Register(registryConfigData []byte)
	// GetServiceURL v1 单实例返回，保留向后兼容。
	// 内部实现通常从多实例中随机选 1 个返回，与 GetServices 等价。
	GetServiceURL(servicename string, groupName ...string) (string, string)
	// GetServices v2 新增：返回该服务的所有可用实例 URL 列表。
	// 用于客户端负载均衡（pkg/loadbalancer）。
	// 找不到实例时返 (nil, nil)，错误时返 (nil, err)。
	GetServices(servicename string, groupName ...string) ([]string, error)
	// DeRegister 注销本服务。
	DeRegister()
}

func NewRegistry() RegistryClient {
	var client RegistryClient
	switch config.Config.Discovery.Registry {
	case "nacos":
		client = &nacos.NacosClient{}
		break
	case "etcd":
		client = &etcd.EtcdClient{}
		break
	case "consul":
		client = &consul.ConsulClient{}
		break
	case "polaris":
		client = &polaris.PolarisClient{}
		break
	}
	return client
}
