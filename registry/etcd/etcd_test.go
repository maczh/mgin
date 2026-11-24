package etcd

import (
	"fmt"
	"testing"

	"github.com/maczh/mgin/config"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// 测试使用的 etcd 接入地址。
const (
	testEtcdServer   = "118.31.124.97"
	testEtcdPort     = "12379"
	testEtcdEndpoint = "http://" + testEtcdServer + ":" + testEtcdPort
	testEtcdService  = "openapi-user"
	testEtcdGroup    = "OpenApi"
)

func testEtcdConfig() []byte {
	// lan 置为 false：开发机通常只有内网地址，按 lanNet 前缀匹配会取不到本机 IP 导致注册失败。
	return []byte(fmt.Sprintf(`go:
  etcd:
    server: %s
    port: %s
    clusterName: DEFAULT
    group: %s
    weight: 1
    lan: false`, testEtcdServer, testEtcdPort, testEtcdGroup))
}

func testEtcdClient(t *testing.T) *EtcdClient {
	t.Helper()
	client := &EtcdClient{}
	config.Config.App.Port = 8080
	config.Config.App.Name = testEtcdService
	config.Config.App.Project = "openapi"
	client.Register(testEtcdConfig())
	if client.registeredKey == "" {
		t.Fatalf("连接 etcd %s 注册失败，无法继续测试", testEtcdEndpoint)
	}
	return client
}

func TestEtcdClient_Register(t *testing.T) {
	client := testEtcdClient(t)
	// 测试写入的是共享 etcd，结束时注销，避免残留实例。
	defer client.DeRegister()
}

func TestEtcdClient_GetServiceURL(t *testing.T) {
	client := testEtcdClient(t)
	defer client.DeRegister()
	url, group := client.GetServiceURL(testEtcdService, testEtcdGroup)
	logger.Debug(fmt.Sprintf("url=%s, group=%s", url, group))
	if url == "" || group != testEtcdGroup {
		t.Fatalf("未发现已注册服务 %s/%s: url=%q, group=%q", testEtcdGroup, testEtcdService, url, group)
	}
}

func TestEtcdClient_DeRegister(t *testing.T) {
	client := testEtcdClient(t)
	client.DeRegister()

	// DeRegister 会关闭客户端，另起一个只读客户端确认注册信息已被清除。
	reader := &EtcdClient{cluster: "DEFAULT", group: testEtcdGroup, keyTemplate: "{prefix}/{group}/{service}/{instance}"}
	reader.client, _ = clientv3.New(clientv3.Config{Endpoints: []string{testEtcdEndpoint}})
	defer reader.client.Close()
	if url, _ := reader.GetServiceURL(testEtcdService, testEtcdGroup); url != "" {
		t.Fatalf("注销后仍能查到服务: %s", url)
	}
}
