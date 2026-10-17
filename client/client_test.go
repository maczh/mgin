package client

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/maczh/mgin/registry"
)

type testRegistry struct{ host string }

func (r testRegistry) Register([]byte)                                  {}
func (r testRegistry) DeRegister()                                      {}
func (r testRegistry) GetServiceURL(string, ...string) (string, string) { return r.host, "" }

func TestTypedCalls(t *testing.T) {
	old := registry.Registry
	defer func() { registry.Registry = old }()
	body := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()
	registry.Registry = testRegistry{host: server.URL}
	type item struct {
		ID int `json:"id"`
	}
	options := func() *Options { return &Options{Method: "GET", Protocol: CONTENT_TYPE_JSON} }
	body = ` {"code":1,"msg":"ok","srvTime":"2026-10-17T10:00:00+08:00","data":{"id":7}} `
	got := CallT[item]("test", "/item", options())
	if got.Code != 1 || got.Msg != "ok" || got.Data.ID != 7 || got.SrvTime.IsZero() {
		t.Fatalf("response=%+v", got)
	}
	body = `{"code":1,"msg":"ok","srvTime":"2026-10-17T10:00:00+08:00","data":{"list":[{"id":7}],"total":20,"page":2,"pageSize":10}}`
	page := CallPageT[item]("test", "/page", options())
	if page.Code != 1 || page.Data == nil || len(page.Data.List) != 1 || page.Data.List[0].ID != 7 || page.Data.Total != 20 || page.Data.Page != 2 || page.Data.PageSize != 10 {
		t.Fatalf("page=%+v", page)
	}
	for _, response := range []string{"", "null", "[]", "<html>error</html>", `{"code":`, `{"code":1,"data":"wrong type"}`} {
		body = response
		if got := CallT[item]("test", "/item", options()); got.Code != -1 || got.Msg == "" {
			t.Errorf("invalid response accepted: %q", body)
		}
		if got := CallPageT[item]("test", "/page", options()); got.Code != -1 || got.Msg == "" {
			t.Errorf("invalid page accepted: %q", body)
		}
	}
	body = `{"code":403,"msg":"denied","data":null}`
	if got := CallT[item]("test", "/item", options()); got.Code != 403 || got.Msg != "denied" {
		t.Fatalf("business error=%+v", got)
	}
	if got := CallPageT[item]("test", "/page", options()); got.Code != 403 || got.Msg != "denied" || got.Data != nil {
		t.Fatalf("page business error=%+v", got)
	}
	registry.Registry = testRegistry{}
	if got := CallT[item]("test", "/item", options()); got.Code != -1 || got.SrvTime.Before(time.Now().Add(-time.Minute)) {
		t.Fatalf("call failure=%+v", got)
	}
	if got := CallPageT[item]("test", "/page", options()); got.Code != -1 || got.Data != nil {
		t.Fatalf("page call failure=%+v", got)
	}
}
