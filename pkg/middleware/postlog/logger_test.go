package postlog

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/maczh/mgin/v2/pkg/logsink"
	"github.com/maczh/mgin/v2/pkg/models"
)

// 验证中间件确实把日志抛给了外部注册的 handler。
func TestRequestLoggerEmitsToHandler(t *testing.T) {
	logsink.Reset()
	defer logsink.Reset()

	var got atomic.Int64
	var uri atomic.Value
	if err := RegisterHandler(logsink.NewHandlerFunc("test", func(e *models.PostLog) error {
		got.Add(1)
		uri.Store(e.Uri)
		return nil
	})); err != nil {
		t.Fatalf("注册 handler 失败: %v", err)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(traceIdStub())
	r.Use(RequestLogger())
	r.GET("/ping", func(c *gin.Context) { c.String(http.StatusOK, "pong") })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/ping", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("响应码异常: %d", w.Code)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if got.Load() == 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if got.Load() != 1 {
		t.Fatalf("handler 应收到 1 条日志, 实际 %d", got.Load())
	}
	if u, _ := uri.Load().(string); u != "/ping" {
		t.Fatalf("日志 Uri 异常: %v", u)
	}
	logsink.Close(time.Second)
}

// 验证没有 handler 时中间件不报错、不阻塞。
func TestRequestLoggerWithoutHandler(t *testing.T) {
	logsink.Reset()
	defer logsink.Reset()

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestLogger())
	r.GET("/ping", func(c *gin.Context) { c.String(http.StatusOK, "pong") })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/ping", nil))
	if w.Code != http.StatusOK || w.Body.String() != "pong" {
		t.Fatalf("响应异常: %d %s", w.Code, w.Body.String())
	}
}

func traceIdStub() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Header.Set("X-Request-ID", "test-request-id")
		c.Next()
	}
}
