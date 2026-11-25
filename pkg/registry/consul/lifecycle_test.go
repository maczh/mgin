package consul

import "testing"

func TestTTLUpdatePanicIsReportedAndDoesNotEscapeLoopBoundary(t *testing.T) {
	if err := safeUpdateTTL(func() error { panic("injected TTL panic") }); err == nil {
		t.Fatal("expected recovered TTL panic to be reported")
	}
	called := false
	if err := safeUpdateTTL(func() error { called = true; return nil }); err != nil {
		t.Fatalf("subsequent TTL update: %v", err)
	}
	if !called {
		t.Fatal("subsequent TTL operation was not executed")
	}
}
