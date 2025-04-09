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
		App:       app{Project: "openapi", Name: "openapi-user", Port: 8085},
		Config:    appConfig{Server: "http://172.29.43.134:8500", Env: "test", Type: "consul"},
		Log:       appLog{},
		Logger:    appLogger{},
		Discovery: discovery{},
	}
	ymlData := cfg.GetConfigData("mysql")
	fmt.Println(string(ymlData))
}

func TestConfig_GetConfigData_Etcd(t *testing.T) {
	yml := `go:
  data:
    mysql: user:pwd@tcp(xxx.xxx.xxx.xxx:3306)/dbname?charset=utf8&parseTime=True&loc=Local
    mysql_debug: true   #打开调试模式
    mysql_pool:     #连接池设置,若无此项则使用单一长连接
      max: 200      #实际最大连接数
      total: 1000   #最大并发数,不填默认为最大连接数5倍
      timeout: 30   #空闲连接超时，秒，默认60秒
      life: 5       #连接生命周期，分钟，默认60分钟`
	cfg := &config{
		Cnf:       nil,
		App:       app{Project: "openapi", Name: "openapi-user", Port: 8085},
		Config:    appConfig{Server: "http://172.29.43.134:2379", Env: "test", Type: "etcd"},
		Log:       appLog{},
		Logger:    appLogger{},
		Discovery: discovery{},
	}
	cli, err := clientv3.New(clientv3.Config{Endpoints: []string{cfg.Config.Server}})
	if err != nil {
		fmt.Println(err)
	}
	resp, err := cli.Put(context.Background(), fmt.Sprintf("/config/%s/%s-%s.yml", cfg.App.Project, "mysql", cfg.Config.Env), yml)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(resp)
	ymlData := cfg.GetConfigData("mysql")
	fmt.Println(string(ymlData))
}
