package client

import (
	"context"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/levigross/grequests"
	"github.com/maczh/mgin/pkg/cache"
	"github.com/maczh/mgin/pkg/config"
	"github.com/maczh/mgin/pkg/loadbalancer"
	"github.com/maczh/mgin/pkg/logs"
	"github.com/maczh/mgin/pkg/middleware/trace"
	"github.com/maczh/mgin/pkg/models"
	"github.com/maczh/mgin/pkg/otel"
	"github.com/maczh/mgin/pkg/registry"
	"github.com/maczh/mgin/pkg/utils"
	otelAttr "go.opentelemetry.io/otel/attribute"
	otelTrace "go.opentelemetry.io/otel/trace"
	"math/rand"
	"net/url"
	"strings"
	"time"
)

// ErrAllInstancesCircuitOpen v2 负载均衡专用错误：当服务的全部实例都被熔断时返此错误，
// 调用方可以快速失败（避免对已挂掉的下游继续施压）。
var ErrAllInstancesCircuitOpen = errors.New("client: all instances circuit-open")

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

	// v2 新增：负载均衡相关字段（可选，零侵入）。
	// LoadBalancer 为空时走 loadbalancer.Default()（RoundRobin）。
	// 命名常量参见 pkg/loadbalancer：roundrobin / random / leastconn / consistenthash。
	LoadBalancer string `json:"loadbalancer,omitempty"`
}

// Call 是 v1 时代的微服务调用入口，等价于 CallCtx(context.Background(), service, uri, op)。
// 保留以兼容存量项目；新代码建议直接使用 CallCtx 以便传入 context（例如超时/取消）。
func Call(service, uri string, op *Options) (string, error) {
	return CallCtx(context.Background(), service, uri, op)
}

// CallCtx 是 v2 新增的"支持 context"版本微服务调用入口。
//
// 背景：v1 的 Call 不支持 context，导致：
//   - 无法把 ctx 透传到 grequests 底层（grequests 的 DoRegularRequest 不接受 ctx）
//   - 业务侧无法用 ctx 实现超时或链路取消
//
// v2 决策（已与用户确认）：**保持 goroutine+timer 兑底**，
// 即调用方在 CallCtx 内部通过 ctx 控制的超时计时器来"及时返回"，但底层 HTTP 请求
// 不会被真正取消，仍会占用连接到对端响应或连接超时。要做到真正取消需要升级
// `levigross/grequests` 或换 net/http 自封装，那是 v2.1+ 的工作。
//
// ctx 在本函数的作用：
//   - 通过 grequests 兼容的 RequestOptions 携带（未来 SDK 支持时直接生效）
//   - 用于超时/取消的"上层兜底"（业务可在外层用 select 控制）
//   - 链路 trace header 由 trace.GetHeaders() 透传，与 ctx 无关
func CallCtx(ctx context.Context, service, uri string, op *Options) (string, error) {
	return callInternal(ctx, service, uri, op)
}

// callInternal 是 CallCtx 与 Call 共享的底层实现。ctx 在此函数内仅用于"占位 + 未来扩展"，
// 当前 goroutine+timer 兑底由 resilience.go 提供的 CallResilientCtx 负责。
func callInternal(ctx context.Context, service, uri string, op *Options) (string, error) {
	// v2.1 增强：OTel 集成（可选）。
	// 业务侧调用 otel.SetTracerProvider(...)
	// 之后，callInternal 会自动开一个 client.call 的 span，并把 traceparent 注入 header。
	// 未启用时 otel.Tracer() 返 noop，开销近零。
	if otel.IsEnabled() {
		var span otelTrace.Span
		ctx, span = otel.StartSpan(ctx, "mgin.client.call",
			otelTrace.WithAttributes(
				otelAttr.String("rpc.service", service),
				otelAttr.String("http.method", methodOrDefault(op)),
				otelAttr.String("rpc.uri", uri),
			),
		)
		defer span.End()
	}
	_ = ctx // 当前实现未直接使用，仅保留扩展位。
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
			u = strings.ReplaceAll(uri, fmt.Sprintf("{%s}", k), url.PathEscape(v))
		}
	}
	if op.Method == "" {
		op.Method = "POST"
	}
	host := ""
	var err error
	// v2 路径：优先尝试 GetServices 多实例，配合 LoadBalancer + per-instance 熔断选 host。
	host, err = selectHostByLB(service, op)
	if err != nil || host == "" {
		// 回退 v1 路径：单实例 GetServiceURL（向后兼容：旧注册中心或单实例部署）。
		host, op.Group = registry.Registry.GetServiceURL(service, op.Group)
	}
	if host == "" {
		//cache.OnGetCache("service", false).Add(fmt.Sprintf("%s@%s", service, op.Group), host, 5*time.Minute)
		//subscribeNacos(service, op.Group)
		//} else {
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
		headers["Content-Type"] = gin.MIMEPOSTForm
		resp, err = grequests.DoRegularRequest(op.Method, url, &grequests.RequestOptions{
			Data:    utils.AnyToMap(op.Data),
			Params:  utils.AnyToMap(op.Query),
			Headers: headers,
		})
	case CONTENT_TYPE_JSON, CONTENT_TYPE_RESTFUL:
		headers["Content-Type"] = gin.MIMEJSON
		resp, err = grequests.DoRegularRequest(op.Method, url, &grequests.RequestOptions{
			JSON:    op.Json,
			Params:  utils.AnyToMap(op.Query),
			Headers: utils.AnyToMap(op.Header),
		})
	case CONTENT_TYPE_FILE:
		delete(headers, "Content-Type")
		resp, err = grequests.Post(url, grequests.FromRequestOptions(&grequests.RequestOptions{
			Data:    utils.AnyToMap(op.Data),
			Params:  utils.AnyToMap(op.Query),
			Headers: headers,
			Files:   op.Files,
		}))
	}
	if err != nil {
		logs.Error("微服务{}请求错误:{}", service, err.Error())
		if op.Retry {
			op.Retry = false
			return callInternal(ctx, service, uri, op)
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
	return CallCtxT[T](context.Background(), service, uri, op)
}

// CallCtxT 是 CallT 的 context 版本，语义对齐 CallCtx。
func CallCtxT[T any](ctx context.Context, service, uri string, op *Options) models.Result[T] {
	resp, err := CallCtx(ctx, service, uri, op)
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
	h, _ := cache.OnGetCache("service", false).Value(serviceName)
	if h == nil {
		logs.Debug("{} 服务无缓存", serviceName)
		return "", errors.New("无此服务缓存")
	} else {
		hosts := strings.Split(h.(string), ",")
		r := rand.New(rand.NewSource(time.Now().UnixNano()))
		return hosts[r.Intn(len(hosts))], nil
	}
}

//func subscribeNacos(serviceName, groupName string) {
//	if groupName == "" {
//		groupName = "DEFAULT_GROUP"
//	}
//	if _, ok := registry.Registry.Subscribes[serviceName]; !ok {
//		logs.Debug("Nacos微服务订阅服务名:{}", serviceName)
//		subsParams := &vo.SubscribeParam{
//			ServiceName: serviceName,
//			Clusters:    []string{"DEFAULT"},
//			GroupName:   groupName,
//			SubscribeCallback: func(services []model.SubscribeService, err error) {
//				subscribeNacosCallback(services, err)
//			},
//		}
//		err := registry.Registry.GetNacosClient().Subscribe(subsParams)
//		if err != nil {
//			logs.Error("Nacos订阅错误:{}", err.Error())
//		}
//		registry.Registry.Subscribes[serviceName] = subsParams
//	}
//}
//
//func subscribeNacosCallback(services []model.SubscribeService, err error) {
//	logs.Debug("Nacos回调:{}", services)
//	if err != nil {
//		logs.Error("Nacos订阅回调错误:{}", err.Error())
//		return
//	}
//	if services == nil || len(services) == 0 {
//		logs.Error("Nacos订阅回调服务列表为空")
//		return
//	}
//	servicesMap := make(map[string]string)
//	for _, s := range services {
//		protocal := "http://"
//		if s.Metadata != nil && s.Metadata["ssl"] == "true" {
//			protocal = "https://"
//		}
//		if s.Metadata != nil && s.Metadata["debug"] == "true" {
//			continue
//		}
//		if servicesMap[s.ServiceName] == "" {
//			servicesMap[s.ServiceName] = protocal + s.Ip + ":" + strconv.Itoa(int(s.Port))
//		} else {
//			servicesMap[s.ServiceName] = servicesMap[s.ServiceName] + "," + protocal + s.Ip + ":" + strconv.Itoa(int(s.Port))
//		}
//	}
//	for serviceName, host := range servicesMap {
//		cache.OnGetCache("nacos").Delete(serviceName)
//		cache.OnGetCache("nacos").Add(serviceName, host, 5*time.Minute)
//	}
//}

// selectHostByLB 是 v2 负载均衡入口：先 GetServices 拿多实例，按 op.LoadBalancer 指定的策略
//（为空则走 loadbalancer.Default()，即 RoundRobin）选 1 个 host，并跳过被熔断的实例（per-instance
// 熔断器：client.GetBreaker(service+"@"+host)）。
//
// 返回：
//   - host != ""：选到可用实例
//   - host == "" && err == nil：无可用实例但 GetServices 返回空，让调用方走 v1 fallback
//   - err != nil：所有实例都被熔断，调用方应快速失败（ErrAllInstancesCircuitOpen）
func selectHostByLB(service string, op *Options) (string, error) {
	if registry.Registry == nil {
		return "", nil
	}
	instances, err := registry.Registry.GetServices(service, op.Group)
	if err != nil || len(instances) == 0 {
		return "", nil // 调用方会回退到 v1 单实例路径
	}

	// 选策略：op.LoadBalancer 非空时按名取；否则走全局 Default。
	lb := loadbalancer.Default()
	if op != nil && op.LoadBalancer != "" {
		if named := loadbalancer.Get(op.LoadBalancer); named != nil {
			lb = named
		}
	}

	// 一致性哈希 key：把 method+service 拼起来，确保同一业务调用尽量命中同一实例
	//（除非该实例被熔断，才会在下一轮重选时换）。
	key := service
	if op != nil && op.Method != "" {
		key = op.Method + "@" + service
	}

	// 最多尝试 len(instances) 次：每次选到被熔断的实例就把它临时剔除再选。
	skipped := make(map[string]struct{}, len(instances))
	for attempt := 0; attempt < len(instances); attempt++ {
		// 构造候选集：剔除已尝试且熔断的实例
		candidates := make([]string, 0, len(instances))
		for _, ins := range instances {
			if _, hit := skipped[ins]; !hit {
				candidates = append(candidates, ins)
			}
		}
		if len(candidates) == 0 {
			break
		}
		picked, pickErr := lb.Pick(candidates, key)
		if pickErr != nil {
			// LB 选不出来（理论上不该发生，候选非空），把这一批都视为不可用，触发 fallback。
			return "", nil
		}
		// per-instance 熔断
		br := GetBreaker(service + "@" + picked)
		if br != nil && !br.Allow() {
			skipped[picked] = struct{}{}
			continue
		}
		return picked, nil
	}

	// 全部实例都被熔断：直接快速失败，不再让请求穿透到下游。
	return "", fmt.Errorf("%w: %s 的全部 %d 个实例当前均处于熔断/打开状态",
		ErrAllInstancesCircuitOpen, service, len(instances))
}

// methodOrDefault 取 op.Method，未设置时返 "POST"（与 callInternal 内部默认保持一致）。
// 仅用于 OTel span attribute，不影响实际 HTTP 请求。
func methodOrDefault(op *Options) string {
	if op == nil {
		return "POST"
	}
	if op.Method == "" {
		return "POST"
	}
	return op.Method
}
