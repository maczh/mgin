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
	"github.com/maczh/mgin/v2/pkg/cache"
	"github.com/maczh/mgin/v2/pkg/config"
	"github.com/maczh/mgin/v2/pkg/utils"
	"github.com/sadlil/gologger"
	clientv3 "go.etcd.io/etcd/client/v3"
)

type EtcdClient struct {
	client      *clientv3.Client
	leaseID     clientv3.LeaseID
	cluster     string
	namespace   string
	group       string
	prefix      string
	keyTemplate string
	lan         bool
	lanNetwork  string
	conf        *koanf.Koanf
	confUrl     string
	confData    []byte
	instanceId  string

	// 注册稳定性增强字段
	registeredKey string             // 本实例在 etcd 上的完整 key，用于注销与重注册
	apiUrl        string             // 本实例注册值（如 http://ip:port）
	ctx           context.Context    // 续约/重注册协程的取消上下文
	cancel        context.CancelFunc // 注销时取消续约协程
}

var logger = gologger.GetLogger()

const leaseTTLSeconds int64 = 10

func (c *EtcdClient) Register(etcdConfigData []byte) {
	if c == nil {
		return
	}
	if etcdConfigData != nil {
		c.confData = etcdConfigData
	}
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
			logger.Error("Etcd 配置缺少有效 server/port")
			return
		}
		serverConfig := clientv3.Config{
			Endpoints: etcd_urls, DialTimeout: 5 * time.Second,
			// 跨 NAT/防火墙的长连接常被中间设备静默丢弃，没有 gRPC 心跳时只能等下一次
			// RPC 失败才发现，续约通道会因此反复关闭。
			DialKeepAliveTime: 10 * time.Second, DialKeepAliveTimeout: 3 * time.Second,
		}
		logger.Debug("Etcd客户端配置: " + toJSON(serverConfig))
		c.client, err = clientv3.New(serverConfig)
		if err != nil {
			logger.Error("Etcd服务连接失败:" + err.Error())
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
					key := string(kv.Key)
					if strings.HasPrefix(key, prefix) {
						instanceIds = append(instanceIds, strings.TrimPrefix(key, prefix))
					}
				}
			}
		}
		if len(instanceIds) > 0 {
			for _, instanceId := range instanceIds {
				c.client.Delete(context.Background(), prefix+instanceId)
			}
		}
		respGrant, err := c.client.Grant(context.Background(), leaseTTLSeconds)
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
		c.registeredKey = key
		c.apiUrl = apiUrl
		// 启动带重注册的续约协程：etcd 连接抖动导致租约通道关闭时自动重新注册，
		// 避免租约过期后本实例从发现列表消失（原实现在通道关闭后会静默退出，造成"假死"）。
		c.ctx, c.cancel = context.WithCancel(context.Background())
		go c.keepAliveLoop(c.ctx)
	}
}

func (c *EtcdClient) GetServiceURL(servicename string, groupName ...string) (string, string) {
	if c == nil || c.client == nil {
		return "", ""
	}
	if len(groupName) == 0 {
		groupName = []string{c.group}
	} else if groupName[0] == "" {
		groupName[0] = c.group
	}
	currentGroup := groupName[0]
	// logger.Debug(fmt.Sprintf("groupName=%s, serviceName=%s", groupName, servicename))
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
		key := string(kv.Key)
		instanceID := ""
		if strings.HasPrefix(key, prefix) {
			instanceID = strings.TrimPrefix(key, prefix)
		}
		cache.OnMemCache("etcd_service").Set(fmt.Sprintf("etcd_%s_%s", servicename, string(kv.Value)), instanceID, 5*time.Second)
		return string(kv.Value), currentGroup
	}
	return "", currentGroup
}

// GetServices v2 新增：返回该服务在 etcd 上注册的全部实例 URL 列表。
// 与 GetServiceURL 走同一条 prefix 扫描路径，只是把"随机选一个"改为"全部返回"。
func (c *EtcdClient) GetServices(servicename string, groupName ...string) ([]string, error) {
	if c == nil || c.client == nil {
		return nil, fmt.Errorf("Etcd client is nil")
	}
	if len(groupName) == 0 || groupName[0] == "" {
		groupName = []string{c.group}
	}
	for _, group := range groupName {
		prefix := c.servicePrefix(group, servicename)
		resp, err := c.client.Get(context.Background(), prefix, clientv3.WithPrefix())
		if err != nil {
			logger.Error("etcd 拉取" + servicename + "实例失败:" + err.Error())
			continue
		}
		if len(resp.Kvs) == 0 {
			continue
		}
		urls := make([]string, 0, len(resp.Kvs))
		for _, kv := range resp.Kvs {
			urls = append(urls, string(kv.Value))
		}
		// logger.Debug("etcd 获取" + servicename + "服务列表成功:" + strings.Join(urls, ","))
		return urls, nil
	}
	return nil, nil
}

// keepAliveLoop 持续为 etcd 租约续约。
// 与原实现不同，本函数会在以下场景自动恢复，避免服务"假死"：
//   - KeepAlive RPC 创建失败：退避后重试；
//   - 续约通道被关闭（连接丢失 / 租约失效）：重新申请租约并重新 Put 本实例，再继续续约；
//   - ctx 被取消（DeRegister）：立即退出。
func (c *EtcdClient) keepAliveLoop(ctx context.Context) {
	backoff := time.Second
	// 通道关闭后已成功探活的场景直接重建通道，不再重复续约一次。
	skipProbe := false
	for {
		if ctx.Err() != nil {
			return
		}
		// 探活必须带超时：网络不可用时 unary 调用会长时间阻塞，期间既不重试也不重注册，
		// 实例会在 etcd 中长时间消失（曾出现阻塞 2 分钟的情况）。
		if !skipProbe {
			if err := c.keepAliveOnce(ctx); err != nil {
				logger.Warn("Etcd立即续约失败，尝试重新注册: " + err.Error())
				if !c.reRegister(ctx) {
					select {
					case <-ctx.Done():
						return
					case <-time.After(backoff):
						backoff = minDuration(backoff*2, 30*time.Second)
					}
				} else {
					backoff = time.Second
				}
				continue
			}
		}
		skipProbe = false
		respKeepAlive, err := c.client.KeepAlive(ctx, c.leaseID)
		if err != nil {
			logger.Error("Etcd租约续约失败:" + err.Error())
			if !c.reRegister(ctx) {
				select {
				case <-ctx.Done():
					return
				case <-time.After(backoff):
					backoff = minDuration(backoff*2, 30*time.Second)
					continue
				}
			}
			backoff = time.Second
			continue
		}
		backoff = time.Second
		alive := true
		for alive {
			select {
			case <-ctx.Done():
				return
			case _, ok := <-respKeepAlive:
				if !ok {
					// 通道关闭不代表租约已失效：先探活一次，租约仍有效时只重建通道，
					// 避免无谓的重注册（重注册会删/写 key，触发消费者的 delete 事件清空缓存）。
					if err := c.keepAliveOnce(ctx); err == nil {
						logger.Info("Etcd租约续约通道关闭，租约仍有效，重建续约通道")
						skipProbe = true
					} else {
						logger.Warn("Etcd租约续约通道关闭且续约失败，准备重新注册实例: " + err.Error())
						if !c.reRegister(ctx) {
							select {
							case <-ctx.Done():
								return
							case <-time.After(backoff):
								backoff = minDuration(backoff*2, 30*time.Second)
							}
						} else {
							backoff = time.Second
						}
					}
					alive = false
				}
			}
		}
	}
}

// reRegister 在租约失效/连接恢复后，重新申请租约并把本实例重新写入 etcd。
// 使用固定的 instanceId 与 key，保证幂等（重复调用不会产生重复实例）。
// 返回 true 表示重注册成功。
func (c *EtcdClient) reRegister(ctx context.Context) bool {
	if c.client == nil || c.apiUrl == "" {
		return false
	}
	// 申请租约与写入分开限时，避免网络抖动时 unary 调用无限阻塞，拖住整个续约协程。
	grantCtx, grantCancel := context.WithTimeout(ctx, 10*time.Second)
	respGrant, err := c.client.Grant(grantCtx, leaseTTLSeconds)
	grantCancel()
	if err != nil {
		logger.Error("Etcd重新注册失败,申请租约错误:" + err.Error())
		return false
	}
	newLeaseID := respGrant.ID
	// 直接覆盖写 registeredKey：不先 Delete，否则写入失败时旧注册已被删除，
	// 实例会从发现列表彻底消失，消费者还会收到 delete 事件。
	putCtx, putCancel := context.WithTimeout(ctx, 10*time.Second)
	_, err = c.client.Put(putCtx, c.registeredKey, c.apiUrl, clientv3.WithLease(newLeaseID))
	putCancel()
	if err != nil {
		logger.Error("Etcd重新注册失败,写入错误:" + err.Error())
		revokeCtx, revokeCancel := context.WithTimeout(ctx, 5*time.Second)
		_, _ = c.client.Revoke(revokeCtx, newLeaseID)
		revokeCancel()
		return false
	}
	oldLeaseID := c.leaseID
	c.leaseID = newLeaseID
	// 新注册已生效后再吊销旧租约；旧租约可能已过期，失败无需处理。
	if oldLeaseID != clientv3.NoLease && oldLeaseID != newLeaseID {
		revokeCtx, revokeCancel := context.WithTimeout(ctx, 5*time.Second)
		_, _ = c.client.Revoke(revokeCtx, oldLeaseID)
		revokeCancel()
	}
	logger.Info("Etcd服务实例重新注册成功:" + c.registeredKey)
	return true
}

// keepAliveOnce 在有限超时内显式续约一次，超时取 TTL/3（不小于 3 秒）。
// 直接传入续约协程的 ctx 时，etcd 客户端会在内部重试直到 ctx 结束，
// 网络异常时可能阻塞很久，期间既不重试也不重注册。
func (c *EtcdClient) keepAliveOnce(ctx context.Context) error {
	timeout := time.Duration(leaseTTLSeconds/3) * time.Second
	if timeout < 3*time.Second {
		timeout = 3 * time.Second
	}
	onceCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	_, err := c.client.KeepAliveOnce(onceCtx, c.leaseID)
	return err
}

func (c *EtcdClient) DeRegister() {
	if c == nil || c.client == nil {
		return
	}
	// 1) 先取消续约协程，避免注销后又被心跳重新写入
	if c.cancel != nil {
		c.cancel()
	}
	// 2) 删除本实例在 etcd 上的 key
	if c.registeredKey != "" {
		resp, err := c.client.Delete(context.Background(), c.registeredKey)
		if err != nil {
			logger.Error("Etcd取消注册服务失败:" + err.Error())
			return
		}
		logger.Debug("Etcd注销服务结果: " + toJSON(resp))
	} else {
		// 兜底：未记录 registeredKey（注册中途失败）时按拼接规则尝试删除
		key := c.serviceKey(c.group, config.Config.App.Name, c.instanceId)
		if _, err := c.client.Delete(context.Background(), key); err != nil {
			logger.Error("Etcd取消注册服务失败:" + err.Error())
			return
		}
	}
	logger.Info("Etcd注销服务成功")
}

func (c *EtcdClient) serviceKey(group, service, instance string) string {
	return strings.TrimSuffix(strings.NewReplacer("{namespace}", c.namespace, "{prefix}", c.prefix, "{group}", group, "{cluster}", c.cluster, "{service}", service, "{instance}", instance).Replace(c.keyTemplate), "/")
}
func (c *EtcdClient) servicePrefix(group, service string) string {
	return c.serviceKey(group, service, "") + "/"
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

// minDuration 返回 a、b 中较小的一个，用于续约重试退避的收敛。
func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
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
