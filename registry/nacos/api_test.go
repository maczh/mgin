package nacos

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	stdjson "encoding/json"
	"strings"
	"testing"

	"github.com/knadh/koanf"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/rawbytes"
)

func TestHeartbeatParamsAcrossVersions(t *testing.T) {
	client := &NacosClient{
		ver: "v1",
		param: map[string]string{
			"serviceName": "svc", "groupName": "DEFAULT_GROUP", "namespaceId": "public",
			"ip": "127.0.0.1", "port": "8080", "clusterName": "DEFAULT",
			"metadata": "{}",
		},
	}
	params := heartbeatParams(client)
	var beat map[string]any
	if err := stdjson.Unmarshal([]byte(params["beat"]), &beat); err != nil {
		t.Fatalf("decode v1 beat: %v", err)
	}
	if beat["ip"] != "127.0.0.1" || beat["port"] != "8080" {
		t.Fatalf("v1 beat = %#v", beat)
	}

	client.ver = "v3"
	params = heartbeatParams(client)
	if params["heartBeat"] != "true" {
		t.Fatalf("v3 heartbeat flag = %q", params["heartBeat"])
	}
	if params["serviceName"] != "svc" {
		t.Fatalf("v3 serviceName = %q", params["serviceName"])
	}
}

func TestNacosVersionPaths(t *testing.T) {
	tests := map[string]string{
		"v1": "http://127.0.0.1:8848/nacos/v1/ns/instance",
		"2":  "http://127.0.0.1:8848/nacos/v2/ns/instance",
		"v3": "http://127.0.0.1:8848/nacos/v3/client/ns/instance",
	}
	for version, want := range tests {
		if got := namingURL("127.0.0.1", "8848", version); got != want {
			t.Errorf("%s path = %q, want %q", version, got, want)
		}
	}
}

func TestDecodeInstancesAcrossVersions(t *testing.T) {
	host := `{"ip":"127.0.0.1","port":8080,"weight":1,"healthy":true,"enabled":true,"serviceName":"DEFAULT_GROUP@@svc","metadata":{}}`
	tests := map[string]string{
		"v1": `{"hosts":[` + host + `]}`,
		"v2": `{"code":0,"message":"success","data":{"hosts":[` + host + `]}}`,
		"v3": `{"code":0,"message":"success","data":[` + host + `]}`,
	}
	for version, body := range tests {
		var result InstanceResp
		if err := decodeInstances(version, []byte(body), &result); err != nil {
			t.Fatalf("%s: %v", version, err)
		}
		if len(result.Hosts) != 1 || result.Hosts[0].Port != 8080 {
			t.Fatalf("%s result = %+v", version, result)
		}
	}
}

func TestMSEAuthorizeSignsNamingRequest(t *testing.T) {
	conf := koanf.New(".")
	if err := conf.Load(rawbytes.Provider([]byte("go:\n  nacos:\n    mse: true\n")), yaml.Parser()); err != nil {
		t.Fatalf("load conf: %v", err)
	}
	n := &NacosClient{conf: conf, accessKey: "ak", secretKey: "sk"}

	params := map[string]string{"serviceName": "svc", "groupName": "DEFAULT_GROUP"}
	n.authorize(params)
	if params["ak"] != "ak" {
		t.Fatalf("ak = %q", params["ak"])
	}
	if !strings.HasSuffix(params["data"], "@@DEFAULT_GROUP@@svc") {
		t.Fatalf("data = %q, want <timestamp>@@DEFAULT_GROUP@@svc", params["data"])
	}
	mac := hmac.New(sha1.New, []byte("sk"))
	_, _ = mac.Write([]byte(params["data"]))
	if want := base64.StdEncoding.EncodeToString(mac.Sum(nil)); params["signature"] != want {
		t.Fatalf("signature = %q, want %q", params["signature"], want)
	}

	// 服务名已带分组前缀时不能重复拼接。
	prefixed := map[string]string{"serviceName": "DEFAULT_GROUP@@svc", "groupName": "DEFAULT_GROUP"}
	n.authorize(prefixed)
	if strings.Contains(prefixed["data"], "DEFAULT_GROUP@@DEFAULT_GROUP") {
		t.Fatalf("data = %q, group prefix duplicated", prefixed["data"])
	}
}
