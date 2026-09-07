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
	jsoniter "github.com/json-iterator/go"
	"github.com/knadh/koanf"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/rawbytes"
	"github.com/maczh/mgin/v2/pkg/config"
	"github.com/sadlil/gologger"
)

// ConsulClient 是实现 RegistryClient 接口的结构体
type ConsulClient struct {
	client     *api.Client
	cluster    string
	group      string
	lan        bool
	lanNetwork string
	conf       *koanf.Koanf
	confData   []byte

	// 注册稳定性增强字段
	instanceID string             // 本实例注册 ID，注销时用它精确反注册（避免重新计算 IP 导致不匹配）
	cancel     context.CancelFunc // 注销时取消 TTL 续活协程
}

var logger = gologger.GetLogger()

// Register 方法用于向 Consul 注册服务
// etcdConfigData 是包含 Consul 配置的字节切片
func (c *ConsulClient) Register(etcdConfigData []byte) {
	if c == nil {
		return
	}
	if etcdConfigData != nil {
		c.confData = etcdConfigData
	}
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
		if c.cluster == "" {
			c.cluster = "DEFAULT"
		}
		ips := strings.Split(ipstr, ",")
		ports := strings.Split(portstr, ",")
		consul_urls := make([]string, 0)
		for i, ip := range ips {
			if strings.TrimSpace(ip) == "" || i >= len(ports) || strings.TrimSpace(ports[i]) == "" {
				continue
			}
			consul_urls = append(consul_urls, fmt.Sprintf("%s:%s", ip, ports[i]))
		}
		if len(consul_urls) == 0 {
			logger.Error("Consul 配置缺少有效 server/port")
			return
		}
		serverConfig := api.DefaultConfig()
		serverConfig.Address = consul_urls[0]
		c.client, err = api.NewClient(serverConfig)
		if err != nil {
			logger.Error("Consul 服务连接失败:" + err.Error())
			return
		}
		localip, _ := localIPv4s(c.lan, c.lanNetwork)
		ip := "127.0.0.1"
		if len(localip) > 0 {
			ip = localip[0]
		}
		if config.Config.App.IpAddr != "" {
			ip = config.Config.App.IpAddr
		}
		port := uint64(config.Config.App.Port)
		protocol := "http://"
		if port == 0 || config.Config.App.PortSSL != 0 {
			port = uint64(config.Config.App.PortSSL)
			protocol = "https://"
		}
		registration := &api.AgentServiceRegistration{
			ID:      getInstanceId(ip, port),
			Name:    config.Config.App.Name,
			Address: ip,
			Port:    int(port),
			Tags:    []string{c.group, c.cluster, protocol},
			// TTL 健康检查：配合 client 侧的续活协程，进程崩溃/卡死时 Consul 能把本实例
			// 标记为 critical（发现层 Health().Service(passingOnly=true) 会自动剔除），
			// 超过 DeregisterCriticalServiceAfter 后自动反注册，避免留下"僵尸"实例。
			Check: &api.AgentServiceCheck{
				CheckID:                        "mgin:" + getInstanceId(ip, port),
				TTL:                           "30s",
				DeregisterCriticalServiceAfter: "1m",
			},
		}
		err = c.client.Agent().ServiceRegister(registration)
		if err != nil {
			logger.Error("Consul 注册服务失败:" + err.Error())
			return
		}
		// 注册成功：记录实例 ID 并启动 TTL 续活协程
		c.instanceID = getInstanceId(ip, port)
		var ctx context.Context
		ctx, c.cancel = context.WithCancel(context.Background())
		go c.passTTLLoop(ctx, c.instanceID)
	}
}

// GetServiceURL 方法用于从 Consul 查询服务地址
// servicename 是要查询的服务名称
// groupName 是可选的服务组名称
func (c *ConsulClient) GetServiceURL(servicename string, groupName ...string) (string, string) {
	if c == nil || c.client == nil {
		return "", ""
	}
	if len(groupName) == 0 {
		groupName = []string{c.group}
	} else if groupName[0] == "" {
		groupName[0] = c.group
	}
	currentGroup := groupName[0]
	for _, group := range groupName {
		services, _, err := c.client.Health().Service(servicename, group, true, &api.QueryOptions{})
		if err != nil {
			continue
		}
		if len(services) == 0 {
			continue
		}
		currentGroup = group
		//fmt.Printf("服务查询结果: %s\n", toJSON(services))
		r := rand.New(rand.NewSource(time.Now().UnixNano()))
		service := services[r.Intn(len(services))]
		protocol := "http://"
		if len(service.Service.Tags) > 2 && service.Service.Tags[2] != "" {
			protocol = service.Service.Tags[2]
		}
		address := fmt.Sprintf("%s%s:%d", protocol, service.Service.Address, service.Service.Port)
		return address, currentGroup
	}
	return "", currentGroup
}

// GetServices v2 新增：返回该服务在 Consul 上的全部健康实例 URL 列表。
// 与 GetServiceURL 走同一条 Health().Service 路径，只是不再随机选 1 个。
func (c *ConsulClient) GetServices(servicename string, groupName ...string) ([]string, error) {
	if c == nil || c.client == nil {
		return nil, fmt.Errorf("Consul client is nil")
	}
	if len(groupName) == 0 || groupName[0] == "" {
		groupName = []string{c.group}
	}
	for _, group := range groupName {
		services, _, err := c.client.Health().Service(servicename, group, true, &api.QueryOptions{})
		if err != nil {
			logger.Error("Consul 拉取" + servicename + "实例失败:" + err.Error())
			continue
		}
		if len(services) == 0 {
			continue
		}
		urls := make([]string, 0, len(services))
		for _, svc := range services {
			protocol := "http://"
			if len(svc.Service.Tags) > 2 && svc.Service.Tags[2] != "" {
				protocol = svc.Service.Tags[2]
			}
			urls = append(urls, fmt.Sprintf("%s%s:%d", protocol, svc.Service.Address, svc.Service.Port))
		}
		logger.Debug("Consul 获取" + servicename + "服务列表成功:" + strings.Join(urls, ","))
		return urls, nil
	}
	return nil, nil
}

// passTTLLoop 周期性向 Consul 上报 TTL 检查为 passing，维持本实例健康。
// ctx 被取消（DeRegister）时退出。
func (c *ConsulClient) passTTLLoop(ctx context.Context, id string) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := c.client.Agent().UpdateTTL("mgin:"+id, "mgin health", api.HealthPassing); err != nil {
				logger.Error("Consul TTL 续活失败:" + err.Error())
			}
		}
	}
}

// DeRegister 方法用于从 Consul 注销服务
func (c *ConsulClient) DeRegister() {
	if c == nil || c.client == nil {
		return
	}
	// 1) 取消 TTL 续活协程，避免注销过程中仍上报健康
	if c.cancel != nil {
		c.cancel()
	}
	// 2) 用注册时记录的实例 ID 精确反注册（不再重新计算 IP，规避 IP 不一致导致反注册失败）
	if c.instanceID != "" {
		if err := c.client.Agent().ServiceDeregister(c.instanceID); err != nil {
			logger.Error("Consul 取消注册服务失败:" + err.Error())
			return
		}
	} else {
		// 兜底：未记录 ID 时回退到原"按 IP 计算"的方式
		localip, _ := localIPv4s(c.lan, c.lanNetwork)
		ip := "127.0.0.1"
		if len(localip) > 0 {
			ip = localip[0]
		}
		if config.Config.App.IpAddr != "" {
			ip = config.Config.App.IpAddr
		}
		port := uint64(config.Config.App.Port)
		if port == 0 || config.Config.App.PortSSL != 0 {
			port = uint64(config.Config.App.PortSSL)
		}
		if err := c.client.Agent().ServiceDeregister(getInstanceId(ip, port)); err != nil {
			logger.Error("Consul 取消注册服务失败:" + err.Error())
			return
		}
	}
	logger.Info("Consul注销服务成功")
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

var json = jsoniter.ConfigCompatibleWithStandardLibrary

// toJSON 函数用于将对象转换为 JSON 字符串
func toJSON(o any) string {
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

// getInstanceId 函数用于生成服务实例 ID
func getInstanceId(ip string, port uint64) string {
	h := md5.New()
	_, _ = io.WriteString(h, fmt.Sprintf("http://%s:%d", ip, port))
	md := fmt.Sprintf("%x", h.Sum(nil))[:4]
	return md
}
