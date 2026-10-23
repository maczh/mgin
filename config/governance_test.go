package config

import (
	"testing"
)

func TestRevisionStoreBoundedAndRollback(t *testing.T) {
	s := NewRevisionStore(2)
	s.Record([]byte("one"), "test")
	s.Record([]byte("two"), "test")
	last := s.Record([]byte("three"), "test")
	if len(s.List()) != 2 {
		t.Fatalf("expected bounded history, got %d", len(s.List()))
	}
	if _, ok := s.Get(1); ok {
		t.Fatal("old revision should be evicted")
	}
	snapshot, err := s.Rollback(last.Version)
	if err != nil || string(snapshot) != "three" {
		t.Fatalf("rollback = %q, %v", snapshot, err)
	}
}

func TestRevisionStoreCopiesSnapshots(t *testing.T) {
	s := NewRevisionStore(1)
	input := []byte("immutable")
	r := s.Record(input, "test")
	input[0] = 'X'
	if got, _ := s.Rollback(r.Version); string(got) != "immutable" {
		t.Fatal("store retained caller buffer")
	}
}
