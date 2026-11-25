package redis

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"strings"
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

func TestPubSubHandlerCanCancelSubscription(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	utils.GoSafe(func() {
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		defer conn.Close()
		if !readUntilPSubscribe(conn) {
			return
		}
		_, _ = io.WriteString(conn, "*3\r\n$10\r\npsubscribe\r\n$6\r\nevents\r\n:1\r\n")
		time.Sleep(50 * time.Millisecond)
		_, _ = io.WriteString(conn, "*4\r\n$8\r\npmessage\r\n$6\r\nevents\r\n$6\r\nevents\r\n$6\r\nupdate\r\n")
		_, _ = io.Copy(io.Discard, conn)
	})

	conn := redisapi.NewClient(&redisapi.Options{Addr: listener.Addr().String(), ReadTimeout: 0})
	client := &RedisClient{clients: map[string]redisapi.UniversalClient{"0": conn}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	called := make(chan struct{}, 1)
	client.PSubscribeContext(ctx, "0", func(msg *redisapi.Message, _ string) {
		if msg.Payload == "update" {
			called <- struct{}{}
			cancel()
		}
	}, "events")
	select {
	case <-called:
	case <-time.After(2 * time.Second):
		t.Fatal("subscription handler did not receive the message")
	}
	waitForSubscriptionCount(t, client, 0)
	client.Close()
}

func TestPubSubReconnectsAfterServerDisconnect(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	accepted := make(chan struct{}, 2)
	utils.GoSafe(func() {
		for i := 0; i < 2; i++ {
			conn, acceptErr := listener.Accept()
			if acceptErr != nil {
				return
			}
			accepted <- struct{}{}
			if readUntilPSubscribe(conn) {
				_, _ = io.WriteString(conn, "*3\r\n$10\r\npsubscribe\r\n$6\r\nevents\r\n:1\r\n")
			}
			_ = conn.Close()
		}
	})

	conn := redisapi.NewClient(&redisapi.Options{Addr: listener.Addr().String(), ReadTimeout: 0})
	client := &RedisClient{clients: map[string]redisapi.UniversalClient{"0": conn}}
	ctx, cancel := context.WithCancel(context.Background())
	client.PSubscribeContext(ctx, "0", func(_ *redisapi.Message, _ string) {}, "events")
	waitForSubscriptionCount(t, client, 1)
	for i := 0; i < 2; i++ {
		select {
		case <-accepted:
		case <-time.After(3 * time.Second):
			cancel()
			client.Close()
			t.Fatalf("Pub/Sub did not establish connection attempt %d", i+1)
		}
	}
	cancel()
	waitForSubscriptionCount(t, client, 0)
	client.Close()
}

func readUntilPSubscribe(conn net.Conn) bool {
	reader := bufio.NewReader(conn)
	for {
		line, err := reader.ReadString('\n')
		if err != nil || !strings.HasPrefix(line, "*") {
			return false
		}
		var count int
		if _, err := fmt.Sscanf(line, "*%d", &count); err != nil {
			return false
		}
		parts := make([]string, 0, count)
		for i := 0; i < count; i++ {
			lengthLine, readErr := reader.ReadString('\n')
			if readErr != nil {
				return false
			}
			var length int
			if _, readErr = fmt.Sscanf(lengthLine, "$%d", &length); readErr != nil {
				return false
			}
			value := make([]byte, length+2)
			if _, readErr = io.ReadFull(reader, value); readErr != nil {
				return false
			}
			parts = append(parts, string(value[:length]))
		}
		if len(parts) > 0 && strings.EqualFold(parts[0], "psubscribe") {
			return true
		}
		if _, err = io.WriteString(conn, "+OK\r\n"); err != nil {
			return false
		}
	}
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
