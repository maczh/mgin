package mgin

import (
	"errors"
	"fmt"
	"reflect"
	"runtime/debug"
	"strings"
	"sync"

	"github.com/maczh/mgin/config"
	"github.com/maczh/mgin/db"
	"github.com/maczh/mgin/i18n"
	"github.com/maczh/mgin/job"
	"github.com/maczh/mgin/logs"
	"github.com/maczh/mgin/middleware/postlog"
	"github.com/maczh/mgin/registry"
	"github.com/sadlil/gologger"
)

type mgin struct {
	plugins  map[string]plugin
	pluginMu sync.RWMutex
	reloadMu sync.Mutex
}

type MginPlugin interface {
	Init(configData []byte)
	Close()
	Check() error
}

type plugin struct {
	InitFunc  dbInitFunc
	CloseFunc dbCloseFunc
	CheckFunc dbCheckFunc
}

// var MGin = &mgin{}
var logger = gologger.GetLogger()

type dbInitFunc func(configData []byte)
type dbCloseFunc func()
type dbCheckFunc func() error

func (m *mgin) UsePlugin(dbConfigName string, mginPlugin MginPlugin) {
	if mginPlugin == nil || isNilLegacyPlugin(mginPlugin) {
		logs.Error("忽略空插件{}", dbConfigName)
		return
	}
	m.Use(dbConfigName, mginPlugin.Init, mginPlugin.Close, mginPlugin.Check)
}

func (m *mgin) Use(dbConfigName string, dbInit dbInitFunc, dbClose dbCloseFunc, dbCheck dbCheckFunc) {
	if m == nil || config.Config == nil || dbInit == nil {
		logs.Error("无法加载{}：MGin、配置或插件初始化函数未就绪", dbConfigName)
		return
	}
	if !strings.Contains(config.Config.Config.Used, dbConfigName) {
		logs.Error("加载{}失败，配置文件中未使用", dbConfigName)
		return
	}
	cnfData := config.Config.GetConfigData(config.Config.GetConfigString("go.config.prefix." + dbConfigName))
	if cnfData == nil {
		logs.Error("{}配置错误，无法获取配置地址", dbConfigName)
		return
	}
	m.pluginMu.Lock()
	if m.plugins == nil {
		m.plugins = make(map[string]plugin)
	}
	m.plugins[dbConfigName] = plugin{
		InitFunc:  dbInit,
		CloseFunc: dbClose,
		CheckFunc: dbCheck,
	}
	m.pluginMu.Unlock()
	logs.Info("正在连接{}", dbConfigName)
	if err := callLegacyPlugin(dbConfigName, "init", func() { dbInit(cnfData) }); err != nil {
		logs.Error("{}初始化失败: {}", dbConfigName, err)
		return
	}
	logs.Info("{}连接成功", dbConfigName)
}

func isNilLegacyPlugin(p MginPlugin) bool {
	v := reflect.ValueOf(p)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice:
		return v.IsNil()
	default:
		return false
	}
}

func (m *mgin) pluginSnapshot() map[string]plugin {
	if m == nil {
		return nil
	}
	m.pluginMu.RLock()
	defer m.pluginMu.RUnlock()
	copy := make(map[string]plugin, len(m.plugins))
	for name, item := range m.plugins {
		copy[name] = item
	}
	return copy
}

func callLegacyPlugin(name, operation string, call func()) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("legacy plugin %s %s panic: %v\n%s", name, operation, recovered, debug.Stack())
		}
	}()
	call()
	return nil
}

func callLegacyPluginCheck(name string, call func() error) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("legacy plugin %s check panic: %v\n%s", name, recovered, debug.Stack())
		}
	}()
	return call()
}

// Reload reloads the application configuration and recreates all resources
// owned by the framework. Calls are serialized because database clients,
// registries and plugin connections are process-wide singletons.
func (m *mgin) Reload(configFile string) error {
	m.reloadMu.Lock()
	defer m.reloadMu.Unlock()

	if config.Config == nil {
		return errors.New("配置未初始化")
	}
	oldUsed := config.Config.Config.Used
	oldRegistry := registry.GetRegistry()
	config.Config.Init(configFile)
	if config.Config.Cnf == nil {
		return errors.New("配置文件加载失败，保留现有连接")
	}

	// Stop the old resources only after the new configuration has loaded.
	m.closeResources(oldUsed, oldRegistry)
	initResources(m)
	return nil
}

func initResources(m *mgin) {
	configs := config.Config.Config.Used
	registry.SetRegistry(registry.NewRegistry())
	currentRegistry := registry.GetRegistry()
	// Service registration is deferred until App.RunContext has acquired every
	// configured listener successfully.

	if strings.Contains(configs, "mysql") {
		db.Mysql.Init(config.Config.GetConfigData(config.Config.Config.Prefix.Mysql))
	}
	if strings.Contains(configs, "sqlite") {
		db.Sqlite.Init(config.Config.Config.Prefix.Sqlite)
	}
	if strings.Contains(configs, "mongodb") {
		db.Mongo.Init(config.Config.GetConfigData(config.Config.Config.Prefix.Mongodb))
	}
	if strings.Contains(configs, "redis") {
		db.Redis.Init(config.Config.GetConfigData(config.Config.Config.Prefix.Redis))
	}
	if _, multi := currentRegistry.(registry.ConfiguredRegistrar); !multi && strings.Contains(configs, "nacos") {
		currentRegistry.Register(config.Config.GetConfigData(config.Config.Config.Prefix.Nacos))
	}
	if _, multi := currentRegistry.(registry.ConfiguredRegistrar); !multi && strings.Contains(configs, "etcd") {
		currentRegistry.Register(config.Config.GetConfigData(config.Config.Config.Prefix.Etcd))
	}
	if _, multi := currentRegistry.(registry.ConfiguredRegistrar); !multi && strings.Contains(configs, "consul") {
		currentRegistry.Register(config.Config.GetConfigData(config.Config.Config.Prefix.Consul))
	}
	if strings.Contains(configs, "job") {
		if err := job.Start(); err != nil {
			logs.Error("定时任务调度器启动失败: {}", err.Error())
		}
	}
	if m != nil {
		for name, pl := range m.pluginSnapshot() {
			if !strings.Contains(configs, name) || pl.InitFunc == nil {
				continue
			}
			data := config.Config.GetConfigData(config.Config.GetConfigString("go.config.prefix." + name))
			if data == nil {
				logs.Error("{}配置错误，无法获取配置地址", name)
				continue
			}
			if err := callLegacyPlugin(name, "init", func() { pl.InitFunc(data) }); err != nil {
				logs.Error("{}初始化失败: {}", name, err)
			}
		}
	}
}

func (m *mgin) closeResources(configs string, currentRegistry registry.RegistryClient) {
	if strings.Contains(configs, "mysql") {
		db.Mysql.Close()
	}
	if strings.Contains(configs, "sqlite") {
		db.Sqlite.Close()
	}
	if strings.Contains(configs, "mongodb") {
		db.Mongo.Close()
	}
	if strings.Contains(configs, "redis") {
		db.Redis.Close()
	}
	_, multi := currentRegistry.(registry.ConfiguredRegistrar)
	if currentRegistry != nil && (multi || strings.Contains(configs, "nacos") || strings.Contains(configs, "etcd") || strings.Contains(configs, "consul") || strings.Contains(configs, "polaris")) {
		currentRegistry.DeRegister()
	}
	if strings.Contains(configs, "job") {
		job.Stop()
	}
	if m != nil {
		for name, pl := range m.pluginSnapshot() {
			if strings.Contains(configs, name) && pl.CloseFunc != nil {
				if err := callLegacyPlugin(name, "close", pl.CloseFunc); err != nil {
					logs.Error("{}关闭失败: {}", name, err)
				}
			}
		}
	}
}
func Init(configFile string) {
	config.Config.Init(configFile)
	if err := logs.InitVictoriaLogs(); err != nil {
		panic(err)
	}
	initResources(nil)
	return
}

func (m *mgin) checkAll() {

	configs := config.Config.Config.Used
	var err error
	if strings.Contains(configs, "mysql") {
		logger.Info("正在检查MySQL")
		err = db.Mysql.Check()
		if err != nil {
			logs.Error("MySQL check failed： {}", err.Error())
		}
	}
	if strings.Contains(configs, "mongodb") {
		logger.Info("正在检查MongoDB")
		err = db.Mongo.Check()
		if err != nil {
			logs.Error("MongoDB check failed： {}", err.Error())
		}
	}
	if strings.Contains(configs, "redis") {
		logger.Info("正在检查Redis")
		err = db.Redis.Check()
		if err != nil {
			logs.Error("Redis check failed： {}", err.Error())
		}
	}
	if m != nil {
		for dbConfigName, pl := range m.pluginSnapshot() {
			if pl.CheckFunc != nil {
				logs.Info("正在检查{}", dbConfigName)
				err := callLegacyPluginCheck(dbConfigName, pl.CheckFunc)
				if err != nil {
					logs.Error("{}连接检查失败:{}", dbConfigName, err.Error())
				}
			}
		}
	}
}

func (m *mgin) SafeExit() {
	if config.Config == nil {
		return
	}
	configs := config.Config.Config.Used

	// 先排空接口日志队列并关闭各日志 handler，再关闭数据库连接
	postlog.Close()
	i18n.Close()

	if strings.Contains(configs, "mysql") {
		logger.Info("正在关闭MySQL连接")
		db.Mysql.Close()
	}
	if strings.Contains(configs, "sqlite") {
		logger.Info("正在关闭SQLite连接")
		db.Sqlite.Close()
	}
	if strings.Contains(configs, "mongodb") {
		logger.Info("正在关闭MongoDB连接")
		db.Mongo.Close()
	}
	if strings.Contains(configs, "redis") {
		logger.Info("正在关闭Redis连接")
		db.Redis.Close()
	}
	currentRegistry := registry.GetRegistry()
	if currentRegistry != nil && (len(config.Config.Discovery.Registries) > 0 || strings.Contains(configs, "nacos") || strings.Contains(configs, "etcd") || strings.Contains(configs, "consul") || strings.Contains(configs, "polaris")) {
		logger.Info("正在注销注册中心")
		currentRegistry.DeRegister()
	}
	if strings.Contains(configs, "job") {
		logger.Info("正在停止定时任务调度器")
		job.Stop()
	}
	if m != nil {
		for dbConfigName, pl := range m.pluginSnapshot() {
			if pl.CloseFunc != nil {
				logs.Info("正在关闭{}", dbConfigName)
				if err := callLegacyPlugin(dbConfigName, "close", pl.CloseFunc); err != nil {
					logs.Error("{}关闭失败: {}", dbConfigName, err)
				}
			}
		}
	}
	//cache.CloseCache()
}
