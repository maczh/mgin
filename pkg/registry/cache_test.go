package registry

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/maczh/mgin/v2/pkg/config"
)

// fakeClient 是一个可观测的 RegistryClient 桩，记录 GetServices 调用次数与返回结果。
type fakeClient struct {
	mu         sync.Mutex
	calls      int
	urls       []string
	err        error
	afterEmpty bool // 第二次调用起返回错误（用于测试降级）
}

func (f *fakeClient) Register([]byte)             {}
func (f *fakeClient) DeRegister()                {}
func (f *fakeClient) GetServiceURL(string, ...string) (string, string) {
	return "", ""
}
func (f *fakeClient) GetServices(string, ...string) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.afterEmpty && f.calls > 1 {
		return nil, errors.New("registry down")
	}
	return f.urls, f.err
}

func TestCachedRegistry_CacheHit(t *testing.T) {
	config.Config.Discovery.CacheTTL = 60 // 大 TTL，保证后续调用命中缓存
	fc := &fakeClient{urls: []string{"http://10.0.0.1:8080"}}
	c := NewCachedRegistry(fc)
	defer c.DeRegister()

	for i := 0; i < 5; i++ {
		urls, err := c.GetServices("svcA")
		if err != nil || len(urls) != 1 {
			t.Fatalf("call %d: unexpected result urls=%v err=%v", i, urls, err)
		}
	}
	if fc.calls != 1 {
		t.Fatalf("期望回源 1 次，实际 %d 次（缓存未生效）", fc.calls)
	}
}

func TestCachedRegistry_StaleFallbackOnError(t *testing.T) {
	// 用极短 TTL 强制缓存过期，使下一次读取必须回源；此时 fake 报错但已有旧数据，应降级返回旧数据。
	config.Config.Discovery.CacheTTL = 1
	fc := &fakeClient{urls: []string{"http://10.0.0.1:8080"}, afterEmpty: true}
	c := NewCachedRegistry(fc)
	defer c.DeRegister()

	// 第一次成功，填充缓存
	if _, err := c.GetServices("svcB"); err != nil {
		t.Fatalf("首次调用应成功: %v", err)
	}
	// 等待缓存过期（TTL=1s），下一次读取回源将失败
	time.Sleep(1200 * time.Millisecond)
	urls, err := c.GetServices("svcB")
	if err != nil {
		t.Fatalf("注册中心抖动时应降级返回旧数据，但报错: %v", err)
	}
	if len(urls) != 1 || urls[0] != "http://10.0.0.1:8080" {
		t.Fatalf("降级应返回旧实例列表，实际 %v", urls)
	}
	// 必须确实回源过（第二次回源失败被降级吞掉），验证未走缓存命中
	if fc.calls < 2 {
		t.Fatalf("期望至少回源 2 次（首次成功 + 过期后降级回源），实际 %d 次", fc.calls)
	}
}

func TestCachedRegistry_ErrorWhenEmpty(t *testing.T) {
	config.Config.Discovery.CacheTTL = 60
	fc := &fakeClient{err: errors.New("registry unreachable")}
	c := NewCachedRegistry(fc)
	defer c.DeRegister()

	urls, err := c.GetServices("svcC")
	if err == nil {
		t.Fatalf("注册中心完全不可达且无缓存时应返回错误")
	}
	if len(urls) != 0 {
		t.Fatalf("无数据时应返回空列表")
	}
}

func TestCachedRegistry_StopOnDeRegister(t *testing.T) {
	config.Config.Discovery.CacheTTL = 60
	fc := &fakeClient{urls: []string{"http://10.0.0.1:8080"}}
	c := NewCachedRegistry(fc)
	// 多次刷新循环后关闭，确保不泄漏、不 panic
	c.DeRegister()
	c.DeRegister() // 幂等，不应 panic
}
