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
	"github.com/maczh/mgin/v2/pkg/config"
	"github.com/sadlil/gologger"

	"github.com/knadh/koanf"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/rawbytes"
)

type NacosClient struct {
	nsurl       string
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
	ticker   *time.Ticker
	quit     chan struct{}
	quitOnce sync.Once
	wg       *sync.WaitGroup
}

func (w *nacosHeartbeatWorker) Start(nc *NacosClient) {
	defer w.wg.Done()
	w.ticker = time.NewTicker(5 * time.Second)
	defer w.ticker.Stop()
	failCount := 0
	for {
		select {
		case <-w.quit:
			return
		case <-w.ticker.C:
			params := heartbeatParams(nc)
			method, endpoint := http.MethodPut, nc.nsurl+"/beat"
			if nc.ver == "v3" {
				method, endpoint = http.MethodPost, nc.nsurl
			}
			resp, err := grequests.DoRegularRequest(method, endpoint, &grequests.RequestOptions{Params: params, Headers: nc.headers})
			if err == nil && (resp == nil || !resp.Ok) {
				err = fmt.Errorf("nacos heartbeat failed")
			}
			if err != nil {
				logger.Error("Nacos心跳失败:" + err.Error())
				failCount++
				// 连续失败达到阈值：本实例可能已被 Nacos 摘除（心跳长期失败），
				// 主动重新注册以恢复，避免"假死"（注册中心里没有、但进程还活着）。
				if failCount >= 3 {
					if rerr := nc.reRegister(); rerr == nil {
						failCount = 0
					}
				}
				continue
			}
			failCount = 0
		}
	}
}

func (n *NacosClient) Register(nacosConfigData []byte) {
	if n == nil {
		return
	}
	if nacosConfigData != nil {
		n.confData = nacosConfigData
	}
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
		n.nsurl = namingURL(ipstr, portstr, n.ver)
		n.group = n.conf.String("go.nacos.group")
		if n.group == "" {
			n.group = "DEFAULT_GROUP"
		}
		localip, _ := localIPv4s(n.lan, n.lanNetwork)
		ip := "127.0.0.1"
		if len(localip) > 0 {
			ip = localip[0]
		}
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
		n.headers = map[string]string{"User-Agent": "Nacos-Go-Client:mgin-v2", "Client-Version": "Nacos-Go-Client:mgin-v2"}
		n.accessToken = n.conf.String("go.nacos.accessToken")
		if n.accessToken == "" {
			n.accessToken = n.login(ipstr, portstr)
		}
		n.param = map[string]string{
			"ip":          ip,
			"port":        fmt.Sprintf("%d", port),
			"weight":      "1",
			"clusterName": n.cluster,
			"groupName":   n.group,
			"serviceName": config.Config.App.Name,
			"metadata":    toJSON(metadata),
			"namespaceId": n.namespace,
		}
		if n.accessToken != "" {
			n.param["accessToken"] = n.accessToken
			n.headers["Authorization"] = "Bearer " + n.accessToken
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
			wg: &sync.WaitGroup{}, quit: make(chan struct{}),
		}
		n.worker.wg.Add(1)
		go n.worker.Start(n)
		logger.Info("Nacos注册服务成功:" + ip + ":" + strconv.Itoa(int(port)))
	}

}

func (n *NacosClient) GetServiceURL(servicename string, groupName ...string) (string, string) {
	var instance InstanceResp
	var err error
	query := map[string]string{
		"serviceName": servicename,
		"namespaceId": n.namespace,
	}
	addAuth(query, n.accessToken)
	addClusterQuery(query, n.ver, n.cluster)
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

// GetServices v2 新增：返回该服务的所有可用实例 URL 列表（健康检查通过 + enabled）。
// 直接复用 GetServiceURL 已有的 /list 调用，去掉"随机选 1 个"这一步。
func (n *NacosClient) GetServices(servicename string, groupName ...string) ([]string, error) {
	var instance InstanceResp
	var resp *grequests.Response
	var err error

	doList := func(group string) {
		q := map[string]string{"serviceName": servicename, "namespaceId": n.namespace}
		addAuth(q, n.accessToken)
		addClusterQuery(q, n.ver, n.cluster)
		if group != "" {
			q["groupName"] = group
		}
		resp, err = grequests.DoRegularRequest(http.MethodGet, n.nsurl+"/list", &grequests.RequestOptions{
			Params: q, Headers: n.headers,
		})
	}

	if len(groupName) > 0 && groupName[0] != "" {
		for _, g := range groupName {
			doList(g)
			if err != nil {
				logger.Error("获取Nacos服务" + servicename + "失败:" + err.Error())
				continue
			}
			if resp == nil || resp.StatusCode != 200 {
				if resp != nil {
					logger.Error("获取Nacos服务" + servicename + "失败:" + resp.String())
				}
				continue
			}
			if uerr := decodeInstances(n.ver, resp.Bytes(), &instance); uerr != nil {
				logger.Error("解析Nacos服务" + servicename + "失败:" + uerr.Error())
				continue
			}
			urls := []string{}
			for _, h := range instance.Hosts {
				if !h.Healthy || !h.Enabled {
					continue
				}
				protocol := "http://"
				if h.Metadata != nil && h.Metadata["ssl"] == "true" {
					protocol = "https://"
				}
				urls = append(urls, protocol+h.IP+":"+strconv.Itoa(h.Port))
			}
			if len(urls) > 0 {
				logger.Debug("Nacos获取" + servicename + "服务列表成功:" + strings.Join(urls, ","))
				return urls, nil
			}
		}
		return nil, nil
	}

	doList("")
	if err != nil {
		return nil, err
	}
	if resp == nil || resp.StatusCode != 200 {
		if resp != nil {
			logger.Error("获取Nacos服务" + servicename + "失败:" + resp.String())
		}
		return nil, nil
	}
	if uerr := decodeInstances(n.ver, resp.Bytes(), &instance); uerr != nil {
		return nil, uerr
	}
	urls := []string{}
	for _, h := range instance.Hosts {
		if !h.Healthy || !h.Enabled {
			continue
		}
		protocol := "http://"
		if h.Metadata != nil && h.Metadata["ssl"] == "true" {
			protocol = "https://"
		}
		urls = append(urls, protocol+h.IP+":"+strconv.Itoa(h.Port))
	}
	logger.Debug("Nacos获取" + servicename + "服务列表成功:" + strings.Join(urls, ","))
	return urls, nil
}

// reRegister 在心跳连续失败后主动重新注册本实例，恢复其在 Nacos 中的可用状态。
// 依赖 Register 阶段已填充的 nsurl / param（含 ip、port、serviceName 等），可安全重复调用。
func (n *NacosClient) reRegister() error {
	resp, err := grequests.DoRegularRequest(http.MethodPost, n.nsurl, &grequests.RequestOptions{
		Params: n.param, Headers: n.headers,
	})
	if err != nil {
		logger.Error("Nacos重新注册失败:" + err.Error())
		return err
	}
	if resp.StatusCode != 200 {
		logger.Error("Nacos重新注册失败:" + resp.String())
		return fmt.Errorf("nacos re-register status %d", resp.StatusCode)
	}
	logger.Info("Nacos服务实例重新注册成功")
	return nil
}

func (n *NacosClient) DeRegister() {
	// 1) 先停止心跳协程，避免注销后又被心跳 PUT 重新写入（复活实例）。
	if n.worker != nil {
		n.worker.quitOnce.Do(func() { close(n.worker.quit) })
		n.worker.wg.Wait()
	}
	// 2) 显式 DELETE，立即从 Nacos 摘除本实例（否则需等待心跳超时）。
	if n.nsurl != "" && len(n.param) > 0 {
		_, err := grequests.DoRegularRequest(http.MethodDelete, n.nsurl, &grequests.RequestOptions{
			Params: n.param, Headers: n.headers,
		})
		if err != nil {
			logger.Error("Nacos注销服务失败:" + err.Error())
		}
	}
	logger.Info("Nacos注销服务成功")
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
	for k, v := range in {
		out[k] = v
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
	} else {
		params["clusterName"] = cluster
	}
}
func heartbeatParams(n *NacosClient) map[string]string {
	if n.ver == "v3" {
		params := cloneParams(n.param)
		params["heartBeat"] = "true"
		return params
	}
	params := map[string]string{"serviceName": n.param["serviceName"], "groupName": n.param["groupName"], "namespaceId": n.param["namespaceId"], "beat": toJSON(map[string]any{"ip": n.param["ip"], "port": n.param["port"], "serviceName": n.param["serviceName"], "cluster": n.param["clusterName"], "weight": 1, "metadata": stdjson.RawMessage(n.param["metadata"])})}
	addAuth(params, n.accessToken)
	return params
}
func decodeInstances(version string, body []byte, target *InstanceResp) error {
	if normalizeVersion(version) == "v1" {
		return json.Unmarshal(body, target)
	}
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
	if normalizeVersion(version) == "v2" {
		return json.Unmarshal(response.Data, target)
	}
	wrapped := append([]byte(`{"hosts":`), response.Data...)
	wrapped = append(wrapped, '}')
	return json.Unmarshal(wrapped, target)
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
	if err != nil || resp == nil || !resp.Ok {
		return ""
	}
	var result struct {
		AccessToken string `json:"accessToken"`
	}
	if resp.JSON(&result) != nil {
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
