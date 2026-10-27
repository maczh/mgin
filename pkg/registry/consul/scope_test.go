package consul

import "testing"

func TestScopedInstanceID(t *testing.T) {
	if getInstanceId("127.0.0.1", 8080, "svc", "jhris") == getInstanceId("127.0.0.1", 8080, "svc", "iot") {
		t.Fatal("scope collision")
	}
}
