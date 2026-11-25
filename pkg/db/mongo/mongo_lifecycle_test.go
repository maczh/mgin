package mongo

import "testing"

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
