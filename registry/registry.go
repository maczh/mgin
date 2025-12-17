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
