package sqlite

import (
	"path/filepath"
	"testing"
)

// TestSqliteReopensAfterClose is the regression test for the reload path:
// Close must not leave the client in a terminal state, because Reload closes
// resources and then calls Init again.
func TestSqliteReopensAfterClose(t *testing.T) {
	client := &Sqlite{}
	file := filepath.Join(t.TempDir(), "reopen.db")

	if err := client.InitWithError(file); err != nil {
		t.Fatalf("initial init: %v", err)
	}
	conn, err := client.GetConnection()
	if err != nil {
		t.Fatalf("initial GetConnection: %v", err)
	}
	if err := conn.Exec("CREATE TABLE reopen_probe (id INTEGER)").Error; err != nil {
		t.Fatalf("create table: %v", err)
	}

	client.Close()

	if err := client.InitWithError(file); err != nil {
		t.Fatalf("reopen after Close: %v", err)
	}
	conn, err = client.GetConnection()
	if err != nil {
		t.Fatalf("GetConnection after reopen: %v", err)
	}
	var count int64
	if err := conn.Raw("SELECT count(*) FROM reopen_probe").Scan(&count).Error; err != nil {
		t.Fatalf("query after reopen: %v", err)
	}
	if count != 0 {
		t.Fatalf("count = %d, want 0", count)
	}
}
