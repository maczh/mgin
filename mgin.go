package mgin

import (
	"github.com/maczh/mgin/cache"
	"github.com/maczh/mgin/config"
	"github.com/maczh/mgin/db"
	"github.com/maczh/mgin/logs"
	"github.com/maczh/mgin/registry"
	"github.com/sadlil/gologger"
	"strings"
	"time"
)

type mgin struct {
	plugins map[string]plugin
}

type MginPlugin interface {
	Init(configUrl string)
	Close()
	Check() error
}

type plugin struct {
	InitFunc  dbInitFunc
	CloseFunc dbCloseFunc
	CheckFunc dbCheckFunc
}

var MGin = &mgin{}
var logger = gologger.GetLogger()

type dbInitFunc func(configUrl string)
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
	cnfUrl := config.Config.GetConfigUrl(config.Config.GetConfigString("go.config.prefix." + dbConfigName))
	if cnfUrl == "" {
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
	dbInit(cnfUrl)
	logs.Info("{}连接成功", dbConfigName)
}
func Init(configFile string) {
	config.Config.Init(configFile)
	configs := config.Config.Config.Used

	if strings.Contains(configs, "mysql") {
		logger.Info("正在连接MySQL")
		db.Mysql.Init(config.Config.GetConfigUrl(config.Config.Config.Prefix.Mysql))
		logger.Info("连接MySQL成功")
	}
	if strings.Contains(configs, "sqlite") {
		logger.Info("正在连接SQLite")
		db.Sqlite.Init(config.Config.Config.Prefix.Sqlite)
		logger.Info("连接SQLite成功")
	}
	if strings.Contains(configs, "mongodb") {
		logger.Info("正在连接MongoDB")
		db.Mongo.Init(config.Config.GetConfigUrl(config.Config.Config.Prefix.Mongodb))
		logger.Info("连接MongoDB成功")
	}
	if strings.Contains(configs, "redis") {
		logger.Info("正在连接Redis")
		db.Redis.Init(config.Config.GetConfigUrl(config.Config.Config.Prefix.Redis))
		logger.Info("连接Redis成功")
	}
	if strings.Contains(configs, "elasticsearch") {
		logger.Info("正在连接ElasticSearch")
		db.ElasticSearch.Init(config.Config.GetConfigUrl(config.Config.Config.Prefix.Elasticsearch))
		logger.Info("连接ElasticSearch成功")
	}
	if strings.Contains(configs, "kafka") {
		logger.Info("正在连接到Kafka")
		db.Kafka.Init(config.Config.GetConfigUrl(config.Config.Config.Prefix.Kafka))
		logger.Info("连接到Kafka成功")
	}
	if strings.Contains(configs, "nacos") {
		logger.Info("正在注册到Nacos")
		registry.Nacos.Register(config.Config.GetConfigUrl(config.Config.Config.Prefix.Nacos))
		logger.Info("注册到Nacos成功")
	}

	//设置定时任务自动检查
	ticker := time.NewTicker(time.Minute * 5)
	go func() {
		for _ = range ticker.C {
			MGin.checkAll()
		}
	}()
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
	if strings.Contains(configs, "elasticsearch") {
		logger.Info("正在检查ElasticSearch")
		err = db.ElasticSearch.Check()
		if err != nil {
			logs.Error("ElasticSearch check failed： {}", err.Error())
		}
	}
	if strings.Contains(configs, "kafka") {
		logger.Info("正在检查kafka")
		err = db.Kafka.Check()
		if err != nil {
			logs.Error("Kafka check failed： {}", err.Error())
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
	if strings.Contains(configs, "elasticsearch") {
		logger.Info("正在关闭ElasticSearch连接")
		db.ElasticSearch.Close()
	}
	if strings.Contains(configs, "kafka") {
		logger.Info("正在关闭Kafka连接")
		db.Kafka.Close()
	}
	if strings.Contains(configs, "nacos") {
		logger.Info("正在注销Nacos")
		registry.Nacos.DeRegister()
	}
	if m.plugins != nil {
		for dbConfigName, pl := range m.plugins {
			if pl.CloseFunc != nil {
				logs.Info("正在关闭{}", dbConfigName)
				pl.CloseFunc()
			}
		}
	}
	cache.CloseCache()
}
