package config

import (
	"context"
	"fmt"
	clientv3 "go.etcd.io/etcd/client/v3"
	"testing"
)

func TestConfig_GetConfigData_Consul(t *testing.T) {
	cfg := &config{
		Cnf:       nil,
		App:       app{Project: "test", Name: "user", Port: 8080},
		Config:    appConfig{Server: "http://192.168.110.15:8500", Env: "test", Type: "consul"},
		Log:       appLog{},
		Logger:    appLogger{},
		Discovery: discovery{},
	}
	ymlData := cfg.GetConfigData("mysql")
	fmt.Println(string(ymlData))
}

func TestConfig_GetConfigData_Etcd(t *testing.T) {
	yml := `go:
  etcd:
    server: 192.168.110.15   #etcd服务IP
    port: 2379            #etcd端口
    clusterName: DEFAULT
    group: test    #根据项目不同配置不同分组
    weight: 1
    lan: true   #以内网地址注册，否则以公网地址注册
    lanNet: 192.168.110.    #网段前缀`
	cfg := &config{
		Cnf:       nil,
		App:       app{Project: "test", Name: "user", Port: 8080},
		Config:    appConfig{Server: "http://192.168.110.15:2379", Env: "test", Type: "etcd"},
		Log:       appLog{},
		Logger:    appLogger{},
		Discovery: discovery{},
	}
	cli, err := clientv3.New(clientv3.Config{Endpoints: []string{cfg.Config.Server}})
	if err != nil {
		fmt.Println(err)
	}
	resp, err := cli.Put(context.Background(), fmt.Sprintf("/config/%s/%s-%s.yml", cfg.App.Project, "etcd", cfg.Config.Env), yml)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(resp)
	ymlData := cfg.GetConfigData("etcd")
	fmt.Println(string(ymlData))
}
