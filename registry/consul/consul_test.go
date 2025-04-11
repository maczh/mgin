package consul

import (
	"fmt"
	"github.com/hashicorp/consul/api"
	"github.com/maczh/mgin/config"
	"testing"
)

func TestConsulClient_Register(t *testing.T) {
	ymlData := `go:
  consul:
    server:  192.168.110.15   #Consul服务IP
    port: 8500            #Consul端口
    clusterName: DEFAULT
    group: OpenApi    #根据项目不同配置不同分组
    weight: 1
    lan: true   #以内网地址注册，否则以公网地址注册
    lanNet: 192.168.110.    #网段前缀`
	client := &ConsulClient{}
	config.Config.App.Port = 8080
	config.Config.App.Name = "openapi-user"
	config.Config.App.Project = "openapi"

	client.Register([]byte(ymlData))
}

func TestConsulClient_GetServiceURL(t *testing.T) {
	client := &ConsulClient{cluster: "DEFAULT"}
	client.client, _ = api.NewClient(&api.Config{Address: "192.168.110.15:8500"})
	url, group := client.GetServiceURL("openapi-user", "OpenApi")
	logger.Debug(fmt.Sprintf("url=%s, group=%s", url, group))
}

func TestConsulClient_DeRegister(t *testing.T) {
	client := &ConsulClient{cluster: "DEFAULT", group: "OpenApi", lan: true, lanNetwork: "192.168.110."}
	client.client, _ = api.NewClient(&api.Config{Address: "192.168.110.15:8500"})
	config.Config.App.Port = 8080
	config.Config.App.Name = "openapi-user"
	config.Config.App.Project = "openapi"
	client.DeRegister()
	logger.Debug("=====重新查询=====")
	TestConsulClient_GetServiceURL(t)
}
