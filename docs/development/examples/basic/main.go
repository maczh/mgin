// Package main 演示最小可运行服务：NewApp + 探针 + 指标 + 自定义路由。
// 运行：go run ./basic
package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/maczh/mgin/v2"
	"github.com/maczh/mgin/v2/pkg/health"
)

const appConfig = `
go:
  application:
    name: example
    port: 0
    debug: false
  config:
    used: ""
    env: dev
  logger:
    level: error
    out: console
  health:
    enabled: true
    autoStarted: true
`

func main() {
	dir, err := os.MkdirTemp("", "mgin-basic")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)

	cfg := filepath.Join(dir, "application.yml")
	if err := os.WriteFile(cfg, []byte(appConfig), 0o600); err != nil {
		panic(err)
	}

	app := mgin.NewApp(cfg, "example", "1.0.0", false)
	app.EnableHealth()
	app.EnableMetrics()
	app.Router.GET("/api/v1/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"msg": "pong"})
	})
	// 实际运行时由 app.Run() 在监听后自动标记（go.health.autoStarted=true）；
	// 这里用 httptest 不起 Run，因此手动标记，避免 /health/startup 一直 503。
	health.MarkStarted()

	// 用 httptest 启动，避免示例阻塞在 Run() 的信号等待上。
	srv := httptest.NewServer(app.Router)
	defer srv.Close()

	for _, path := range []string{"/health/live", "/health/ready", "/health/startup", "/api/v1/ping", "/metrics"} {
		resp, err := http.Get(srv.URL + path)
		if err != nil {
			fmt.Printf("%-18s ERROR %v\n", path, err)
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		fmt.Printf("%-18s %d %s\n", path, resp.StatusCode, truncate(string(body), 80))
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
