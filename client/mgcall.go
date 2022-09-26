package client

import (
	"errors"
	"fmt"
	"github.com/levigross/grequests"
	"github.com/maczh/mgin/cache"
	"github.com/maczh/mgin/config"
	"github.com/maczh/mgin/logs"
	"github.com/maczh/mgin/middleware/trace"
	"github.com/maczh/mgin/middleware/xlang"
	"github.com/maczh/mgin/registry"
	"github.com/nacos-group/nacos-sdk-go/model"
	"github.com/nacos-group/nacos-sdk-go/vo"
	"math/rand"
	"strconv"
	"strings"
	"time"
)

type mginClient struct {
	reqType string
}

var Nacos = &mginClient{}

//Call 微服务调用其他服务的接口
// x-form模式 Call(service string, uri string, params map[string]string)
// json模式 Call(service string, uri string, method string, queryParams map[string]string, jsonBody interface{})
// restful模式 Call(service string, uri string, method string, pathParams map[string]string, jsonBody interface{})
func (c *mginClient) Call(service string, uri string, params ...interface{}) (string, error) {
	if c.reqType == "" {
		c.reqType = config.Config.Discovery.CallType
		if c.reqType == "" {
			c.reqType = "x-form"
		}
	}
	switch c.reqType {
	case "x-form":
		var data map[string]string
		if len(params) == 1 {
			data = params[0].(map[string]string)
		}
		return CallWithHeader(service, uri, data, map[string]string{})
	case "json":
		var query map[string]string
		if len(params) == 0 {
			return GetJson(service, uri, nil, nil)
		}
		var body interface{}
		if len(params) == 3 {
			query = params[2].(map[string]string)
			body = params[1]
		} else if len(params) == 2 {
			body = params[1]
		}

		if params[0].(string) == "POST" {
			return PostJson(service, uri, body, query)
		} else {
			return GetJson(service, uri, body, query)
		}
	case "restful":
		method := "GET"
		var pathParams map[string]string
		var body interface{}
		switch len(params) {
		case 0:
			break
		case 1:
			method = params[0].(string)
		case 2:
			method = params[0].(string)
			pathParams = params[1].(map[string]string)
		case 3:
			method = params[0].(string)
			pathParams = params[1].(map[string]string)
			body = params[2]
		default:
			method = params[0].(string)
			pathParams = params[1].(map[string]string)
			body = params[2]
		}
		return RestfulWithHeader(method, service, uri, pathParams, nil, body)
	default:
		return "", fmt.Errorf("微服务接口协议模式设置错误")
	}
}

func (c *mginClient) CallForm(service string, uri string, params map[string]string) (string, error) {
	return CallWithHeader(service, uri, params, map[string]string{})
}

func (c *mginClient) CallJson(service string, uri string, method string, queryParams map[string]string, jsonBody interface{}) (string, error) {
	if method == "POST" {
		return PostJson(service, uri, jsonBody, queryParams)
	} else {
		return GetJson(service, uri, jsonBody, queryParams)
	}
}

func (c *mginClient) CallRestful(service string, uri string, method string, pathParams map[string]string, jsonBody interface{}) (string, error) {
	return RestfulWithHeader(method, service, uri, pathParams, map[string]string{}, jsonBody)
}

func Get(service string, uri string, params map[string]string) (string, error) {
	return GetWithHeader(service, uri, params, map[string]string{})
}

func GetWithHeader(service string, uri string, params map[string]string, header map[string]string) (string, error) {
	host, err := getHostFromCache(service)
	group := "DEFAULT_GROUP"
	if err != nil || host == "" {
		discovery := config.Config.Discovery.Registry
		if discovery == "" {
			discovery = "nacos"
		}
		switch discovery {
		case "nacos":
			host, group = registry.Nacos.GetServiceURL(service)
			if host != "" && !cache.OnGetCache("nacos").IsExist("nacos:subscribe:"+service) {
				subscribeNacos(service, group)
				cache.OnGetCache("nacos").Add("nacos:subscribe:"+service, "true", 0)
			}
			//case "consul":
			//	host = registry.Nacos.GetConsulServiceURL(service)
		}
		if host != "" {
			cache.OnGetCache("nacos").Add(service, host, 5*time.Minute)
		} else {
			return "", errors.New("微服务获取" + service + "服务主机IP端口失败")
		}
	}
	url := host + uri
	logs.Debug("Nacos微服务请求:{}\n请求参数:{}", url, params)
	header["X-Request-Id"] = trace.GetRequestId()
	header["X-Lang"] = xlang.GetCurrentLanguage()
	resp, err := grequests.Get(url, &grequests.RequestOptions{
		Params:             params,
		Headers:            header,
		InsecureSkipVerify: true,
	})
	logs.Debug("Nacos微服务返回结果:{}", resp.String())
	if err != nil {
		cache.OnGetCache("nacos").Delete(service)
		discovery := config.Config.Discovery.Registry
		if discovery == "" {
			discovery = "nacos"
		}
		switch discovery {
		case "nacos":
			host, group = registry.Nacos.GetServiceURL(service)
			if host != "" && !cache.OnGetCache("nacos").IsExist("nacos:subscribe:"+service) {
				subscribeNacos(service, group)
				cache.OnGetCache("nacos").Add("nacos:subscribe:"+service, "true", 0)
			}
			//case "consul":
			//	host = registry.Nacos.GetConsulServiceURL(service)
		}
		if host != "" {
			cache.OnGetCache("nacos").Add(service, host, 5*time.Minute)
		} else {
			return "", errors.New("微服务获取" + service + "服务主机IP端口失败")
		}
		url = host + uri
		resp, err = grequests.Get(url, &grequests.RequestOptions{
			Params:             params,
			Headers:            header,
			InsecureSkipVerify: true,
		})
		logs.Debug("Nacos微服务返回结果:{}", resp.String())
		if err != nil {
			return "", err
		} else {
			return resp.String(), nil
		}
	} else {
		return resp.String(), err
	}
}

//微服务调用其他服务的接口,带header
func CallWithHeader(service string, uri string, params map[string]string, header map[string]string) (string, error) {
	host, err := getHostFromCache(service)
	group := "DEFAULT_GROUP"
	if err != nil || host == "" {
		discovery := config.Config.Discovery.Registry
		if discovery == "" {
			discovery = "nacos"
		}
		switch discovery {
		case "nacos":
			host, group = registry.Nacos.GetServiceURL(service)
			if host != "" && !cache.OnGetCache("nacos").IsExist("nacos:subscribe:"+service) {
				subscribeNacos(service, group)
				cache.OnGetCache("nacos").Add("nacos:subscribe:"+service, "true", 0)
			}
			//case "consul":
			//	host = registry.Nacos.GetConsulServiceURL(service)
		}
		if host != "" {
			cache.OnGetCache("nacos").Add(service, host, 5*time.Minute)
		} else {
			return "", errors.New("微服务获取" + service + "服务主机IP端口失败")
		}
	}
	url := host + uri
	header["X-Request-Id"] = trace.GetRequestId()
	header["X-Lang"] = xlang.GetCurrentLanguage()
	logs.Debug("Nacos微服务请求:{}\n请求参数:{}\n请求头:{}", url, params, header)
	resp, err := grequests.Post(url, &grequests.RequestOptions{
		Data:               params,
		Headers:            header,
		InsecureSkipVerify: true,
	})
	logs.Debug("Nacos微服务返回结果:{}", resp.String())
	if err != nil {
		cache.OnGetCache("nacos").Delete(service)
		discovery := config.Config.Discovery.Registry
		if discovery == "" {
			discovery = "nacos"
		}
		switch discovery {
		case "nacos":
			host, group = registry.Nacos.GetServiceURL(service)
			if host != "" && !cache.OnGetCache("nacos").IsExist("nacos:subscribe:"+service) {
				subscribeNacos(service, group)
				cache.OnGetCache("nacos").Add("nacos:subscribe:"+service, "true", 0)
			}
			//case "consul":
			//	host = registry.Nacos.GetConsulServiceURL(service)
		}
		if host != "" {
			cache.OnGetCache("nacos").Add(service, host, 5*time.Minute)
		} else {
			return "", errors.New("微服务获取" + service + "服务主机IP端口失败")
		}
		url = host + uri
		resp, err = grequests.Post(url, &grequests.RequestOptions{
			Data:    params,
			Headers: header,
		})
		logs.Debug("Nacos微服务返回结果:{}", resp.String())
		if err != nil {
			return "", err
		} else {
			return resp.String(), nil
		}
	} else {
		return resp.String(), err
	}
}

func CallWithFiles(service string, uri string, params map[string]string, files []grequests.FileUpload) (string, error) {
	return CallWithFilesHeader(service, uri, params, files, map[string]string{})
}

//微服务调用其他服务的接口,带文件
func CallWithFilesHeader(service string, uri string, params map[string]string, files []grequests.FileUpload, header map[string]string) (string, error) {
	host, err := getHostFromCache(service)
	group := "DEFAULT_GROUP"
	if err != nil || host == "" {
		discovery := config.Config.Discovery.Registry
		if discovery == "" {
			discovery = "nacos"
		}
		switch discovery {
		case "nacos":
			host, group = registry.Nacos.GetServiceURL(service)
			if host != "" && !cache.OnGetCache("nacos").IsExist("nacos:subscribe:"+service) {
				subscribeNacos(service, group)
				cache.OnGetCache("nacos").Add("nacos:subscribe:"+service, "true", 0)
			}
			//case "consul":
			//	host = registry.Nacos.GetConsulServiceURL(service)
		}
		if host != "" {
			cache.OnGetCache("nacos").Add(service, host, 5*time.Minute)
		} else {
			return "", errors.New("微服务获取" + service + "服务主机IP端口失败")
		}
	}
	url := host + uri
	header["X-Request-Id"] = trace.GetRequestId()
	header["X-Lang"] = xlang.GetCurrentLanguage()
	logs.Debug("Nacos微服务请求:{}\n请求参数:{}", url, params)
	resp, err := grequests.Post(url, &grequests.RequestOptions{
		Data:               params,
		Files:              files,
		Headers:            header,
		InsecureSkipVerify: true,
	})
	logs.Debug("Nacos微服务返回结果:{}", resp.String())
	if err != nil {
		cache.OnGetCache("nacos").Delete(service)
		discovery := config.Config.Discovery.Registry
		if discovery == "" {
			discovery = "nacos"
		}
		switch discovery {
		case "nacos":
			host, group = registry.Nacos.GetServiceURL(service)
			if host != "" && !cache.OnGetCache("nacos").IsExist("nacos:subscribe:"+service) {
				subscribeNacos(service, group)
				cache.OnGetCache("nacos").Add("nacos:subscribe:"+service, "true", 0)
			}
			//case "consul":
			//	host = registry.Nacos.GetConsulServiceURL(service)
		}
		if host != "" {
			cache.OnGetCache("nacos").Add(service, host, 5*time.Minute)
		} else {
			return "", errors.New("微服务获取" + service + "服务主机IP端口失败")
		}
		url = host + uri
		resp, err = grequests.Post(url, &grequests.RequestOptions{
			Data:               params,
			Headers:            header,
			InsecureSkipVerify: true,
		})
		logs.Debug("Nacos微服务返回结果:{}", resp.String())
		if err != nil {
			return "", err
		} else {
			return resp.String(), nil
		}
	} else {
		return resp.String(), err
	}
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
