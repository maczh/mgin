package es

import (
	"fmt"
	"github.com/knadh/koanf"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/rawbytes"
	"github.com/levigross/grequests"
	"github.com/olivere/elastic"
	"github.com/sadlil/gologger"
	"io/ioutil"
	"log"
	"os"
	"strings"
)

type ElasticSearch struct {
	Elastic *elastic.Client
	conf    *koanf.Koanf
	confUrl string
}

var logger = gologger.GetLogger()

func (e *ElasticSearch) Init(elasticConfigUrl string) {
	if elasticConfigUrl != "" {
		e.confUrl = elasticConfigUrl
	}
	if e.confUrl == "" {
		logger.Error("ElasticSearch配置Url为空")
		return
	}
	var err error
	if e.Elastic == nil {
		if e.conf == nil {
			var confData []byte
			var err error
			if strings.HasPrefix(e.confUrl, "http://") {
				resp, err := grequests.Get(e.confUrl, nil)
				if err != nil {
					logger.Error("ElasticSearch配置下载失败! " + err.Error())
					return
				}
				confData = []byte(resp.String())
			} else {
				confData, err = ioutil.ReadFile(e.confUrl)
				if err != nil {
					logger.Error(fmt.Sprintf("ElasticSearch本地配置文件%s读取失败:%s", e.confUrl, err.Error()))
					return
				}
			}
			e.conf = koanf.New(".")
			err = e.conf.Load(rawbytes.Provider(confData), yaml.Parser())
			if err != nil {
				logger.Error("ElasticSearch配置解析错误:" + err.Error())
				e.conf = nil
				return
			}
		}
		//logger.Debug("Elastic地址:" + cfg.String("go.elasticsearch.uri"))
		user := e.conf.String("go.elasticsearch.user")
		password := e.conf.String("go.elasticsearch.password")
		servers := strings.Split(e.conf.String("go.elasticsearch.uri"), ",")
		if user != "" && password != "" {
			//logger.Debug("user:"+user+"   password:"+password)
			e.Elastic, err = elastic.NewClient(elastic.SetURL(servers...), elastic.SetBasicAuth(user, password), elastic.SetInfoLog(log.New(os.Stdout, "Elasticsearch", log.LstdFlags)), elastic.SetSniff(false))
		} else {
			e.Elastic, err = elastic.NewClient(elastic.SetURL(servers...), elastic.SetInfoLog(log.New(os.Stdout, "Elasticsearch", log.LstdFlags)), elastic.SetSniff(false))
		}
		if err != nil {
			logger.Error("Elasticsearch连接错误:" + err.Error())
		}
	} else {
		user := e.conf.String("go.elasticsearch.user")
		password := e.conf.String("go.elasticsearch.password")
		if user != "" && password != "" {
			//logger.Debug("user:"+user+"   password:"+password)
			e.Elastic, err = elastic.NewClient(elastic.SetURL(e.conf.String("go.elasticsearch.uri")), elastic.SetBasicAuth(user, password), elastic.SetInfoLog(log.New(os.Stdout, "Elasticsearch", log.LstdFlags)), elastic.SetSniff(false))
		} else {
			e.Elastic, err = elastic.NewClient(elastic.SetURL(e.conf.String("go.elasticsearch.uri")), elastic.SetInfoLog(log.New(os.Stdout, "Elasticsearch", log.LstdFlags)), elastic.SetSniff(false))
		}
		if err != nil {
			logger.Error("Elasticsearch连接错误:" + err.Error())
		}
	}
}

func (e *ElasticSearch) Close() {
	e.Elastic = nil
}

func (e *ElasticSearch) Check() error {
	if e.Elastic == nil || !e.Elastic.IsRunning() {
		logger.Error("Elasticsearch检查连接异常,尝试重连中")
		e.Init("")
		if e.Elastic == nil || !e.Elastic.IsRunning() {
			logger.Error("Elasticsearch重新连接失败")
			return fmt.Errorf("Elasticsearch连接检查失败")
		}
	}
	return nil
}
