package config

import (
	"strings"
	"testing"
)

func TestNacosConfigURLs(t *testing.T) {
	for version, path := range map[string]string{
		"v1": "/nacos/v1/cs/configs?",
		"v2": "/nacos/v2/cs/config?",
		"v3": "/nacos/v3/client/cs/config?",
	} {
		cfg := &config{App: app{Project: "project"}, Config: appConfig{Type: "nacos", Server: "http://127.0.0.1:8848", Env: "test", APIVersion: version}}
		if got := cfg.GetConfigUrl("app"); !strings.Contains(got, path) {
			t.Errorf("%s URL = %q", version, got)
		}
	}
}

func TestDecodeNacosConfigAcrossVersions(t *testing.T) {
	tests := map[string]string{
		"v1": "content",
		"v2": `{"code":0,"data":"content"}`,
		"v3": `{"code":0,"data":{"content":"content"}}`,
	}
	for version, body := range tests {
		if got := string(decodeNacosConfig(version, []byte(body))); got != "content" {
			t.Errorf("%s content = %q", version, got)
		}
	}
}
