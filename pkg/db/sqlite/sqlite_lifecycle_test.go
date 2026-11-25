package sqlite

import (
	"errors"
	"testing"

	sqlitedriver "gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestNilAndUninitializedConnectionAreErrors(t *testing.T) {
	var client *Sqlite
	if _, err := client.GetConnection(); err == nil {
		t.Fatal("expected nil receiver error")
	}
	client = &Sqlite{}
	if _, err := client.GetConnection(); err == nil {
		t.Fatal("expected unopened connection error")
	}
	if err := client.Check(); err == nil {
		t.Fatal("expected unopened check error")
	}
}

func TestCloseIsTerminal(t *testing.T) {
	client := &Sqlite{}
	client.Close()
	if err := client.InitWithError(""); err != ErrClosed {
		t.Fatalf("expected ErrClosed from Init after Close, got %v", err)
	}
	if _, err := client.GetConnection(); err != ErrClosed {
		t.Fatalf("expected ErrClosed from GetConnection after Close, got %v", err)
	}
	if err := client.Check(); err != ErrClosed {
		t.Fatalf("expected ErrClosed from Check after Close, got %v", err)
	}
}

func TestCloseClosesUnderlyingDatabaseEvenWhenGormHasError(t *testing.T) {
	conn, err := gorm.Open(sqlitedriver.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := conn.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := sqlDB.Ping(); err != nil {
		t.Fatal(err)
	}
	conn.Error = errors.New("previous query failed")
	client := &Sqlite{sqlite: conn}
	client.Close()
	if err := sqlDB.Ping(); err == nil {
		t.Fatal("underlying database remains open after Close")
	}
}
