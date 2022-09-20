package mgin

import (
	"github.com/maczh/mgin/client"
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
	Config *config.Config
	Mysql  *mysql.MySQL
	Mongo  *mongo.MongoDB
	Redis  *redis.Redis
	Nacos  *nacos.Nacos
	Client *client.Client
}

var MGin *mgin
var logger = gologger.GetLogger()

func Init(configFile string) {
	MGin = &mgin{
		Client: &client.Client{},
	}
	MGin.Config.Init(configFile)
	configs := MGin.Config.GetConfigString("go.config.used")

	if strings.Contains(configs, "mysql") {
		logger.Info("正在连接MySQL")
		MGin.Mysql = &mysql.MySQL{}
		MGin.Mysql.Init(MGin.Config.GetConfigUrl(MGin.Config.GetConfigString("go.config.prefix.mysql")))
	}
	if strings.Contains(configs, "mongodb") {
		logger.Info("正在连接MongoDB")
		MGin.Mongo = &mongo.MongoDB{}
		MGin.Mongo.Init(MGin.Config.GetConfigUrl(MGin.Config.GetConfigString("go.config.prefix.mongodb")))
	}
	if strings.Contains(configs, "redis") {
		logger.Info("正在连接Redis")
		MGin.Redis = &redis.Redis{}
		MGin.Redis.Init(MGin.Config.GetConfigUrl(MGin.Config.GetConfigString("go.config.prefix.redis")))
	}
	if strings.Contains(configs, "nacos") {
		logger.Info("正在注册到Nacos")
		MGin.Nacos = &nacos.Nacos{}
		MGin.Nacos.Register(MGin.Config.GetConfigUrl(MGin.Config.GetConfigString("go.config.prefix.nacos")))
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

	configs := m.Config.GetConfigString("go.config.used")

	if strings.Contains(configs, "mysql") {
		logger.Debug("正在检查MySQL")
		m.Mysql.Check()
	}
	if strings.Contains(configs, "mongodb") {
		logger.Debug("正在检查MongoDB")
		m.Mongo.Check()
	}
	if strings.Contains(configs, "redis") {
		logger.Debug("正在检查Redis")
		m.Redis.Check()
	}

}

func (m *mgin) SafeExit() {
	configs := m.Config.GetConfigString("go.config.used")

	if strings.Contains(configs, "mysql") {
		logger.Debug("正在检查MySQL")
		m.Mysql.Close()
	}
	if strings.Contains(configs, "mongodb") {
		logger.Debug("正在检查MongoDB")
		m.Mongo.Close()
	}
	if strings.Contains(configs, "redis") {
		logger.Debug("正在检查Redis")
		m.Redis.Close()
	}
	if strings.Contains(configs, "nacos") {
		logger.Info("正在注销Nacos")
		m.Nacos.DeRegister()
	}

}
