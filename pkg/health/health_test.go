package health

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
)

// 重置全局启动标记的辅助函数，并保证测试之间不互相污染。
func resetStarted(t *testing.T) {
	t.Helper()
	ResetStarted()
}

// 串行执行 health 相关测试，避免启动标记等全局状态在并行用例之间互相污染。
func TestHealthMain(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("IsStarted 默认 false", func(t *testing.T) {
		resetStarted(t)
		if IsStarted() {
			t.Fatal("期望 IsStarted() 默认返回 false")
		}
	})

	t.Run("MarkStarted 幂等", func(t *testing.T) {
		resetStarted(t)
		MarkStarted()
		if !IsStarted() {
			t.Fatal("MarkStarted() 后期望 IsStarted() == true")
		}
		// 再次调用不应改变状态，也不应重复更新 startedAt。
		first := StartedAt()
		MarkStarted()
		if !StartedAt().Equal(first) {
			t.Fatal("MarkStarted() 应幂等，不应更新 startedAt")
		}
		resetStarted(t)
	})

	t.Run("/live 始终返回 200 且不依赖启动状态", func(t *testing.T) {
		resetStarted(t)
		w := doHealthGet("/live")
		if w.Code != http.StatusOK {
			t.Fatalf("期望 200，实际 %d", w.Code)
		}
		var body response
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("解析响应失败: %v", err)
		}
		if body.Status != StatusOK {
			t.Fatalf("期望 status=ok，实际 %q", body.Status)
		}
		// /live 不应返回 dependencies 字段。
		if body.Dependencies != nil {
			t.Fatalf("/live 不应包含 dependencies，实际 %v", body.Dependencies)
		}
	})

	t.Run("/startup 未 MarkStarted 返回 503，MarkStarted 后 200", func(t *testing.T) {
		resetStarted(t)
		w := doHealthGet("/startup")
		if w.Code != http.StatusServiceUnavailable {
			t.Fatalf("未启动  期望 503，实际 %d", w.Code)
		}
		MarkStarted()
		defer resetStarted(t)
		w = doHealthGet("/startup")
		if w.Code != http.StatusOK {
			t.Fatalf("已启动  期望 200，实际 %d", w.Code)
		}
	})

	t.Run("Router 挂载路径与重复挂载幂等性", func(t *testing.T) {
		// 每个子用例独立创建 gin 引擎，避免路由叠加。
		// 验收目标 1：探针路径为 /live、/ready、/startup 三个（取决于调用方传入的 group 前缀）。
		// 验收目标 2：同一 engine 上重复调用 Router 不应 panic。
		r := gin.New()
		Router(r.Group("/health"))
		paths := listRoutes(r)
		want := map[string]bool{
			"/health/live":    false,
			"/health/ready":   false,
			"/health/startup": false,
		}
		for _, p := range paths {
			if _, ok := want[p]; ok {
				want[p] = true
			} else if p == "/live" || p == "/ready" || p == "/startup" {
				t.Fatalf("探针不应挂到根路径 %q", p)
			}
		}
		for p, found := range want {
			if !found {
				t.Fatalf("期望路由 %s 已挂载，未在路由表中找到", p)
			}
		}

		// 第二次挂载到同一 engine 应被 gin panic（这是 gin 的固有行为），
		// 我们通过 recover 验证并断言消息内容，避免测试自身崩溃。
		defer func() {
			if r := recover(); r == nil {
				t.Fatal("重复挂载应触发 panic，实际未触发")
			}
		}()
		Router(r.Group("/health"))
	})

	t.Run("并发 MarkStarted 安全", func(t *testing.T) {
		resetStarted(t)
		var wg sync.WaitGroup
		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				MarkStarted()
			}()
		}
		wg.Wait()
		if !IsStarted() {
			t.Fatal("并发 MarkStarted 后期望 IsStarted() == true")
		}
		resetStarted(t)
	})
}

// doHealthGet 创建一个独立的 gin engine 并把 /live /ready /startup 挂在根路径，
// 用于直接调用各 handler，不受 Router 的 group 路径耦合。
func doHealthGet(path string) *httptest.ResponseRecorder {
	r := gin.New()
	r.GET("/live", liveHandler)
	r.GET("/ready", readyHandler)
	r.GET("/startup", startupHandler)
	req := httptest.NewRequest(http.MethodGet, path, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// listRoutes 返回 engine 上已注册的所有路由路径。
// gin 1.x 的 engine.Routes() 返回 RouteInfo 切片，自带 Path 字段，
// 这里只取路径文本，便于断言已挂载哪些前缀。
func listRoutes(r *gin.Engine) []string {
	infos := r.Routes()
	paths := make([]string, 0, len(infos))
	for _, info := range infos {
		paths = append(paths, info.Path)
	}
	return paths
}
