package mongo

import (
	"errors"
	"net"
	"reflect"
	"testing"
	"time"

	"github.com/maczh/mgin/v2/pkg/utils"
	"github.com/maczh/mgo"
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

func TestMongoCheckReconnectsAndPublishesReplacement(t *testing.T) {
	oldSession := &mgo.Session{}
	newSession := &mgo.Session{}
	client := &Mongodb{
		max: 12,
		conns: map[string]connection{
			"0": {conn: oldSession, db: "app", url: "mongodb://localhost/app"},
		},
	}
	closed := make([]*mgo.Session, 0, 2)
	err := client.checkAndReconnectWith("0",
		func(session *mgo.Session) error {
			if session != oldSession {
				t.Fatalf("Ping session=%p, want old session %p", session, oldSession)
			}
			return errors.New("connection lost")
		},
		func(uri string, timeout time.Duration, max int) (*mgo.Session, error) {
			if uri != "mongodb://localhost/app" || timeout != 10*time.Second || max != 12 {
				t.Fatalf("Dial args=(%q, %v, %d)", uri, timeout, max)
			}
			return newSession, nil
		},
		func(session *mgo.Session) { closed = append(closed, session) },
	)
	if err != nil {
		t.Fatalf("checkAndReconnectWith: %v", err)
	}
	if got := client.conns["0"]; got.conn != newSession || got.db != "app" || got.url != "mongodb://localhost/app" {
		t.Fatalf("published connection=%+v", got)
	}
	if !reflect.DeepEqual(closed, []*mgo.Session{oldSession}) {
		t.Fatalf("closed sessions=%v, want only old session", closed)
	}
}

func TestMongoReconnectFailureKeepsCurrentConnection(t *testing.T) {
	oldSession := &mgo.Session{}
	client := &Mongodb{conns: map[string]connection{"0": {conn: oldSession, url: "mongodb://localhost/app"}}}
	dialErr := errors.New("database unavailable")
	closed := false
	err := client.checkAndReconnectWith("0", func(*mgo.Session) error { return errors.New("ping failed") },
		func(string, time.Duration, int) (*mgo.Session, error) { return nil, dialErr },
		func(*mgo.Session) { closed = true })
	if !errors.Is(err, dialErr) {
		t.Fatalf("reconnect error=%v, want %v", err, dialErr)
	}
	if client.conns["0"].conn != oldSession || closed {
		t.Fatal("failed reconnect must preserve the current session")
	}
}

func TestRepeatedInitPreservesPublishedTags(t *testing.T) {
	want := []string{"orders", "users"}
	client := &Mongodb{
		conns: map[string]connection{
			"orders": {},
			"users":  {},
		},
		tags:     append([]string(nil), want...),
		confData: []byte("go:\n  data:\n    mongodb:\n      multidb: true\n"),
	}
	if err := client.InitWithError(nil); err != nil {
		t.Fatalf("repeated InitWithError: %v", err)
	}
	if !reflect.DeepEqual(client.tags, want) {
		t.Fatalf("repeated init changed published tags: got %v, want %v", client.tags, want)
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
