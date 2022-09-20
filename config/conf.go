package config

import (
	"github.com/knadh/koanf"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/file"
	"github.com/sadlil/gologger"
)

type Config struct {
	Config *koanf.Koanf
}

var logger = gologger.GetLogger()

const config_file = "./application.yml"
const AUTO_CHECK_MINUTES = 30 //自动检查连接间隔时间，单位为分钟

func (c *Config) Init(cf string) {
	if cf == "" {
		cf = config_file
	}
	logger.Debug("读取配置文件:" + cf)
	c.Config = koanf.New(".")
	f := file.Provider(cf)
	err := c.Config.Load(f, yaml.Parser())
	if err != nil {
		logger.Error("读取配置文件错误:" + err.Error())
	}

}

func (c *Config) GetConfigString(name string) string {
	if c.Config == nil {
		return ""
	}
	if c.Config.Exists(name) {
		return c.Config.String(name)
	} else {
		return ""
	}
}

func (c *Config) GetConfigInt(name string) int {
	if c.Config == nil {
		return 0
	}
	if c.Config.Exists(name) {
		return c.Config.Int(name)
	} else {
		return 0
	}
}

func (c *Config) GetConfigUrl(prefix string) string {
	serverType := c.Config.String("go.config.server_type")
	configUrl := c.Config.String("go.config.server")
	switch serverType {
	case "nacos":
		configUrl = configUrl + "nacos/v1/cs/configs?group=DEFAULT_GROUP&dataId=" + prefix + c.Config.String("go.config.mid") + c.Config.String("go.config.env") + c.Config.String("go.config.type")
	case "consul":
		configUrl = configUrl + "v1/kv/" + prefix + c.Config.String("go.config.mid") + c.Config.String("go.config.env") + c.Config.String("go.config.type") + "?dc=dc1&raw=true"
	case "springconfig":
		configUrl = configUrl + prefix + c.Config.String("go.config.mid") + c.Config.String("go.config.env") + c.Config.String("go.config.type")
	default:
		configUrl = configUrl + prefix + c.Config.String("go.config.mid") + c.Config.String("go.config.env") + c.Config.String("go.config.type")
	}
	return configUrl
}
