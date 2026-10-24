package client

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/levigross/grequests"
	"github.com/maczh/mgin/config"
	"github.com/maczh/mgin/logs"
	"github.com/maczh/mgin/logsink"
	"github.com/maczh/mgin/middleware/trace"
	"github.com/maczh/mgin/models"
	"github.com/maczh/mgin/models/vo"
	"github.com/maczh/mgin/registry"
	"github.com/maczh/mgin/utils"
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
	Header   any                    `json:"header"`   //额外的头部参数
	Query    any                    `json:"query"`    //URL Query参数
	Data     any                    `json:"data"`     //x-form Postform参数
	Json     any                    `json:"json"`     //json或restful模式的body参数
	Path     map[string]string      `json:"path"`     //restful模式的路径参数
	Files    []grequests.FileUpload //文件上传数据
	Retry    bool                   `json:"retry"` //是否重试
	// Timeout bounds the complete outbound request. Zero uses the framework default.
	Timeout    time.Duration              `json:"timeout"`
	Fallback   func(error) (string, bool) `json:"-"`
	CircuitKey string                     `json:"circuit_key"`
	// CircuitFailures and CircuitOpenWindow override the safe defaults when set.
	CircuitFailures   int           `json:"circuit_failures"`
	CircuitOpenWindow time.Duration `json:"circuit_open_window"`
}

var responseCache = struct {
	sync.RWMutex
	values map[string]string
}{values: make(map[string]string)}

type circuitState struct {
	failures int
	openedAt time.Time
	halfOpen bool
}

var circuits = struct {
	sync.Mutex
	values map[string]*circuitState
}{values: make(map[string]*circuitState)}

const circuitFailureThreshold = 3
const circuitOpenWindow = 30 * time.Second

func Call(service, uri string, op *Options) (result string, callErr error) {
	if op == nil {
		return "", errors.New("client options cannot be nil")
	}
	// Call may be used concurrently with a shared Options value. Work on a
	// shallow copy and never mutate the caller's options or headers.
	options := *op
	op = &options
	if op.Fallback == nil && config.Config.GetConfigBool("go.client.fallback.enabled") {
		op.Fallback = CachedFallback(service, uri, requestParams(op))
	}
	fallbackUsed := false
	if op.Fallback != nil {
		fallback := op.Fallback
		op.Fallback = func(err error) (string, bool) {
			value, ok := fallback(err)
			if ok { fallbackUsed = true }
			return value, ok
		}
	}
	if op.Header != nil {
		op.Header = cloneHeader(op.Header)
	}
	key := op.CircuitKey
	if key == "" {
		key = service + "\x00" + uri
	}
	threshold, window := op.CircuitFailures, op.CircuitOpenWindow
	if threshold <= 0 {
		threshold = config.Config.GetConfigInt("go.discovery.circuitFailures")
	}
	if threshold <= 0 {
		threshold = circuitFailureThreshold
	}
	if window <= 0 {
		window = config.Config.GetConfigDuration("go.discovery.circuitOpenWindow", circuitOpenWindow)
	}
	if !circuitAllow(key, window) {
		err := errors.New("circuit breaker is open")
		if op.Fallback != nil {
			if value, ok := op.Fallback(err); ok {
				return value, nil
			}
		}
		return "", err
	}
	result, err := call(service, uri, op)
	if err != nil || fallbackUsed {
		circuitFailure(key, threshold)
	} else {
		circuitSuccess(key)
	}
	return result, err
}

func circuitAllow(key string, window time.Duration) bool {
	circuits.Lock()
	defer circuits.Unlock()
	s := circuits.values[key]
	if s == nil {
		s = &circuitState{}
		circuits.values[key] = s
	}
	if s.openedAt.IsZero() {
		return true
	}
	if time.Since(s.openedAt) < window {
		return false
	}
	if s.halfOpen {
		return false
	}
	s.halfOpen = true
	return true
}
func circuitFailure(key string, threshold int) {
	circuits.Lock()
	defer circuits.Unlock()
	s := circuits.values[key]
	if s == nil {
		return
	}
	s.failures++
	s.halfOpen = false
	if s.failures >= threshold {
		s.openedAt = time.Now()
	}
}
func circuitSuccess(key string) {
	circuits.Lock()
	defer circuits.Unlock()
	if s := circuits.values[key]; s != nil {
		s.failures = 0
		s.openedAt = time.Time{}
		s.halfOpen = false
	}
}

func call(service, uri string, op *Options) (result string, callErr error) {
	start := time.Now()
	requestID := trace.GetRequestId()
	defer func() {
		if !config.Config.Log.VictoriaLogs.Client {
			return
		}
		level := "info"
		message := fmt.Sprintf("client %s %s %s (%dms)", service, op.Method, uri, time.Since(start).Milliseconds())
		if callErr != nil {
			level = "error"
			message += ": " + callErr.Error()
		}
		logsink.EmitRecordTo(logs.VictoriaLogsClientTarget, &logsink.Record{
			Time: start.Format(time.RFC3339Nano), App: config.Config.App.Name, Level: level,
			Message: message, RequestID: requestID,
		})
	}()

	if op.Protocol == "" {
		op.Protocol = config.Config.Discovery.CallType
	}
	headers := trace.GetHeaders()
	if op.Header != nil {
		for k, v := range utils.AnyToMap(op.Header) {
			headers[k] = v
		}
	}
	op.Header = headers
	u := uri
	if op.Protocol == "restful" && len(op.Path) > 0 {
		for k, v := range op.Path {
			u = strings.ReplaceAll(u, fmt.Sprintf("{%s}", k), url.PathEscape(v))
		}
	}
	if op.Method == "" {
		op.Method = "POST"
	}
	if op.Timeout <= 0 {
		op.Timeout = config.Config.GetConfigDuration("go.client.timeout", 10*time.Second)
	}
	host := ""
	var err error
	if registry.Registry == nil {
		err := errors.New("服务注册中心未初始化")
		if op.Fallback != nil {
			if value, ok := op.Fallback(err); ok {
				return value, nil
			}
		}
		return "", err
	}
	host, op.Group = registry.Registry.GetServiceURL(service, op.Group)
	if host == "" {
		err := errors.New("微服务获取" + service + "服务主机IP端口失败")
		if op.Fallback != nil {
			if value, ok := op.Fallback(err); ok {
				return value, nil
			}
		}
		return "", err
	}
	if op.Files != nil && len(op.Files) > 0 {
		op.Protocol = "file"
	}
	var resp *grequests.Response
	url := fmt.Sprintf("%s%s", host, u)
	logs.Debug("微服务{}请求: {} {}", service, op.Method, url)
	switch op.Protocol {
	case CONTENT_TYPE_FORM:
		headers["Content-Type"] = gin.MIMEPOSTForm
		resp, err = grequests.DoRegularRequest(op.Method, url, &grequests.RequestOptions{
			Data:           utils.AnyToMap(op.Data),
			Params:         utils.AnyToMap(op.Query),
			Headers:        headers,
			RequestTimeout: op.Timeout,
		})
	case CONTENT_TYPE_JSON, CONTENT_TYPE_RESTFUL:
		headers["Content-Type"] = gin.MIMEJSON
		resp, err = grequests.DoRegularRequest(op.Method, url, &grequests.RequestOptions{
			JSON:           op.Json,
			Params:         utils.AnyToMap(op.Query),
			Headers:        utils.AnyToMap(op.Header),
			RequestTimeout: op.Timeout,
		})
	case CONTENT_TYPE_FILE:
		delete(headers, "Content-Type")
		resp, err = grequests.Post(url, grequests.FromRequestOptions(&grequests.RequestOptions{
			Data:           utils.AnyToMap(op.Data),
			Params:         utils.AnyToMap(op.Query),
			Headers:        headers,
			Files:          op.Files,
			RequestTimeout: op.Timeout,
		}))
	default:
		return "", fmt.Errorf("unsupported client protocol %q", op.Protocol)
	}
	if err != nil {
		logs.Error("微服务{}请求错误:{}", service, err.Error())
		if op.Retry {
			op.Retry = false
			return call(service, uri, op)
		}
		if strings.Contains(err.Error(), "dial tcp") {
			err = errors.New("微服务获取" + service + "服务主机IP端口失败")
		}
		if op.Fallback != nil {
			if value, ok := op.Fallback(err); ok {
				return value, nil
			}
		}
		return "", err
	}
	if resp == nil {
		err := errors.New("service returned no response")
		if op.Fallback != nil {
			if value, ok := op.Fallback(err); ok {
				return value, nil
			}
		}
		return "", err
	}
	if !resp.Ok {
		err := fmt.Errorf("service returned HTTP %d", resp.StatusCode)
		if op.Fallback != nil {
			if value, ok := op.Fallback(err); ok {
				return value, nil
			}
		}
		return "", err
	}
	logs.Debug("微服务{}返回 HTTP {}", service, resp.StatusCode)
	result = resp.String()
	responseCache.Lock()
	responseCache.values[cacheKey(service, uri, requestParams(op))] = result
	responseCache.Unlock()
	return result, nil
}

// CachedFallback supplies the latest successful response when a downstream
// call fails. It is opt-in and therefore preserves existing fail-fast behavior.
func CachedFallback(service, uri string, params ...map[string]any) func(error) (string, bool) {
	var request map[string]any
	if len(params) > 0 {
		request = params[0]
	}
	return func(_ error) (string, bool) {
		responseCache.RLock()
		value, ok := responseCache.values[cacheKey(service, uri, request)]
		responseCache.RUnlock()
		return value, ok
	}
}

func requestParams(op *Options) map[string]any {
	return map[string]any{"query": op.Query, "data": op.Data, "json": op.Json, "path": op.Path}
}

func cacheKey(service, uri string, params map[string]any) string {
	raw, _ := json.Marshal(params)
	sum := sha256.Sum256(raw)
	return service + "\x00" + uri + "\x00" + fmt.Sprintf("%x", sum[:])
}

func cloneHeader(header any) any {
	values := utils.AnyToMap(header)
	copyValues := make(map[string]string, len(values))
	for k, v := range values {
		copyValues[k] = v
	}
	return copyValues
}

// CallResultT 调用使用 status/data/msg 协议的旧版微服务。
func CallResultT[T any](service, uri string, op *Options) models.Result[T] {
	resp, err := Call(service, uri, op)
	if err != nil {
		return models.ErrorT[T](-1, err.Error())
	}
	if err := decodeResponse(resp, &struct{}{}); err != nil {
		return models.ErrorT[T](-1, "Service error")
	}
	var result models.Result[T]
	utils.FromJSON(resp, &result)
	return result
}

// CallT 调用微服务并解析 code/data/msg/srvTime 响应。
func CallT[T any](service, uri string, op *Options) vo.Response[T] {
	resp, err := Call(service, uri, op)
	if err != nil {
		return vo.ErrorT[T](-1, err.Error())
	}
	var result vo.Response[T]
	if err := decodeResponse(resp, &result); err != nil {
		return vo.ErrorT[T](-1, "Service response error: "+err.Error())
	}
	return result
}

// CallPageT 调用微服务并解析 data.list/total/page/pageSize 分页响应。
func CallPageT[T any](service, uri string, op *Options) vo.ResponsePage[T] {
	resp, err := Call(service, uri, op)
	if err != nil {
		return vo.ErrorPageT[T](-1, err.Error())
	}
	var result vo.ResponsePage[T]
	if err := decodeResponse(resp, &result); err != nil {
		return vo.ErrorPageT[T](-1, "Service response error: "+err.Error())
	}
	return result
}

func decodeResponse(resp string, result any) error {
	resp = strings.TrimSpace(resp)
	if !strings.HasPrefix(resp, "{") {
		return errors.New("expected JSON object")
	}
	return json.Unmarshal([]byte(resp), result)
}
