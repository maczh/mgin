// Package main 演示日志、指标与错误码。
// 运行：go run ./observability
package main

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"runtime"

	"github.com/maczh/mgin/v2/pkg/errcode"
	"github.com/maczh/mgin/v2/pkg/logs"
	"github.com/maczh/mgin/v2/pkg/metrics"
	"github.com/maczh/mgin/v2/pkg/utils"
)

func main() {
	logs.Info("示例启动 app={} go={}", "examples", runtime.Version())
	logs.Warn("这是一条告警 level={}", "warn")

	metrics.SetBuildInfo("1.0.0", "abc123", runtime.Version())
	metrics.SetDependencyUp("mysql", true)
	metrics.SetPluginHealth("redis", false)

	srv := httptest.NewServer(metrics.Handler())
	defer srv.Close()
	resp, err := http.Get(srv.URL)
	if err == nil {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		fmt.Println("metrics contains mgin_build_info:", contains(string(body), "mgin_build_info"))
	}

	def := errcode.New("order", 4101, 404, "order.not_found")
	errcode.RegisterDef(def)
	fmt.Printf("definition: code=%d http=%d key=%s module=%s\n",
		def.Code, def.HTTPStatus, def.MessageKey, def.Module)

	found := errcode.LookupDef(4101)
	fmt.Println("lookup http status:", found.HTTPStatus)
	fmt.Println("unknown code http status:", errcode.LookupDef(999999).HTTPStatus)

	// 错误链判断
	wrapped := fmt.Errorf("rsa failed: %w", utils.ErrDataLen)
	if errors.Is(wrapped, utils.ErrDataLen) {
		fmt.Println("errors.Is works for utils sentinel:", wrapped)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}
