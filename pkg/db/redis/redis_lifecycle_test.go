package redis

import (
	"context"
	"io"
	"net"
	"testing"
	"time"

	redisapi "github.com/go-redis/redis/v7"
	"github.com/maczh/mgin/v2/pkg/utils"
)

func TestCloseMakesClientTerminal(t *testing.T) {
	client := &RedisClient{}
	client.Close()
	if _, err := client.GetConnection(); err != ErrClosed {
		t.Fatalf("expected ErrClosed, got %v", err)
	}
	if err := client.Check(); err != ErrClosed {
		t.Fatalf("expected ErrClosed, got %v", err)
	}
}

func TestPubSubStopsOnContextCancellation(t *testing.T) {
	client := &RedisClient{}
	ctx, cancel := context.WithCancel(context.Background())
	client.PSubscribeContext(ctx, "0", func(_ *redisapi.Message, _ string) {}, "events")

	waitForSubscriptionCount(t, client, 1)
	cancel()
	waitForSubscriptionCount(t, client, 0)
}

func TestCloseCancelsPubSubRetry(t *testing.T) {
	client := &RedisClient{}
	client.PSubscribeContext(context.Background(), "0", func(_ *redisapi.Message, _ string) {}, "events")
	waitForSubscriptionCount(t, client, 1)
	client.Close()
	waitForSubscriptionCount(t, client, 0)
}

func TestCloseCancelsPubSubHandshake(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	accepted := make(chan struct{})
	utils.GoSafe(func() {
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		defer conn.Close()
		close(accepted)
		_, _ = io.Copy(io.Discard, conn)
	})

	redisConn := redisapi.NewClient(&redisapi.Options{Addr: listener.Addr().String(), ReadTimeout: 0})
	client := &RedisClient{clients: map[string]redisapi.UniversalClient{"0": redisConn}}
	client.PSubscribeContext(context.Background(), "0", func(_ *redisapi.Message, _ string) {}, "events")
	waitForSubscriptionCount(t, client, 1)
	select {
	case <-accepted:
	case <-time.After(time.Second):
		t.Fatal("Pub/Sub did not open the expected handshake connection")
	}
	client.Close()
	waitForSubscriptionCount(t, client, 0)
}

func TestInitNetworkIODoesNotHoldClientStateLock(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	accepted := make(chan net.Conn, 1)
	utils.GoSafe(func() {
		conn, acceptErr := listener.Accept()
		if acceptErr == nil {
			accepted <- conn
		}
	})

	client := &RedisClient{}
	initDone := make(chan error, 1)
	utils.GoSafe(func() {
		initDone <- client.InitWithError([]byte("go:\n  data:\n    redis:\n      uri: " + listener.Addr().String() + "\n"))
	})
	var conn net.Conn
	select {
	case conn = <-accepted:
	case <-time.After(2 * time.Second):
		t.Fatal("Redis initialization did not reach the test endpoint")
	}
	defer conn.Close()

	checked := make(chan struct{})
	utils.GoSafe(func() {
		_, _ = client.GetConnection()
		close(checked)
	})
	select {
	case <-checked:
	case <-time.After(250 * time.Millisecond):
		t.Fatal("GetConnection blocked behind Redis network initialization")
	}
	client.Close()
	_ = conn.Close()
	select {
	case initErr := <-initDone:
		if initErr != ErrClosed {
			t.Fatalf("InitWithError after concurrent Close=%v, want ErrClosed", initErr)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Redis initialization did not exit after endpoint closed")
	}
}

func waitForSubscriptionCount(t *testing.T, client *RedisClient, want int) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		client.subMu.Lock()
		got := len(client.subscriptions)
		client.subMu.Unlock()
		if got == want {
			return
		}
		time.Sleep(time.Millisecond)
	}
	client.subMu.Lock()
	got := len(client.subscriptions)
	client.subMu.Unlock()
	t.Fatalf("subscriptions=%d, want %d", got, want)
}
