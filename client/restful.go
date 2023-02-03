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
	"net/url"
	"strconv"
	"strings"
	"time"
)

func RestfulWithHeader(method, service string, uri string, pathparams, queryparams, header, body interface{}) (string, error) {
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
	for k, v := range utils.AnyToMap(pathparams) {
		uri = strings.ReplaceAll(uri, fmt.Sprintf("{%s}", k), url.PathEscape(v))
	}
	url := host + uri
	headers := trace.GetHeaders()
	if header == nil {
		h := utils.AnyToMap(header)
		for k, v := range h {
			if headers[k] == "" {
				headers[k] = v
			}
		}
	}
	headers["Content-Type"] = "application/json"
	//通过X-Timeout来控制链路接口请求超时
	timeout := 90 * time.Second
	t := headers["X-Timeout"]
	if t != "" {
		ti, _ := strconv.Atoi(t)
		if ti > 0 {
			timeout = time.Duration(ti) * time.Second
		}
	}
	logs.Debug("Nacos微服务请求:{}\n请求参数:{}\n请求头:{}", url, body, header)
	var resp *grequests.Response
	switch method {
	case "GET":
		resp, err = grequests.Get(url, &grequests.RequestOptions{
			Headers:            headers,
			Params:             utils.AnyToMap(queryparams),
			InsecureSkipVerify: true,
			JSON:               body,
			RequestTimeout:     timeout,
		})
	case "POST":
		resp, err = grequests.Post(url, &grequests.RequestOptions{
			Headers:            headers,
			Params:             utils.AnyToMap(queryparams),
			InsecureSkipVerify: true,
			JSON:               body,
			RequestTimeout:     timeout,
		})
	case "DELETE":
		resp, err = grequests.Delete(url, &grequests.RequestOptions{
			Headers:            headers,
			Params:             utils.AnyToMap(queryparams),
			InsecureSkipVerify: true,
			JSON:               body,
			RequestTimeout:     timeout,
		})
	case "PUT":
		resp, err = grequests.Put(url, &grequests.RequestOptions{
			Headers:            headers,
			Params:             utils.AnyToMap(queryparams),
			InsecureSkipVerify: true,
			JSON:               body,
			RequestTimeout:     timeout,
		})
	case "OPTIONS":
		resp, err = grequests.Options(url, &grequests.RequestOptions{
			Headers:            headers,
			Params:             utils.AnyToMap(queryparams),
			InsecureSkipVerify: true,
			JSON:               body,
			RequestTimeout:     timeout,
		})
	case "HEAD":
		resp, err = grequests.Head(url, &grequests.RequestOptions{
			Headers:            headers,
			Params:             utils.AnyToMap(queryparams),
			InsecureSkipVerify: true,
			JSON:               body,
			RequestTimeout:     timeout,
		})
	}
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
				Headers:            headers,
				Params:             utils.AnyToMap(queryparams),
				InsecureSkipVerify: true,
				JSON:               body,
				RequestTimeout:     timeout,
			})
		case "POST":
			resp, err = grequests.Post(url, &grequests.RequestOptions{
				Headers:            headers,
				Params:             utils.AnyToMap(queryparams),
				InsecureSkipVerify: true,
				JSON:               body,
				RequestTimeout:     timeout,
			})
		case "DELETE":
			resp, err = grequests.Delete(url, &grequests.RequestOptions{
				Headers:            headers,
				Params:             utils.AnyToMap(queryparams),
				InsecureSkipVerify: true,
				JSON:               body,
				RequestTimeout:     timeout,
			})
		case "PUT":
			resp, err = grequests.Put(url, &grequests.RequestOptions{
				Headers:            headers,
				Params:             utils.AnyToMap(queryparams),
				InsecureSkipVerify: true,
				JSON:               body,
				RequestTimeout:     timeout,
			})
		case "OPTIONS":
			resp, err = grequests.Options(url, &grequests.RequestOptions{
				Headers:            headers,
				Params:             utils.AnyToMap(queryparams),
				InsecureSkipVerify: true,
				JSON:               body,
				RequestTimeout:     timeout,
			})
		case "HEAD":
			resp, err = grequests.Head(url, &grequests.RequestOptions{
				Headers:            headers,
				Params:             utils.AnyToMap(queryparams),
				InsecureSkipVerify: true,
				JSON:               body,
				RequestTimeout:     timeout,
			})
		}
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
