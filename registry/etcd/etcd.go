package etcd

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"strings"
	"time"

	jsoniter "github.com/json-iterator/go"
	"github.com/knadh/koanf"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/rawbytes"
	"github.com/maczh/mgin/cache"
	"github.com/maczh/mgin/config"
	"github.com/maczh/mgin/utils"
	"github.com/sadlil/gologger"
	clientv3 "go.etcd.io/etcd/client/v3"
)

type EtcdClient struct {
	client     *clientv3.Client
	cluster    string
	group      string
	prefix     string
	lan        bool
	lanNetwork string
	conf       *koanf.Koanf
	confUrl    string
	confData   []byte
	instanceId string
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
	logger.Debug("etcd配置文件:\n" + string(c.confData))
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
			c.group = config.Config.App.Project
			if c.group == "" {
				c.group = "DEFAULT_GROUP"
			}
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
		logger.Debug("Etcd客户端配置: " + toJSON(serverConfig))
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
		prefix := fmt.Sprintf("services/%s/%s/%s/", c.cluster, c.group, config.Config.App.Name)
		resp, err := c.client.Get(context.Background(), prefix, clientv3.WithPrefix())
		if err != nil {
			logger.Error("Etcd获取服务失败:" + err.Error())
			return
		}
		instanceIds := make([]string, 0)
		if len(resp.Kvs) > 0 {
			for _, kv := range resp.Kvs {
				if string(kv.Value) == apiUrl {
					instanceIds = append(instanceIds, string(kv.Key)[len(prefix):])
				}
			}
		}
		if len(instanceIds) > 0 {
			for _, instanceId := range instanceIds {
				c.client.Delete(context.Background(), prefix+instanceId)
			}
		}
		c.instanceId = utils.NewUUIDString()
		key := fmt.Sprintf("services/%s/%s/%s/%s", c.cluster, c.group, config.Config.App.Name, c.instanceId)
		logger.Debug("etcd服务的key: " + key + "，值：" + apiUrl)
		res, regerr := c.client.Put(context.Background(), key, apiUrl)
		if regerr != nil {
			logger.Error("Etcd注册服务失败:" + regerr.Error())
			return
		}
		//cache.OnMemCache("etcd_service").Set("instance_id", c.instanceId, 5*time.Second)
		logger.Debug("etcd服务注册结果: " + toJSON(res))
	}
}

func (c *EtcdClient) GetServiceURL(servicename string, groupName ...string) (string, string) {
	if groupName[0] == "" {
		groupName[0] = c.group
	}
	currentGroup := groupName[0]
	logger.Debug(fmt.Sprintf("groupName=%s, etcdClient=%s", toJSON(groupName), toJSON(c)))
	for _, group := range groupName {
		prefix := fmt.Sprintf("services/%s/%s/%s/", c.cluster, group, servicename)
		logger.Debug("查询前缀: " + prefix)
		resp, err := c.client.Get(context.Background(), prefix, clientv3.WithPrefix())
		if err != nil {
			continue
		}
		if len(resp.Kvs) == 0 {
			continue
		}
		logger.Debug("查询服务结果: " + toJSON(resp))
		currentGroup = group
		r := rand.New(rand.NewSource(time.Now().UnixNano()))
		kv := resp.Kvs[r.Intn(len(resp.Kvs))]
		//将当前服务实例对应的instanceId保存到缓存当中
		cache.OnMemCache("etcd_service").Set(fmt.Sprintf("etcd_%s_%s", servicename, string(kv.Value)), string(kv.Key)[len(prefix):], 5*time.Second)
		return string(kv.Value), currentGroup
	}
	return "", currentGroup
}

func (c *EtcdClient) DeRegister() {
	//localip, _ := localIPv4s(c.lan, c.lanNetwork)
	//ip := localip[0]
	//if config.Config.App.IpAddr != "" {
	//	ip = config.Config.App.IpAddr
	//}
	//port := uint64(config.Config.App.Port)
	//if port == 0 || config.Config.App.PortSSL != 0 {
	//	port = uint64(config.Config.App.PortSSL)
	//}
	fmt.Printf("注销服务: instanceId=%s", c.instanceId)
	key := fmt.Sprintf("services/%s/%s/%s/%s", c.cluster, c.group, config.Config.App.Name, c.instanceId)
	fmt.Println(key)
	resp, err := c.client.Delete(context.Background(), key)
	if err != nil {
		logger.Error("Etcd取消注册服务失败:" + err.Error())
		return
	}
	fmt.Println(toJSON(resp))
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
