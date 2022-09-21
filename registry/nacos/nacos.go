package nacos

import (
	jsoniter "github.com/json-iterator/go"
	"github.com/maczh/mgin/config"
	"github.com/sadlil/gologger"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/knadh/koanf"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/rawbytes"
	"github.com/levigross/grequests"
	"github.com/nacos-group/nacos-sdk-go/clients"
	"github.com/nacos-group/nacos-sdk-go/clients/naming_client"
	"github.com/nacos-group/nacos-sdk-go/common/constant"
	"github.com/nacos-group/nacos-sdk-go/model"
	"github.com/nacos-group/nacos-sdk-go/vo"
)

type NacosClient struct {
	client     naming_client.INamingClient
	cluster    string
	group      string
	lan        bool
	lanNetwork string
	conf       *koanf.Koanf
	confUrl    string
}

var logger = gologger.GetLogger()

func (n *NacosClient) GetNacosClient() naming_client.INamingClient {
	return n.client
}

func (n *NacosClient) Register(nacosConfigUrl string) {
	if nacosConfigUrl != "" {
		n.confUrl = nacosConfigUrl
	}
	if n.confUrl == "" {
		logger.Error("Nacos配置Url为空")
		return
	}
	if n.conf == nil {
		resp, err := grequests.Get(n.confUrl, nil)
		if err != nil {
			logger.Error("Nacos配置下载失败! " + err.Error())
			return
		}
		n.conf = koanf.New(".")
		err = n.conf.Load(rawbytes.Provider([]byte(resp.String())), yaml.Parser())
		if err != nil {
			logger.Error("Nacos配置文件解析错误:" + err.Error())
			n.conf = nil
			return
		}
		path, _ := filepath.Abs(filepath.Dir(os.Args[0]))
		path += "/cache"
		_, err = os.Stat(path)
		if err != nil && os.IsNotExist(err) {
			os.Mkdir(path, 0777)
			path += "/naming"
			os.Mkdir(path, 0777)
		}
		n.lan = n.conf.Bool("go.nacos.lan")
		n.lanNetwork = n.conf.String("go.nacos.lanNet")
		serverConfigs := []constant.ServerConfig{}
		ipstr := n.conf.String("go.nacos.server")
		portstr := n.conf.String("go.nacos.port")
		n.group = n.conf.String("go.nacos.group")
		if n.group == "" {
			n.group = "DEFAULT_GROUP"
		}
		ips := strings.Split(ipstr, ",")
		ports := strings.Split(portstr, ",")
		for i, ip := range ips {
			port, _ := strconv.Atoi(ports[i])
			serverConfig := constant.ServerConfig{
				IpAddr:      ip,
				Port:        uint64(port),
				ContextPath: "/nacos",
			}
			serverConfigs = append(serverConfigs, serverConfig)
		}
		logger.Debug("Nacos服务器配置: " + toJSON(serverConfigs))
		clientConfig := constant.ClientConfig{}
		clientConfig.LogLevel = "error"
		if n.conf.Exists("go.nacos.clientConfig.logLevel") {
			clientConfig.LogLevel = n.conf.String("go.nacos.clientConfig.logLevel")
		}
		clientConfig.UpdateCacheWhenEmpty = true
		if n.conf.Exists("go.nacos.clientConfig.updateCacheWhenEmpty") {
			clientConfig.UpdateCacheWhenEmpty = n.conf.Bool("go.nacos.client.updateCacheWhenEmpty")
		}
		logger.Debug("Nacos客户端配置: " + toJSON(clientConfig))
		n.client, err = clients.CreateNamingClient(map[string]interface{}{
			"serverConfigs": serverConfigs,
			"clientConfig":  clientConfig,
		})
		if err != nil {
			logger.Error("Nacos服务连接失败:" + err.Error())
			return
		}
		localip, _ := localIPv4s(n.lan, n.lanNetwork)
		ip := localip[0]
		if config.Config.App.IpAddr != "" {
			ip = config.Config.App.IpAddr
		}
		n.cluster = n.conf.String("go.nacos.clusterName")
		port := uint64(config.Config.App.Port)
		metadata := make(map[string]string)
		if port == 0 || config.Config.App.PortSSL != 0 {
			port = uint64(config.Config.App.PortSSL)
			metadata["ssl"] = "true"
		}
		if config.Config.App.Debug {
			metadata["debug"] = "true"
		}
		success, regerr := n.client.RegisterInstance(vo.RegisterInstanceParam{
			Ip:          ip,
			Port:        port,
			ServiceName: config.Config.App.Name,
			Weight:      1,
			ClusterName: n.cluster,
			Enable:      true,
			Healthy:     true,
			Ephemeral:   true,
			Metadata:    metadata,
			GroupName:   n.group,
		})
		if !success {
			logger.Error("Nacos注册服务失败:" + regerr.Error())
			return
		}

		err = n.client.Subscribe(&vo.SubscribeParam{
			ServiceName: config.Config.App.Name,
			Clusters:    []string{n.cluster},
			GroupName:   n.group,
			SubscribeCallback: func(services []model.SubscribeService, err error) {
				logger.Debug("callback return services:" + toJSON(services))
			},
		})
		if err != nil {
			logger.Error("Nacos服务订阅失败:" + err.Error())
		}
	}

}

func (n *NacosClient) GetServiceURL(servicename string) (string, string) {
	var instance *model.Instance
	var err error
	serviceGroup := n.group
	for i := 0; i < 3; i++ {
		instance, err = n.client.SelectOneHealthyInstance(vo.SelectOneHealthInstanceParam{
			ServiceName: servicename,
			Clusters:    []string{n.cluster},
			GroupName:   serviceGroup,
		})
		if err != nil {
			serviceGroup = "DEFAULT_GROUP"
			instance, err = n.client.SelectOneHealthyInstance(vo.SelectOneHealthInstanceParam{
				ServiceName: servicename,
				Clusters:    []string{n.cluster},
				GroupName:   serviceGroup,
			})
			if err != nil {
				logger.Error("获取Nacos服务" + servicename + "失败:" + err.Error())
				return "", ""
			}
		}
		if instance.Metadata != nil && instance.Metadata["debug"] != "true" {
			break
		}
	}
	url := "http://" + instance.Ip + ":" + strconv.Itoa(int(instance.Port))
	if instance.Metadata != nil && instance.Metadata["ssl"] == "true" {
		url = "https://" + instance.Ip + ":" + strconv.Itoa(int(instance.Port))
	}
	logger.Debug("Nacos获取" + servicename + "服务成功:" + url)
	return url, serviceGroup
}

func (n *NacosClient) DeRegister() {
	err := n.client.Unsubscribe(&vo.SubscribeParam{
		ServiceName: config.Config.App.Name,
		Clusters:    []string{n.cluster},
		GroupName:   n.group,
		SubscribeCallback: func(services []model.SubscribeService, err error) {
			logger.Debug("callback return services:" + toJSON(services))
		},
	})
	if err != nil {
		logger.Error("Nacos服务订阅失败:" + err.Error())
	}
	ips, _ := localIPv4s(n.lan, n.lanNetwork)
	ip := ips[0]
	if config.Config.Exists("go.application.ip") {
		ip = config.Config.App.IpAddr
	}
	success, regerr := n.client.DeregisterInstance(vo.DeregisterInstanceParam{
		Ip:          ip,
		Port:        uint64(config.Config.App.Port),
		ServiceName: config.Config.App.Name,
		Cluster:     n.cluster,
		Ephemeral:   true,
	})
	if !success {
		logger.Error("Nacos取消注册服务失败:" + regerr.Error())
		return
	}

}

func localIPv4s(lan bool, lanNetwork string) ([]string, error) {
	var ips, ipLans, ipWans []string
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ips, err
	}

	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
			if ipnet.IP.IsPrivate() {
				ipLans = append(ipLans, ipnet.IP.String())
				if lan && strings.HasPrefix(ipnet.IP.String(), lanNetwork) {
					ips = append(ips, ipnet.IP.String())
				}
			}
			if !ipnet.IP.IsPrivate() {
				ipWans = append(ipWans, ipnet.IP.String())
				if !lan {
					ips = append(ips, ipnet.IP.String())
				}
			}
		}
	}
	if len(ips) == 0 {
		if lan {
			ips = append(ips, ipWans...)
		} else {
			ips = append(ips, ipLans...)
		}
	}
	return ips, nil
}

var json = jsoniter.ConfigCompatibleWithStandardLibrary

func toJSON(o interface{}) string {
	j, err := json.Marshal(o)
	if err != nil {
		return "{}"
	} else {
		js := string(j)
		js = strings.Replace(js, "\\u003c", "<", -1)
		js = strings.Replace(js, "\\u003e", ">", -1)
		js = strings.Replace(js, "\\u0026", "&", -1)
		return js
	}
}
