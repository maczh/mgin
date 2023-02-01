package registry

import (
	"github.com/maczh/mgin/registry/nacos"
	"github.com/nacos-group/nacos-sdk-go/vo"
)

var Nacos = &nacos.NacosClient{
	Subscribes: make(map[string]*vo.SubscribeParam),
}
