package etcd

import (
	"context"
	"crypto/md5"
	"fmt"
	jsoniter "github.com/json-iterator/go"
	"github.com/knadh/koanf"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/rawbytes"
	"github.com/maczh/mgin/config"
	"github.com/sadlil/gologger"
	clientv3 "go.etcd.io/etcd/client/v3"
	"io"
	"math/rand"
	"net"
	"strings"
	"time"
)

type EtcdClient struct {
	client     *clientv3.Client
	cluster    string
	group      string
	lan        bool
	lanNetwork string
	conf       *koanf.Koanf
	confUrl    string
	confData   []byte
}

var logger = gologger.GetLogger()

func (c *EtcdClient) Register(etcdConfigData []byte) {
	if etcdConfigData != nil {
		c.confData = etcdConfigData
	}
	//if c.confUrl == "" {
	//	logger.Error("Etcd配置Url为空")
	//	return
	//}
	if c.conf == nil {
		//var confData []byte
		var err error
		//if strings.HasPrefix(c.confUrl, "http://") {
		//	resp, err := grequests.Get(c.confUrl, nil)
		//	if err != nil {
		//		logger.Error("Etcd注册中心配置下载失败! " + err.Error())
		//		return
		//	}
		//	confData = []byte(resp.String())
		//} else {
		//	confData, err = ioutil.ReadFile(c.confUrl)
		//	if err != nil {
		//		logger.Error(fmt.Sprintf("Etcd注册中心本地配置文件%s读取失败:%s", c.confUrl, err.Error()))
		//		return
		//	}
		//}
		c.conf = koanf.New(".")
		err = c.conf.Load(rawbytes.Provider(c.confData), yaml.Parser())
		if err != nil {
			logger.Error("Etcd注册中心配置文件解析错误:" + err.Error())
			c.conf = nil
			return
		}
		c.lan = c.conf.Bool("go.etcd.lan")
		c.lanNetwork = c.conf.String("go.etcd.lanNet")
		ipstr := c.conf.String("go.etcd.server")
		portstr := c.conf.String("go.etcd.port")
		c.group = c.conf.String("go.etcd.group")
		if c.group == "" {
			c.group = "DEFAULT_GROUP"
		}
		c.cluster = c.conf.String("go.etcd.clusterName")
		if c.cluster == "" {
			c.cluster = "DEFAULT"
		}
		ips := strings.Split(ipstr, ",")
		ports := strings.Split(portstr, ",")
		etcd_urls := make([]string, 0)
		for i, ip := range ips {
			etcd_urls = append(etcd_urls, fmt.Sprintf("http://%s:%s", ip, ports[i]))
		}
		serverConfig := clientv3.Config{Endpoints: etcd_urls, DialTimeout: 5 * time.Second}
		//logger.Debug("Etcd客户端配置: " + toJSON(serverConfig))
		c.client, err = clientv3.New(serverConfig)
		if err != nil {
			logger.Error("Etcd服务连接失败:" + err.Error())
			return
		}
		localip, _ := localIPv4s(c.lan, c.lanNetwork)
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
		apiUrl := fmt.Sprintf("%s%s:%d", protocol, ip, port)
		//if config.Config.App.Debug {
		//	metadata["debug"] = "true"
		//}
		key := fmt.Sprintf("/registry/%s/%s/%s/%s", c.cluster, c.group, config.Config.App.Name, getInstanceId(ip, port))
		_, regerr := c.client.Put(context.Background(), key, apiUrl)
		if regerr != nil {
			logger.Error("Etcd注册服务失败:" + regerr.Error())
			return
		}
	}
}

func (c *EtcdClient) GetServiceURL(servicename string, groupName ...string) (string, string) {
	if len(groupName) == 0 {
		groupName = append(groupName, "DEFAULT_GROUP")
	}
	currentGroup := groupName[0]
	for _, group := range groupName {
		prefix := fmt.Sprintf("/registry/%s/%s/%s/", c.cluster, group, servicename)
		resp, err := c.client.Get(context.Background(), prefix, clientv3.WithPrefix())
		if err != nil {
			continue
		}
		if len(resp.Kvs) == 0 {
			continue
		}
		currentGroup = group
		r := rand.New(rand.NewSource(time.Now().UnixNano()))
		kv := resp.Kvs[r.Intn(len(resp.Kvs))]
		return string(kv.Value), currentGroup
	}
	return "", currentGroup
}

func (c *EtcdClient) DeRegister() {
	localip, _ := localIPv4s(c.lan, c.lanNetwork)
	ip := localip[0]
	if config.Config.App.IpAddr != "" {
		ip = config.Config.App.IpAddr
	}
	port := uint64(config.Config.App.Port)
	if port == 0 || config.Config.App.PortSSL != 0 {
		port = uint64(config.Config.App.PortSSL)
	}
	key := fmt.Sprintf("/registry/%s/%s/%s/%s", c.cluster, c.group, config.Config.App.Name, getInstanceId(ip, port))
	_, err := c.client.Delete(context.Background(), key)
	if err != nil {
		logger.Error("Etcd取消注册服务失败:" + err.Error())
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

func getInstanceId(ip string, port uint64) string {
	h := md5.New()
	_, _ = io.WriteString(h, fmt.Sprintf("http://%s:%d", ip, port))
	md := fmt.Sprintf("%x", h.Sum(nil))[:4]
	return md
}
