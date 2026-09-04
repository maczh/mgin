package config

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/knadh/koanf"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/file"
	"github.com/levigross/grequests"
	"github.com/sadlil/gologger"
	clientv3 "go.etcd.io/etcd/client/v3"
)

type config struct {
	Cnf     *koanf.Koanf
	WorkDir string `json:"-" bson:"-"`
	App     app    `json:"app" bson:"app"`
	// Framework v2 新增：框架自身元配置（go.framework.* 节点）。
	// 用于把"框架开关"与"应用配置"分离，便于多套环境共用同一份 application.yml 时只切换 Framework。
	Framework framework `json:"framework" bson:"framework"`
	// Runtime v2 新增：运行时元数据（启动时间、commit hash、构建时间）。
	// 不从 application.yml 读，由构建脚本通过 -ldflags 注入（参考 cmd/templates.go 中 tmplVersion 的方式）。
	Runtime   runtime   `json:"runtime" bson:"runtime"`
	Config    appConfig `json:"config" bson:"config"`
	Log       appLog    `json:"log" bson:"log"`
	Logger    appLogger `json:"logger" bson:"logger"`
	Discovery discovery `json:"discovery" bson:"discovery"`
	Jwt       jwtConfig `json:"jwt" bson:"jwt"`
	Sys       sys       `json:"sys" bson:"sys"`
	Casbin    casbin    `json:"casbin" bson:"casbin"`
}

// framework 是 mgin v2 新增的"框架元配置"层。
// 与 application 的区别：application 描述业务侧（端口/DB 连接串），framework 描述框架行为开关。
// 老项目未配置 go.framework.* 节点时，全部字段为零值，由调用方按需回退到合理默认。
type framework struct {
	// HealthEnabled 框架级默认是否启用健康探针；应用代码可再通过 App.EnableHealth() 覆盖。
	HealthEnabled bool `json:"healthEnabled" bson:"healthEnabled"`
	// I18nEnabled 框架级默认是否启用国际化中间件（xlang）。
	I18nEnabled bool `json:"i18nEnabled" bson:"i18nEnabled"`
	// RatelimitEnabled 框架级默认是否启用单例限流中间件。
	RatelimitEnabled bool `json:"ratelimitEnabled" bson:"ratelimitEnabled"`
	// RecoverEnabled 框架级默认是否启用 panic 恢复中间件（nice.Recovery）。
	RecoverEnabled bool `json:"recoverEnabled" bson:"recoverEnabled"`
	// LogRequests 默认是否输出接口日志（postlog）。
	LogRequests bool `json:"logRequests" bson:"logRequests"`
}

// runtime 是 mgin v2 新增的"运行时元数据"层。
// 这些字段**不**从 application.yml 读取，由构建时通过 -ldflags 注入（见 cmd/scaffold.go 的 tmplVersion）。
// 业务可通过 Config.Runtime 在 /health/ready 响应、metrics label、日志上下文等位置引用。
type runtime struct {
	// CommitHash 构建时注入的 git commit hash（短）。
	CommitHash string `json:"commitHash" bson:"commitHash"`
	// BuildTime 构建时注入的 ISO8601 时间。
	BuildTime string `json:"buildTime" bson:"buildTime"`
	// GoVersion 构建时 Go 工具链版本（runtime.Version()）。
	GoVersion string `json:"goVersion" bson:"goVersion"`
	// StartedAt 进程启动时间（由框架在 NewApp 时填入）。
	StartedAt time.Time `json:"startedAt" bson:"startedAt"`
	// Pid 进程 ID（由框架在 NewApp 时填入）。
	Pid int `json:"pid" bson:"pid"`
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
	// ShutdownTimeout 优雅关闭的超时时间（秒），对应配置项 go.application.shutdownTimeout。
	// 未配置或配置值 <= 0 时回退为 DefaultShutdownTimeout（5 秒），
	// 保证未升级配置的老项目行为与升级前完全一致。
	ShutdownTimeout int `json:"shutdownTimeout" bson:"shutdownTimeout"`
}

type appConfig struct {
	Server string `json:"server" bson:"server"`
	Type   string `json:"type" bson:"type"`
	Token  string `json:"token" bson:"token"` //polaris使用的token
	Path   string `json:"path" bson:"path"`
	Env    string `json:"env" bson:"env"`
	Used   string `json:"used" bson:"used"`
	Prefix struct {
		Mysql         string `json:"mysql" bson:"mysql"`
		Postgres      string `json:"postgres" bson:"postgres"`
		Mongodb       string `json:"mongodb" bson:"mongodb"`
		Redis         string `json:"redis" bson:"redis"`
		Clickhouse    string `json:"clickhouse" bson:"clickhouse"`
		Nacos         string `json:"nacos" bson:"nacos"`
		Elasticsearch string `json:"elasticsearch" bson:"elasticsearch"`
		Kafka         string `json:"kafka" bson:"kafka"`
		Sqlite        string `json:"sqlite" bson:"sqlite"`
		Etcd          string `json:"etcd" bson:"etcd"`
		Consul        string `json:"consul" bson:"consul"`
	} `json:"prefix" bson:"prefix"`
}

type jwtConfig struct {
	Secret string `json:"secret" bson:"secret"`
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
	Get              string `json:"get" bson:"get"`
	Req              string `json:"req" bson:"req"`
	Api              string `json:"api" bson:"api"` //api日志表名
	Kafka            struct {
		Use   bool   `json:"use" bson:"use"`
		Topic string `json:"topic" bson:"topic"`
	} `json:"kafka" bson:"kafka"`
}

type discovery struct {
	Registry string `json:"registry" bson:"registry"`
	CallType string `json:"callType" bson:"callType"`
}

type sys struct {
	Enabled bool   `json:"enabled" bson:"enabled"` //是否启用系统内置基础功能(sys模块)
	Initdb  bool   `json:"initdb" bson:"initdb"`   //是否初始化基础数据(sys模块)
	BaseUri string `json:"baseUri" bson:"baseUri"` //基础API路径
	Swagger struct {
		Enabled bool   `json:"enabled" bson:"enabled"` //是否启用swagger
		Uri     string `json:"uri" bson:"uri"`         //swagger路径
	}
	Casbin bool `json:"casbin" bson:"casbin"` //是否在sys模块中启用casbin
}

type casbin struct {
	Enabled   bool   `json:"enabled" bson:"enabled"`       //是否启用casbin
	ModelFile string `json:"model_file" bson:"model_file"` //casbin模型文件路径
}

var Config = &config{}

var logger = gologger.GetLogger()

const config_file = "./application.yml"

// DefaultShutdownTimeout 优雅关闭的默认超时时间。
// 未配置 go.application.shutdownTimeout 或配置值非法（<=0）时使用该值，
// 与升级前硬编码的 5 秒保持一致，确保向后兼容。
const DefaultShutdownTimeout = 5 * time.Second

// GetShutdownTimeout 返回优雅关闭的超时时间。
// 读取 go.application.shutdownTimeout（单位：秒）；未配置或 <= 0 时回退为 DefaultShutdownTimeout。
func (c *config) GetShutdownTimeout() time.Duration {
	if c.App.ShutdownTimeout > 0 {
		return time.Duration(c.App.ShutdownTimeout) * time.Second
	}
	return DefaultShutdownTimeout
}

func (c *config) Init(cf string) {
	if cf == "" {
		cf = config_file
	}
	c.WorkDir = filepath.Dir(cf)
	logger.Debug("读取配置文件:" + cf)
	c.Cnf = koanf.New(".")
	f := file.Provider(cf)
	err := c.Cnf.Load(f, yaml.Parser())
	if err != nil {
		logger.Error("读取配置文件错误:" + err.Error())
	}
	c.App.Name = c.Cnf.String("go.application.name")
	c.App.Project = c.Cnf.String("go.application.project")
	if c.App.Project == "" {
		c.App.Project = "DEFAULT"
	}
	c.App.Port = c.Cnf.Int("go.application.port")
	c.App.PortSSL = c.Cnf.Int("go.application.port_ssl")
	c.App.Cert = c.Cnf.String("go.application.cert")
	c.App.Key = c.Cnf.String("go.application.key")
	c.App.Debug = c.Cnf.Bool("go.application.debug")
	c.App.IpAddr = c.Cnf.String("go.application.ip")
	c.App.ShutdownTimeout = c.Cnf.Int("go.application.shutdownTimeout")
	c.Config.Server = c.Cnf.String("go.config.server")
	c.Config.Type = c.Cnf.String("go.config.server_type")
	c.Config.Token = c.Cnf.String("go.config.token")
	c.Config.Path = c.Cnf.String("go.config.path")
	c.Config.Env = c.Cnf.String("go.config.env")
	c.Config.Used = c.Cnf.String("go.config.used")
	c.Config.Prefix.Mysql = c.Cnf.String("go.config.prefix.mysql")
	c.Config.Prefix.Clickhouse = c.Cnf.String("go.config.prefix.clickhouse")
	c.Config.Prefix.Mongodb = c.Cnf.String("go.config.prefix.mongodb")
	c.Config.Prefix.Redis = c.Cnf.String("go.config.prefix.redis")
	c.Config.Prefix.Elasticsearch = c.Cnf.String("go.config.prefix.elasticsearch")
	c.Config.Prefix.Kafka = c.Cnf.String("go.config.prefix.kafka")
	c.Config.Prefix.Sqlite = c.Cnf.String("go.config.prefix.sqlite")
	c.Config.Prefix.Nacos = c.Cnf.String("go.config.prefix.nacos")
	c.Config.Prefix.Etcd = c.Cnf.String("go.config.prefix.etcd")
	c.Config.Prefix.Consul = c.Cnf.String("go.config.prefix.consul")
	c.Log.LogDb = c.Cnf.String("go.log.db")
	c.Log.RequestTableName = c.Cnf.String("go.log.req")
	c.Log.CallTableName = c.Cnf.String("go.log.call")
	c.Log.DbName = c.Cnf.String("go.log.dbName")
	c.Log.Get = c.Cnf.String("go.log.get")
	c.Log.Req = c.Cnf.String("go.log.req")
	c.Log.Api = c.Cnf.String("go.log.api")
	c.Cnf.Unmarshal("go.log.kafka", &c.Log.Kafka)
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
	c.Jwt.Secret = c.Cnf.String("go.jwt.secret")
	c.Sys.Enabled = c.Cnf.Bool("go.sys.enabled")
	c.Sys.Initdb = c.Cnf.Bool("go.sys.initdb")
	c.Sys.BaseUri = c.Cnf.String("go.sys.baseUri")
	c.Sys.Swagger.Enabled = c.Cnf.Bool("go.sys.swagger.enabled")
	c.Sys.Swagger.Uri = c.Cnf.String("go.sys.swagger.uri")
	if c.Sys.BaseUri == "" {
		c.Sys.BaseUri = "/api/v1"
	}
	if c.Sys.Swagger.Uri == "" {
		c.Sys.Swagger.Uri = "/swagger/sys"
	}
	c.Casbin.Enabled = c.Cnf.Bool("go.casbin.enabled")
	c.Casbin.ModelFile = c.Cnf.String("go.casbin.model_file")
	if !(strings.Contains(c.Casbin.ModelFile, "/") || strings.Contains(c.Casbin.ModelFile, "\\")) {
		c.Casbin.ModelFile = filepath.Join(c.WorkDir, c.Casbin.ModelFile)
	}
	c.Sys.Casbin = c.Cnf.Bool("go.sys.casbin")

	// v2：加载 go.framework.* 节点到 Framework 字段。
	// 老项目未配置时全部字段为零值，调用方按需回退到合理默认（见各组件的 Enabled() 实现）。
	c.Framework.HealthEnabled = c.Cnf.Bool("go.framework.healthEnabled")
	c.Framework.I18nEnabled = c.Cnf.Bool("go.framework.i18nEnabled")
	c.Framework.RatelimitEnabled = c.Cnf.Bool("go.framework.ratelimitEnabled")
	c.Framework.RecoverEnabled = c.Cnf.Bool("go.framework.recoverEnabled")
	c.Framework.LogRequests = c.Cnf.Bool("go.framework.logRequests")
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

func (c *config) GetConfigStringArray(name string) []string {
	if c.Cnf == nil {
		return nil
	}
	if c.Cnf.Exists(name) {
		return c.Cnf.Strings(name)
	} else {
		return nil
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
		configUrl = configUrl + "nacos/v1/cs/configs?group=" + c.App.Project + "&dataId=" + prefix + "-" + c.Config.Env + ".yml"
	case "polaris":
		if strings.Contains(prefix, "@") {
			strs := strings.Split(prefix, "@v")
			prefix = strs[0]
			version := strs[1]
			configUrl = configUrl + "/config/v1/configfiles/release?namespace=default&group=" + c.App.Project + "&name=" + prefix + "-" + c.Config.Env + ".yml&release_name=" + version
			break
		} else {
			configUrl = configUrl + "/config/v1/configfiles/release?namespace=default&group=" + c.App.Project + "&name=" + prefix + "-" + c.Config.Env + ".yml"
		}
	case "consul":
		configUrl = fmt.Sprintf("%s/v1/kv/%s/%s-%s.yml?dc=dc1&raw=true", configUrl, c.App.Project, prefix, c.Config.Env)
	case "springconfig":
		configUrl = fmt.Sprintf("%s/%s/%s-%s.yml", configUrl, c.App.Project, prefix, c.Config.Env)
	case "file":
		if c.WorkDir != "" {
			configUrl = c.WorkDir + "/" + prefix + "-" + c.Config.Env + ".yml"
			break
		}
		path, _ := filepath.Abs(filepath.Dir(os.Args[0]))
		if c.Config.Path != "" {
			path = strings.TrimSuffix(c.Config.Path, "/")
		}
		configUrl = path + "/" + prefix + "-" + c.Config.Env + ".yml"
	default:
		configUrl = configUrl + prefix + "-" + c.Config.Env + ".yml"
	}
	logger.Debug("配置文件地址: " + configUrl)
	return configUrl
}

func (c *config) GetConfigData(prefix string) []byte {
	switch c.Config.Type {
	case "etcd":
		cli, err := clientv3.New(clientv3.Config{Endpoints: []string{c.Config.Server}})
		if err != nil {
			return nil
		}
		resp, err := cli.Get(context.Background(), fmt.Sprintf("/config/%s/%s-%s.yml", c.App.Project, prefix, c.Config.Env))
		if err != nil {
			return nil
		}
		if len(resp.Kvs) == 0 {
			logger.Debug("etcd中配置文件不存在,疑似project配置错误,key为: " + fmt.Sprintf("/config/%s/%s-%s.yml", c.App.Project, prefix, c.Config.Env))
			return nil
		}
		return resp.Kvs[0].Value
	case "polaris":
		resp, err := grequests.Get(c.GetConfigUrl(prefix), grequests.FromRequestOptions(&grequests.RequestOptions{
			Headers: map[string]string{
				"X-Polaris-Token": c.Config.Token,
			},
		}))
		if err != nil {
			fmt.Println(err.Error())
			return nil
		}
		res := polaris_config_resp{}
		err = resp.JSON(&res)
		if err != nil {
			fmt.Println(err.Error())
			return nil
		}
		if res.Code != 200000 {
			fmt.Println(res.Info)
			return nil
		}
		return []byte(res.ConfigFileRelease.Content)
	case "file":
		data, err := ioutil.ReadFile(c.GetConfigUrl(prefix))
		if err != nil {
			return nil
		}
		return data
	default:
		resp, err := grequests.DoRegularRequest("GET", c.GetConfigUrl(prefix), nil)
		if err != nil {
			return nil
		}
		return resp.Bytes()
	}
}

type polaris_config_resp struct {
	Code              int         `json:"code"`
	Info              string      `json:"info"`
	ConfigFileGroup   interface{} `json:"configFileGroup"`
	ConfigFile        interface{} `json:"configFile"`
	ConfigFileRelease struct {
		Id                 string        `json:"id"`
		Name               string        `json:"name"`
		Namespace          string        `json:"namespace"`
		Group              string        `json:"group"`
		FileName           string        `json:"fileName"`
		Content            string        `json:"content"`
		Comment            string        `json:"comment"`
		Md5                string        `json:"md5"`
		Version            string        `json:"version"`
		CreateTime         string        `json:"createTime"`
		CreateBy           string        `json:"createBy"`
		ModifyTime         string        `json:"modifyTime"`
		ModifyBy           string        `json:"modifyBy"`
		Tags               []interface{} `json:"tags"`
		Active             bool          `json:"active"`
		Format             string        `json:"format"`
		ReleaseDescription string        `json:"releaseDescription"`
		ReleaseType        string        `json:"releaseType"`
		BetaLabels         []interface{} `json:"betaLabels"`
	} `json:"configFileRelease"`
	ConfigFileReleaseHistory interface{} `json:"configFileReleaseHistory"`
	ConfigFileTemplate       interface{} `json:"configFileTemplate"`
}
