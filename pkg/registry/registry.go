package registry

import (
	"fmt"
	"strings"
	"sync"

	"github.com/maczh/mgin/v2/pkg/config"
	"github.com/maczh/mgin/v2/pkg/logs"
	"github.com/maczh/mgin/v2/pkg/registry/consul"
	"github.com/maczh/mgin/v2/pkg/registry/etcd"
	"github.com/maczh/mgin/v2/pkg/registry/nacos"
	"github.com/maczh/mgin/v2/pkg/registry/polaris"
	"gopkg.in/yaml.v3"
)

var Registry RegistryClient

// RegistryClient 是 mgin v2 扩展后的统一注册中心客户端接口。
//
// v1 接口：Register / GetServiceURL / DeRegister（保持不变，存量项目无需改动）。
// v2 新增：GetServices 多实例列表，供 client.CallCtx 的负载均衡使用。
type RegistryClient interface {
	// Register 把本服务注册到配置中心。
	Register(registryConfigData []byte)
	// GetServiceURL v1 单实例返回，保留向后兼容。
	// 内部实现通常从多实例中随机选 1 个返回，与 GetServices 等价。
	GetServiceURL(servicename string, groupName ...string) (string, string)
	// GetServices v2 新增：返回该服务的所有可用实例 URL 列表。
	// 用于客户端负载均衡（pkg/loadbalancer）。
	// 找不到实例时返 (nil, nil)，错误时返 (nil, err)。
	GetServices(servicename string, groupName ...string) ([]string, error)
	// DeRegister 注销本服务。
	DeRegister()
}

type NamedRegistryResolver interface {
	GetServiceURLFrom(registryName, service string, group ...string) (string, string)
	GetServicesFrom(registryName, service string, group ...string) ([]string, error)
}

type ConfiguredRegistrar interface{ RegisterConfigured() }

func NewRegistry() RegistryClient {
	if len(config.Config.Discovery.Registries) > 0 {
		return newRegistryCollection(config.Config.Discovery.Registries, config.Config.Discovery.DefaultRegistry)
	}
	var client RegistryClient
	switch config.Config.Discovery.Registry {
	case "nacos":
		client = &nacos.NacosClient{}
		break
	case "etcd":
		client = &etcd.EtcdClient{}
		break
	case "consul":
		client = &consul.ConsulClient{}
		break
	case "polaris":
		client = &polaris.PolarisClient{}
		break
	}
	if client == nil {
		return &noopRegistryClient{}
	}
	// v2.1：用本地缓存层包裹真实客户端，提升发现性能与稳定性。
	// 旧逻辑（每次调用都打注册中心）保留在各后端内部，缓存层仅做"读写加速 + 降级"。
	return NewCachedRegistry(client)
}

type registryEntry struct {
	prefix    string
	kind      string
	overrides config.RegistryConfig
	client    RegistryClient
}

type RegistryCollection struct {
	mu          sync.RWMutex
	entries     map[string]registryEntry
	defaultName string
	once        sync.Once
}

func newRegistryCollection(definitions []config.RegistryConfig, defaultName string) *RegistryCollection {
	c := &RegistryCollection{entries: make(map[string]registryEntry)}
	for _, definition := range definitions {
		name, kind := strings.TrimSpace(definition.Name), strings.ToLower(strings.TrimSpace(definition.Type))
		if name == "" || kind == "" {
			continue
		}
		if _, exists := c.entries[name]; exists {
			continue
		}
		var backend RegistryClient
		switch kind {
		case "nacos":
			backend = &nacos.NacosClient{}
		case "etcd":
			backend = &etcd.EtcdClient{}
		case "consul":
			backend = &consul.ConsulClient{}
		case "polaris":
			backend = &polaris.PolarisClient{}
		default:
			continue
		}
		prefix := strings.TrimSpace(definition.Prefix)
		if prefix == "" {
			prefix = kind
		}
		c.entries[name] = registryEntry{prefix: prefix, kind: kind, overrides: definition, client: NewCachedRegistry(backend)}
		if c.defaultName == "" {
			c.defaultName = name
		}
	}
	if _, ok := c.entries[defaultName]; ok {
		c.defaultName = defaultName
	}
	return c
}

func (c *RegistryCollection) RegisterConfigured() {
	c.mu.RLock()
	entries := make([]registryEntry, 0, len(c.entries))
	for _, entry := range c.entries {
		entries = append(entries, entry)
	}
	c.mu.RUnlock()
	for _, entry := range entries {
		data, err := applyRegistryOverrides(config.Config.GetConfigData(entry.prefix), entry.kind, entry.overrides)
		if err != nil {
			logs.Error("注册中心{}配置覆盖失败:{}", entry.overrides.Name, err.Error())
			continue
		}
		entry.client.Register(data)
	}
}
func (c *RegistryCollection) Register(data []byte) {
	if entry, ok := c.entry(""); ok {
		entry.client.Register(data)
	}
}
func (c *RegistryCollection) GetServiceURL(service string, group ...string) (string, string) {
	return c.GetServiceURLFrom("", service, group...)
}
func (c *RegistryCollection) GetServices(service string, group ...string) ([]string, error) {
	return c.GetServicesFrom("", service, group...)
}
func (c *RegistryCollection) GetServiceURLFrom(name, service string, group ...string) (string, string) {
	entry, ok := c.entry(name)
	if !ok {
		return "", ""
	}
	return entry.client.GetServiceURL(service, group...)
}
func (c *RegistryCollection) GetServicesFrom(name, service string, group ...string) ([]string, error) {
	entry, ok := c.entry(name)
	if !ok {
		return nil, fmt.Errorf("registry %q is not configured", name)
	}
	return entry.client.GetServices(service, group...)
}
func (c *RegistryCollection) entry(name string) (registryEntry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if name == "" {
		name = c.defaultName
	}
	entry, ok := c.entries[name]
	return entry, ok
}
func (c *RegistryCollection) DeRegister() {
	c.once.Do(func() {
		c.mu.RLock()
		clients := make([]RegistryClient, 0, len(c.entries))
		for _, entry := range c.entries {
			clients = append(clients, entry.client)
		}
		c.mu.RUnlock()
		for _, client := range clients {
			client.DeRegister()
		}
	})
}

func applyRegistryOverrides(data []byte, kind string, definition config.RegistryConfig) ([]byte, error) {
	if len(data) == 0 || (definition.Namespace == "" && definition.Group == "" && definition.ClusterName == "") {
		return data, nil
	}
	root := make(map[string]any)
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, err
	}
	goConfig, ok := root["go"].(map[string]any)
	if !ok {
		goConfig = make(map[string]any)
		root["go"] = goConfig
	}
	backend, ok := goConfig[kind].(map[string]any)
	if !ok {
		backend = make(map[string]any)
		goConfig[kind] = backend
	}
	if definition.Namespace != "" {
		backend["namespace"] = definition.Namespace
	}
	if definition.Group != "" {
		backend["group"] = definition.Group
	}
	if definition.ClusterName != "" {
		backend["clusterName"] = definition.ClusterName
	}
	if kind == "etcd" {
		if _, exists := backend["keyTemplate"]; !exists && (definition.Namespace != "" || definition.ClusterName != "") {
			backend["keyTemplate"] = "{namespace}/{prefix}/{group}/{cluster}/{service}/{instance}"
		}
	}
	return yaml.Marshal(root)
}

type noopRegistryClient struct{}

func (*noopRegistryClient) Register([]byte)                                  {}
func (*noopRegistryClient) GetServiceURL(string, ...string) (string, string) { return "", "" }
func (*noopRegistryClient) GetServices(string, ...string) ([]string, error)  { return nil, nil }
func (*noopRegistryClient) DeRegister()                                      {}
