package client

import (
	"errors"
	"github.com/levigross/grequests"
	"github.com/maczh/mgin/cache"
	"github.com/maczh/mgin/config"
	"github.com/maczh/mgin/logs"
	"github.com/maczh/mgin/middleware/trace"
	"github.com/maczh/mgin/middleware/xlang"
	"github.com/maczh/mgin/registry"
	"time"
)

func JsonWithHeader(method, service, uri string, header map[string]string, body interface{}, query map[string]string) (string, error) {
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
			//	host = mgconfig.GetConsulServiceURL(service)
		}
		if host != "" {
			cache.OnGetCache("nacos").Add(service, host, 5*time.Minute)
		} else {
			return "", errors.New("微服务获取" + service + "服务主机IP端口失败")
		}
	}

	url := host + uri
	logs.Debug("Nacos微服务请求:{}\n请求参数:{}", url, body)
	if header == nil {
		header = make(map[string]string)
	}
	header["X-Request-Id"] = trace.GetRequestId()
	header["X-Lang"] = xlang.GetCurrentLanguage()
	header["Content-Type"] = "application/json"
	var resp *grequests.Response
	switch method {
	case "GET":
		resp, err = grequests.Get(url, &grequests.RequestOptions{
			Headers:            header,
			Params:             query,
			InsecureSkipVerify: true,
			JSON:               body,
		})
	case "POST":
		resp, err = grequests.Post(url, &grequests.RequestOptions{
			Headers:            header,
			Params:             query,
			InsecureSkipVerify: true,
			JSON:               body,
		})
	case "DELETE":
		resp, err = grequests.Delete(url, &grequests.RequestOptions{
			Headers:            header,
			Params:             query,
			InsecureSkipVerify: true,
			JSON:               body,
		})
	case "PUT":
		resp, err = grequests.Put(url, &grequests.RequestOptions{
			Headers:            header,
			Params:             query,
			InsecureSkipVerify: true,
			JSON:               body,
		})
	case "OPTIONS":
		resp, err = grequests.Options(url, &grequests.RequestOptions{
			Headers:            header,
			Params:             query,
			InsecureSkipVerify: true,
			JSON:               body,
		})
	case "HEAD":
		resp, err = grequests.Head(url, &grequests.RequestOptions{
			Headers:            header,
			Params:             query,
			InsecureSkipVerify: true,
			JSON:               body,
		})
	}
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
			//	host = mgconfig.GetConsulServiceURL(service)
		}
		if host != "" {
			cache.OnGetCache("nacos").Add(service, host, 5*time.Minute)
		} else {
			return "", errors.New("微服务获取" + service + "服务主机IP端口失败")
		}
		url = host + uri
		switch method {
		case "GET":
			resp, err = grequests.Get(url, &grequests.RequestOptions{
				Headers:            header,
				Params:             query,
				InsecureSkipVerify: true,
				JSON:               body,
			})
		case "POST":
			resp, err = grequests.Post(url, &grequests.RequestOptions{
				Headers:            header,
				Params:             query,
				InsecureSkipVerify: true,
				JSON:               body,
			})
		case "DELETE":
			resp, err = grequests.Delete(url, &grequests.RequestOptions{
				Headers:            header,
				Params:             query,
				InsecureSkipVerify: true,
				JSON:               body,
			})
		case "PUT":
			resp, err = grequests.Put(url, &grequests.RequestOptions{
				Headers:            header,
				Params:             query,
				InsecureSkipVerify: true,
				JSON:               body,
			})
		case "OPTIONS":
			resp, err = grequests.Options(url, &grequests.RequestOptions{
				Headers:            header,
				Params:             query,
				InsecureSkipVerify: true,
				JSON:               body,
			})
		case "HEAD":
			resp, err = grequests.Head(url, &grequests.RequestOptions{
				Headers:            header,
				Params:             query,
				InsecureSkipVerify: true,
				JSON:               body,
			})
		}
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

func PostJson(service, uri string, body interface{}, query map[string]string) (string, error) {
	return JsonWithHeader("POST", service, uri, nil, body, query)
}

func GetJson(service, uri string, body interface{}, query map[string]string) (string, error) {
	return JsonWithHeader("GET", service, uri, nil, body, query)
}
