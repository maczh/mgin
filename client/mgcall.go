package client

import (
	"errors"
	"fmt"
	"github.com/levigross/grequests"
	"github.com/maczh/mgin"
	"github.com/maczh/mgin/cache"
	"github.com/maczh/mgin/logs"
	"github.com/maczh/mgin/middleware/trace"
	"github.com/maczh/mgin/middleware/xlang"
	"github.com/nacos-group/nacos-sdk-go/model"
	"github.com/nacos-group/nacos-sdk-go/vo"
	"math/rand"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	reqType string
}

//微服务调用其他服务的接口
func (c *Client) Call(service string, uri string, params interface{}, opts ...interface{}) (string, error) {
	if c.reqType == "" {
		c.reqType = mgin.MGin.Config.GetConfigString("go.microservice.type")
		if c.reqType == "" {
			c.reqType = "x-form"
		}
	}
	switch c.reqType {
	case "x-form":
		return CallWithHeader(service, uri, params.(map[string]string), map[string]string{})
	case "json":
		if len(opts) == 0 || opts[0].(string) == "POST" {
			return PostJson(service, uri, params)
		} else {
			return GetJson(service, uri, params)
		}
	case "restful":
		return RestfulWithHeader(opts[0].(string), service, uri, opts[1].(map[string]string), nil, params)
	default:
		return "", fmt.Errorf("微服务接口协议模式设置错误")
	}
}

func Get(service string, uri string, params map[string]string) (string, error) {
	return GetWithHeader(service, uri, params, map[string]string{})
}

func GetWithHeader(service string, uri string, params map[string]string, header map[string]string) (string, error) {
	host, err := getHostFromCache(service)
	group := "DEFAULT_GROUP"
	if err != nil || host == "" {
		discovery := mgin.MGin.Config.GetConfigString("go.discovery")
		if discovery == "" {
			discovery = "nacos"
		}
		switch discovery {
		case "nacos":
			host, group = mgin.MGin.Nacos.GetServiceURL(service)
			if host != "" && !cache.OnGetCache("nacos").IsExist("nacos:subscribe:"+service) {
				subscribeNacos(service, group)
				cache.OnGetCache("nacos").Add("nacos:subscribe:"+service, "true", 0)
			}
			//case "consul":
			//	host = mgin.MGin.Nacos.GetConsulServiceURL(service)
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
		discovery := mgin.MGin.Config.GetConfigString("go.discovery")
		if discovery == "" {
			discovery = "nacos"
		}
		switch discovery {
		case "nacos":
			host, group = mgin.MGin.Nacos.GetServiceURL(service)
			if host != "" && !cache.OnGetCache("nacos").IsExist("nacos:subscribe:"+service) {
				subscribeNacos(service, group)
				cache.OnGetCache("nacos").Add("nacos:subscribe:"+service, "true", 0)
			}
			//case "consul":
			//	host = mgin.MGin.Nacos.GetConsulServiceURL(service)
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
		discovery := mgin.MGin.Config.GetConfigString("go.discovery")
		if discovery == "" {
			discovery = "nacos"
		}
		switch discovery {
		case "nacos":
			host, group = mgin.MGin.Nacos.GetServiceURL(service)
			if host != "" && !cache.OnGetCache("nacos").IsExist("nacos:subscribe:"+service) {
				subscribeNacos(service, group)
				cache.OnGetCache("nacos").Add("nacos:subscribe:"+service, "true", 0)
			}
			//case "consul":
			//	host = mgin.MGin.Nacos.GetConsulServiceURL(service)
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
		discovery := mgin.MGin.Config.GetConfigString("go.discovery")
		if discovery == "" {
			discovery = "nacos"
		}
		switch discovery {
		case "nacos":
			host, group = mgin.MGin.Nacos.GetServiceURL(service)
			if host != "" && !cache.OnGetCache("nacos").IsExist("nacos:subscribe:"+service) {
				subscribeNacos(service, group)
				cache.OnGetCache("nacos").Add("nacos:subscribe:"+service, "true", 0)
			}
			//case "consul":
			//	host = mgin.MGin.Nacos.GetConsulServiceURL(service)
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
		discovery := mgin.MGin.Config.GetConfigString("go.discovery")
		if discovery == "" {
			discovery = "nacos"
		}
		switch discovery {
		case "nacos":
			host, group = mgin.MGin.Nacos.GetServiceURL(service)
			if host != "" && !cache.OnGetCache("nacos").IsExist("nacos:subscribe:"+service) {
				subscribeNacos(service, group)
				cache.OnGetCache("nacos").Add("nacos:subscribe:"+service, "true", 0)
			}
			//case "consul":
			//	host = mgin.MGin.Nacos.GetConsulServiceURL(service)
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
		discovery := mgin.MGin.Config.GetConfigString("go.discovery")
		if discovery == "" {
			discovery = "nacos"
		}
		switch discovery {
		case "nacos":
			host, group = mgin.MGin.Nacos.GetServiceURL(service)
			if host != "" && !cache.OnGetCache("nacos").IsExist("nacos:subscribe:"+service) {
				subscribeNacos(service, group)
				cache.OnGetCache("nacos").Add("nacos:subscribe:"+service, "true", 0)
			}
			//case "consul":
			//	host = mgin.MGin.Nacos.GetConsulServiceURL(service)
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
	err := mgin.MGin.Nacos.GetNacosClient().Subscribe(&vo.SubscribeParam{
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
		if servicesMap[s.ServiceName] == "" {
			servicesMap[s.ServiceName] = "http://" + s.Ip + ":" + s.Ip + strconv.Itoa(int(s.Port))
		} else {
			servicesMap[s.ServiceName] = servicesMap[s.ServiceName] + ",http://" + s.Ip + ":" + s.Ip + strconv.Itoa(int(s.Port))
		}
	}
	for serviceName, host := range servicesMap {
		cache.OnGetCache("nacos").Delete(serviceName)
		cache.OnGetCache("nacos").Add(serviceName, host, 5*time.Minute)
	}
}
