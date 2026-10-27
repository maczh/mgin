package registry

import (
	"testing"

	"github.com/maczh/mgin/v2/pkg/config"
	"gopkg.in/yaml.v3"
)

type namedFake struct{ url string }

func (*namedFake) Register([]byte)                                    {}
func (f *namedFake) GetServiceURL(string, ...string) (string, string) { return f.url, "group" }
func (f *namedFake) GetServices(string, ...string) ([]string, error)  { return []string{f.url}, nil }
func (*namedFake) DeRegister()                                        {}

func TestRegistryCollectionRoutesGetServices(t *testing.T) {
	c := &RegistryCollection{entries: map[string]registryEntry{"a": {client: &namedFake{url: "a"}}, "b": {client: &namedFake{url: "b"}}}, defaultName: "a"}
	if urls, _ := c.GetServices("svc"); len(urls) != 1 || urls[0] != "a" {
		t.Fatalf("default urls=%v", urls)
	}
	if urls, _ := c.GetServicesFrom("b", "svc"); len(urls) != 1 || urls[0] != "b" {
		t.Fatalf("named urls=%v", urls)
	}
}

func TestApplyRegistryOverrides(t *testing.T) {
	data, err := applyRegistryOverrides([]byte("go:\n  nacos:\n    server: 127.0.0.1\n"), "nacos", config.RegistryConfig{Namespace: "finance", Group: "bank", ClusterName: "ledger"})
	if err != nil {
		t.Fatal(err)
	}
	var root map[string]any
	if err := yaml.Unmarshal(data, &root); err != nil {
		t.Fatal(err)
	}
	backend := root["go"].(map[string]any)["nacos"].(map[string]any)
	if backend["namespace"] != "finance" || backend["group"] != "bank" || backend["clusterName"] != "ledger" {
		t.Fatalf("config=%v", backend)
	}
}
