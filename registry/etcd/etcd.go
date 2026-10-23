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
	registryModel "github.com/maczh/mgin/registry/model"
	"github.com/maczh/mgin/utils"
	"github.com/sadlil/gologger"
	clientv3 "go.etcd.io/etcd/client/v3"
)

type EtcdClient struct {
	client        *clientv3.Client
	leaseID       clientv3.LeaseID
	cluster       string
	group         string
	prefix        string
	lan           bool
	lanNetwork    string
	conf          *koanf.Koanf
	confUrl       string
	confData      []byte
	instanceId    string
	registeredKey string
	keyTemplate   string
	apiUrl        string
	ctx           context.Context
	cancel        context.CancelFunc
}

var logger = gologger.GetLogger()

func (c *EtcdClient) Register(etcdConfigData []byte) {
	if etcdConfigData == nil || len(etcdConfigData) == 0 {
		logger.Error("Etcd 配置错误，无法获取配置地址")
		return
	}
	c.confData = etcdConfigData
	logger.Debug("etcd配置文件:\n" + string(c.confData))
	if c.conf == nil {
		var err error
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
		c.prefix = c.conf.String("go.etcd.prefix")
		c.keyTemplate = c.conf.String("go.etcd.keyTemplate")
		if c.keyTemplate == "" {
			c.keyTemplate = "{prefix}/{group}/{service}/{instance}"
		}
		c.group = c.conf.String("go.etcd.group")
		if c.group == "" {
			c.group = config.Config.App.Project
		}
		c.cluster = c.conf.String("go.etcd.clusterName")
		if c.cluster == "" {
			c.cluster = "DEFAULT"
		}
		ips := strings.Split(ipstr, ",")
		ports := strings.Split(portstr, ",")
		etcd_urls := make([]string, 0)
		for i, ip := range ips {
			if i < len(ports) {
				etcd_urls = append(etcd_urls, fmt.Sprintf("http://%s:%s", ip, ports[i]))
			}
		}
		if len(etcd_urls) == 0 {
			logger.Error("Etcd 注册中心地址配置为空")
			return
		}
		serverConfig := clientv3.Config{
			Endpoints: etcd_urls, DialTimeout: 5 * time.Second,
			Username: c.conf.String("go.etcd.username"), Password: c.conf.String("go.etcd.password"),
		}
		logger.Debug("Etcd客户端配置: " + toJSON(serverConfig))
		c.client, err = clientv3.New(serverConfig)
		if err != nil {
			logger.Error("Etcd服务连接失败:" + err.Error())
			return
		}
		localip, err := localIPv4s(c.lan, c.lanNetwork)
		if err != nil || len(localip) == 0 {
			logger.Error("Etcd 注册中心获取本机IP失败")
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
		apiUrl := fmt.Sprintf("%s%s:%d", protocol, ip, port)
		prefix := c.servicePrefix(c.group, config.Config.App.Name)
		resp, err := c.client.Get(context.Background(), prefix, clientv3.WithPrefix())
		if err != nil {
			logger.Error("Etcd获取服务失败:" + err.Error())
			return
		}
		instanceIds := make([]string, 0)
		if len(resp.Kvs) > 0 {
			for _, kv := range resp.Kvs {
				if string(kv.Value) == apiUrl {
					if len(kv.Key) > len(prefix) {
						instanceIds = append(instanceIds, string(kv.Key)[len(prefix):])
					}
				}
			}
		}
		if len(instanceIds) > 0 {
			for _, instanceId := range instanceIds {
				c.client.Delete(context.Background(), prefix+instanceId)
			}
		}
		respGrant, err := c.client.Grant(context.Background(), 10000)
		if err != nil {
			logger.Error("Etcd注册服务失败:" + err.Error())
			return
		}
		c.leaseID = respGrant.ID
		c.instanceId = utils.NewUUIDString()
		key := c.serviceKey(c.group, config.Config.App.Name, c.instanceId)
		logger.Debug("etcd服务的key: " + key + "，值：" + apiUrl)
		res, regerr := c.client.Put(context.Background(), key, apiUrl, clientv3.WithLease(c.leaseID))
		if regerr != nil {
			logger.Error("Etcd注册服务失败:" + regerr.Error())
			return
		}
		//cache.OnMemCache("etcd_service").Set("instance_id", c.instanceId, 5*time.Second)
		logger.Debug("etcd服务注册结果: " + toJSON(res))
		c.apiUrl = apiUrl
		c.registeredKey = key
		c.ctx, c.cancel = context.WithCancel(context.Background())
		go c.keepAliveLoop()
	}
}

func (c *EtcdClient) GetServiceURL(servicename string, groupName ...string) (string, string) {
	if len(groupName) == 0 {
		groupName = []string{c.group}
	} else if groupName[0] == "" {
		groupName[0] = c.group
	}
	currentGroup := groupName[0]
	// logger.Debug(fmt.Sprintf("groupName=%s, etcdClient=%s", toJSON(groupName), toJSON(c)))
	if c.client == nil {
		return "", currentGroup
	}
	for _, group := range groupName {
		prefix := c.servicePrefix(group, servicename)
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
		instanceId := ""
		if len(kv.Key) > len(prefix) {
			instanceId = string(kv.Key)[len(prefix):]
		}
		cache.OnMemCache("etcd_service").Set(fmt.Sprintf("etcd_%s_%s", servicename, string(kv.Value)), instanceId, 5*time.Second)
		return string(kv.Value), currentGroup
	}
	return "", currentGroup
}

func (c *EtcdClient) GetServiceInstances(servicename string, groupName ...string) ([]registryModel.ServiceInstance, string) {
	group := c.group
	if len(groupName) > 0 && groupName[0] != "" {
		group = groupName[0]
	}
	if c.client == nil {
		return nil, group
	}
	resp, err := c.client.Get(context.Background(), c.servicePrefix(group, servicename), clientv3.WithPrefix())
	if err != nil {
		return nil, group
	}
	instances := make([]registryModel.ServiceInstance, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		if len(kv.Value) > 0 {
			instances = append(instances, registryModel.ServiceInstance{URL: string(kv.Value), Weight: 1})
		}
	}
	return instances, group
}

func (c *EtcdClient) DeRegister() {
	if c.client == nil {
		logger.Error("Etcd 未初始化，无法注销服务")
		return
	}
	if c.cancel != nil {
		c.cancel()
	}
	key := c.registeredKey
	if key == "" {
		// 兜底：按 instanceId 拼出 key（极端情况下 registeredKey 未记录）
		key = c.serviceKey(c.group, config.Config.App.Name, c.instanceId)
	}
	resp, err := c.client.Delete(context.Background(), key)
	if err != nil {
		logger.Error("Etcd取消注册服务失败:" + err.Error())
		return
	}
	logger.Debug("Etcd注销服务结果: " + toJSON(resp))
}

func (c *EtcdClient) serviceKey(group, service, instance string) string {
	return strings.TrimSuffix(strings.NewReplacer("{prefix}", c.prefix, "{group}", group, "{service}", service, "{instance}", instance).Replace(c.keyTemplate), "/")
}
func (c *EtcdClient) servicePrefix(group, service string) string {
	return c.serviceKey(group, service, "") + "/"
}

// SubscribeOffline watches delete events and reports the affected service.
// The callback is invoked asynchronously so a slow cache consumer never
// blocks etcd's watch loop.
func (c *EtcdClient) SubscribeOffline(callback func(service, group string)) error {
	if c.client == nil || callback == nil {
		return fmt.Errorf("etcd is not initialized")
	}
	watchCtx := context.Background()
	if c.ctx != nil {
		watchCtx = c.ctx
	}
	go func() {
		watch := c.client.Watch(watchCtx, strings.TrimSuffix(c.prefix, "/")+"/", clientv3.WithPrefix())
		for response := range watch {
			for _, event := range response.Events {
				if event.Type != clientv3.EventTypeDelete {
					continue
				}
				parts := strings.Split(strings.Trim(string(event.Kv.Key), "/"), "/")
				if len(parts) < 4 {
					continue
				}
				service := parts[len(parts)-2]
				group := parts[len(parts)-3]
				go callback(service, group)
			}
		}
	}()
	return nil
}

func (c *EtcdClient) WatchService(service, group string, callback func()) (func(), error) {
	if c.client == nil || callback == nil {
		return nil, fmt.Errorf("etcd is not initialized")
	}
	if group == "" {
		group = c.group
	}
	ctx, cancel := context.WithCancel(context.Background())
	prefix := c.servicePrefix(group, service)
	go func() {
		watch := c.client.Watch(ctx, prefix, clientv3.WithPrefix())
		for response := range watch {
			for _, event := range response.Events {
				if event.Type == clientv3.EventTypeDelete {
					callback()
					return
				}
			}
		}
	}()
	return cancel, nil
}

// keepAliveLoop 持续续约租约；当租约通道关闭或续约失败时自动重建租约并重新注册，
// 避免连接抖动导致实例在 etcd 中“假死”（租约过期被摘除）。
func (c *EtcdClient) keepAliveLoop() {
	for {
		select {
		case <-c.ctx.Done():
			return
		default:
		}
		respKeepAlive, err := c.client.KeepAlive(c.ctx, c.leaseID)
		if err != nil {
			logger.Error("Etcd自动续约失败，尝试重新注册: " + err.Error())
			if !c.reRegister() {
				select {
				case <-c.ctx.Done():
					return
				case <-time.After(time.Second):
				}
			}
			continue
		}
	keepAlive:
		for {
			select {
			case <-c.ctx.Done():
				return
			case _, ok := <-respKeepAlive:
				if !ok {
					logger.Warn("Etcd租约续约通道关闭，重新注册服务")
					if !c.reRegister() {
						select {
						case <-c.ctx.Done():
							return
						case <-time.After(time.Second):
						}
					}
					break keepAlive
				}
			}
		}
	}
}

// reRegister 吊销旧租约并重新申请租约、重新写入本实例，保持注册信息有效。
func (c *EtcdClient) reRegister() bool {
	if c.client == nil || c.apiUrl == "" || c.registeredKey == "" {
		return false
	}
	if c.leaseID != clientv3.NoLease {
		_, _ = c.client.Revoke(context.Background(), c.leaseID)
	}
	respGrant, err := c.client.Grant(context.Background(), 10000)
	if err != nil {
		logger.Error("Etcd重新申请租约失败: " + err.Error())
		return false
	}
	c.leaseID = respGrant.ID
	_, err = c.client.Put(context.Background(), c.registeredKey, c.apiUrl, clientv3.WithLease(c.leaseID))
	if err != nil {
		logger.Error("Etcd重新注册服务失败: " + err.Error())
		return false
	}
	logger.Info("Etcd重新注册服务成功: " + c.registeredKey)
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
