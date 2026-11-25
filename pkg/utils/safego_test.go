package utils

import (
	"testing"
	"time"
)

func TestGoSafeRunsAndRecovers(t *testing.T) {
	done := make(chan struct{})
	GoSafe(func() {
		defer close(done)
		panic("expected test panic")
	})

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("safe goroutine did not finish")
	}
	GoSafe(nil)
}
