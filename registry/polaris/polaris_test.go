package polaris

import (
	"fmt"
	"github.com/maczh/mgin/config"
	"testing"
)

func TestPolarisClient_Register(t *testing.T) {
	ymlData := `go:
  polaris:
    server: 127.0.0.1   #polaris服务IP
    port: 8090            #polaris端口
    namespace: oss
    token: 7aqoGj/aaK631nUUIa54+vao+kVzwAJNb+2w7chkklJjLG/VkdnyvVKojbZycf4OtZZCLGSlqYj9oghc9XM=    #使用哪个用户的token
    weight: 1
    lan: true   #以内网地址注册，否则以公网地址注册
    lanNet: 192.168.113.    #网段前缀`
	client := &PolarisClient{}
	config.Config.App.Port = 8080
	config.Config.App.Name = "openapi-user"
	config.Config.App.Project = "openapi"

	client.Register([]byte(ymlData))
}

func TestPolarisClient_GetServiceURL(t *testing.T) {
	client := &PolarisClient{
		namespace:  "oss",
		apiBaseUrl: "http://127.0.0.1:8090",
		token:      "7aqoGj/aaK631nUUIa54+vao+kVzwAJNb+2w7chkklJjLG/VkdnyvVKojbZycf4OtZZCLGSlqYj9oghc9XM=",
	}
	url, group := client.GetServiceURL("openapi-user")
	logger.Debug(fmt.Sprintf("url=%s, group=%s", url, group))
}

func TestPolarisClient_DeRegister(t *testing.T) {
	client := &PolarisClient{
		namespace:  "oss",
		apiBaseUrl: "http://127.0.0.1:8090",
		token:      "7aqoGj/aaK631nUUIa54+vao+kVzwAJNb+2w7chkklJjLG/VkdnyvVKojbZycf4OtZZCLGSlqYj9oghc9XM=",
	}
	config.Config.App.Port = 8080
	config.Config.App.Name = "openapi-user"
	config.Config.App.Project = "openapi"
	client.DeRegister()
	logger.Debug("=====重新查询=====")
	TestPolarisClient_GetServiceURL(t)
}
