package polaris

import (
	"crypto/md5"
	"fmt"
	jsoniter "github.com/json-iterator/go"
	"github.com/knadh/koanf"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/rawbytes"
	"github.com/levigross/grequests"
	"github.com/maczh/mgin/v2/pkg/cache"
	"github.com/maczh/mgin/v2/pkg/config"
	"github.com/sadlil/gologger"
	"io"
	"math/rand"
	"net"
	"strings"
	"time"
)

type PolarisClient struct {
	apiBaseUrl string
	namespace  string
	token      string
	weight     int
	lan        bool
	lanNetwork string
	conf       *koanf.Koanf
	confUrl    string
	confData   []byte
}

var logger = gologger.GetLogger()

func (c *PolarisClient) Register(registryConfigData []byte) {
	if registryConfigData != nil {
		c.confData = registryConfigData
	}
	if c.conf == nil {
		c.conf = koanf.New(".")
		err := c.conf.Load(rawbytes.Provider(c.confData), yaml.Parser())
		if err != nil {
			logger.Error("Consul 注册中心配置文件解析错误:" + err.Error())
			c.conf = nil
			return
		}
		c.token = c.conf.String("go.polaris.token")
		c.lan = c.conf.Bool("go.polaris.lan")
		c.lanNetwork = c.conf.String("go.polaris.lanNet")
		ipstr := c.conf.String("go.polaris.server")
		portstr := c.conf.String("go.polaris.port")
		c.apiBaseUrl = fmt.Sprintf("http://%s:%s", ipstr, portstr)
		c.namespace = c.conf.String("go.polaris.namespace")
		if c.namespace == "" {
			c.namespace = "default"
		}
		localip, _ := localIPv4s(c.lan, c.lanNetwork)
		ip := localip[0]
		if config.Config.App.IpAddr != "" {
			ip = config.Config.App.IpAddr
		}
		//创建服务
		serviceUrl := fmt.Sprintf("%s/naming/v1/services", c.apiBaseUrl)
		createSerfviceReq := []ServiceCreateRequest{
			{
				Name:      config.Config.App.Name,
				Namespace: c.namespace,
			},
		}
		resp, err := grequests.Post(serviceUrl, grequests.FromRequestOptions(&grequests.RequestOptions{
			Headers: map[string]string{
				"X-Polaris-Token": c.token,
			},
			JSON: createSerfviceReq,
		}))
		if err != nil {
			logger.Error("Polaris 服务创建失败:" + err.Error())
			return
		}
		var res ServiceCreateResponse
		err = resp.JSON(&res)
		if res.Code != SUCCESS && res.Code != EXISTS {
			logger.Error("Polaris 服务创建失败:" + res.Info)
			return
		}
		//注册服务实例
		instanceUrl := fmt.Sprintf("%s/naming/v1/instances", c.apiBaseUrl)
		registerReq := []InstanceRegisterRequest{
			{
				Service:   config.Config.App.Name,
				Namespace: c.namespace,
				Host:      ip,
				Port:      config.Config.App.Port,
				Weight:    &c.weight,
				Metadata: map[string]string{
					"ssl": "false",
				},
			},
		}
		registerReq[0].SetHealthy(true)
		registerReq[0].SetProtocol("http")
		if config.Config.App.PortSSL != 0 {
			registerReq[0].Port = config.Config.App.PortSSL
			registerReq[0].Metadata["ssl"] = "true"
		}
		logger.Debug("注册实例请求参数: " + toJSON(registerReq))
		resp, err = grequests.Post(instanceUrl, grequests.FromRequestOptions(&grequests.RequestOptions{
			Headers: map[string]string{
				"X-Polaris-Token": c.token,
			},
			JSON: registerReq,
		}))
		if err != nil {
			logger.Error("Polaris 服务实例注册失败:" + err.Error())
			return
		}
		var res1 InstanceRegisterResponse
		err = resp.JSON(&res1)
		if res1.Code != SUCCESS && res1.Code != EXISTS {
			logger.Error("Polaris 服务实例注册失败:" + toJSON(res1))
			return
		}
		serviceId := res1.Responses[0].Instance.Id
		cache.OnGetCache("polaris").Set("serviceId", serviceId, 0)
		logger.Info(fmt.Sprintf("%s服务注册成功", config.Config.App.Name))
	}
}

func (c *PolarisClient) GetServiceURL(servicename string, namespaces ...string) (string, string) {
	if len(namespaces) == 0 {
		namespaces = append(namespaces, c.namespace)
	}
	if namespaces[0] == "" {
		namespaces[0] = c.namespace
	}
	currentNameSpace := namespaces[0]
	logger.Debug(fmt.Sprintf("namespaces=%s, etcdClient=%s", toJSON(namespaces), toJSON(c)))
	for _, namespace := range namespaces {
		query := map[string]string{
			"service":   servicename,
			"namespace": namespace,
		}
		resp, err := grequests.Get(fmt.Sprintf("%s/naming/v1/instances", c.apiBaseUrl), grequests.FromRequestOptions(&grequests.RequestOptions{
			Headers: map[string]string{
				"X-Polaris-Token": c.token,
			},
			Params: query,
		}))
		var res QueryInstanceResponse
		err = resp.JSON(&res)
		if err != nil {
			logger.Error("Polaris 查询服务实例失败:" + err.Error())
			continue
		}
		if res.Code != SUCCESS {
			logger.Error("Polaris 查询服务实例失败:" + res.Info)
			continue
		}
		if len(res.Instances) == 0 {
			continue
		}
		logger.Debug("查询服务结果: " + toJSON(res.Instances))
		currentNameSpace = namespace
		r := rand.New(rand.NewSource(time.Now().UnixNano()))
		ins := res.Instances[r.Intn(len(res.Instances))]
		protocol := "http"
		if ins.Metadata["ssl"] == "true" {
			protocol = "https"
		}
		return fmt.Sprintf("%s://%s:%d", protocol, ins.Host, ins.Port), currentNameSpace
	}
	return "", currentNameSpace
}

// GetServices v2 新增：返回该服务在 Polaris 上的全部健康实例 URL 列表。
// 与 GetServiceURL 走同一条 /naming/v1/instances 路径。
func (c *PolarisClient) GetServices(servicename string, namespaces ...string) ([]string, error) {
	if len(namespaces) == 0 {
		namespaces = append(namespaces, c.namespace)
	}
	if namespaces[0] == "" {
		namespaces[0] = c.namespace
	}
	for _, namespace := range namespaces {
		query := map[string]string{
			"service":   servicename,
			"namespace": namespace,
		}
		resp, err := grequests.Get(fmt.Sprintf("%s/naming/v1/instances", c.apiBaseUrl), grequests.FromRequestOptions(&grequests.RequestOptions{
			Headers: map[string]string{
				"X-Polaris-Token": c.token,
			},
			Params: query,
		}))
		if err != nil {
			logger.Error("Polaris 拉取" + servicename + "实例失败:" + err.Error())
			continue
		}
		var res QueryInstanceResponse
		if uerr := resp.JSON(&res); uerr != nil {
			logger.Error("Polaris 解析" + servicename + "实例失败:" + uerr.Error())
			continue
		}
		if res.Code != SUCCESS || len(res.Instances) == 0 {
			continue
		}
		urls := make([]string, 0, len(res.Instances))
		for _, ins := range res.Instances {
			protocol := "http"
			if ins.Metadata["ssl"] == "true" {
				protocol = "https"
			}
			urls = append(urls, fmt.Sprintf("%s://%s:%d", protocol, ins.Host, ins.Port))
		}
		logger.Debug("Polaris 获取" + servicename + "服务列表成功:" + strings.Join(urls, ","))
		return urls, nil
	}
	return nil, nil
}

func (c *PolarisClient) DeRegister() {
	query := DeregisterInstanceRequest{}
	serviceId, exists := cache.OnGetCache("polaris").Get("serviceId")
	if exists {
		query.ID = serviceId.(string)
	} else {

		localip, _ := localIPv4s(c.lan, c.lanNetwork)
		ip := localip[0]
		if config.Config.App.IpAddr != "" {
			ip = config.Config.App.IpAddr
		}
		query.Service = config.Config.App.Name
		query.Namespace = c.namespace
		query.Host = ip
		port := int64(config.Config.App.Port)
		if port == 0 || config.Config.App.PortSSL != 0 {
			port = int64(config.Config.App.PortSSL)
		}
		query.Port = port
	}
	_, err := grequests.Post(fmt.Sprintf("%s/naming/v1/instances/delete", c.apiBaseUrl), grequests.FromRequestOptions(&grequests.RequestOptions{
		Headers: map[string]string{
			"X-Polaris-Token": c.token,
		},
		JSON: []DeregisterInstanceRequest{query},
	}))
	if err != nil {
		logger.Error("Polaris 服务实例注销失败:" + err.Error())
		return
	}
	cache.OnGetCache("polaris").Delete("serviceId")
	//查询服务是否存在其他实例
	resp, err := grequests.Get(fmt.Sprintf("%s/naming/v1/instances", c.apiBaseUrl), grequests.FromRequestOptions(&grequests.RequestOptions{
		Headers: map[string]string{
			"X-Polaris-Token": c.token,
		},
		Params: map[string]string{
			"service":   config.Config.App.Name,
			"namespace": c.namespace,
		},
	}))
	var res QueryInstanceResponse
	err = resp.JSON(&res)
	if err != nil {
		logger.Error("Polaris 查询服务实例失败:" + err.Error())
		return
	}
	if res.Code != SUCCESS {
		logger.Error("Polaris 查询服务实例失败:" + res.Info)
		return
	}
	if len(res.Instances) > 0 {
		//存在其他实例,不注销服务
		return
	}
	//注销服务
	resp, err = grequests.Post(fmt.Sprintf("%s/naming/v1/services/delete", c.apiBaseUrl), grequests.FromRequestOptions(&grequests.RequestOptions{
		Headers: map[string]string{
			"X-Polaris-Token": c.token,
		},
		JSON: []map[string]string{
			{
				"name":      config.Config.App.Name,
				"namespace": c.namespace,
			},
		},
	}))
	if err != nil {
		logger.Error("Polaris 服务注销失败:" + err.Error())
		return
	}
	var res1 DeleteServiceResponse
	err = resp.JSON(&res1)
	if res1.Code != SUCCESS {
		logger.Error("Polaris 服务注销失败:" + res.Info)
		return
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
