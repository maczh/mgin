package etcd

import "testing"

func TestServiceKeyIncludesConfiguredScope(t *testing.T) {
	c := &EtcdClient{namespace: "finance", prefix: "mgin", group: "bank", cluster: "ledger", keyTemplate: "{namespace}/{prefix}/{group}/{cluster}/{service}/{instance}"}
	if got, want := c.serviceKey("bank", "account", "id"), "finance/mgin/bank/ledger/account/id"; got != want {
		t.Fatalf("service key = %q, want %q", got, want)
	}
	if got, want := c.watchPrefix(), "finance/mgin/"; got != want {
		t.Fatalf("watch prefix = %q, want %q", got, want)
	}
	service, group, ok := c.scopeFromKey("finance/mgin/bank/ledger/account/id")
	if !ok || service != "account" || group != "bank" {
		t.Fatalf("scope = %q, %q, %v", service, group, ok)
	}
}
