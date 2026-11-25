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
