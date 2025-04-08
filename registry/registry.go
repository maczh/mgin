package registry

import (
	"github.com/maczh/mgin/config"
	"github.com/maczh/mgin/registry/etcd"
	"github.com/maczh/mgin/registry/nacos"
	"github.com/nacos-group/nacos-sdk-go/vo"
)

var Registry RegistryClient

type RegistryClient interface {
	Register(etcdConfigUrl string)
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
	}
	return client
}
