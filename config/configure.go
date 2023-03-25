package config

import (
	"github.com/knadh/koanf"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/file"
	"github.com/sadlil/gologger"
	"os"
	"path/filepath"
	"strings"
)

type config struct {
	Cnf       *koanf.Koanf
	App       app       `json:"app" bson:"app"`
	Config    appConfig `json:"config" bson:"config"`
	Log       appLog    `json:"log" bson:"log"`
	Logger    appLogger `json:"logger" bson:"logger"`
	Discovery discovery `json:"discovery" bson:"discovery"`
}

type app struct {
	Name    string `json:"name" bson:"name"`
	Project string `json:"project" bson:"project"`
	Port    int    `json:"port" bson:"port"`
	PortSSL int    `json:"portSSL" bson:"portSSL"`
	Cert    string `json:"cert" bson:"cert"`
	Key     string `json:"key" bson:"key"`
	Debug   bool   `json:"debug" bson:"debug"`
	IpAddr  string `json:"ipAddr" bson:"ipAddr"`
}

type appConfig struct {
	Server string `json:"server" bson:"server"`
	Type   string `json:"type" bson:"type"`
	Path   string `json:"path" bson:"path"`
	Env    string `json:"env" bson:"env"`
	Used   string `json:"used" bson:"used"`
	Prefix struct {
		Mysql         string `json:"mysql" bson:"mysql"`
		Mongodb       string `json:"mongodb" bson:"mongodb"`
		Redis         string `json:"redis" bson:"redis"`
		Nacos         string `json:"nacos" bson:"nacos"`
		Elasticsearch string `json:"elasticsearch" bson:"elasticsearch"`
		Kafka         string `json:"kafka" bson:"kafka"`
	} `json:"prefix" bson:"prefix"`
}

type appLogger struct {
	Level string `json:"level" bson:"level"`
	Out   string `json:"out" bson:"out"`
	File  string `json:"file" bson:"file"`
}

type appLog struct {
	RequestTableName string `json:"request" bson:"request"`
	CallTableName    string `json:"call" bson:"call"`
	LogDb            string `json:"logDb" bson:"logDb"`
	DbName           string `json:"dbName" bson:"dbName"`
	Kafka            struct {
		Use   bool   `json:"use" bson:"use"`
		Topic string `json:"topic" bson:"topic"`
	} `json:"kafka" bson:"kafka"`
}

type discovery struct {
	Registry string `json:"registry" bson:"registry"`
	CallType string `json:"callType" bson:"callType"`
}

var Config = &config{}

var logger = gologger.GetLogger()

const config_file = "./application.yml"

func (c *config) Init(cf string) {
	if cf == "" {
		cf = config_file
	}
	logger.Debug("读取配置文件:" + cf)
	c.Cnf = koanf.New(".")
	f := file.Provider(cf)
	err := c.Cnf.Load(f, yaml.Parser())
	if err != nil {
		logger.Error("读取配置文件错误:" + err.Error())
	}
	c.App.Name = c.Cnf.String("go.application.name")
	c.App.Project = c.Cnf.String("go.application.project")
	c.App.Port = c.Cnf.Int("go.application.port")
	c.App.PortSSL = c.Cnf.Int("go.application.port_ssl")
	c.App.Cert = c.Cnf.String("go.application.cert")
	c.App.Key = c.Cnf.String("go.application.key")
	c.App.Debug = c.Cnf.Bool("go.application.debug")
	c.App.IpAddr = c.Cnf.String("go.application.ip")
	c.Config.Server = c.Cnf.String("go.config.server")
	c.Config.Type = c.Cnf.String("go.config.server_type")
	c.Config.Path = c.Cnf.String("go.config.path")
	c.Config.Env = c.Cnf.String("go.config.env")
	c.Config.Used = c.Cnf.String("go.config.used")
	c.Config.Prefix.Mysql = c.Cnf.String("go.config.prefix.mysql")
	c.Config.Prefix.Mongodb = c.Cnf.String("go.config.prefix.mongodb")
	c.Config.Prefix.Redis = c.Cnf.String("go.config.prefix.redis")
	c.Config.Prefix.Elasticsearch = c.Cnf.String("go.config.prefix.elasticsearch")
	c.Config.Prefix.Nacos = c.Cnf.String("go.config.prefix.nacos")
	c.Config.Prefix.Kafka = c.Cnf.String("go.config.prefix.kafka")
	c.Log.LogDb = c.Cnf.String("go.log.db")
	c.Log.RequestTableName = c.Cnf.String("go.log.req")
	c.Log.CallTableName = c.Cnf.String("go.log.call")
	c.Log.DbName = c.Cnf.String("go.log.dbName")
	c.Log.Kafka.Use = c.Cnf.Bool("go.log.kafka.use")
	c.Log.Kafka.Topic = c.Cnf.String("go.log.kafka.topic")
	if c.Log.Kafka.Topic == "" {
		c.Log.Kafka.Topic = c.App.Name
	}
	c.Logger.Level = c.Cnf.String("go.logger.level")
	c.Logger.Out = c.Cnf.String("go.logger.out")
	c.Logger.File = c.Cnf.String("go.logger.file")
	c.Discovery.Registry = c.Cnf.String("go.discovery.registry")
	c.Discovery.CallType = c.Cnf.String("go.discovery.callType")
	if c.Discovery.CallType == "" {
		c.Discovery.CallType = "x-form"
	}
}

func (c *config) GetConfigString(name string) string {
	if c.Cnf == nil {
		return ""
	}
	if c.Cnf.Exists(name) {
		return c.Cnf.String(name)
	} else {
		return ""
	}
}

func (c *config) GetConfigInt(name string) int {
	if c.Cnf == nil {
		return 0
	}
	if c.Cnf.Exists(name) {
		return c.Cnf.Int(name)
	} else {
		return 0
	}
}

func (c *config) GetConfigBool(name string) bool {
	if c.Cnf == nil {
		return false
	}
	if c.Cnf.Exists(name) {
		return c.Cnf.Bool(name)
	} else {
		return false
	}
}

func (c *config) Exists(name string) bool {
	if c.Cnf == nil {
		return false
	}
	return c.Cnf.Exists(name)
}

func (c *config) GetConfigUrl(prefix string) string {
	configUrl := c.Config.Server
	switch c.Config.Type {
	case "nacos":
		configUrl = configUrl + "nacos/v1/cs/configs?group=DEFAULT_GROUP&dataId=" + prefix + "-" + c.Config.Env + ".yml"
	case "consul":
		configUrl = configUrl + "v1/kv/" + prefix + "-" + c.Config.Env + ".yml" + "?dc=dc1&raw=true"
	case "springconfig":
		configUrl = configUrl + prefix + "-" + c.Config.Env + ".yml"
	case "file":
		path, _ := filepath.Abs(filepath.Dir(os.Args[0]))
		if c.Config.Path != "" {
			path = strings.TrimSuffix(c.Config.Path, "/")
		}
		configUrl = path + "/" + prefix + "-" + c.Config.Env + ".yml"
	default:
		configUrl = configUrl + prefix + "-" + c.Config.Env + ".yml"
	}
	return configUrl
}
