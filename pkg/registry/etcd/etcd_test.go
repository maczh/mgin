package etcd

import (
	"fmt"
	"github.com/maczh/mgin/v2/pkg/config"
	clientv3 "go.etcd.io/etcd/client/v3"
	"testing"
)

func TestEtcdClient_Register(t *testing.T) {
	ymlData := `go:
  etcd:
    server:  192.168.110.15   #etcd服务IP
    port: 2379            #etcd端口
    clusterName: DEFAULT
    group: OpenApi    #根据项目不同配置不同分组
    weight: 1
    lan: true   #以内网地址注册，否则以公网地址注册
    lanNet: 192.168.110.    #网段前缀`
	etcdClient := &EtcdClient{}
	config.Config.App.Port = 8080
	config.Config.App.Name = "openapi-user"
	config.Config.App.Project = "openapi"

	etcdClient.Register([]byte(ymlData))
}

func TestEtcdClient_GetServiceURL(t *testing.T) {
	client := &EtcdClient{cluster: "DEFAULT"}
	client.client, _ = clientv3.New(clientv3.Config{Endpoints: []string{"http://192.168.110.15:2379"}})
	url, group := client.GetServiceURL("openapi-user", "OpenApi")
	logger.Debug(fmt.Sprintf("url=%s, group=%s", url, group))
}

func TestEtcdClient_DeRegister(t *testing.T) {
	client := &EtcdClient{cluster: "DEFAULT", group: "OpenApi", lan: true, lanNetwork: "192.168.110."}
	client.client, _ = clientv3.New(clientv3.Config{Endpoints: []string{"http://192.168.110.15:2379"}})
	config.Config.App.Port = 8080
	config.Config.App.Name = "openapi-user"
	config.Config.App.Project = "openapi"
	client.DeRegister()
	logger.Debug("=====重新查询=====")
	TestEtcdClient_GetServiceURL(t)
}
