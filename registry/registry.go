package registry

import (
	"github.com/maczh/mgin/config"
	"github.com/maczh/mgin/registry/consul"
	"github.com/maczh/mgin/registry/etcd"
	"github.com/maczh/mgin/registry/nacos"
	"github.com/maczh/mgin/registry/polaris"
)

var Registry RegistryClient

type RegistryClient interface {
	Register(registryConfigData []byte)
	GetServiceURL(servicename string, groupName ...string) (string, string)
	DeRegister()
}

func NewRegistry() RegistryClient {
	var client RegistryClient
	switch config.Config.Discovery.Registry {
	case "nacos":
		client = &nacos.NacosClient{}
	case "etcd":
		client = &etcd.EtcdClient{}
	case "consul":
		client = &consul.ConsulClient{}
	case "polaris":
		client = &polaris.PolarisClient{}
	default:
		// 未配置或未知的注册中心类型时返回空实现，避免调用方对 nil 接口调用方法而 panic
		client = &noopRegistryClient{}
	}
	return client
}

// noopRegistryClient 在未配置注册中心时使用的空实现，保证接口调用不会 panic
type noopRegistryClient struct{}

func (n *noopRegistryClient) Register(registryConfigData []byte) {}
func (n *noopRegistryClient) GetServiceURL(servicename string, groupName ...string) (string, string) {
	return "", ""
}
func (n *noopRegistryClient) DeRegister() {}
