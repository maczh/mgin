package nacos

import (
	stdjson "encoding/json"
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
	nsurl       string
	endpoints   []string
	group       string
	cluster     string
	lan         bool
	lanNetwork  string
	conf        *koanf.Koanf
	confUrl     string
	confData    []byte
	ver         string
	namespace   string
	accessToken string
	headers     map[string]string
	param       map[string]string
	worker      *nacosHeartbeatWorker
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
			params := heartbeatParams(nc)
			method, endpoint := http.MethodPut, nc.nsurl+"/beat"
			if nc.ver == "v3" {
				method, endpoint = http.MethodPost, nc.nsurl
				params["heartBeat"] = "true"
			}
			_, err := grequests.DoRegularRequest(method, endpoint, &grequests.RequestOptions{Params: params, Headers: nc.headers})
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
		n.ver = normalizeVersion(n.conf.String("go.nacos.apiversion"))
		n.namespace = n.conf.String("go.nacos.namespace")
		ips := strings.Split(ipstr, ",")
		ports := strings.Split(portstr, ",")
		for i, ip := range ips {
			if i < len(ports) && strings.TrimSpace(ip) != "" {
				n.endpoints = append(n.endpoints, namingURL(strings.TrimSpace(ip), strings.TrimSpace(ports[i]), n.ver))
			}
		}
		if len(n.endpoints) > 0 {
			n.nsurl = n.endpoints[0]
		} else {
			n.nsurl = namingURL(ipstr, portstr, n.ver)
		}
		n.group = n.conf.String("go.nacos.group")
		n.param = make(map[string]string)
		n.headers = map[string]string{"User-Agent": "Nacos-Go-Client:mgin", "Client-Version": "Nacos-Go-Client:mgin"}
		n.accessToken = n.conf.String("go.nacos.accessToken")
		if n.accessToken == "" {
			n.accessToken = n.login(ipstr, portstr)
		}
		if n.accessToken != "" {
			n.param["accessToken"] = n.accessToken
			n.headers["Authorization"] = "Bearer " + n.accessToken
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
			"clusterName": n.cluster,
			"groupName":   n.group,
			"serviceName": config.Config.App.Name,
			"metadata":    toJSON(metadata),
			"namespaceId": n.namespace,
		} {
			n.param[k] = v
		}
		resp, err := grequests.DoRegularRequest(http.MethodPost, n.nsurl, &grequests.RequestOptions{
			Params: n.param, Headers: n.headers,
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
		"namespaceId": n.namespace,
	}
	addClusterQuery(query, n.ver, n.cluster)
	addAuth(query, n.accessToken)
	var resp *grequests.Response
	if len(groupName) > 0 {
		for _, g := range groupName {
			query["groupName"] = g
			resp, err = grequests.DoRegularRequest(http.MethodGet, n.nsurl+"/list", &grequests.RequestOptions{
				Params: query, Headers: n.headers,
			})
			if err != nil {
				logger.Error("获取Nacos服务" + servicename + "失败:" + err.Error())
				continue
			}
			if resp.StatusCode != 200 {
				logger.Error("获取Nacos服务" + servicename + "失败:" + resp.String())
				continue
			}
			err = decodeInstances(n.ver, resp.Bytes(), &instance)
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
			Params: query, Headers: n.headers,
		})
		if err != nil {
			logger.Error("获取Nacos服务" + servicename + "失败:" + err.Error())
			return "", ""
		}
		if resp.StatusCode != 200 {
			logger.Error("获取Nacos服务" + servicename + "失败:" + resp.String())
			return "", ""
		}
		err = decodeInstances(n.ver, resp.Bytes(), &instance)
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
	params := map[string]string{"serviceName": service, "groupName": group, "namespaceId": n.namespace}
	addClusterQuery(params, n.ver, n.cluster)
	addAuth(params, n.accessToken)
	resp, err := grequests.Get(n.nsurl+"/list", grequests.FromRequestOptions(&grequests.RequestOptions{Params: params, Headers: n.headers}))
	if err != nil || resp.StatusCode != 200 {
		return nil, group
	}
	var result InstanceResp
	if err := decodeInstances(n.ver, resp.Bytes(), &result); err != nil {
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
		weight := int(host.Weight)
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
	var stopOnce sync.Once
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
	return func() { stopOnce.Do(func() { close(stop) }) }, nil
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
		Params: n.param, Headers: n.headers,
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
		Params: n.param, Headers: n.headers,
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

func normalizeVersion(version string) string {
	switch strings.ToLower(strings.TrimSpace(version)) {
	case "2", "v2":
		return "v2"
	case "3", "v3":
		return "v3"
	default:
		return "v1"
	}
}

func namingURL(server, port, version string) string {
	base := fmt.Sprintf("http://%s:%s/nacos", strings.TrimSpace(server), strings.TrimSpace(port))
	if normalizeVersion(version) == "v3" {
		return base + "/v3/client/ns/instance"
	}
	return base + "/" + normalizeVersion(version) + "/ns/instance"
}

func cloneParams(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func addAuth(params map[string]string, token string) {
	if token != "" {
		params["accessToken"] = token
	}
}

func addClusterQuery(params map[string]string, version, cluster string) {
	if cluster == "" {
		return
	}
	if normalizeVersion(version) == "v1" {
		params["clusters"] = cluster
		return
	}
	params["clusterName"] = cluster
}

func heartbeatParams(n *NacosClient) map[string]string {
	if n.ver == "v3" {
		params := cloneParams(n.param)
		params["heartBeat"] = "true"
		return params
	}
	params := map[string]string{
		"serviceName": n.param["serviceName"], "groupName": n.param["groupName"],
		"namespaceId": n.param["namespaceId"],
		"beat": toJSON(map[string]any{
			"ip": n.param["ip"], "port": n.param["port"], "serviceName": n.param["serviceName"],
			"cluster": n.param["clusterName"], "weight": 1, "metadata": stdjson.RawMessage(n.param["metadata"]),
		}),
	}
	addAuth(params, n.accessToken)
	return params
}

func decodeInstances(version string, body []byte, target *InstanceResp) error {
	switch normalizeVersion(version) {
	case "v1":
		return json.Unmarshal(body, target)
	case "v2":
		var response struct {
			Code    int                `json:"code"`
			Message string             `json:"message"`
			Data    stdjson.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(body, &response); err != nil {
			return err
		}
		if response.Code != 0 {
			return fmt.Errorf("nacos: %s", response.Message)
		}
		return json.Unmarshal(response.Data, target)
	default:
		var response struct {
			Code    int                `json:"code"`
			Message string             `json:"message"`
			Data    stdjson.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(body, &response); err != nil {
			return err
		}
		if response.Code != 0 {
			return fmt.Errorf("nacos: %s", response.Message)
		}
		wrapped := append([]byte(`{"hosts":`), response.Data...)
		wrapped = append(wrapped, '}')
		return json.Unmarshal(wrapped, target)
	}
}

func (n *NacosClient) login(server, port string) string {
	username := n.conf.String("go.nacos.username")
	if username == "" {
		return ""
	}
	path := "/nacos/v1/auth/login"
	if n.ver == "v3" {
		path = "/nacos/v3/auth/user/login"
	}
	resp, err := grequests.Post(fmt.Sprintf("http://%s:%s%s", strings.TrimSpace(server), strings.TrimSpace(port), path), grequests.FromRequestOptions(&grequests.RequestOptions{Data: map[string]string{"username": username, "password": n.conf.String("go.nacos.password")}}))
	if err != nil || !resp.Ok {
		logger.Error("Nacos登录失败")
		return ""
	}
	var result struct {
		AccessToken string `json:"accessToken"`
	}
	if err := resp.JSON(&result); err != nil {
		return ""
	}
	return result.AccessToken
}

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
		Weight      float64           `json:"weight"`
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
