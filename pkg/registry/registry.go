package registry

import (
	"github.com/maczh/mgin/pkg/config"
	"github.com/maczh/mgin/pkg/registry/consul"
	"github.com/maczh/mgin/pkg/registry/etcd"
	"github.com/maczh/mgin/pkg/registry/nacos"
	"github.com/maczh/mgin/pkg/registry/polaris"
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
