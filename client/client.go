package client

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/levigross/grequests"
	"github.com/maczh/mgin/config"
	"github.com/maczh/mgin/internal/safego"
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
	Registry string                 `json:"registry"` //目标注册中心实例名称，留空使用默认实例
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
	// Cache opts a successful GET response into the shared response cache and
	// installs the built-in cached fallback when Fallback is nil. It can also be
	// enabled process-wide with go.client.cache.enabled.
	Cache bool `json:"cache"`
	// CacheTTL bounds how long a cached response stays usable; zero uses
	// go.client.cache.ttl (default 30s).
	CacheTTL time.Duration `json:"cache_ttl"`
	// CacheScope isolates cached entries by an explicit tenant, principal or
	// permission version. Never place raw tokens in it. When empty, an opaque
	// identity is derived from the credential headers.
	CacheScope string `json:"cache_scope"`
	// OnFallback is invoked with the original error when a fallback result is
	// returned, letting callers mark the response as degraded.
	OnFallback func(error) `json:"-"`
}

type cacheEntry struct {
	value     string
	storedAt  time.Time
	expiresAt time.Time
}

var responseCache = struct {
	sync.RWMutex
	values map[string]cacheEntry
}{values: make(map[string]cacheEntry)}

const defaultCacheTTL = 30 * time.Second
const defaultCacheMaxEntries = 1024

// responseCacheCapacity overrides the configured capacity when non-zero.
var responseCacheCapacity int

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
	return CallContext(context.Background(), service, uri, op)
}

// CallContext performs discovery and the outbound request within one caller-owned deadline.
func CallContext(ctx context.Context, service, uri string, op *Options) (result string, callErr error) {
	if ctx == nil {
		return "", errors.New("client context cannot be nil")
	}
	if op == nil {
		return "", errors.New("client options cannot be nil")
	}
	// Call may be used concurrently with a shared Options value. Work on a
	// shallow copy and never mutate the caller's options or headers.
	options := *op
	op = &options
	discoveryService := service
	if op.Registry != "" {
		discoveryService = op.Registry + "\x00" + service
	}
	if op.Fallback == nil && (op.Cache || config.Config.GetConfigBool("go.client.fallback.enabled")) {
		op.Fallback = cachedFallbackFor(discoveryService, uri, op)
	}
	fallbackUsed := false
	if op.Fallback != nil {
		fallback := op.Fallback
		op.Fallback = func(err error) (string, bool) {
			value, ok := fallback(err)
			if ok {
				fallbackUsed = true
				if op.OnFallback != nil {
					op.OnFallback(err)
				}
			}
			return value, ok
		}
	}
	if op.Header != nil {
		op.Header = cloneHeader(op.Header)
	}
	circuitEnabled := circuitEnabled("go.discovery.circuit.enabled")
	if !circuitEnabled {
		if op.Timeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, op.Timeout)
			defer cancel()
		}
		return call(ctx, service, uri, op)
	}

	key := op.CircuitKey
	if key == "" {
		key = discoveryService + "\x00" + uri
	}
	threshold, window := op.CircuitFailures, op.CircuitOpenWindow
	if threshold <= 0 {
		threshold = config.Config.GetConfigInt("go.discovery.circuit.failures")
	}
	if threshold <= 0 {
		threshold = circuitFailureThreshold
	}
	if window <= 0 {
		window = config.Config.GetConfigDuration("go.discovery.circuit.openWindow", circuitOpenWindow)
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
	if op.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, op.Timeout)
		defer cancel()
	}
	result, err := call(ctx, service, uri, op)
	// Client-side rejections (401/403/4xx) say nothing about downstream health,
	// so they must not trip the breaker for other callers of the same endpoint.
	if (err != nil && fallbackEligible(err)) || fallbackUsed {
		circuitFailure(key, threshold)
	} else {
		circuitSuccess(key)
	}
	return result, err
}

func circuitEnabled(key string) bool { return config.Config != nil && config.Config.GetConfigBool(key) }

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

func call(ctx context.Context, service, uri string, op *Options) (result string, callErr error) {
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
	if _, ok := ctx.Deadline(); !ok {
		timeout := config.Config.GetConfigDuration("go.client.timeout", 10*time.Second)
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	if err := ctx.Err(); err != nil {
		return "", err
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
	// Keep op.Group as requested by the caller so cache keys stay stable even
	// when discovery fails; the resolved group is only needed for the lookup.
	if resolver, ok := registry.Registry.(registry.NamedRegistryResolver); ok {
		host, _ = resolveWithContext(ctx, resolver, op.Registry, service, op.Group)
	} else {
		host, _ = resolveLegacyWithContext(ctx, registry.Registry, service, op.Group)
	}
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
			RequestTimeout: remaining(ctx), Context: ctx,
		})
	case CONTENT_TYPE_JSON, CONTENT_TYPE_RESTFUL:
		headers["Content-Type"] = gin.MIMEJSON
		resp, err = grequests.DoRegularRequest(op.Method, url, &grequests.RequestOptions{
			JSON:           op.Json,
			Params:         utils.AnyToMap(op.Query),
			Headers:        utils.AnyToMap(op.Header),
			RequestTimeout: remaining(ctx), Context: ctx,
		})
	case CONTENT_TYPE_FILE:
		delete(headers, "Content-Type")
		resp, err = grequests.Post(url, grequests.FromRequestOptions(&grequests.RequestOptions{
			Data:           utils.AnyToMap(op.Data),
			Params:         utils.AnyToMap(op.Query),
			Headers:        headers,
			Files:          op.Files,
			RequestTimeout: remaining(ctx), Context: ctx,
		}))
	default:
		return "", fmt.Errorf("unsupported client protocol %q", op.Protocol)
	}
	if err != nil {
		logs.Error("微服务{}请求错误:{}", service, err.Error())
		if op.Retry && retryableMethod(op.Method) && ctx.Err() == nil {
			op.Retry = false
			return call(ctx, service, uri, op)
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
		err := &httpStatusError{statusCode: resp.StatusCode}
		// Authentication, authorization and other client-side rejections must
		// surface to the caller; replaying a stale success would bypass them.
		if op.Fallback != nil && fallbackEligible(err) {
			if value, ok := op.Fallback(err); ok {
				return value, nil
			}
		}
		return "", err
	}
	logs.Debug("微服务{}返回 HTTP {}", service, resp.StatusCode)
	result = resp.String()
	if cacheable(op) {
		discoveryService := service
		if op.Registry != "" {
			discoveryService = op.Registry + "\x00" + service
		}
		cachePut(op.responseCacheKey(discoveryService, uri), result, op.CacheTTL)
	}
	return result, nil
}

func remaining(ctx context.Context) time.Duration {
	if deadline, ok := ctx.Deadline(); ok {
		return time.Until(deadline)
	}
	return 0
}

func retryableMethod(method string) bool {
	switch strings.ToUpper(method) {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodPut, http.MethodDelete:
		return true
	default:
		return false
	}
}

type contextURLResolver interface {
	GetServiceURLFromContext(context.Context, string, string, ...string) (string, string)
}

func resolveWithContext(ctx context.Context, r registry.NamedRegistryResolver, name, service string, group ...string) (string, string) {
	if cr, ok := any(r).(contextURLResolver); ok {
		return cr.GetServiceURLFromContext(ctx, name, service, group...)
	}
	return resolveLegacyWithContext(ctx, namedResolverAdapter{r, name}, service, group...)
}

type namedResolverAdapter struct {
	registry.NamedRegistryResolver
	name string
}

func (a namedResolverAdapter) GetServiceURL(service string, group ...string) (string, string) {
	return a.NamedRegistryResolver.GetServiceURLFrom(a.name, service, group...)
}
func resolveLegacyWithContext(ctx context.Context, r interface {
	GetServiceURL(string, ...string) (string, string)
}, service string, group ...string) (string, string) {
	type result struct{ host, group string }
	done := make(chan result, 1)
	safego.Run(func() { h, g := r.GetServiceURL(service, group...); done <- result{h, g} })
	select {
	case v := <-done:
		return v.host, v.group
	case <-ctx.Done():
		return "", ""
	}
}

// CachedFallback is the legacy entry point. It only reads responses cached for
// the current caller's credential headers, so it can never replay another
// principal's data, but it cannot match entries cached under a Method/Group
// variant. Prefer CachedFallbackFor.
//
// Deprecated: use CachedFallbackFor.
func CachedFallback(service, uri string, params ...map[string]any) func(error) (string, bool) {
	var request map[string]any
	if len(params) > 0 {
		request = params[0]
	}
	return func(err error) (string, bool) {
		if !fallbackEligible(err) {
			return "", false
		}
		return cacheGet(cacheKey(service, http.MethodGet, "", "", cacheIdentity(trace.GetHeaders()), uri, request))
	}
}

// cachedFallbackFor replays the response cached for exactly these options:
// method, group, caller identity (or CacheScope) and request parameters. It
// refuses to degrade authentication and business rejections. Call installs it
// with the effective options, so the key always matches the one used to write.
func cachedFallbackFor(service, uri string, op *Options) func(error) (string, bool) {
	return func(err error) (string, bool) {
		if op == nil || !fallbackEligible(err) {
			return "", false
		}
		return cacheGet(op.responseCacheKey(service, uri))
	}
}

// httpStatusError carries the downstream status so fallbacks can classify it.
type httpStatusError struct {
	statusCode int
}

func (e *httpStatusError) Error() string {
	return fmt.Sprintf("service returned HTTP %d", e.statusCode)
}

// fallbackEligible reports whether a failure may be degraded to a success
// response. Authentication (401), authorization (403) and other client-side
// rejections are never eligible; transport errors, 5xx, 408 and 429 are.
func fallbackEligible(err error) bool {
	if err == nil {
		return false
	}
	var statusErr *httpStatusError
	if errors.As(err, &statusErr) {
		switch {
		case statusErr.statusCode == http.StatusRequestTimeout, statusErr.statusCode == http.StatusTooManyRequests:
			return true
		case statusErr.statusCode >= 500 && statusErr.statusCode <= 599:
			return true
		}
		return false
	}
	return true
}

func requestParams(op *Options) map[string]any {
	return map[string]any{"query": op.Query, "data": op.Data, "json": op.Json, "path": op.Path}
}

// cacheable reports whether a response may be cached. Caching is opt-in and
// restricted to GET so that state-changing calls are never replayed.
func cacheable(op *Options) bool {
	if !op.Cache && !config.Config.GetConfigBool("go.client.cache.enabled") {
		return false
	}
	return strings.EqualFold(op.Method, http.MethodGet)
}

func (op *Options) responseCacheKey(service, uri string) string {
	scope := op.CacheScope
	identity := ""
	if scope == "" {
		identity = cacheIdentity(utils.AnyToMap(op.Header))
	}
	return cacheKey(service, strings.ToUpper(op.Method), op.Group, scope, identity, uri, requestParams(op))
}

func cacheGet(key string) (string, bool) {
	responseCache.Lock()
	defer responseCache.Unlock()
	entry, ok := responseCache.values[key]
	if !ok {
		return "", false
	}
	if !entry.expiresAt.IsZero() && time.Now().After(entry.expiresAt) {
		delete(responseCache.values, key)
		return "", false
	}
	return entry.value, true
}

func cachePut(key, value string, ttl time.Duration) {
	if ttl <= 0 {
		ttl = config.Config.GetConfigDuration("go.client.cache.ttl", defaultCacheTTL)
	}
	if ttl <= 0 {
		return
	}
	maxEntries := responseCacheCapacity
	if maxEntries <= 0 {
		maxEntries = config.Config.GetConfigInt("go.client.cache.maxEntries")
	}
	if maxEntries <= 0 {
		maxEntries = defaultCacheMaxEntries
	}
	now := time.Now()
	responseCache.Lock()
	defer responseCache.Unlock()
	for k, entry := range responseCache.values {
		if !entry.expiresAt.IsZero() && now.After(entry.expiresAt) {
			delete(responseCache.values, k)
		}
	}
	for len(responseCache.values) >= maxEntries {
		oldestKey := ""
		var oldest time.Time
		for k, entry := range responseCache.values {
			if oldestKey == "" || entry.storedAt.Before(oldest) {
				oldestKey, oldest = k, entry.storedAt
			}
		}
		if oldestKey == "" {
			return
		}
		delete(responseCache.values, oldestKey)
	}
	responseCache.values[key] = cacheEntry{value: value, storedAt: now, expiresAt: now.Add(ttl)}
}

var cacheIdentityHeaders = []string{"Authorization", "Token", "AccessToken", "X-User-Id", "X-Tenant-Id", "Cookie"}

// cacheIdentity hashes the credential headers into an opaque identity. Raw
// tokens are never embedded in cache keys or logs.
func cacheIdentity(headers map[string]string) string {
	parts := make([]string, 0, len(cacheIdentityHeaders))
	for _, name := range cacheIdentityHeaders {
		value := headerValue(headers, name)
		if value == "" {
			continue
		}
		sum := sha256.Sum256([]byte(value))
		parts = append(parts, name+"="+fmt.Sprintf("%x", sum[:]))
	}
	if len(parts) == 0 {
		return ""
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return fmt.Sprintf("%x", sum[:])
}

func headerValue(headers map[string]string, name string) string {
	if value, ok := headers[name]; ok {
		return value
	}
	// Pick the smallest matching key so the identity stays deterministic when
	// a header appears under several casings.
	matchedKey, matchedValue := "", ""
	for k, v := range headers {
		if strings.EqualFold(k, name) && (matchedKey == "" || k < matchedKey) {
			matchedKey, matchedValue = k, v
		}
	}
	return matchedValue
}

func cacheKey(service, method, group, scope, identity, uri string, params map[string]any) string {
	raw, _ := json.Marshal(params)
	sum := sha256.Sum256(raw)
	return strings.Join([]string{service, method, group, scope, identity, uri, fmt.Sprintf("%x", sum[:])}, "\x00")
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
