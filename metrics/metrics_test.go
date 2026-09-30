package metrics

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestMetricsHandler_ExposesBuildInfo 验证 /metrics 端点输出包含 mgin_build_info 行。
func TestMetricsHandler_ExposesBuildInfo(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Middleware())
	r.GET("/metrics", gin.WrapH(Handler()))
	SetBuildInfo("v2.1.0", "abc123", "go1.25.0")

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/metrics", nil)
	r.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "mgin_build_info") {
		t.Errorf("expected mgin_build_info in metrics output, got: %s", body)
	}
	if !strings.Contains(body, `v2.1.0`) {
		t.Errorf("expected version label in metrics output, got: %s", body)
	}
}

// TestMiddleware_CountsRequests 验证中间件会记录 http_requests_total。
func TestMiddleware_CountsRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	// /metrics 路由必须先注册，否则抓取端点会走到 NoRoute（path label 为空）。
	r.Use(Middleware())
	r.GET("/metrics", gin.WrapH(Handler()))
	r.GET("/hello/:name", func(c *gin.Context) { c.Status(200) })

	// 触发 3 次请求
	for i := 0; i < 3; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/hello/world", nil)
		r.ServeHTTP(w, req)
	}

	// 通过 /metrics 端点抓取并断言
	w := httptest.NewRecorder()
	mreq := httptest.NewRequest("GET", "/metrics", nil)
	r.ServeHTTP(w, mreq)

	body := w.Body.String()
	// 路径必须是 /hello/:name（模板），而不是 /hello/world（高基数会爆）
	if !strings.Contains(body, `path="/hello/:name"`) {
		t.Errorf("expected path label to be /hello/:name template, got body:\n%s", body)
	}
	if !strings.Contains(body, "mgin_http_requests_total") {
		t.Errorf("expected mgin_http_requests_total metric")
	}
	if !strings.Contains(body, "mgin_http_request_duration_seconds") {
		t.Errorf("expected duration histogram")
	}
}

// TestSetPluginHealth 与 TestSetDependencyUp 验证辅助函数不会 panic。
func TestSetPluginHealth(t *testing.T) {
	SetPluginHealth("mysql", true)
	SetPluginHealth("redis", false)
	SetPluginHealth("", true) // 空名应该 no-op
	SetDependencyUp("mysql", true)
	SetDependencyUp("", false)
	// 没断言，验证不 panic 即可
}
