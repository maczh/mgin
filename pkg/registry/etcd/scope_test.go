package etcd

import "testing"

func TestScopedServiceKey(t *testing.T) {
	c := &EtcdClient{namespace: "finance", prefix: "mgin", cluster: "ledger", keyTemplate: "{namespace}/{prefix}/{group}/{cluster}/{service}/{instance}"}
	if got := c.serviceKey("bank", "account", "id"); got != "finance/mgin/bank/ledger/account/id" {
		t.Fatal(got)
	}
}
