// Package mgin 是框架入口，导出 NewApp / Run / Init / Use / UsePlugin 等公共符号。
//
// v2 起，组件的生命周期（Init / Close / Health）由 pkg/plugin.PluginRegistry 统一驱动，
// 内置 11 类组件（mysql / postgres / sqlite / mongodb / redis / clickhouse /
// elasticsearch / kafka / s3 / registry / job）已在 plugin.RegisterBuiltins() 中注册，
// 业务侧仍可通过 mgin.Use / mgin.UsePlugin 注册自定义组件（这些是旧 API 兼容入口），
// 也可直接调用 plugin.Register 注册任意实现 plugin.Plugin 的组件。
package mgin

import (
	"context"
	"os"
	"reflect"
	goruntime "runtime"
	"strings"
	"time"

	"github.com/maczh/mgin/v2/pkg/config"
	"github.com/maczh/mgin/v2/pkg/logs"
	"github.com/maczh/mgin/v2/pkg/plugin"
	"github.com/maczh/mgin/v2/pkg/registry"
	"github.com/sadlil/gologger"
)

type mgin struct {
	// plugins 保留旧版以 mgin.Use / mgin.UsePlugin 注册的组件，v2 仍然兼容，
	// 但其 Close/Check 仍由 SafeExit / checkAll 显式驱动，不走 plugin.PluginRegistry。
	// 注意：当业务使用旧 API 时，可同时通过 plugin.Register 注入新框架，二者不冲突。
	plugins map[string]legacyPlugin
}

// MginPlugin 是 v1 时代暴露的"组件适配接口"，保留以兼容存量项目。
// 业务也可改为直接实现 plugin.Plugin 接口（新接口），用 plugin.Register 注册。
type MginPlugin interface {
	Init(configData []byte)
	Close()
	Check() error
}

type legacyPlugin struct {
	InitFunc  dbInitFunc
	CloseFunc dbCloseFunc
	CheckFunc dbCheckFunc
}

// var MGin = &mgin{}
var logger = gologger.GetLogger()

type dbInitFunc func(configData []byte)
type dbCloseFunc func()
type dbCheckFunc func() error

// Use 是 v1 时代的"按配置启用 + 立即初始化"兼容入口。
// v2 行为：若 go.config.used 启用了 dbConfigName，则按 cnfData 立即执行 Init，
// 并把组件纳入 m.plugins 生命周期。仍由 SafeExit / checkAll 显式驱动。
func (m *mgin) Use(dbConfigName string, dbInit dbInitFunc, dbClose dbCloseFunc, dbCheck dbCheckFunc) {
	if m == nil || dbInit == nil {
		return
	}
	if !configuredComponent(config.Config.Config.Used, dbConfigName) {
		logs.Error("加载{}失败，配置文件中未使用", dbConfigName)
		return
	}
	cnfData := config.Config.GetConfigData(config.Config.GetConfigString("go.config.prefix." + dbConfigName))
	if cnfData == nil {
		logs.Error("{}配置错误，无法获取配置地址", dbConfigName)
		return
	}
	if m.plugins == nil {
		m.plugins = make(map[string]legacyPlugin)
	}
	m.plugins[dbConfigName] = legacyPlugin{
		InitFunc:  dbInit,
		CloseFunc: dbClose,
		CheckFunc: dbCheck,
	}
	logs.Info("正在连接{}", dbConfigName)
	dbInit(cnfData)
	logs.Info("{}连接成功", dbConfigName)
}

// UsePlugin 是 v1 时代基于 MginPlugin 接口的便捷入口，等价于 Use 的对象化版本。
func (m *mgin) UsePlugin(dbConfigName string, mginPlugin MginPlugin) {
	if m == nil || isNilPlugin(mginPlugin) {
		return
	}
	m.Use(dbConfigName, mginPlugin.Init, mginPlugin.Close, mginPlugin.Check)
}

func configuredComponent(used, name string) bool {
	name = strings.TrimSpace(name)
	if name == "" {
		return false
	}
	for _, component := range strings.Split(used, ",") {
		if strings.TrimSpace(component) == name {
			return true
		}
	}
	return false
}

func isNilPlugin(p MginPlugin) bool {
	if p == nil {
		return true
	}
	v := reflect.ValueOf(p)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return v.IsNil()
	default:
		return false
	}
}

// Init 是框架统一初始化入口。v2 行为：
//  1. 加载配置（application.yml / nacos / consul / etcd / polaris / springconfig）
//  2. 创建全局 registry.Registry 实例（保留 v1 行为，让 client.Call 可用）
//  3. 注册全部内置组件到 plugin.PluginRegistry
//  4. 按 Order 升序调用 plugin.InitAll，依次初始化内置组件
//  5. 兼容旧 initPlugins：把 s3 等组件用 m.UsePlugin 纳入旧生命周期
func Init(configFile string) {
	config.Config.Init(configFile)
	registry.Registry = registry.NewRegistry()

	// v2：填充 Runtime 运行时元数据（由本进程在启动时计算）。
	// CommitHash 与 BuildTime 由构建脚本通过 -ldflags 注入（参考 cmd/scaffold.go 中 tmplVersion 的方式）。
	config.Config.Runtime.StartedAt = time.Now()
	config.Config.Runtime.Pid = getpid()
	config.Config.Runtime.GoVersion = goruntime.Version()
	// CommitHash/BuildTime 若由 ldflags 注入则保持非空；未注入时为 ""，运行期不补。

	// v2：内置组件注册到 PluginRegistry，由 plugin.InitAll 统一驱动。
	// RegisterBuiltins 内部把 11 类组件（db.* / s3 / registry.* / job）都注册进去，
	// InitAll 会按 Order（DB=10 / Redis=15 / Kafka=20 / Registry=30 / S3=40 / Job=90）
	// 升序初始化。
	plugin.RegisterBuiltins()
	if err := plugin.InitAll(context.Background()); err != nil {
		// 旧 mgin.Init 对组件初始化失败采用"打印错误但不中断"的策略，
		// v2 保留这一行为：记录首个错误，进程继续。
		logs.Error("内置插件初始化部分失败：{}", err.Error())
	}

	// v1 兼容：initPlugins 把 s3 等组件挂到 m.plugins 走旧生命周期。
	// 旧路径由 app.go 在 NewApp 阶段触发，新路径已由 plugin.S3 接管，
	// 此处仅保留调用，s3 适配器会同时挂到 plugin.PluginRegistry，因此不会重复初始化。
	initPlugins(&mgin{})
	return
}

// getpid 包装 os.Getpid，便于 Runtime 字段填充。
func getpid() int {
	return os.Getpid()
}

// initPlugins 兼容 v1 路径，把需要 v1 风格 Close 的组件也挂上 m.plugins。
// v2 下所有内置组件已通过 plugin.PluginRegistry 关闭，此处主要为业务自注册组件保留入口。
func initPlugins(m *mgin) {
	// 业务侧使用 MginPlugin 接口注册时，框架会通过 m.UsePlugin 自行入 m.plugins，
	// 这里无需主动挂载任何内置组件。
	_ = m
}

// checkAll 是 mgin 暴露给 app.go 的 5 分钟轮询探针。
// v2 行为：调用 plugin.HealthAll() 汇总所有已启用组件的 Health()，
// 同时保留对 v1 旧 m.plugins 中组件的 Check 遍历。
func (m *mgin) checkAll() {
	if m == nil {
		return
	}
	// v2：插件注册表驱动
	healths := plugin.HealthAll()
	for name, err := range healths {
		if err != nil {
			logs.Error("{} 连接检查失败:{}", name, err.Error())
		} else {
			logs.Info("正在检查{}", name)
		}
	}
	// v1 兼容：遍历 m.plugins 中的旧接口组件
	if m.plugins != nil {
		for dbConfigName, pl := range m.plugins {
			if pl.CheckFunc != nil {
				logs.Info("正在检查{}", dbConfigName)
				if err := pl.CheckFunc(); err != nil {
					logs.Error("{}连接检查失败:{}", dbConfigName, err.Error())
				}
			}
		}
	}
}

// SafeExit 在收到退出信号后被调用，按 plugin.PluginRegistry 的 Order 逆序关闭内置组件，
// 同时兼容旧 m.plugins 中的组件。
func (m *mgin) SafeExit() {
	if m == nil {
		return
	}
	// 给关闭动作一个 5 秒上限（与配置无关，SafeExit 不阻塞退出）
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// v2：按 Order 逆序关闭所有已启用的内置组件。
	if err := plugin.CloseAll(ctx); err != nil {
		logs.Error("内置插件关闭部分失败：{}", err.Error())
	}

	// v1 兼容：旧 m.plugins 中的组件。
	if m.plugins != nil {
		for dbConfigName, pl := range m.plugins {
			if pl.CloseFunc != nil {
				logs.Info("正在关闭{}", dbConfigName)
				pl.CloseFunc()
			}
		}
	}
}
