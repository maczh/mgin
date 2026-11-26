package mysql

import (
	"errors"
	"testing"
)

// TestMysqlInitWithErrorAfterCloseReportsCandidateError documents the offline
// behaviour of reopening: the candidate connection is built before the new
// snapshot is published, so an unreachable/invalid DSN fails with its own
// error rather than ErrClosed. Successful reopen needs a live server and is
// covered without external services by db/sqlite's reopen test.
func TestMysqlInitWithErrorAfterCloseReportsCandidateError(t *testing.T) {
	client := &MysqlClient{}
	client.Close()
	if err := client.InitWithError([]byte("go:\n  data:\n    mysql: test\n")); err == nil || errors.Is(err, ErrClosed) {
		t.Fatalf("InitWithError after Close error = %v, want a configuration/DSN error", err)
	}
	if _, err := client.GetConnection(); !errors.Is(err, ErrClosed) {
		t.Fatalf("GetConnection after Close error = %v, want %v", err, ErrClosed)
	}
}

func TestMysqlInitWithErrorRejectsEmptyConfiguration(t *testing.T) {
	client := &MysqlClient{}
	if err := client.InitWithError(nil); err == nil {
		t.Fatal("InitWithError(nil) unexpectedly succeeded")
	}
}
