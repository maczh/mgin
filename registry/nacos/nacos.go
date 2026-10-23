package nacos

import (
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	jsoniter "github.com/json-iterator/go"
	"github.com/levigross/grequests"
	"github.com/maczh/mgin/config"
	registryModel "github.com/maczh/mgin/registry/model"
	"github.com/sadlil/gologger"

	"github.com/knadh/koanf"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/rawbytes"
)

type NacosClient struct {
	nsurl      string
	endpoints  []string
	group      string
	cluster    string
	lan        bool
	lanNetwork string
	conf       *koanf.Koanf
	confUrl    string
	confData   []byte
	ver        string
	param      map[string]string
	worker     *nacosHeartbeatWorker
}

var logger = gologger.GetLogger()

type nacosHeartbeatWorker struct {
	ticker    *time.Ticker
	quit      chan struct{}
	quitOnce  sync.Once
	wg        *sync.WaitGroup
	failCount int
	maxFails  int
}

func (w *nacosHeartbeatWorker) Start(nc *NacosClient) {
	defer w.wg.Done()
	w.ticker = time.NewTicker(5 * time.Second)
	defer w.ticker.Stop()
	if w.maxFails <= 0 {
		w.maxFails = 3
	}
	for {
		select {
		case <-w.quit:
			return
		case <-w.ticker.C:
			_, err := grequests.DoRegularRequest(http.MethodPut, nc.nsurl, &grequests.RequestOptions{
				Params: nc.param,
			})
			if err != nil {
				w.failCount++
				logger.Error(fmt.Sprintf("Nacos心跳失败(%d/%d): %s", w.failCount, w.maxFails, err.Error()))
				if w.failCount >= w.maxFails {
					w.failCount = 0
					if !nc.reRegister() {
						logger.Error("Nacos心跳连续失败且重新注册失败")
					}
				}
				continue
			}
			w.failCount = 0
		}
	}
}

func (n *NacosClient) Register(nacosConfigData []byte) {
	if nacosConfigData == nil || len(nacosConfigData) == 0 {
		logger.Error("Nacos 配置错误，无法获取配置地址")
		return
	}
	n.confData = nacosConfigData
	if n.conf == nil {
		var err error
		n.conf = koanf.New(".")
		err = n.conf.Load(rawbytes.Provider(n.confData), yaml.Parser())
		if err != nil {
			logger.Error("Nacos注册中心配置文件解析错误:" + err.Error())
			n.conf = nil
			return
		}

		n.lan = n.conf.Bool("go.nacos.lan")
		n.lanNetwork = n.conf.String("go.nacos.lanNet")
		ipstr := n.conf.String("go.nacos.server")
		portstr := n.conf.String("go.nacos.port")
		n.ver = n.conf.String("go.nacos.apiversion")
		if n.ver == "" {
			n.ver = "v1"
		}
		ips := strings.Split(ipstr, ",")
		ports := strings.Split(portstr, ",")
		for i, ip := range ips {
			if i < len(ports) && strings.TrimSpace(ip) != "" {
				n.endpoints = append(n.endpoints, fmt.Sprintf("http://%s:%s/nacos/%s/ns/instance", strings.TrimSpace(ip), strings.TrimSpace(ports[i]), n.ver))
			}
		}
		if len(n.endpoints) > 0 {
			n.nsurl = n.endpoints[0]
		} else {
			n.nsurl = fmt.Sprintf("http://%s:%s/nacos/%s/ns/instance", ipstr, portstr, n.ver)
		}
		n.group = n.conf.String("go.nacos.group")
		n.param = make(map[string]string)
		if username := n.conf.String("go.nacos.username"); username != "" {
			n.param = map[string]string{"username": username, "password": n.conf.String("go.nacos.password")}
		}
		if n.group == "" {
			n.group = "DEFAULT_GROUP"
		}
		localip, err := localIPv4s(n.lan, n.lanNetwork)
		if err != nil || len(localip) == 0 {
			logger.Error("Nacos注册中心获取本机IP失败")
			return
		}
		ip := localip[0]
		if config.Config.App.IpAddr != "" {
			ip = config.Config.App.IpAddr
		}
		n.cluster = n.conf.String("go.nacos.clusterName")
		if n.cluster == "" {
			n.cluster = "DEFAULT"
		}
		port := uint64(config.Config.App.Port)
		metadata := make(map[string]string)
		if port == 0 || config.Config.App.PortSSL != 0 {
			port = uint64(config.Config.App.PortSSL)
			metadata["ssl"] = "true"
		}
		for k, v := range map[string]string{
			"ip":          ip,
			"port":        fmt.Sprintf("%d", port),
			"weight":      "1",
			"cluster":     n.cluster,
			"groupName":   n.group,
			"serviceName": config.Config.App.Name,
			"meta":        toJSON(metadata),
		} {
			n.param[k] = v
		}
		resp, err := grequests.DoRegularRequest(http.MethodPost, n.nsurl, &grequests.RequestOptions{
			Params: n.param,
		})
		if err != nil {
			logger.Error("Nacos注册服务失败:" + err.Error())
			return
		}
		if resp.StatusCode != 200 {
			logger.Error("Nacos注册服务失败:" + resp.String())
			return
		}
		// 启动心跳线程
		n.worker = &nacosHeartbeatWorker{
			wg:       &sync.WaitGroup{},
			quit:     make(chan struct{}),
			maxFails: 3,
		}
		n.worker.wg.Add(1)
		go n.worker.Start(n)
		logger.Info("Nacos注册服务成功:" + ip + ":" + strconv.Itoa(int(port)))
	}

}

func (n *NacosClient) GetServiceURL(servicename string, groupName ...string) (string, string) {
	if len(n.endpoints) > 1 {
		for _, endpoint := range n.endpoints {
			copyClient := *n
			copyClient.nsurl = endpoint
			copyClient.endpoints = nil
			if url, group := copyClient.GetServiceURL(servicename, groupName...); url != "" {
				return url, group
			}
		}
		return "", ""
	}
	var instance InstanceResp
	var err error
	query := map[string]string{
		"serviceName": servicename,
	}
	var resp *grequests.Response
	if len(groupName) > 0 {
		for _, g := range groupName {
			query["groupName"] = g
			resp, err = grequests.DoRegularRequest(http.MethodGet, n.nsurl+"/list", &grequests.RequestOptions{
				Params: query,
			})
			if err != nil {
				logger.Error("获取Nacos服务" + servicename + "失败:" + err.Error())
				continue
			}
			if resp.StatusCode != 200 {
				logger.Error("获取Nacos服务" + servicename + "失败:" + resp.String())
				continue
			}
			err = json.Unmarshal(resp.Bytes(), &instance)
			if err != nil {
				logger.Error("解析Nacos服务" + servicename + "失败:" + err.Error())
				continue
			}
			if len(instance.Hosts) == 0 {
				continue
			}
			r := rand.New(rand.NewSource(time.Now().UnixNano()))
			idx := r.Intn(len(instance.Hosts))
			protocol := "http://"
			if instance.Hosts[idx].Metadata != nil && instance.Hosts[idx].Metadata["ssl"] == "true" {
				protocol = "https://"
			}
			url := protocol + instance.Hosts[idx].IP + ":" + strconv.Itoa(instance.Hosts[idx].Port)
			logger.Debug("Nacos获取" + servicename + "服务成功:" + url)
			return url, g

		}
	} else {
		resp, err = grequests.DoRegularRequest(http.MethodGet, n.nsurl+"/list", &grequests.RequestOptions{
			Params: query,
		})
		if err != nil {
			logger.Error("获取Nacos服务" + servicename + "失败:" + err.Error())
			return "", ""
		}
		if resp.StatusCode != 200 {
			logger.Error("获取Nacos服务" + servicename + "失败:" + resp.String())
			return "", ""
		}
		err = json.Unmarshal(resp.Bytes(), &instance)
		if err != nil {
			logger.Error("解析Nacos服务" + servicename + "失败:" + err.Error())
			return "", ""
		}
		if len(instance.Hosts) == 0 {
			return "", ""
		}
		r := rand.New(rand.NewSource(time.Now().UnixNano()))
		idx := r.Intn(len(instance.Hosts))
		protocol := "http://"
		if instance.Hosts[idx].Metadata != nil && instance.Hosts[idx].Metadata["ssl"] == "true" {
			protocol = "https://"
		}
		url := protocol + instance.Hosts[idx].IP + ":" + strconv.Itoa(instance.Hosts[idx].Port)
		logger.Debug("Nacos获取" + servicename + "服务成功:" + url)
		strs := strings.Split(instance.Hosts[idx].ServiceName, "@@")
		return url, strs[0]
	}
	return "", ""
}

func (n *NacosClient) GetServiceInstances(service string, groupName ...string) ([]registryModel.ServiceInstance, string) {
	group := n.group
	if len(groupName) > 0 && groupName[0] != "" {
		group = groupName[0]
	}
	resp, err := grequests.Get(n.nsurl+"/list", grequests.FromRequestOptions(&grequests.RequestOptions{Params: map[string]string{"serviceName": service, "groupName": group}}))
	if err != nil || resp.StatusCode != 200 {
		return nil, group
	}
	var result InstanceResp
	if err := resp.JSON(&result); err != nil {
		return nil, group
	}
	out := make([]registryModel.ServiceInstance, 0, len(result.Hosts))
	for _, host := range result.Hosts {
		if !host.Healthy || !host.Enabled || host.Marked {
			continue
		}
		protocol := "http://"
		if host.Metadata["ssl"] == "true" {
			protocol = "https://"
		}
		weight := host.Weight
		if weight <= 0 {
			weight = 1
		}
		out = append(out, registryModel.ServiceInstance{URL: fmt.Sprintf("%s%s:%d", protocol, host.IP, host.Port), Weight: weight, Metadata: host.Metadata})
	}
	return out, group
}

// SubscribeOffline provides a portable fallback for Nacos installations that
// do not expose a client-side instance event stream. It emits only on a
// healthy-to-empty transition, so cache consumers are not invalidated repeatedly.
func (n *NacosClient) SubscribeOffline(callback func(service, group string)) error {
	_, err := n.WatchService(config.Config.App.Name, "", func() { callback(config.Config.App.Name, "") })
	return err
}

func (n *NacosClient) WatchService(service, group string, callback func()) (func(), error) {
	if callback == nil || n.nsurl == "" {
		return nil, fmt.Errorf("nacos is not initialized")
	}
	stop := make(chan struct{})
	go func() {
		wasAvailable := true
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				url, _ := n.GetServiceURL(service, group)
				available := url != ""
				if wasAvailable && !available {
					callback()
				}
				wasAvailable = available
			}
		}
	}()
	return func() { close(stop) }, nil
}

func (n *NacosClient) DeRegister() {
	if n.worker != nil {
		n.worker.quitOnce.Do(func() { close(n.worker.quit) })
		n.worker.wg.Wait()
	}
	if n.nsurl == "" || n.param == nil {
		return
	}
	_, err := grequests.DoRegularRequest(http.MethodDelete, n.nsurl, &grequests.RequestOptions{
		Params: n.param,
	})
	if err != nil {
		logger.Error("Nacos注销服务失败:" + err.Error())
	}
	logger.Info("Nacos注销服务成功")
}

// reRegister 在心跳连续失败（注册中心抖动/会话失效）时重新发起注册，
// 保证实例不脱管。
func (n *NacosClient) reRegister() bool {
	if n.nsurl == "" || n.param == nil {
		return false
	}
	resp, err := grequests.DoRegularRequest(http.MethodPost, n.nsurl, &grequests.RequestOptions{
		Params: n.param,
	})
	if err != nil {
		logger.Error("Nacos重新注册失败: " + err.Error())
		return false
	}
	if resp.StatusCode != 200 {
		logger.Error("Nacos重新注册失败: " + resp.String())
		return false
	}
	logger.Info("Nacos心跳失败已重新注册成功")
	return true
}

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

type InstanceResp struct {
	Name        string `json:"name"`
	Clusters    string `json:"clusters"`
	CacheMillis int    `json:"cacheMillis"`
	Hosts       []struct {
		Service     string            `json:"service"`
		IP          string            `json:"ip"`
		Port        int               `json:"port"`
		ClusterName string            `json:"clusterName"`
		Weight      int               `json:"weight"`
		Healthy     bool              `json:"healthy"`
		InstanceID  string            `json:"instanceId"`
		Metadata    map[string]string `json:"metadata"`
		Marked      bool              `json:"marked"`
		Enabled     bool              `json:"enabled"`
		ServiceName string            `json:"serviceName"`
		Ephemeral   bool              `json:"ephemeral"`
	} `json:"hosts"`
	LastRefTime                 int64             `json:"lastRefTime"`
	Checksum                    string            `json:"checksum"`
	UseSpecifiedURL             bool              `json:"useSpecifiedURL"`
	Env                         string            `json:"env"`
	ProtectThreshold            interface{}       `json:"protectThreshold"`
	ReachLocalSiteCallThreshold interface{}       `json:"reachLocalSiteCallThreshold"`
	Dom                         string            `json:"dom"`
	Metadata                    map[string]string `json:"metadata"`
}
