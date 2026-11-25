package consul

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hashicorp/consul/api"
)

func TestGetServicesSkipsNilServiceEntries(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/v1/health/service/") {
			t.Errorf("unexpected Consul endpoint: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[null, {"Service":{"Address":"127.0.0.1","Port":8080,"Tags":["DEFAULT"]}}]`))
	}))
	defer server.Close()

	clientConfig := api.DefaultConfig()
	clientConfig.Address = strings.TrimPrefix(server.URL, "http://")
	apiClient, err := api.NewClient(clientConfig)
	if err != nil {
		t.Fatal(err)
	}
	client := &ConsulClient{client: apiClient, cluster: "DEFAULT", group: "DEFAULT_GROUP"}
	urls, err := client.GetServices("svc")
	if err != nil {
		t.Fatalf("GetServices: %v", err)
	}
	if len(urls) != 1 || urls[0] != "http://127.0.0.1:8080" {
		t.Fatalf("GetServices returned %v, want one valid instance", urls)
	}
}
