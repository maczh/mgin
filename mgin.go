package mgin

import (
	"github.com/maczh/mgin/config"
	"github.com/maczh/mgin/db/mongo"
	"github.com/maczh/mgin/db/mysql"
	"github.com/maczh/mgin/db/redis"
	"github.com/maczh/mgin/registry/nacos"
	"github.com/sadlil/gologger"
	"strings"
	"time"
)

type mgin struct {
}

var MGin = &mgin{}
var logger = gologger.GetLogger()

func Init(configFile string) {
	config.Config.Init(configFile)
	configs := config.Config.GetConfigString("go.config.used")

	if strings.Contains(configs, "mysql") {
		logger.Info("正在连接MySQL")
		mysql.Mysql.Init(config.Config.GetConfigUrl(config.Config.GetConfigString("go.config.prefix.mysql")))
	}
	if strings.Contains(configs, "mongodb") {
		logger.Info("正在连接MongoDB")
		mongo.Mongo.Init(config.Config.GetConfigUrl(config.Config.GetConfigString("go.config.prefix.mongodb")))
	}
	if strings.Contains(configs, "redis") {
		logger.Info("正在连接Redis")
		redis.Redis.Init(config.Config.GetConfigUrl(config.Config.GetConfigString("go.config.prefix.redis")))
	}
	if strings.Contains(configs, "nacos") {
		logger.Info("正在注册到Nacos")
		nacos.Nacos.Register(config.Config.GetConfigUrl(config.Config.GetConfigString("go.config.prefix.nacos")))
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

	configs := config.Config.GetConfigString("go.config.used")

	if strings.Contains(configs, "mysql") {
		logger.Debug("正在检查MySQL")
		mysql.Mysql.Check()
	}
	if strings.Contains(configs, "mongodb") {
		logger.Debug("正在检查MongoDB")
		mongo.Mongo.Check()
	}
	if strings.Contains(configs, "redis") {
		logger.Debug("正在检查Redis")
		redis.Redis.Check()
	}

}

func (m *mgin) SafeExit() {
	configs := config.Config.GetConfigString("go.config.used")

	if strings.Contains(configs, "mysql") {
		logger.Debug("正在检查MySQL")
		mysql.Mysql.Close()
	}
	if strings.Contains(configs, "mongodb") {
		logger.Debug("正在检查MongoDB")
		mongo.Mongo.Close()
	}
	if strings.Contains(configs, "redis") {
		logger.Debug("正在检查Redis")
		redis.Redis.Close()
	}
	if strings.Contains(configs, "nacos") {
		logger.Info("正在注销Nacos")
		nacos.Nacos.DeRegister()
	}

}
