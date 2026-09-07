package plugin

import (
	"context"
	"strings"

	"github.com/maczh/mgin/v2/pkg/config"
	"github.com/maczh/mgin/v2/pkg/db"
	"github.com/maczh/mgin/v2/pkg/job"
	"github.com/maczh/mgin/v2/pkg/registry"
)

// funcPlugin 是以函数字段实现 Plugin 接口的通用适配器，避免逐组件手写样板。
// 各内置组件通过下面的构造函数包装为 Plugin。
type funcPlugin struct {
	name      string
	order     int
	initFn    func(ctx context.Context) error
	closeFn   func(ctx context.Context) error
	healthFn  func() error
	enabledFn func() bool
}

func (p *funcPlugin) Name() string { return p.name }
func (p *funcPlugin) Order() int   { return p.order }
func (p *funcPlugin) Init(ctx context.Context) error {
	if p.initFn != nil {
		return p.initFn(ctx)
	}
	return nil
}
func (p *funcPlugin) Close(ctx context.Context) error {
	if p.closeFn != nil {
		return p.closeFn(ctx)
	}
	return nil
}
func (p *funcPlugin) Health() error {
	if p.healthFn != nil {
		return p.healthFn()
	}
	return nil
}
func (p *funcPlugin) Enabled() bool {
	if p.enabledFn != nil {
		return p.enabledFn()
	}
	return false
}

// enabled 判断某组件是否在 go.config.used 中被启用。
func enabled(name string) bool {
	return strings.Contains(config.Config.Config.Used, name)
}

// getCfg 取某前缀对应的配置字节（nil 表示未配置）。
func getCfg(prefix string) []byte {
	return config.Config.GetConfigData(prefix)
}

// ---- 数据源 / 缓存 / 消息队列 适配 ----

func Mysql() Plugin {
	return &funcPlugin{
		name: "mysql", order: 10,
		enabledFn: func() bool { return enabled("mysql") },
		initFn: func(ctx context.Context) error {
			db.Mysql.Init(getCfg(config.Config.Config.Prefix.Mysql))
			return nil
		},
		closeFn:  func(ctx context.Context) error { db.Mysql.Close(); return nil },
		healthFn: func() error { return db.Mysql.Check() },
	}
}

func Sqlite() Plugin {
	return &funcPlugin{
		name: "sqlite", order: 10,
		enabledFn: func() bool { return enabled("sqlite") },
		// sqlite 的 Init 接收前缀字符串（而非配置字节），与旧 mgin.Init 行为一致
		initFn: func(ctx context.Context) error {
			db.Sqlite.Init(config.Config.Config.Prefix.Sqlite)
			return nil
		},
		closeFn:  func(ctx context.Context) error { db.Sqlite.Close(); return nil },
		healthFn: func() error { return db.Sqlite.Check() },
	}
}

func Mongodb() Plugin {
	return &funcPlugin{
		name: "mongodb", order: 10,
		enabledFn: func() bool { return enabled("mongodb") },
		initFn: func(ctx context.Context) error {
			db.Mongo.Init(getCfg(config.Config.Config.Prefix.Mongodb))
			return nil
		},
		closeFn:  func(ctx context.Context) error { db.Mongo.Close(); return nil },
		healthFn: func() error { return db.Mongo.Check() },
	}
}

func Redis() Plugin {
	return &funcPlugin{
		name: "redis", order: 15, // 缓存先于业务 DB 之后、MQ 之前
		enabledFn: func() bool { return enabled("redis") },
		initFn: func(ctx context.Context) error {
			db.Redis.Init(getCfg(config.Config.Config.Prefix.Redis))
			return nil
		},
		closeFn:  func(ctx context.Context) error { db.Redis.Close(); return nil },
		healthFn: func() error { return db.Redis.Check() },
	}
}

// ---- 注册中心 适配 ----
// 仅处理旧 mgin.Init 中实际注册的三类（nacos/etcd/consul），与既有行为保持一致。
func Registry() Plugin {
	return &funcPlugin{
		name: "registry", order: 30,
		enabledFn: func() bool {
			used := config.Config.Config.Used
			return strings.Contains(used, "nacos") ||
				strings.Contains(used, "etcd") ||
				strings.Contains(used, "consul")
		},
		initFn: func(ctx context.Context) error {
			used := config.Config.Config.Used
			if strings.Contains(used, "nacos") {
				registry.Registry.Register(getCfg(config.Config.Config.Prefix.Nacos))
			}
			if strings.Contains(used, "etcd") {
				registry.Registry.Register(getCfg(config.Config.Config.Prefix.Etcd))
			}
			if strings.Contains(used, "consul") {
				registry.Registry.Register(getCfg(config.Config.Config.Prefix.Consul))
			}
			return nil
		},
		closeFn:  func(ctx context.Context) error { registry.Registry.DeRegister(); return nil },
		healthFn: func() error { return nil },
	}
}

// ---- 定时任务 适配 ----

func Job() Plugin {
	return &funcPlugin{
		name: "job", order: 90, // 最后启动、最先关闭
		enabledFn: func() bool { return enabled("job") },
		initFn: func(ctx context.Context) error {
			return job.Start()
		},
		closeFn: func(ctx context.Context) error {
			job.Stop()
			return nil
		},
		healthFn: func() error {
			if m := job.GetManager(); m != nil {
				return m.Check()
			}
			return nil
		},
	}
}

// RegisterBuiltins 注册全部内置插件。在 mgin.Init 中、配置加载之后调用，
// 之后由 plugin.InitAll 统一驱动初始化。
func RegisterBuiltins() {
	Register(Mysql())
	Register(Sqlite())
	Register(Mongodb())
	Register(Redis())
	Register(Registry())
	Register(Job())
}
