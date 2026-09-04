package client

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/maczh/mgin/v2/pkg/logs"
)

// 重试退避的默认值：在调用方未显式指定 BaseDelay / MaxDelay 时使用。
const (
	// defaultBaseDelay 指数退避的初始延迟。
	defaultBaseDelay = 100 * time.Millisecond
	// defaultMaxDelay 指数退避的延迟上限。
	defaultMaxDelay = 2 * time.Second
)

// 调用级韧性策略的错误哨兵，便于调用方用 errors.Is 判断。
var (
	// ErrCircuitOpen 熔断器处于打开状态，请求被快速失败（未真正发起调用）。
	ErrCircuitOpen = errors.New("client: 熔断器已打开，请求被快速失败")
	// ErrCallTimeout 调用超过 ResilienceOptions.Timeout 仍未返回。
	ErrCallTimeout = errors.New("client: 微服务调用超时")
)

// ResilienceOptions 描述在 client.Call 之外叠加的韧性策略。
// 所有字段都是可选的；零值 / nil 表示“不使用该能力”，行为与直接调用 client.Call 完全一致。
type ResilienceOptions struct {
	// Timeout 单次调用的超时时间。<=0 表示不启用超时（与现有行为一致）。
	Timeout time.Duration
	// MaxRetries 除首次调用外最多重试的次数。<=0 表示不重试（与现有行为一致）。
	MaxRetries int
	// BaseDelay 指数退避的初始延迟，<=0 时使用 defaultBaseDelay(100ms)。
	BaseDelay time.Duration
	// MaxDelay 指数退避的延迟上限，<=0 时使用 defaultMaxDelay(2s)。
	MaxDelay time.Duration
	// RetryOnStatus 判定哪些 HTTP 状态码可重试。
	//
	// 【重要限制】现有 client.Call 的返回值只有响应体字符串与 error：
	// 非 2xx 响应（如 500/503）在 grequests 层面并不算 error，会以正常响应体返回，
	// 因此通过 CallResilient 调用时通常拿不到状态码，本回调也就不会被触发。
	// 只有当错误文本中出现形如 "status code 503" / "HTTP 500" / "code=502" 的
	// 状态码信息时（自定义错误、网关错误、未来 Call 返回带状态码的错误）才会生效。
	// 解析失败时回退到下面的幂等性默认策略。
	// 若后续允许为 Options 增加可选的状态码输出字段，本回调即可完整生效。
	RetryOnStatus func(code int) bool
	// Breaker 熔断器，nil 表示不启用熔断。可用 client.GetBreaker(service) 获取全局共享实例。
	Breaker *CircuitBreaker
}

// callResult 承载一次异步调用的结果，用于超时兜底。
type callResult struct {
	body string
	err  error
}

// CallResilient 在 client.Call 之上叠加超时、指数退避（带 jitter）重试与熔断。
//
// 幂等性保护（非常重要）：
//   - 幂等读（GET / HEAD / OPTIONS / TRACE）默认可重试；
//   - 非幂等写（POST / PUT / DELETE / PATCH）默认不重试，避免重复下单、重复扣款等副作用；
//   - 唯一例外：调用方显式提供 RetryOnStatus，且从错误中解析出的状态码被该回调判定为可重试；
//   - Options.Method 为空时，按 client.Call 的既有默认值视为 POST（非幂等，默认不重试）。
//
// 默认安全：ro 为 nil 或其所有字段为零值时，等价于直接调用一次 client.Call
// （不重试、不超时、不熔断），既有调用方不受任何影响。
//
// 参数:
//   - service: 微服务名。
//   - uri: 接口路径。
//   - op: 调用参数，与原 client.Call 一致；函数不会修改调用方传入的 op。
//   - ro: 韧性策略，可为 nil。
//
// 返回值:
//   - string: 成功时返回响应体，与原 client.Call 一致。
//   - error: 失败时返回最后一次的错误；熔断打开时返回包装了 ErrCircuitOpen 的错误。
//
// CallResilient 是 Call 的"带韧性策略"版本，等价于 CallResilientCtx(context.Background(), service, uri, op, ro)。
// 保留以兼容 v1 时代使用 CallResilient 的项目。
func CallResilient(service, uri string, op *Options, ro *ResilienceOptions) (string, error) {
	return CallResilientCtx(context.Background(), service, uri, op, ro)
}

// CallResilientCtx 是 CallResilient 的 context 版本（v2 新增）。
//
// 与 CallResilient 的差异：把 context 透传到 callWithTimeout 的 goroutine 调用链，
// 调用方可在 ctx 中带超时或取消信号。需要 ctx 真正取消底层 HTTP 时，
// 当前实现是"goroutine+timer 兑底"（见 v2-design §6.2），不依赖 grequests 升级。
func CallResilientCtx(ctx context.Context, service, uri string, op *Options, ro *ResilienceOptions) (string, error) {
	if op == nil {
		return "", errors.New("client: CallResilient 的 Options 参数不能为空")
	}
	if ro == nil {
		ro = &ResilienceOptions{}
	}

	// 熔断：打开状态下直接快速失败，不再消耗下游资源
	if ro.Breaker != nil && !ro.Breaker.Allow() {
		return "", fmt.Errorf("%w: %s", ErrCircuitOpen, ro.Breaker.String())
	}

	// client.Call 在 Method 为空时按 POST 处理，这里保持同一口径用于幂等判定
	method := op.Method
	if strings.TrimSpace(method) == "" {
		method = http.MethodPost
	}

	attempts := ro.MaxRetries + 1
	if attempts < 1 {
		attempts = 1
	}

	var body string
	var err error
	for i := 0; i < attempts; i++ {
		// 每次重试都使用 op 的副本：
		// 1) client.Call 会就地修改 Options（补全默认值、关闭内部重试），副本可保证每次重试入参一致；
		// 2) 超时后残留的调用 goroutine 仍会写这份副本，用副本可避免与调用方的 op 产生数据竞争。
		attempt := *op
		// 重试统一由本函数负责，关闭 client.Call 内部的朴素递归重试，否则重试次数会翻倍
		attempt.Retry = false

		body, err = callWithTimeoutCtx(ctx, service, uri, &attempt, ro.Timeout)
		if err == nil {
			break
		}
		if i >= attempts-1 {
			break
		}
		if !shouldRetry(err, method, ro) {
			break
		}
		delay := backoffDelay(i, ro)
		logs.Warn("微服务{}调用{}失败({})，{}后进行第{}次重试", service, uri, err.Error(), delay, i+1)
		time.Sleep(delay)
	}

	// 熔断计数以“整个操作的最终结果”为准，避免单次抖动就把熔断器打穿
	if ro.Breaker != nil {
		if err != nil {
			ro.Breaker.Failure()
		} else {
			ro.Breaker.Success()
		}
	}
	if err != nil {
		return "", err
	}
	return body, nil
}

// callWithTimeout 为单次调用叠加超时（v1 兼容入口）。ctx 取自 background。
func callWithTimeout(service, uri string, op *Options, timeout time.Duration) (string, error) {
	return callWithTimeoutCtx(context.Background(), service, uri, op, timeout)
}

// callWithTimeoutCtx 是单次调用的 ctx+超时实现。
//
// 【实现说明与局限 —— 这是一个兜底方案】
// grequests 的 RequestOptions 确实支持传入自定义 *http.Client（HTTPClient 字段），
// 但现有 client.Call 既没有把 http.Client 暴露到 Options 上，也没有接收 context.Context。
// 为了避免改动 client.Call 的既有签名与行为，这里不侵入 Call，而是在其外层用
// goroutine + timer 做"调用级"超时兜底：
//   - 优点：对 client.Call 零侵入，现有调用路径完全不受影响；
//   - 局限 1：超时后底层 HTTP 请求不会被取消，连接会一直占用到对端响应或连接超时，
//     真正的取消必须把 context / 自定义 http.Client 注入到 Call 内部才能实现；
//   - 局限 2：被调用方长期不返回时，发起调用的 goroutine 会挂起到请求结束。
//     这里把结果 channel 设为缓冲 1，保证 goroutine 一定能发送成功并退出，
//     不会出现因无人接收结果而导致的必然泄漏。
//
// 后续如果允许为 Options 增加可选字段（如 HTTPClient 或 Context），应优先改为原生超时。
func callWithTimeoutCtx(ctx context.Context, service, uri string, op *Options, timeout time.Duration) (string, error) {
	if timeout <= 0 {
		return CallCtx(ctx, service, uri, op)
	}
	ch := make(chan callResult, 1)
	go func() {
		body, err := CallCtx(ctx, service, uri, op)
		ch <- callResult{body: body, err: err}
	}()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case result := <-ch:
		return result.body, result.err
	case <-timer.C:
		return "", fmt.Errorf("%w: 微服务%s请求%s 超过 %s 未返回", ErrCallTimeout, service, uri, timeout)
	}
}

// shouldRetry 判断某次失败是否值得重试。
// 优先级：显式 RetryOnStatus（且能解析出状态码）> 幂等性默认策略。
func shouldRetry(err error, method string, ro *ResilienceOptions) bool {
	if err == nil {
		return false
	}
	if ro.RetryOnStatus != nil {
		if code, ok := extractStatusCode(err); ok {
			return ro.RetryOnStatus(code)
		}
		// 解析不到状态码说明是传输层错误（dial / 超时 / 连接被重置等），
		// 交由下面的幂等性默认策略决定
	}
	return isIdempotentMethod(method)
}

// statusCodePattern 用于从错误文本中提取 HTTP 状态码。
// 要求状态码前必须有 status / status code / code / http 之类的关键字，
// 避免把 “dial tcp 127.0.0.1:8080” 中的 IP 片段或端口号误判为状态码。
var statusCodePattern = regexp.MustCompile(`(?i)(?:status(?:\s*code)?|code|http)[\s:=]+([1-5][0-9]{2})\b`)

// extractStatusCode 尽力从错误信息中解析 HTTP 状态码。
// 返回值:
//   - int: 解析出的状态码；解析失败时为 0。
//   - bool: 是否成功解析。
func extractStatusCode(err error) (int, bool) {
	if err == nil {
		return 0, false
	}
	m := statusCodePattern.FindStringSubmatch(err.Error())
	if len(m) < 2 {
		return 0, false
	}
	code, convErr := atoi3(m[1])
	if convErr != nil {
		return 0, false
	}
	if code < 100 || code > 599 {
		return 0, false
	}
	return code, true
}

// atoi3 把三位数字字符串转为整数，非法输入返回错误。
func atoi3(s string) (int, error) {
	if len(s) != 3 {
		return 0, errors.New("client: 非三位数字")
	}
	n := 0
	for i := 0; i < 3; i++ {
		c := s[i]
		if c < '0' || c > '9' {
			return 0, errors.New("client: 含非数字字符")
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
}

// isIdempotentMethod 判断 HTTP 方法是否幂等（可安全重试）。
// 幂等读：GET / HEAD / OPTIONS / TRACE；其余（POST / PUT / DELETE / PATCH 等）一律视为非幂等。
func isIdempotentMethod(method string) bool {
	switch strings.ToUpper(strings.TrimSpace(method)) {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
		return true
	default:
		return false
	}
}

// backoffDelay 计算第 attempt 次重试（0 基）前的等待时间：指数退避 + 全抖动。
// 退避公式：delay = min(BaseDelay * 2^attempt, MaxDelay)；
// 抖动策略：最终等待时间取 [delay/2, delay) 区间内的随机值，打散重试时刻，避免惊群。
func backoffDelay(attempt int, ro *ResilienceOptions) time.Duration {
	base := ro.BaseDelay
	if base <= 0 {
		base = defaultBaseDelay
	}
	max := ro.MaxDelay
	if max <= 0 {
		max = defaultMaxDelay
	}
	if max < base {
		max = base
	}

	delay := base
	for i := 0; i < attempt; i++ {
		if delay >= max {
			break
		}
		delay *= 2
		// 乘法溢出保护：超过上限直接收敛到 max
		if delay <= 0 || delay > max {
			delay = max
			break
		}
	}
	if delay > max {
		delay = max
	}

	half := delay / 2
	if half <= 0 {
		return delay
	}
	return half + time.Duration(rand.Int63n(int64(half)))
}
