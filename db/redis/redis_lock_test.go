package redis

import (
	"fmt"
	"net"
	"testing"
	"time"
)

// TestInitDoesNotBlockReaders reproduces the audit's black-hole probe: while
// Init is stuck dialing/pinging, read-only accessors must return immediately.
func TestInitDoesNotBlockReaders(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	accepted := make(chan net.Conn, 1)
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			accepted <- conn
		}
	}()

	cfg := []byte(fmt.Sprintf("go:\n  data:\n    redis:\n      uri: %s\n    redis_pool:\n      timeout: 3\n", ln.Addr().String()))
	client := &RedisClient{}
	done := make(chan struct{})
	go func() {
		client.Init(cfg)
		close(done)
	}()

	select {
	case conn := <-accepted:
		defer conn.Close()
	case <-time.After(time.Second):
		t.Fatal("Init never dialed the test listener")
	}

	// Give Init time to enter Ping with the connection established but silent.
	time.Sleep(50 * time.Millisecond)
	start := time.Now()
	_ = client.IsMultiDB()
	_ = client.ListConnNames()
	if elapsed := time.Since(start); elapsed > 200*time.Millisecond {
		t.Fatalf("readers blocked for %v while Init was dialing", elapsed)
	}

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("Init did not finish")
	}
}
