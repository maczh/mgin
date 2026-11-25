package mongo

import (
	"net"
	"testing"
	"time"

	"github.com/maczh/mgin/v2/pkg/utils"
)

func TestCloseMakesClientTerminal(t *testing.T) {
	client := &Mongodb{}
	client.Close()
	if _, err := client.GetConnection(); err != ErrClosed {
		t.Fatalf("expected ErrClosed, got %v", err)
	}
	if err := client.Check(); err != ErrClosed {
		t.Fatalf("expected ErrClosed, got %v", err)
	}
}

func TestMongoURIRequiresDatabaseAndPreservesExistingPath(t *testing.T) {
	if _, err := mongoURIWithDatabase("mongodb://localhost:27017", ""); err == nil {
		t.Fatal("expected missing database to fail")
	}
	got, err := mongoURIWithDatabase("mongodb://localhost:27017", "app")
	if err != nil || got != "mongodb://localhost:27017/app" {
		t.Fatalf("got %q, err=%v", got, err)
	}
	got, err = mongoURIWithDatabase("mongodb://localhost:27017/existing", "configured")
	if err != nil || got != "mongodb://localhost:27017/existing" {
		t.Fatalf("existing database path changed: got %q, err=%v", got, err)
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

	client := &Mongodb{}
	initDone := make(chan error, 1)
	uri := "mongodb://" + listener.Addr().String() + "/app"
	utils.GoSafe(func() {
		initDone <- client.InitWithError([]byte("go:\n  data:\n    mongodb:\n      uri: " + uri + "\n      db: app\n"))
	})
	var conn net.Conn
	select {
	case conn = <-accepted:
	case <-time.After(2 * time.Second):
		t.Fatal("MongoDB initialization did not reach the test endpoint")
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
		t.Fatal("GetConnection blocked behind MongoDB network initialization")
	}
	client.Close()
	_ = conn.Close()
	select {
	case initErr := <-initDone:
		if initErr != ErrClosed {
			t.Fatalf("InitWithError after concurrent Close=%v, want ErrClosed", initErr)
		}
	case <-time.After(12 * time.Second):
		t.Fatal("MongoDB initialization did not exit after endpoint closed")
	}
}
