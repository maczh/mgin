package consul

import "testing"

func TestInstanceIDIncludesLogicalScope(t *testing.T) {
	left := getInstanceId("127.0.0.1", 8080, "svc", "", "jhris", "DEFAULT")
	right := getInstanceId("127.0.0.1", 8080, "svc", "", "iot", "DEFAULT")
	if left == right {
		t.Fatalf("different groups produced the same instance ID %q", left)
	}
}
