package consul

import (
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"math/rand"
	"net"
	"strings"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/knadh/koanf"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/rawbytes"
	"github.com/maczh/mgin/config"
	registryModel "github.com/maczh/mgin/registry/model"
	"github.com/sadlil/gologger"
)

// ConsulClient 是实现 RegistryClient 接口的结构体
type ConsulClient struct {
	client     *api.Client
	clients    []*api.Client
	cluster    string
	namespace  string
	group      string
	lan        bool
	lanNetwork string
	conf       *koanf.Koanf
	confData   []byte
	instanceID string
	ctx        context.Context
	cancel     context.CancelFunc
}

var logger = gologger.GetLogger()

// Register 方法用于向 Consul 注册服务
// etcdConfigData 是包含 Consul 配置的字节切片
func (c *ConsulClient) Register(etcdConfigData []byte) {
	if etcdConfigData == nil || len(etcdConfigData) == 0 {
		logger.Error("Consul 配置错误，无法获取配置地址")
		return
	}
	c.confData = etcdConfigData
	if c.conf == nil {
		c.conf = koanf.New(".")
		err := c.conf.Load(rawbytes.Provider(c.confData), yaml.Parser())
		if err != nil {
			logger.Error("Consul 注册中心配置文件解析错误:" + err.Error())
			c.conf = nil
			return
		}
		c.lan = c.conf.Bool("go.consul.lan")
		c.lanNetwork = c.conf.String("go.consul.lanNet")
		ipstr := c.conf.String("go.consul.server")
		portstr := c.conf.String("go.consul.port")
		c.group = c.conf.String("go.consul.group")
		if c.group == "" {
			c.group = "DEFAULT_GROUP"
		}
		c.cluster = c.conf.String("go.consul.clusterName")
		c.namespace = c.conf.String("go.consul.namespace")
		if c.cluster == "" {
			c.cluster = "DEFAULT"
		}
		ips := strings.Split(ipstr, ",")
		ports := strings.Split(portstr, ",")
		consul_urls := make([]string, 0)
		for i, ip := range ips {
			if i < len(ports) {
				consul_urls = append(consul_urls, fmt.Sprintf("%s:%s", ip, ports[i]))
			}
		}
		if len(consul_urls) == 0 {
			logger.Error("Consul 注册中心地址配置为空")
			return
		}
		for _, address := range consul_urls {
			serverConfig := api.DefaultConfig()
			serverConfig.Address = address
			serverConfig.Token = c.conf.String("go.consul.token")
			serverConfig.Namespace = c.namespace
			client, clientErr := api.NewClient(serverConfig)
			if clientErr == nil {
				c.clients = append(c.clients, client)
			}
		}
		if len(c.clients) == 0 {
			logger.Error("Consul 服务连接失败")
			return
		}
		c.client = c.clients[0]
		localip, err := localIPv4s(c.lan, c.lanNetwork)
		if err != nil || len(localip) == 0 {
			logger.Error("Consul 注册中心获取本机IP失败")
			return
		}
		ip := localip[0]
		if config.Config.App.IpAddr != "" {
			ip = config.Config.App.IpAddr
		}
		port := uint64(config.Config.App.Port)
		protocol := "http://"
		if port == 0 || config.Config.App.PortSSL != 0 {
			port = uint64(config.Config.App.PortSSL)
			protocol = "https://"
		}
		c.instanceID = getInstanceId(ip, port, config.Config.App.Name, c.namespace, c.group, c.cluster)
		registration := &api.AgentServiceRegistration{
			ID:      c.instanceID,
			Name:    config.Config.App.Name,
			Address: ip,
			Port:    int(port),
			Tags:    []string{c.group, c.cluster, protocol},
			Check: &api.AgentServiceCheck{
				DeregisterCriticalServiceAfter: "10m",
				TLSSkipVerify:                  false,
				TTL:                            "30s",
			},
		}
		err = c.client.Agent().ServiceRegister(registration)
		if err != nil {
			logger.Error("Consul 注册服务失败:" + err.Error())
			return
		}
		c.ctx, c.cancel = context.WithCancel(context.Background())
		go c.passTTL()
	}
}

// GetServiceURL 方法用于从 Consul 查询服务地址
// servicename 是要查询的服务名称
// groupName 是可选的服务组名称
func (c *ConsulClient) GetServiceURL(servicename string, groupName ...string) (string, string) {
	if c.client == nil {
		return "", c.group
	}
	if len(groupName) == 0 {
		groupName = []string{c.group}
	} else if groupName[0] == "" {
		groupName[0] = c.group
	}
	currentGroup := groupName[0]
	clients := c.clients
	if len(clients) == 0 {
		clients = []*api.Client{c.client}
	}
	for _, group := range groupName {
		for _, consulClient := range clients {
			services, _, err := consulClient.Health().Service(servicename, group, true, &api.QueryOptions{})
			services = filterCluster(services, c.cluster)
			if err != nil || len(services) == 0 {
				continue
			}
			currentGroup = group
			r := rand.New(rand.NewSource(time.Now().UnixNano()))
			service := services[r.Intn(len(services))]
			protocol := "http://"
			if len(service.Service.Tags) >= 3 {
				protocol = service.Service.Tags[2]
			}
			address := fmt.Sprintf("%s%s:%d", protocol, service.Service.Address, service.Service.Port)
			return address, currentGroup
		}
	}
	return "", currentGroup
}

func (c *ConsulClient) GetServiceInstances(servicename string, groupName ...string) ([]registryModel.ServiceInstance, string) {
	group := c.group
	if len(groupName) > 0 && groupName[0] != "" {
		group = groupName[0]
	}
	clients := c.clients
	if len(clients) == 0 && c.client != nil {
		clients = []*api.Client{c.client}
	}
	for _, client := range clients {
		services, _, err := client.Health().Service(servicename, group, true, &api.QueryOptions{})
		services = filterCluster(services, c.cluster)
		if err != nil || len(services) == 0 {
			continue
		}
		out := make([]registryModel.ServiceInstance, 0, len(services))
		for _, service := range services {
			protocol := "http://"
			if len(service.Service.Tags) >= 3 {
				protocol = service.Service.Tags[2]
			}
			weight := 1
			for _, tag := range service.Service.Tags {
				if strings.HasPrefix(tag, "weight=") {
					if _, err := fmt.Sscanf(tag, "weight=%d", &weight); err != nil {
						weight = 1
					}
				}
			}
			out = append(out, registryModel.ServiceInstance{URL: fmt.Sprintf("%s%s:%d", protocol, service.Service.Address, service.Service.Port), Weight: weight})
		}
		return out, group
	}
	return nil, group
}

func filterCluster(services []*api.ServiceEntry, cluster string) []*api.ServiceEntry {
	if cluster == "" {
		return services
	}
	filtered := make([]*api.ServiceEntry, 0, len(services))
	for _, service := range services {
		for _, tag := range service.Service.Tags {
			if tag == cluster {
				filtered = append(filtered, service)
				break
			}
		}
	}
	return filtered
}

// WatchService polls Consul health with edge-triggered semantics. The cancel
// function is safe to call once and is used by CachedRegistry on shutdown.
func (c *ConsulClient) WatchService(service, group string, callback func()) (func(), error) {
	if c.client == nil || callback == nil {
		return nil, fmt.Errorf("consul is not initialized")
	}
	if group == "" {
		group = c.group
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		available := true
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				found := false
				clients := c.clients
				if len(clients) == 0 {
					clients = []*api.Client{c.client}
				}
				for _, client := range clients {
					instances, _, err := client.Health().Service(service, group, true, &api.QueryOptions{})
					instances = filterCluster(instances, c.cluster)
					if err == nil && len(instances) > 0 {
						found = true
						break
					}
				}
				if available && !found {
					callback()
				}
				available = found
			}
		}
	}()
	return cancel, nil
}

// DeRegister 方法用于从 Consul 注销服务
func (c *ConsulClient) DeRegister() {
	if c.cancel != nil {
		c.cancel()
	}
	if c.client == nil {
		logger.Error("Consul 未初始化，无法注销服务")
		return
	}
	id := c.instanceID
	if id == "" {
		// 兜底：按本机 IP/Port 重新计算实例ID
		localip, err := localIPv4s(c.lan, c.lanNetwork)
		if err == nil && len(localip) > 0 {
			ip := localip[0]
			if config.Config.App.IpAddr != "" {
				ip = config.Config.App.IpAddr
			}
			port := uint64(config.Config.App.Port)
			if port == 0 || config.Config.App.PortSSL != 0 {
				port = uint64(config.Config.App.PortSSL)
			}
			id = getInstanceId(ip, port, config.Config.App.Name, c.namespace, c.group, c.cluster)
		}
	}
	if id == "" {
		logger.Error("Consul 无法确定实例ID，无法注销服务")
		return
	}
	err := c.client.Agent().ServiceDeregister(id)
	if err != nil {
		logger.Error("Consul 取消注册服务失败:" + err.Error())
		return
	}
}

// passTTL 周期性向 Consul 上报 TTL 健康检查通过，避免实例被自动反注册。
func (c *ConsulClient) passTTL() {
	if c.client == nil || c.instanceID == "" {
		return
	}
	ticker := time.NewTicker(25 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			err := c.client.Agent().UpdateTTL(c.instanceID, "", api.HealthPassing)
			if err != nil {
				logger.Error("Consul TTL健康检查更新失败: " + err.Error())
			}
		}
	}
}

// localIPv4s 函数用于获取本地 IPv4 地址
// lan 表示是否使用局域网地址
// lanNetwork 是局域网网络前缀
func localIPv4s(lan bool, lanNetwork string) ([]string, error) {
	var ips, ipLans, ipWans []string
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ips, err
	}

	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok && ipnet.IP.IsGlobalUnicast() && ipnet.IP.To4() != nil {
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

// getInstanceId 函数用于生成服务实例 ID
func getInstanceId(ip string, port uint64, scope ...string) string {
	h := md5.New()
	_, _ = io.WriteString(h, fmt.Sprintf("http://%s:%d/%s", ip, port, strings.Join(scope, "/")))
	md := fmt.Sprintf("%x", h.Sum(nil))[:12]
	return md
}
