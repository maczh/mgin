package etcd

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"strings"
	"sync"
	"time"

	jsoniter "github.com/json-iterator/go"
	"github.com/knadh/koanf"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/rawbytes"
	"github.com/maczh/mgin/config"
	registryModel "github.com/maczh/mgin/registry/model"
	"github.com/maczh/mgin/utils"
	"github.com/sadlil/gologger"
	clientv3 "go.etcd.io/etcd/client/v3"
)

type EtcdClient struct {
	client        *clientv3.Client
	leaseID       clientv3.LeaseID
	leaseTTL      int64
	cluster       string
	namespace     string
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
	closeOnce     sync.Once
	workerWG      sync.WaitGroup
}

var logger = gologger.GetLogger()

// defaultLeaseTTL 是服务注册租约的默认有效期（秒），可通过 go.etcd.leaseTTL 覆盖。
// keepAlive 按 TTL/3 自动续约，TTL 必须明显大于一次续约往返的耗时：
// 跨网络访问 etcd 时 10s 这类过小的值会在抖动时直接过期，导致反复重注册。
const defaultLeaseTTL int64 = 30

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
		c.namespace = c.conf.String("go.etcd.namespace")
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
		c.leaseTTL = c.conf.Int64("go.etcd.leaseTTL")
		ips := strings.Split(ipstr, ",")
		ports := strings.Split(portstr, ",")
		// 多成员集群可只配一个端口，其余成员沿用第一个端口，避免漏配时丢成员导致无故障转移。
		etcd_urls := make([]string, 0)
		for i, ip := range ips {
			ip = strings.TrimSpace(ip)
			port := ""
			if i < len(ports) {
				port = strings.TrimSpace(ports[i])
			} else if len(ports) > 0 {
				port = strings.TrimSpace(ports[0])
			}
			if ip != "" && port != "" {
				etcd_urls = append(etcd_urls, fmt.Sprintf("http://%s:%s", ip, port))
			}
		}
		if len(etcd_urls) == 0 {
			logger.Error("Etcd 注册中心地址配置为空")
			return
		}
		serverConfig := clientv3.Config{
			Endpoints: etcd_urls, DialTimeout: 5 * time.Second,
			// 跨 NAT/防火墙的长连接常被中间设备静默丢弃，没有 gRPC 心跳时只能等下一次
			// RPC 失败才发现，续约通道会因此反复关闭。
			DialKeepAliveTime: 10 * time.Second, DialKeepAliveTimeout: 3 * time.Second,
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
		respGrant, err := c.client.Grant(context.Background(), c.ttl())
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
		logger.Debug("etcd服务注册结果: " + toJSON(res))
		c.apiUrl = apiUrl
		c.registeredKey = key
		c.ctx, c.cancel = context.WithCancel(context.Background())
		c.workerWG.Add(1)
		go func() { defer c.workerWG.Done(); c.keepAliveLoop() }()
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
	c.closeOnce.Do(func() {
		if c.cancel != nil {
			c.cancel()
		}
		c.workerWG.Wait()
		if c.client == nil {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if c.registeredKey != "" {
			resp, err := c.client.Delete(ctx, c.registeredKey)
			if err != nil {
				logger.Error("Etcd取消注册服务失败:" + err.Error())
			} else {
				logger.Debug("Etcd注销服务结果: " + toJSON(resp))
			}
		}
		if c.leaseID != clientv3.NoLease {
			if _, err := c.client.Revoke(ctx, c.leaseID); err != nil {
				logger.Error("Etcd撤销租约失败:" + err.Error())
			}
		}
		if err := c.client.Close(); err != nil {
			logger.Error("Etcd关闭客户端失败:" + err.Error())
		}
	})
}

// ttl 返回本实例的租约有效期（秒），未配置时使用默认值。
func (c *EtcdClient) ttl() int64 {
	if c.leaseTTL > 0 {
		return c.leaseTTL
	}
	return defaultLeaseTTL
}

func (c *EtcdClient) serviceKey(group, service, instance string) string {
	return strings.TrimSuffix(strings.NewReplacer("{namespace}", c.namespace, "{prefix}", c.prefix, "{group}", group, "{cluster}", c.cluster, "{service}", service, "{instance}", instance).Replace(c.keyTemplate), "/")
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
	c.workerWG.Add(1)
	go func() {
		defer c.workerWG.Done()
		watch := c.client.Watch(watchCtx, c.watchPrefix(), clientv3.WithPrefix())
		for response := range watch {
			for _, event := range response.Events {
				if event.Type != clientv3.EventTypeDelete {
					continue
				}
				service, group, ok := c.scopeFromKey(string(event.Kv.Key))
				if !ok {
					continue
				}
				go callback(service, group)
			}
		}
	}()
	return nil
}

func (c *EtcdClient) watchPrefix() string {
	prefix := c.keyTemplate
	first := -1
	for _, marker := range []string{"{group}", "{service}", "{instance}"} {
		if index := strings.Index(prefix, marker); index >= 0 && (first < 0 || index < first) {
			first = index
		}
	}
	if first >= 0 {
		prefix = prefix[:first]
	}
	prefix = strings.NewReplacer("{namespace}", c.namespace, "{prefix}", c.prefix, "{cluster}", c.cluster).Replace(prefix)
	return strings.TrimSuffix(prefix, "/") + "/"
}

func (c *EtcdClient) scopeFromKey(key string) (service, group string, ok bool) {
	templateParts := strings.Split(strings.Trim(c.keyTemplate, "/"), "/")
	keyParts := strings.Split(strings.Trim(key, "/"), "/")
	if len(templateParts) != len(keyParts) {
		return "", "", false
	}
	for index, part := range templateParts {
		switch part {
		case "{service}":
			service = keyParts[index]
		case "{group}":
			group = keyParts[index]
		}
	}
	return service, group, service != "" && group != ""
}

func (c *EtcdClient) WatchService(service, group string, callback func()) (func(), error) {
	if c.client == nil || callback == nil {
		return nil, fmt.Errorf("etcd is not initialized")
	}
	if group == "" {
		group = c.group
	}
	ctx, cancel := context.WithCancel(context.Background())
	c.workerWG.Add(1)
	prefix := c.servicePrefix(group, service)
	go func() {
		defer c.workerWG.Done()
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

// keepAliveLoop 持续续约租约；当续约通道关闭或续约失败时自动重建租约并重新注册，
// 避免连接抖动导致实例在 etcd 中“假死”（租约过期被摘除）。
func (c *EtcdClient) keepAliveLoop() {
	// 通道关闭后已成功探活的场景直接重建通道，不再重复续约一次。
	skipProbe := false
	for {
		select {
		case <-c.ctx.Done():
			return
		default:
		}
		// KeepAlive 会在内部持续续约，但显式先续约一次，避免新租约或
		// 重注册完成后等待客户端续约调度，确保租约立即进入续约状态。
		// 探活必须带超时：网络不可用时 unary 调用会长时间阻塞，期间既不重试也不重注册。
		if !skipProbe {
			if err := c.keepAliveOnce(); err != nil {
				logger.Warn("Etcd立即续约失败，尝试重新注册: " + err.Error())
				if !c.reRegister() && !c.retryDelay() {
					return
				}
				continue
			}
		}
		skipProbe = false
		respKeepAlive, err := c.client.KeepAlive(c.ctx, c.leaseID)
		if err != nil {
			logger.Error("Etcd自动续约失败，尝试重新注册: " + err.Error())
			if !c.reRegister() && !c.retryDelay() {
				return
			}
			continue
		}
	keepAlive:
		for {
			select {
			case <-c.ctx.Done():
				return
			case _, ok := <-respKeepAlive:
				if ok {
					continue
				}
				// 通道关闭不代表租约已失效：先探活一次，租约仍有效时只重建通道。
				// 每次重新注册都会吊销租约删除 key，触发所有消费者的 delete 事件清空缓存。
				err := c.keepAliveOnce()
				if err == nil {
					logger.Info("Etcd租约续约通道关闭，租约仍有效，重建续约通道")
					skipProbe = true
					break keepAlive
				}
				logger.Warn("Etcd租约续约通道关闭且续约失败，重新注册服务: " + err.Error())
				if !c.reRegister() && !c.retryDelay() {
					return
				}
				break keepAlive
			}
		}
	}
}

// keepAliveOnce 在有限超时内显式续约一次，超时取 TTL/3（不小于 3 秒）。
// 直接使用 c.ctx 时，etcd 客户端会在内部重试直到 ctx 结束，网络异常时可能阻塞两分钟。
func (c *EtcdClient) keepAliveOnce() error {
	timeout := time.Duration(c.ttl()/3) * time.Second
	if timeout < 3*time.Second {
		timeout = 3 * time.Second
	}
	ctx, cancel := context.WithTimeout(c.ctx, timeout)
	defer cancel()
	_, err := c.client.KeepAliveOnce(ctx, c.leaseID)
	return err
}

// retryDelay 在重试前等待 1 秒，ctx 结束时返回 false。
func (c *EtcdClient) retryDelay() bool {
	select {
	case <-c.ctx.Done():
		return false
	case <-time.After(time.Second):
		return true
	}
}

// reRegister 重新申请租约并重写本实例的注册信息。
// 顺序是 Grant → Put → 吊销旧租约：若先吊销旧租约，网络抖动导致 Grant 失败时旧注册
// 已被删除，实例会在 etcd 中彻底消失，消费者还会收到 delete 事件失效缓存。
func (c *EtcdClient) reRegister() bool {
	if c.client == nil || c.apiUrl == "" || c.registeredKey == "" {
		return false
	}
	if c.ctx == nil || c.ctx.Err() != nil {
		return false
	}
	grantCtx, grantCancel := context.WithTimeout(c.ctx, 10*time.Second)
	respGrant, err := c.client.Grant(grantCtx, c.ttl())
	grantCancel()
	if err != nil {
		logger.Error("Etcd重新申请租约失败: " + err.Error())
		return false
	}
	newLeaseID := respGrant.ID
	putCtx, putCancel := context.WithTimeout(c.ctx, 10*time.Second)
	_, err = c.client.Put(putCtx, c.registeredKey, c.apiUrl, clientv3.WithLease(newLeaseID))
	putCancel()
	if err != nil {
		logger.Error("Etcd重新注册服务失败: " + err.Error())
		revokeCtx, revokeCancel := context.WithTimeout(c.ctx, 5*time.Second)
		_, _ = c.client.Revoke(revokeCtx, newLeaseID)
		revokeCancel()
		return false
	}
	// 新注册已生效后再吊销旧租约；旧租约可能已过期，失败无需处理。
	// 短超时避免网络异常时阻塞重注册，也让 DeRegister 的退出能及时返回。
	oldLeaseID := c.leaseID
	c.leaseID = newLeaseID
	if oldLeaseID != clientv3.NoLease && oldLeaseID != newLeaseID {
		revokeCtx, revokeCancel := context.WithTimeout(c.ctx, 5*time.Second)
		_, _ = c.client.Revoke(revokeCtx, oldLeaseID)
		revokeCancel()
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
