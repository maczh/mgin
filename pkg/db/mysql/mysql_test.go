package mysql

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/maczh/mgin/v2/pkg/utils"
	"gorm.io/gorm"
)

var _ interface {
	Init([]byte)
	Close()
	Check() error
} = (*MysqlClient)(nil)

func TestInitFailureDoesNotPublishConnection(t *testing.T) {
	client := &MysqlClient{}
	if err := client.InitWithError([]byte("go:\n  data:\n    mysql: user:pass@tcp(127.0.0.1:1)/mgin?timeout=100ms\n")); err == nil {
		t.Fatal("expected initialization error")
	}

	conn, err := client.GetConnection()
	if err == nil {
		t.Fatal("expected connection error after failed initialization")
	}
	if conn != nil {
		t.Fatal("failed initialization published a connection")
	}
}

func TestCheckFailureDoesNotCloseConnectionPool(t *testing.T) {
	gdb := newScriptedGormDB(t, errors.New("ping failed"))
	client := &MysqlClient{mysql: gdb}

	if err := client.Check(); err == nil {
		t.Fatal("expected health check error")
	}
	conn, err := client.GetConnection()
	if err != nil {
		t.Fatalf("health check closed the shared pool: %v", err)
	}
	if conn == nil {
		t.Fatal("expected the existing connection after a transient ping failure")
	}
}

func TestClosedClientDoesNotReconnect(t *testing.T) {
	client := &MysqlClient{confData: []byte("go: [")}
	client.Close()

	conn, err := client.GetConnection()
	if err == nil {
		t.Fatal("expected closed client error")
	}
	if err.Error() != "mysql client is closed" {
		t.Fatalf("closed client attempted to reconnect: %v", err)
	}
	if conn != nil {
		t.Fatal("closed client returned a connection")
	}
}

func TestConcurrentConnectionAccessAndClose(t *testing.T) {
	gdb := newScriptedGormDB(t)
	client := &MysqlClient{mysql: gdb}
	start := make(chan struct{})
	errs := make(chan error, 32)
	var wg sync.WaitGroup

	for i := 0; i < 32; i++ {
		worker := i
		wg.Add(1)
		utils.GoSafe(func() {
			defer wg.Done()
			<-start
			for j := 0; j < 100; j++ {
				if worker%2 == 0 {
					conn, err := client.GetConnection()
					if err != nil && !errors.Is(err, ErrClosed) {
						errs <- err
						return
					}
					if err == nil && conn == nil {
						errs <- errors.New("successful GetConnection returned nil")
						return
					}
					continue
				}
				if err := client.Check(); err != nil && !errors.Is(err, ErrClosed) {
					errs <- err
					return
				}
			}
		})
	}

	close(start)
	client.Close()
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	if _, err := client.GetConnection(); !errors.Is(err, ErrClosed) {
		t.Fatalf("expected ErrClosed after concurrent shutdown, got %v", err)
	}
}

func TestMultiDBCheckFailureDoesNotCloseConnectionPool(t *testing.T) {
	gdb := newScriptedGormDB(t, errors.New("ping failed"))
	client := &MysqlClient{
		multi:  true,
		mysqls: map[string]*gorm.DB{"one": gdb},
		conns:  []string{"one"},
	}
	t.Cleanup(client.Close)

	if err := client.Check(); err == nil {
		t.Fatal("expected multi-database health check error")
	}
	if err := client.Check(); err != nil {
		t.Fatalf("health check closed the multi-database pool: %v", err)
	}
}

func TestMultiDBInitFailureDoesNotPublishConnections(t *testing.T) {
	client := &MysqlClient{}
	data := []byte(`go:
  data:
    mysql:
      multidb: true
      dbNames: "one,two"
      one: user:pass@tcp(127.0.0.1:1)/one?timeout=100ms
      two: user:pass@tcp(127.0.0.1:1)/two?timeout=100ms
`)
	if err := client.InitWithError(data); err == nil {
		t.Fatal("expected multi-database initialization error")
	}
	if names := client.ListConnNames(); len(names) != 0 {
		t.Fatalf("failed initialization published connections: %v", names)
	}
	conn, err := client.GetConnection("one")
	if err == nil {
		t.Fatal("expected connection error after failed multi-database initialization")
	}
	if conn != nil {
		t.Fatal("failed multi-database initialization returned a connection")
	}
}

func TestInitFailureKeepsExistingConnection(t *testing.T) {
	existing := newScriptedGormDB(t)
	client := &MysqlClient{
		mysql: existing,
		openConn: func(string) (*gorm.DB, error) {
			return nil, errors.New("replacement failed")
		},
	}

	if err := client.InitWithError([]byte("go:\n  data:\n    mysql: replacement\n")); err == nil {
		t.Fatal("expected replacement initialization error")
	}
	conn, err := client.GetConnection()
	if err != nil {
		t.Fatalf("failed replacement closed the existing pool: %v", err)
	}
	if conn != existing {
		t.Fatal("failed replacement changed the existing connection")
	}
}

func TestInitReplacesAndClosesPreviousConnection(t *testing.T) {
	existing := newScriptedGormDB(t)
	existingSQL, err := existing.DB()
	if err != nil {
		t.Fatal(err)
	}
	replacement := newScriptedGormDB(t)
	client := &MysqlClient{
		mysql: existing,
		openConn: func(string) (*gorm.DB, error) {
			return replacement, nil
		},
	}
	t.Cleanup(client.Close)

	if err := client.InitWithError([]byte("go:\n  data:\n    mysql: replacement\n")); err != nil {
		t.Fatalf("initialize replacement: %v", err)
	}
	if err := existingSQL.Ping(); err == nil {
		t.Fatal("expected the replaced connection pool to be closed")
	}
	conn, err := client.GetConnection()
	if err != nil {
		t.Fatalf("get replacement connection: %v", err)
	}
	if conn != replacement {
		t.Fatal("expected the replacement connection to be published")
	}
}

func TestInitRejectsUninitializedConnection(t *testing.T) {
	client := &MysqlClient{
		openConn: func(string) (*gorm.DB, error) {
			return &gorm.DB{}, nil
		},
	}

	if err := client.InitWithError([]byte("go:\n  data:\n    mysql: invalid\n")); err == nil {
		t.Fatal("expected an error for an uninitialized GORM connection")
	}
	if client.mysql != nil {
		t.Fatal("uninitialized connection was published")
	}
}

func TestCloseDoesNotWaitForInitialization(t *testing.T) {
	existing := newScriptedGormDB(t)
	replacement := newScriptedGormDB(t)
	started := make(chan struct{})
	unblock := make(chan struct{})
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(unblock) }) }
	t.Cleanup(release)
	client := &MysqlClient{
		mysql: existing,
		openConn: func(string) (*gorm.DB, error) {
			close(started)
			<-unblock
			return replacement, nil
		},
	}

	initDone := make(chan error, 1)
	utils.GoSafe(func() {
		initDone <- client.InitWithError([]byte("go:\n  data:\n    mysql: replacement\n"))
	})
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("initialization did not start")
	}

	closeDone := make(chan struct{})
	utils.GoSafe(func() {
		client.Close()
		close(closeDone)
	})
	select {
	case <-closeDone:
	case <-time.After(time.Second):
		release()
		<-closeDone
		<-initDone
		t.Fatal("Close waited for network initialization")
	}

	release()
	if err := <-initDone; !errors.Is(err, ErrClosed) {
		t.Fatalf("expected initialization to stop after Close, got %v", err)
	}
	replacementSQL, err := replacement.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := replacementSQL.Ping(); err == nil {
		t.Fatal("expected the unpublished replacement pool to be closed")
	}
}

func TestMultiDBInitFailureClosesOpenedConnections(t *testing.T) {
	opened := newScriptedGormDB(t)
	openedSQL, err := opened.DB()
	if err != nil {
		t.Fatal(err)
	}
	client := &MysqlClient{
		openConn: func(dsn string) (*gorm.DB, error) {
			if dsn == "one" {
				return opened, nil
			}
			return nil, errors.New("open failed")
		},
	}
	data := []byte(`go:
  data:
    mysql:
      multidb: true
      dbNames: "one,two"
      one: one
      two: two
`)
	if err := client.InitWithError(data); err == nil {
		t.Fatal("expected multi-database initialization error")
	}
	if err := openedSQL.Ping(); err == nil {
		t.Fatal("expected the partially opened connection pool to be closed")
	}
	if names := client.ListConnNames(); len(names) != 0 {
		t.Fatalf("failed initialization published connections: %v", names)
	}
}

func TestMultiDBInitFailureClosesInvalidConnection(t *testing.T) {
	opened := newScriptedGormDB(t)
	invalid := newScriptedGormDB(t)
	invalidSQL, err := invalid.DB()
	if err != nil {
		t.Fatal(err)
	}
	invalid.Statement = nil
	client := &MysqlClient{
		openConn: func(dsn string) (*gorm.DB, error) {
			if dsn == "one" {
				return opened, nil
			}
			return invalid, nil
		},
	}
	data := []byte(`go:
  data:
    mysql:
      multidb: true
      dbNames: "one,two"
      one: one
      two: two
`)
	if err := client.InitWithError(data); err == nil {
		t.Fatal("expected multi-database initialization error")
	}
	if err := invalidSQL.Ping(); err == nil {
		t.Fatal("expected the invalid connection pool to be closed")
	}
}

func TestMultiDBDuplicateNamesOpenOnce(t *testing.T) {
	var opens atomic.Int32
	client := &MysqlClient{
		openConn: func(string) (*gorm.DB, error) {
			opens.Add(1)
			return newScriptedGormDB(t), nil
		},
	}
	t.Cleanup(client.Close)
	data := []byte(`go:
  data:
    mysql:
      multidb: true
      dbNames: "one,one"
      one: one
`)
	if err := client.InitWithError(data); err != nil {
		t.Fatalf("initialize duplicate database names: %v", err)
	}
	if got := opens.Load(); got != 1 {
		t.Fatalf("expected one connection for duplicate names, got %d", got)
	}
	if names := client.ListConnNames(); len(names) != 1 || names[0] != "one" {
		t.Fatalf("unexpected connection names: %v", names)
	}
}

var scriptedDriverID atomic.Uint64

type scriptedPingDriver struct {
	mu      sync.Mutex
	results []error
}

func (d *scriptedPingDriver) Open(string) (driver.Conn, error) {
	return &scriptedPingConn{owner: d}, nil
}

func (d *scriptedPingDriver) ping() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.results) == 0 {
		return nil
	}
	err := d.results[0]
	d.results = d.results[1:]
	return err
}

type scriptedPingConn struct {
	owner *scriptedPingDriver
}

func (c *scriptedPingConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("prepare is not supported")
}

func (c *scriptedPingConn) Close() error { return nil }

func (c *scriptedPingConn) Begin() (driver.Tx, error) {
	return nil, errors.New("transactions are not supported")
}

func (c *scriptedPingConn) Ping(context.Context) error { return c.owner.ping() }

func newScriptedGormDB(t *testing.T, pingResults ...error) *gorm.DB {
	t.Helper()
	name := fmt.Sprintf("mgin-mysql-test-%d", scriptedDriverID.Add(1))
	sql.Register(name, &scriptedPingDriver{results: pingResults})
	sqlDB, err := sql.Open(name, "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return &gorm.DB{
		Config:    &gorm.Config{ConnPool: sqlDB},
		Statement: &gorm.Statement{ConnPool: sqlDB},
	}
}
