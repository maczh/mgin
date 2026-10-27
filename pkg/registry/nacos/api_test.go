package nacos

import "testing"

func TestNacosAPIVersions(t *testing.T) {
	if got := namingURL("127.0.0.1", "8848", "v3"); got != "http://127.0.0.1:8848/nacos/v3/client/ns/instance" {
		t.Fatal(got)
	}
	host := `{"ip":"127.0.0.1","port":8080,"weight":1.0,"healthy":true,"enabled":true,"metadata":{}}`
	for version, body := range map[string]string{"v1": `{"hosts":[` + host + `]}`, "v2": `{"code":0,"data":{"hosts":[` + host + `]}}`, "v3": `{"code":0,"data":[` + host + `]}`} {
		var result InstanceResp
		if err := decodeInstances(version, []byte(body), &result); err != nil || len(result.Hosts) != 1 {
			t.Fatalf("%s: %+v %v", version, result, err)
		}
	}
}
