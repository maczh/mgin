package client

import (
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/levigross/grequests"
	"github.com/maczh/mgin/cache"
	"github.com/maczh/mgin/config"
	"github.com/maczh/mgin/logs"
	"github.com/maczh/mgin/middleware/trace"
	"github.com/maczh/mgin/models"
	"github.com/maczh/mgin/registry"
	"github.com/maczh/mgin/utils"
	"github.com/nacos-group/nacos-sdk-go/model"
	"github.com/nacos-group/nacos-sdk-go/vo"
	"math/rand"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	CONTENT_TYPE_FORM    = "x-form"
	CONTENT_TYPE_JSON    = "json"
	CONTENT_TYPE_RESTFUL = "restful"
	CONTENT_TYPE_FILE    = "file"
)

type Options struct {
	Method   string                 `json:"method"`   //接口方法 GET|POST|PUT|DELETE
	Protocol string                 `json:"protocol"` //协议 x-form|json|restful
	Group    string                 `json:"group"`    //应用分组，用于nacos中分组，不传为当前nacos分组及默认分组
	Header   map[string]string      `json:"header"`   //额外的头部参数
	Query    map[string]string      `json:"query"`    //URL Query参数
	Data     map[string]string      `json:"data"`     //x-form Postform参数
	Json     any                    `json:"json"`     //json或restful模式的body参数
	Path     map[string]string      `json:"path"`     //restful模式的路径参数
	Files    []grequests.FileUpload //文件上传数据
	Retry    bool                   `json:"retry"` //是否重试
}

func Call(service, uri string, op *Options) (string, error) {
	if op.Protocol == "" {
		op.Protocol = config.Config.Discovery.CallType
	}
	headers := trace.GetHeaders()
	if op.Header != nil && len(op.Header) > 0 {
		for k, v := range op.Header {
			headers[k] = v
		}
	}
	op.Header = headers
	u := uri
	if op.Protocol == "restful" && len(op.Path) > 0 {
		for k, v := range op.Path {
			u = strings.ReplaceAll(uri, fmt.Sprintf("{%s}", k), url.PathEscape(v))
		}
	}
	if op.Method == "" {
		op.Method = "POST"
	}
	host, err := getHostFromCache(service)
	if err != nil || host == "" {
		host, op.Group = registry.Nacos.GetServiceURL(service, op.Group)
	}
	if host != "" {
		cache.OnGetCache("nacos").Add(service, host, 5*time.Minute)
		if !cache.OnGetCache("nacos").IsExist("nacos:subscribe:" + service) {
			subscribeNacos(service, op.Group)
			cache.OnGetCache("nacos").Add("nacos:subscribe:"+service, "true", 0)
		}
	} else {
		return "", errors.New("微服务获取" + service + "服务主机IP端口失败")
	}
	if op.Files != nil && len(op.Files) > 0 {
		op.Protocol = "file"
	}
	var resp *grequests.Response
	url := fmt.Sprintf("%s%s", host, u)
	logs.Debug("微服务{}请求: {} {}\n请求头: {}\nQuery:{}\n请求参数: {}\n请求体:{}", service, op.Method, url, op.Header, op.Query, op.Data, op.Json)
	switch op.Protocol {
	case CONTENT_TYPE_FORM:
		op.Header["Content-Type"] = gin.MIMEPOSTForm
		resp, err = grequests.DoRegularRequest(op.Method, url, &grequests.RequestOptions{
			Data:    op.Data,
			Params:  op.Query,
			Headers: op.Header,
		})
	case CONTENT_TYPE_JSON, CONTENT_TYPE_RESTFUL:
		op.Header["Content-Type"] = gin.MIMEJSON
		resp, err = grequests.DoRegularRequest(op.Method, url, &grequests.RequestOptions{
			JSON:    op.Json,
			Params:  op.Query,
			Headers: op.Header,
		})
	case CONTENT_TYPE_FILE:
		delete(op.Header, "Content-Type")
		resp, err = grequests.Post(url, &grequests.RequestOptions{
			Data:    op.Data,
			Params:  op.Query,
			Headers: op.Header,
			Files:   op.Files,
		})
	}
	if err != nil {
		logs.Error("微服务{}请求错误:{}", service, err.Error())
		if op.Retry {
			op.Retry = false
			return Call(service, uri, op)
		}
		if strings.Contains(err.Error(), "dial tcp") {
			return "", errors.New("微服务获取" + service + "服务主机IP端口失败")
		}
		return "", err
	}
	logs.Debug("微服务{}返回结果:{}", service, resp.String())
	return resp.String(), nil
}

func CallT[T any](service, uri string, op *Options) models.Result[T] {
	resp, err := Call(service, uri, op)
	if err != nil {
		return models.ErrorT[T](-1, err.Error())
	}
	if resp[:1] != "{" {
		return models.ErrorT[T](-1, "Service error")
	}
	var result models.Result[T]
	utils.FromJSON(resp, &result)
	return result
}

func getHostFromCache(serviceName string) (string, error) {
	h, _ := cache.OnGetCache("nacos").Value(serviceName)
	if h == nil {
		logs.Debug("{}服务无缓存", serviceName)
		return "", errors.New("无此服务缓存")
	} else {
		hosts := strings.Split(h.(string), ",")
		r := rand.New(rand.NewSource(time.Now().UnixNano()))
		return hosts[r.Intn(len(hosts))], nil
	}
}

func subscribeNacos(serviceName, groupName string) {
	logs.Debug("Nacos微服务订阅服务名:{}", serviceName)
	if groupName == "" {
		groupName = "DEFAULT_GROUP"
	}
	err := registry.Nacos.GetNacosClient().Subscribe(&vo.SubscribeParam{
		ServiceName: serviceName,
		Clusters:    []string{"DEFAULT"},
		GroupName:   groupName,
		SubscribeCallback: func(services []model.SubscribeService, err error) {
			subscribeNacosCallback(services, err)
		},
	})
	if err != nil {
		logs.Error("Nacos订阅错误:{}", err.Error())
	}
}

func subscribeNacosCallback(services []model.SubscribeService, err error) {
	logs.Debug("Nacos回调:{}", services)
	if err != nil {
		logs.Error("Nacos订阅回调错误:{}", err.Error())
		return
	}
	if services == nil || len(services) == 0 {
		logs.Error("Nacos订阅回调服务列表为空")
		return
	}
	servicesMap := make(map[string]string)
	for _, s := range services {
		protocal := "http://"
		if s.Metadata != nil && s.Metadata["ssl"] == "true" {
			protocal = "https://"
		}
		if s.Metadata != nil && s.Metadata["debug"] == "true" {
			continue
		}
		if servicesMap[s.ServiceName] == "" {
			servicesMap[s.ServiceName] = protocal + s.Ip + ":" + strconv.Itoa(int(s.Port))
		} else {
			servicesMap[s.ServiceName] = servicesMap[s.ServiceName] + "," + protocal + s.Ip + ":" + strconv.Itoa(int(s.Port))
		}
	}
	for serviceName, host := range servicesMap {
		cache.OnGetCache("nacos").Delete(serviceName)
		cache.OnGetCache("nacos").Add(serviceName, host, 5*time.Minute)
	}
}
