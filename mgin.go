package mgin

import (
	"errors"
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
	plugins map[string]plugin
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
	m.Use(dbConfigName, mginPlugin.Init, mginPlugin.Close, mginPlugin.Check)
}

func (m *mgin) Use(dbConfigName string, dbInit dbInitFunc, dbClose dbCloseFunc, dbCheck dbCheckFunc) {
	if !strings.Contains(config.Config.Config.Used, dbConfigName) {
		logs.Error("加载{}失败，配置文件中未使用", dbConfigName)
		return
	}
	cnfData := config.Config.GetConfigData(config.Config.GetConfigString("go.config.prefix." + dbConfigName))
	if cnfData == nil {
		logs.Error("{}配置错误，无法获取配置地址", dbConfigName)
		return
	}
	if m.plugins == nil {
		m.plugins = make(map[string]plugin)
	}
	m.plugins[dbConfigName] = plugin{
		InitFunc:  dbInit,
		CloseFunc: dbClose,
		CheckFunc: dbCheck,
	}
	logs.Info("正在连接{}", dbConfigName)
	dbInit(cnfData)
	logs.Info("{}连接成功", dbConfigName)
}

// Reload reloads the application configuration and recreates all resources
// owned by the framework. Calls are serialized because database clients,
// registries and plugin connections are process-wide singletons.
func (m *mgin) Reload(configFile string) error {
	m.reloadMu.Lock()
	defer m.reloadMu.Unlock()

	oldUsed := config.Config.Config.Used
	oldRegistry := registry.Registry
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
	registry.Registry = registry.NewRegistry()

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
	if strings.Contains(configs, "nacos") {
		registry.Registry.Register(config.Config.GetConfigData(config.Config.Config.Prefix.Nacos))
	}
	if strings.Contains(configs, "etcd") {
		registry.Registry.Register(config.Config.GetConfigData(config.Config.Config.Prefix.Etcd))
	}
	if strings.Contains(configs, "consul") {
		registry.Registry.Register(config.Config.GetConfigData(config.Config.Config.Prefix.Consul))
	}
	if strings.Contains(configs, "job") {
		if err := job.Start(); err != nil {
			logs.Error("定时任务调度器启动失败: {}", err.Error())
		}
	}
	if m != nil {
		for name, pl := range m.plugins {
			if !strings.Contains(configs, name) || pl.InitFunc == nil {
				continue
			}
			data := config.Config.GetConfigData(config.Config.GetConfigString("go.config.prefix." + name))
			if data == nil {
				logs.Error("{}配置错误，无法获取配置地址", name)
				continue
			}
			pl.InitFunc(data)
		}
	}
}

func (m *mgin) closeResources(configs string, currentRegistry registry.RegistryClient) {
	if strings.Contains(configs, "mysql") { db.Mysql.Close() }
	if strings.Contains(configs, "sqlite") { db.Sqlite.Close() }
	if strings.Contains(configs, "mongodb") { db.Mongo.Close() }
	if strings.Contains(configs, "redis") { db.Redis.Close() }
	if currentRegistry != nil && (strings.Contains(configs, "nacos") || strings.Contains(configs, "etcd") || strings.Contains(configs, "consul") || strings.Contains(configs, "polaris")) {
		currentRegistry.DeRegister()
	}
	if strings.Contains(configs, "job") { job.Stop() }
	if m != nil {
		for name, pl := range m.plugins {
			if strings.Contains(configs, name) && pl.CloseFunc != nil { pl.CloseFunc() }
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
	if m.plugins != nil {
		for dbConfigName, pl := range m.plugins {
			if pl.CheckFunc != nil {
				logs.Info("正在检查{}", dbConfigName)
				err := pl.CheckFunc()
				if err != nil {
					logs.Error("{}连接检查失败:{}", dbConfigName, err.Error())
				}
			}
		}
	}
}

func (m *mgin) SafeExit() {
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
	if strings.Contains(configs, "nacos") {
		logger.Info("正在注销Nacos")
		registry.Registry.DeRegister()
	}
	if strings.Contains(configs, "etcd") {
		logger.Info("正在注销Etcd")
		registry.Registry.DeRegister()
	}
	if strings.Contains(configs, "consul") {
		logger.Info("正在注销Consul")
		registry.Registry.DeRegister()
	}
	if strings.Contains(configs, "job") {
		logger.Info("正在停止定时任务调度器")
		job.Stop()
	}
	if strings.Contains(configs, "job") {
		if err := job.GetManager().Check(); err != nil {
			logs.Error("定时任务调度器健康检查失败: {}", err.Error())
		}
	}
	if m.plugins != nil {
		for dbConfigName, pl := range m.plugins {
			if pl.CloseFunc != nil {
				logs.Info("正在关闭{}", dbConfigName)
				pl.CloseFunc()
			}
		}
	}
	//cache.CloseCache()
}
