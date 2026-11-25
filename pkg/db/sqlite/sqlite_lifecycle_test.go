package sqlite

import "testing"

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
