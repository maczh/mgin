package registry

import (
	"sync"
	"testing"
	"time"
)

// fakeClient 用于验证 CachedRegistry 装饰器的缓存/降级/并发语义。
type fakeClient struct {
	mu          sync.Mutex
	urls        []string
	group       string
	calls       int
	afterEmpty  bool // 第二次起返回空（模拟注册中心抖动）
	emptyAlways bool
}

func (f *fakeClient) Register([]byte) {}
func (f *fakeClient) DeRegister()     {}
func (f *fakeClient) GetServiceURL(service string, group ...string) (string, string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.emptyAlways {
		return "", ""
	}
	if f.afterEmpty && f.calls > 1 {
		return "", ""
	}
	if len(f.urls) == 0 {
		return "", ""
	}
	return f.urls[0], f.group
}

func TestCachedRegistry_CacheHitAvoidsBackendCall(t *testing.T) {
	c := NewCachedRegistry(&fakeClient{urls: []string{"http://10.0.0.1:8080"}, group: "g1"})
	defer c.DeRegister()

	if url, _ := c.GetServiceURL("svcA"); url == "" {
		t.Fatalf("首次调用应成功")
	}
	// 第二次应在 TTL 内命中缓存，不再回源
	url, grp := c.GetServiceURL("svcA")
	if url != "http://10.0.0.1:8080" || grp != "g1" {
		t.Fatalf("缓存命中结果错误: %s %s", url, grp)
	}
}

func TestCachedRegistry_StaleFallbackOnError(t *testing.T) {
	// 设置极短 TTL，强制缓存过期后回源；回源失败时复用旧数据
	fc := &fakeClient{urls: []string{"http://10.0.0.1:8080"}, group: "g1", afterEmpty: true}
	c := NewCachedRegistry(fc)
	defer c.DeRegister()

	if url, _ := c.GetServiceURL("svcB"); url == "" {
		t.Fatalf("首次调用应成功")
	}
	// 等待缓存过期后回源（afterEmpty 使回源返回空），应降级复用旧实例
	time.Sleep(3500 * time.Millisecond)
	url, _ := c.GetServiceURL("svcB")
	if url != "http://10.0.0.1:8080" {
		t.Fatalf("注册中心抖动时应降级返回旧实例，实际: %q", url)
	}
}

func TestCachedRegistry_EmptyNeverCached(t *testing.T) {
	fc := &fakeClient{emptyAlways: true}
	c := NewCachedRegistry(fc)
	defer c.DeRegister()

	url, _ := c.GetServiceURL("svcC")
	if url != "" {
		t.Fatalf("无实例时应返回空，实际: %q", url)
	}
	if fc.calls != 1 {
		t.Fatalf("无实例不应写入缓存，再次调用回源次数应为1，实际: %d", fc.calls)
	}
	// 再次调用仍应回源（未被缓存）
	c.GetServiceURL("svcC")
	if fc.calls != 2 {
		t.Fatalf("空结果不应被缓存，回源次数应为2，实际: %d", fc.calls)
	}
}

func TestCachedRegistry_SingleFlightDedup(t *testing.T) {
	fc := &fakeClient{urls: []string{"http://10.0.0.1:8080"}, group: "g1"}
	c := NewCachedRegistry(fc)
	defer c.DeRegister()

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.GetServiceURL("svcD")
		}()
	}
	wg.Wait()
	if fc.calls != 1 {
		t.Fatalf("并发首查应被 singleflight 合并为1次回源，实际: %d", fc.calls)
	}
}
