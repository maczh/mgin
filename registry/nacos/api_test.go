package nacos

import "testing"

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
