package client

import (
	"errors"
	"fmt"
	"github.com/levigross/grequests"
	"github.com/maczh/mgin/cache"
	"github.com/maczh/mgin/config"
	"github.com/maczh/mgin/logs"
	"github.com/maczh/mgin/middleware/trace"
	"github.com/maczh/mgin/registry"
	"github.com/maczh/mgin/utils"
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

// Call 微服务调用其他服务的接口
// x-form模式 Call(service string, uri string, params map[string]string)
// json模式 Call(service string, uri string, method string, queryParams map[string]string, jsonBody interface{})
// restful模式 Call(service string, uri string, method string, pathParams map[string]string, queryparams map[string]string, header map[string]string, jsonBody interface{})
func (c *mginClient) Call(service string, uri string, params ...interface{}) (string, error) {
	if c.reqType == "" {
		c.reqType = config.Config.Discovery.CallType
		if c.reqType == "" {
			c.reqType = "x-form"
		}
	}
	switch c.reqType {
	case "x-form":
		return CallWithHeader(service, uri, params[0], map[string]string{"Content-Type": "application/x-www-form-urlencoded"})
	case "json":
		var query interface{}
		if len(params) == 0 {
			return GetJson(service, uri, nil, nil)
		}
		var body interface{}
		if len(params) == 3 {
			query = params[2]
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
		var pathParams, queryParams, header interface{}
		var body interface{}
		switch len(params) {
		case 0:
			break
		case 1:
			method = params[0].(string)
		case 2:
			method = params[0].(string)
			pathParams = params[1]
		case 3:
			method = params[0].(string)
			pathParams = params[1]
			body = params[2]
		case 4:
			method = params[0].(string)
			pathParams = params[1]
			queryParams = params[2]
			body = params[3]
		case 5:
			method = params[0].(string)
			pathParams = params[1]
			queryParams = params[2]
			header = params[3].(map[string]string)
			body = params[4]
		default:
			method = params[0].(string)
			pathParams = params[1]
			queryParams = params[2]
			header = params[3]
			body = params[4]
		}
		return RestfulWithHeader(method, service, uri, pathParams, queryParams, header, body)
	default:
		return "", fmt.Errorf("微服务接口协议模式设置错误")
	}
}

func (c *mginClient) CallForm(service string, uri string, params interface{}) (string, error) {
	return CallWithHeader(service, uri, utils.AnyToMap(params), map[string]string{"Content-Type": "application/x-www-form-urlencoded"})
}

func (c *mginClient) CallJson(service string, uri string, method string, queryParams interface{}, jsonBody interface{}) (string, error) {
	if method == "POST" {
		return PostJson(service, uri, jsonBody, utils.AnyToMap(queryParams))
	} else {
		return GetJson(service, uri, jsonBody, utils.AnyToMap(queryParams))
	}
}

func (c *mginClient) CallRestful(service string, uri string, method string, pathParams, queryParams, header, jsonBody interface{}) (string, error) {
	return RestfulWithHeader(method, service, uri, pathParams, queryParams, header, jsonBody)
}

func Get(service string, uri string, params interface{}) (string, error) {
	return GetWithHeader(service, uri, params, map[string]string{})
}

func GetWithHeader(service string, uri string, params, header interface{}) (string, error) {
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
	headers := trace.GetHeaders()
	if header != nil {
		h := utils.AnyToMap(header)
		for k, v := range h {
			if headers[k] == "" {
				headers[k] = v
			}
		}
	}
	//通过X-Timeout来控制链路接口请求超时
	timeout := 90 * time.Second
	t := headers["X-Timeout"]
	if t != "" {
		ti, _ := strconv.Atoi(t)
		if ti > 0 {
			timeout = time.Duration(ti) * time.Second
		}
	}
	logs.Debug("Nacos微服务请求:{}\n请求参数:{}\n请求头:{}", url, params, header)
	resp, err := grequests.Get(url, &grequests.RequestOptions{
		Params:             utils.AnyToMap(params),
		Headers:            headers,
		InsecureSkipVerify: true,
		RequestTimeout:     timeout,
	})
	logs.Debug("Nacos微服务返回结果:{}", resp.String())
	if err != nil && strings.Contains(err.Error(), "connection refused") {
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
			Params:             utils.AnyToMap(params),
			Headers:            headers,
			InsecureSkipVerify: true,
		})
		logs.Debug("Nacos微服务返回结果:{}", resp.String())
		if err != nil {
			if err != nil && strings.Contains(err.Error(), "dial tcp") {
				return "", fmt.Errorf("Service unavailable")
			}
			return "", err
		} else {
			return resp.String(), nil
		}
	} else {
		if err != nil && strings.Contains(err.Error(), "dial tcp") {
			return "", fmt.Errorf("Service unavailable")
		}
		return resp.String(), err
	}
}

// 微服务调用其他服务的接口,带header
func CallWithHeader(service string, uri string, params, header interface{}) (string, error) {
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
	headers := trace.GetHeaders()
	if header != nil {
		h := utils.AnyToMap(header)
		for k, v := range h {
			if headers[k] == "" {
				headers[k] = v
			}
		}
	}
	//通过X-Timeout来控制链路接口请求超时
	timeout := 90 * time.Second
	t := headers["X-Timeout"]
	if t != "" {
		ti, _ := strconv.Atoi(t)
		if ti > 0 {
			timeout = time.Duration(ti) * time.Second
		}
	}
	logs.Debug("Nacos微服务请求:{}\n请求参数:{}\n请求头:{}", url, params, header)
	resp, err := grequests.Post(url, &grequests.RequestOptions{
		Data:               utils.AnyToMap(params),
		Headers:            headers,
		InsecureSkipVerify: true,
		RequestTimeout:     timeout,
	})
	logs.Debug("Nacos微服务返回结果:{}", resp.String())
	if err != nil && strings.Contains(err.Error(), "connection refused") {
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
			Data:    utils.AnyToMap(params),
			Headers: headers,
		})
		logs.Debug("Nacos微服务返回结果:{}", resp.String())
		if err != nil {
			if strings.Contains(err.Error(), "dial tcp") {
				return "", fmt.Errorf("Service unavailable")
			}
			return "", err
		} else {
			return resp.String(), nil
		}
	} else {
		if err != nil && strings.Contains(err.Error(), "dial tcp") {
			return "", fmt.Errorf("Service unavailable")
		}
		return resp.String(), err
	}
}

func CallWithFiles(service string, uri string, params interface{}, files []grequests.FileUpload) (string, error) {
	return CallWithFilesHeader(service, uri, params, files, map[string]string{})
}

// 微服务调用其他服务的接口,带文件
func CallWithFilesHeader(service string, uri string, params interface{}, files []grequests.FileUpload, header interface{}) (string, error) {
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
	headers := trace.GetHeaders()
	if header != nil {
		h := utils.AnyToMap(header)
		for k, v := range h {
			if headers[k] == "" {
				headers[k] = v
			}
		}
	}
	delete(headers, "Content-Type")
	logs.Debug("Nacos微服务请求:{}\n请求参数:{}\n请求头:{}", url, params, header)
	resp, err := grequests.Post(url, &grequests.RequestOptions{
		Data:               utils.AnyToMap(params),
		Files:              files,
		Headers:            headers,
		InsecureSkipVerify: true,
	})
	logs.Debug("Nacos微服务返回结果:{}", resp.String())
	if err != nil && strings.Contains(err.Error(), "connection refused") {
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
			Data:               utils.AnyToMap(params),
			Files:              files,
			Headers:            headers,
			InsecureSkipVerify: true,
		})
		logs.Debug("Nacos微服务返回结果:{}", resp.String())
		if err != nil {
			if strings.Contains(err.Error(), "dial tcp") {
				return "", fmt.Errorf("Service unavailable")
			}
			return "", err
		} else {
			return resp.String(), nil
		}
	} else {
		if err != nil && strings.Contains(err.Error(), "dial tcp") {
			return "", fmt.Errorf("Service unavailable")
		}
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
	if groupName == "" {
		groupName = "DEFAULT_GROUP"
	}
	if _, ok := registry.Nacos.Subscribes[serviceName]; !ok {
		logs.Debug("Nacos微服务订阅服务名:{}", serviceName)
		subsParams := &vo.SubscribeParam{
			ServiceName: serviceName,
			Clusters:    []string{"DEFAULT"},
			GroupName:   groupName,
			SubscribeCallback: func(services []model.SubscribeService, err error) {
				subscribeNacosCallback(services, err)
			},
		}
		err := registry.Nacos.GetNacosClient().Subscribe(subsParams)
		if err != nil {
			logs.Error("Nacos订阅错误:{}", err.Error())
		}
		registry.Nacos.Subscribes[serviceName] = subsParams
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
