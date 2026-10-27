package registry

import (
	"fmt"
	"strings"
	"sync"

	"github.com/maczh/mgin/config"
	"github.com/maczh/mgin/registry/consul"
	"github.com/maczh/mgin/registry/etcd"
	registryModel "github.com/maczh/mgin/registry/model"
	"github.com/maczh/mgin/registry/nacos"
	"github.com/maczh/mgin/registry/polaris"
	"gopkg.in/yaml.v3"
)

var Registry RegistryClient

type RegistryClient interface {
	Register(registryConfigData []byte)
	GetServiceURL(servicename string, groupName ...string) (string, string)
	DeRegister()
}

// ServiceInstance is the common discovery model used by optional registry
// extensions. Existing RegistryClient implementations remain source compatible.
type ServiceInstance = registryModel.ServiceInstance

// InstanceProvider is optional. Backends that can return the complete healthy
// instance set may implement it; the legacy URL API remains the fallback.
type InstanceProvider interface {
	GetServiceInstances(service string, group ...string) ([]ServiceInstance, string)
}

type OfflineSubscriber interface {
	SubscribeOffline(func(service, group string)) error
}

type ServiceWatcher interface {
	WatchService(service, group string, callback func()) (cancel func(), err error)
}

// NamedRegistryResolver is an optional extension implemented by a registry
// collection. It keeps the legacy RegistryClient interface source compatible.
type NamedRegistryResolver interface {
	GetServiceURLFrom(registryName, service string, group ...string) (string, string)
}

// ConfiguredRegistrar initializes registry instances from their named config
// prefixes. Applications normally let mgin.Init/NewApp call this method.
type ConfiguredRegistrar interface {
	RegisterConfigured()
}

type SelectionStrategy string

const (
	StrategyRandom     SelectionStrategy = "random"
	StrategyRoundRobin SelectionStrategy = "round_robin"
	StrategyWeighted   SelectionStrategy = "weighted"
)

func NewRegistry() RegistryClient {
	var client RegistryClient
	if config.Config == nil {
		return &noopRegistryClient{}
	}
	if len(config.Config.Discovery.Registries) > 0 {
		return newRegistryCollection(config.Config.Discovery.Registries, config.Config.Discovery.DefaultRegistry)
	}
	switch config.Config.Discovery.Registry {
	case "nacos":
		client = NewCachedRegistry(&nacos.NacosClient{})
	case "etcd":
		client = NewCachedRegistry(&etcd.EtcdClient{})
	case "consul":
		client = NewCachedRegistry(&consul.ConsulClient{})
	case "polaris":
		client = NewCachedRegistry(&polaris.PolarisClient{})
	default:
		// 未配置或未知的注册中心类型时返回空实现，避免调用方对 nil 接口调用方法而 panic
		client = &noopRegistryClient{}
	}
	return client
}

type registryEntry struct {
	prefix    string
	kind      string
	overrides config.RegistryConfig
	client    RegistryClient
}

// RegistryCollection holds independently cached named registry instances.
// Each child owns its own cache, refresh loop and service subscriptions.
type RegistryCollection struct {
	mu          sync.RWMutex
	entries     map[string]registryEntry
	defaultName string
	once        sync.Once
}

func newRegistryCollection(definitions []config.RegistryConfig, defaultName string) *RegistryCollection {
	c := &RegistryCollection{entries: make(map[string]registryEntry)}
	for _, definition := range definitions {
		name := strings.TrimSpace(definition.Name)
		kind := strings.ToLower(strings.TrimSpace(definition.Type))
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

// NewRegistryCollection constructs a named registry collection. Most
// applications should use NewRegistry, which reads these definitions from config.
func NewRegistryCollection(definitions []config.RegistryConfig, defaultName string) RegistryClient {
	return newRegistryCollection(definitions, defaultName)
}

func (c *RegistryCollection) RegisterConfigured() {
	c.mu.RLock()
	entries := make([]registryEntry, 0, len(c.entries))
	for _, entry := range c.entries {
		entries = append(entries, entry)
	}
	c.mu.RUnlock()
	for _, entry := range entries {
		data := config.Config.GetConfigData(entry.prefix)
		if configured, err := applyRegistryOverrides(data, entry.kind, entry.overrides); err != nil {
			logger.Error(fmt.Sprintf("注册中心 %s 配置覆盖失败: %s", entry.overrides.Name, err.Error()))
			continue
		} else {
			data = configured
		}
		entry.client.Register(data)
	}
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

// Register preserves the legacy contract and applies the data to the default registry.
func (c *RegistryCollection) Register(data []byte) {
	if entry, ok := c.entry(""); ok {
		entry.client.Register(data)
	}
}
func (c *RegistryCollection) GetServiceURL(service string, group ...string) (string, string) {
	return c.GetServiceURLFrom("", service, group...)
}
func (c *RegistryCollection) GetServiceURLFrom(name, service string, group ...string) (string, string) {
	entry, ok := c.entry(name)
	if !ok {
		return "", ""
	}
	return entry.client.GetServiceURL(service, group...)
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

// noopRegistryClient 在未配置注册中心时使用的空实现，保证接口调用不会 panic
type noopRegistryClient struct{}

func (n *noopRegistryClient) Register(registryConfigData []byte) {}
func (n *noopRegistryClient) GetServiceURL(servicename string, groupName ...string) (string, string) {
	return "", ""
}
func (n *noopRegistryClient) DeRegister() {}
