package loadbalancer

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestRoundRobin(t *testing.T) {
	ins := []string{"a", "b", "c"}
	seen := map[string]int{}
	for i := 0; i < 9; i++ {
		got, err := RoundRobinLB.Pick(ins, "")
		if err != nil {
			t.Fatalf("RoundRobin Pick err=%v", err)
		}
		seen[got]++
	}
	for _, k := range ins {
		if seen[k] != 3 {
			t.Errorf("RoundRobin 期望每个实例被选 3 次，实际 %s=%d", k, seen[k])
		}
	}
}

func TestRandom(t *testing.T) {
	ins := []string{"a", "b", "c", "d"}
	seen := map[string]int{}
	for i := 0; i < 1000; i++ {
		got, err := RandomLB.Pick(ins, "")
		if err != nil {
			t.Fatalf("Random Pick err=%v", err)
		}
		seen[got]++
	}
	if len(seen) != 4 {
		t.Errorf("Random 1000 次应覆盖全部 4 实例，实际 %d", len(seen))
	}
}

func TestLeastConnections(t *testing.T) {
	// 用独立实例避免与其它测试共享全局 LB 状态。
	lc := &LeastConnections{}
	ins := []string{"a", "b", "c"}
	// 给 a 加 3 个活跃连接
	for i := 0; i < 3; i++ {
		dec, _ := lc.Inc("a")
		defer dec()
	}
	// 给 b 加 1 个
	dec, _ := lc.Inc("b")
	defer dec()
	// c 是 0 连接
	got, err := lc.Pick(ins, "")
	if err != nil {
		t.Fatalf("LC Pick err=%v", err)
	}
	if got != "c" {
		t.Errorf("LeastConnections 期望选 c，实际 %s", got)
	}
}

func TestConsistentHash(t *testing.T) {
	ins := []string{"a", "b", "c", "d"}
	// 同一 key 必须路由到同一实例
	for i := 0; i < 10; i++ {
		got, _ := ConsistentHashLB.Pick(ins, "session-123")
		want, _ := ConsistentHashLB.Pick(ins, "session-123")
		if got != want {
			t.Errorf("ConsistentHash 同一 key 应选同一实例，got=%s want=%s", got, want)
		}
	}
	// 不同 key 应能覆盖到不同实例（抽样检查，不强制要求全覆盖）
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		got, _ := ConsistentHashLB.Pick(ins, "key-"+string(rune('A'+i%26)))
		seen[got] = true
	}
	if len(seen) < 2 {
		t.Errorf("ConsistentHash 多 key 应覆盖多个实例，实际只覆盖 %d", len(seen))
	}
}

func TestEmptyInstances(t *testing.T) {
	if _, err := RoundRobinLB.Pick(nil, ""); err == nil {
		t.Error("空实例应返回错误")
	}
	if _, err := RandomLB.Pick([]string{}, ""); err == nil {
		t.Error("空实例应返回错误")
	}
}

func TestDefaultIsRoundRobin(t *testing.T) {
	if Default() != RoundRobinLB {
		t.Error("Default 应回退到 RoundRobinLB")
	}
}

func TestSetDefault(t *testing.T) {
	prev := Default()
	defer SetDefault(prev)
	SetDefault(RandomLB)
	if Default() != RandomLB {
		t.Error("SetDefault 后 Default() 应返回 RandomLB")
	}
	SetDefault(nil)
	if Default() != RoundRobinLB {
		t.Error("SetDefault(nil) 应回退到 RoundRobinLB")
	}
}

func TestRegisterGet(t *testing.T) {
	if Get("round_robin") != RoundRobinLB {
		t.Error("Get(round_robin) 应返回 RoundRobinLB")
	}
	if Get("nonexistent") != nil {
		t.Error("Get 未注册策略应返回 nil")
	}
}

func TestLeastConnectionsConcurrency(t *testing.T) {
	ins := []string{"a", "b", "c"}
	lc := &LeastConnections{}
	var wg sync.WaitGroup
	var counters atomic.Int64
	// 100 个 goroutine 各 Inc 然后 Pick
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			dec, _ := lc.Inc("a")
			defer dec()
			_, err := lc.Pick(ins, "")
			if err == nil {
				counters.Add(1)
			}
		}()
	}
	wg.Wait()
	if counters.Load() != 100 {
		t.Errorf("100 次并发 Pick 应全部成功，实际 %d", counters.Load())
	}
}
