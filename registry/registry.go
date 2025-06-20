package registry

import (
	"github.com/maczh/mgin/config"
	"github.com/maczh/mgin/registry/consul"
	"github.com/maczh/mgin/registry/etcd"
	"github.com/maczh/mgin/registry/nacos"
	"github.com/maczh/mgin/registry/polaris"
	"github.com/nacos-group/nacos-sdk-go/vo"
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
		client = &nacos.NacosClient{
			Subscribes: make(map[string]*vo.SubscribeParam),
		}
	case "etcd":
		client = &etcd.EtcdClient{}
	case "consul":
		client = &consul.ConsulClient{}
	case "polaris":
		client = &polaris.PolarisClient{}
	}
	return client
}
