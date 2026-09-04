// Package metrics 提供 mgin v2 的 Prometheus 指标能力。
//
// 设计目标：
//   - 零依赖业务代码即可启用：业务 controller / service 不需要主动埋点，
//     pkg/metrics.Middleware() 自动记录所有 gin 路由的请求计数与时延。
//   - 与 health 包同样策略：端点 /metrics 在 baseRouter 最早注册，避免被 casbin / jwt 中间件拦截。
//   - 与 plugin.Plugin 协同：plugin.HealthAll() 的结果每周期上报为 mgin_dependency_up。
//
// 注意：本包依赖 prometheus/client_golang（v1.19.1），是 v2.1 唯一新增的 direct 第三方依赖。
// 如需在离线环境使用，可注释掉 middleware 引用，仅保留 build_info / dependency_up 等轻量指标。
package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// httpRequestsTotal 计数器：所有经过 mgin 路由的 HTTP 请求。
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mgin_http_requests_total",
			Help: "Total number of HTTP requests handled, labeled by method/path/status.",
		},
		[]string{"method", "path", "status"},
	)

	// httpRequestDuration 直方图：HTTP 请求时延分布。
	httpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "mgin_http_request_duration_seconds",
			Help:    "HTTP request latency in seconds.",
			Buckets: prometheus.DefBuckets, // 0.005, 0.01, 0.025, ... , 10
		},
		[]string{"method", "path"},
	)

	// httpRequestsInFlight Gauge：当前正在处理的请求数。
	httpRequestsInFlight = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "mgin_http_requests_in_flight",
			Help: "Number of HTTP requests currently being handled.",
		},
	)

	// buildInfo 恒为 1 的 Gauge，业务可在 /metrics 里直接读到 version / commit / go_version。
	buildInfo = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "mgin_build_info",
			Help: "Build information of the running mgin process (always 1).",
		},
		[]string{"version", "commit", "go_version"},
	)

	// pluginHealth Gauge：各 plugin 当前健康状态（1=ok, 0=fail），由 PeriodicPluginHealth 周期性更新。
	pluginHealth = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "mgin_plugin_health",
			Help: "Health of registered plugins (1=healthy, 0=unhealthy).",
		},
		[]string{"name"},
	)

	// dependencyUp Gauge：各外部依赖当前可用性（1=up, 0=down），由 plugin.HealthAll 周期上报。
	dependencyUp = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "mgin_dependency_up",
			Help: "Up/down status of external dependencies (1=up, 0=down).",
		},
		[]string{"name"},
	)
)

// SetBuildInfo 在启动时填入版本信息。version/commit/goVersion 任一为空会被填为 "unknown"。
// 建议在 mgin.NewApp 之后、app.Run 之前调用一次。
func SetBuildInfo(version, commit, goVersion string) {
	if version == "" {
		version = "unknown"
	}
	if commit == "" {
		commit = "unknown"
	}
	if goVersion == "" {
		goVersion = "unknown"
	}
	buildInfo.WithLabelValues(version, commit, goVersion).Set(1)
}

// SetPluginHealth 一次性更新某个 plugin 的健康状态。name 为空时 no-op。
func SetPluginHealth(name string, healthy bool) {
	if name == "" {
		return
	}
	v := 0.0
	if healthy {
		v = 1.0
	}
	pluginHealth.WithLabelValues(name).Set(v)
}

// SetDependencyUp 一次性更新某个外部依赖的可用性。
func SetDependencyUp(name string, up bool) {
	if name == "" {
		return
	}
	v := 0.0
	if up {
		v = 1.0
	}
	dependencyUp.WithLabelValues(name).Set(v)
}

// Middleware 返回一个 gin handler，用于自动记录 HTTP 指标。
// 用法：r.Use(metrics.Middleware())
//
// 注意：path 用 c.FullPath()，避免把 URL 参数（/users/123）当成不同的 label，
// 否则高基数会撑爆 Prometheus。
func Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		httpRequestsInFlight.Inc()
		defer httpRequestsInFlight.Dec()

		c.Next()

		// 在 c.Next() 之后取 path 与 status，确保拿到最终路由匹配结果。
		path := c.FullPath()
		if path == "" {
			// NoRoute 路径：c.FullPath() 为空，用占位符避免 cardinality 爆炸。
			path = "<nomatch>"
		}
		status := strconv.Itoa(c.Writer.Status())
		method := c.Request.Method

		httpRequestsTotal.WithLabelValues(method, path, status).Inc()
		httpRequestDuration.WithLabelValues(method, path).Observe(time.Since(start).Seconds())
	}
}

// Handler 返回 promhttp 的 /metrics handler，可直接挂到 gin 路由。
func Handler() http.Handler {
	return promhttp.Handler()
}
