// Package main 演示服务间调用：用桩注册中心替代 Nacos，httptest 作为下游。
// 运行：go run ./call
package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/maczh/mgin/v2/pkg/client"
	"github.com/maczh/mgin/v2/pkg/registry"
)

// stubRegistry 是 registry.RegistryClient 的最小实现。
type stubRegistry struct {
	urls []string
}

func (s stubRegistry) Register([]byte)       {}
func (s stubRegistry) DeRegister()           {}
func (s stubRegistry) GetServiceURL(string, ...string) (string, string) {
	if len(s.urls) == 0 {
		return "", ""
	}
	return s.urls[0], "DEFAULT"
}
func (s stubRegistry) GetServices(string, ...string) ([]string, error) {
	return s.urls, nil
}

func main() {
	downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"code":1,"msg":"ok","data":{"id":7,"name":"demo"}}`)
	}))
	defer downstream.Close()

	registry.Registry = stubRegistry{urls: []string{downstream.URL}}

	body, err := client.CallResilientCtx(context.Background(), "demo-service", "/api/v1/orders/7",
		&client.Options{
			Method:       "GET",
			Protocol:     client.CONTENT_TYPE_JSON,
			LoadBalancer: "round_robin",
		},
		&client.ResilienceOptions{Timeout: 2 * time.Second, MaxRetries: 1})
	if err != nil {
		fmt.Println("call error:", err)
		return
	}
	fmt.Println("raw:", body)

	type order struct {
		Id   int    `json:"id"`
		Name string `json:"name"`
	}
	resp := client.CallT[order]("demo-service", "/api/v1/orders/7", &client.Options{
		Method: "GET", Protocol: client.CONTENT_TYPE_JSON,
	})
	fmt.Printf("parsed: code=%d msg=%s id=%d name=%s\n", resp.Code, resp.Msg, resp.Data.Id, resp.Data.Name)
}
