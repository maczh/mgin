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
	"github.com/maczh/mgin/v2/pkg/config"
	"github.com/sadlil/gologger"

	"github.com/knadh/koanf"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/rawbytes"
)

type NacosClient struct {
	nsurl      string
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
	ticker *time.Ticker
	quit   chan struct{}
	wg     *sync.WaitGroup
}

func (w *nacosHeartbeatWorker) Start(nc *NacosClient) {
	defer w.wg.Done()
	w.ticker = time.NewTicker(5 * time.Second)
	defer w.ticker.Stop()
	w.quit = make(chan struct{})
	for {
		select {
		case <-w.quit:
			return
		case <-w.ticker.C:
			_, err := grequests.DoRegularRequest(http.MethodPut, nc.nsurl, &grequests.RequestOptions{
				Params: nc.param,
			})
			if err != nil {
				logger.Error("Nacos心跳失败:" + err.Error())
			}
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
		n.ver = n.conf.String("go.nacos.apiversion")
		if n.ver == "" {
			n.ver = "v1"
		}
		n.nsurl = fmt.Sprintf("http://%s:%s/nacos/%s/ns/instance", ipstr, portstr, n.ver)
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
		n.param = map[string]string{
			"ip":          ip,
			"port":        fmt.Sprintf("%d", port),
			"weight":      "1",
			"cluster":     n.cluster,
			"groupName":   n.group,
			"serviceName": config.Config.App.Name,
			"meta":        toJSON(metadata),
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
			wg: &sync.WaitGroup{},
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

// GetServices v2 新增：返回该服务的所有可用实例 URL 列表（健康检查通过 + enabled）。
// 直接复用 GetServiceURL 已有的 /list 调用，去掉"随机选 1 个"这一步。
func (n *NacosClient) GetServices(servicename string, groupName ...string) ([]string, error) {
	var instance InstanceResp
	var resp *grequests.Response
	var err error

	doList := func(group string) {
		q := map[string]string{"serviceName": servicename}
		if group != "" {
			q["groupName"] = group
		}
		resp, err = grequests.DoRegularRequest(http.MethodGet, n.nsurl+"/list", &grequests.RequestOptions{
			Params: q,
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
			if uerr := json.Unmarshal(resp.Bytes(), &instance); uerr != nil {
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
	if uerr := json.Unmarshal(resp.Bytes(), &instance); uerr != nil {
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

func (n *NacosClient) DeRegister() {
	_, err := grequests.DoRegularRequest(http.MethodDelete, n.nsurl, &grequests.RequestOptions{
		Params: n.param,
	})
	if err != nil {
		logger.Error("Nacos注销服务失败:" + err.Error())
	}
	n.worker.quit <- struct{}{}
	n.worker.wg.Wait()
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
