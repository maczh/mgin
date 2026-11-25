package utils

import (
	"sync"
	"testing"
	"time"
)

func TestRSASignAndVerifyWithoutKeysReturnErrors(t *testing.T) {
	security := &RSASecurity{}
	signers := []struct {
		name string
		call func() (string, error)
	}{
		{"sha1", func() (string, error) { return security.SignSha1WithRsa("data") }},
		{"sha256", func() (string, error) { return security.SignSha256WithRsa("data") }},
		{"sha256 hex", func() (string, error) { return security.SignSha256WithRsaHex("data") }},
		{"sha256 url", func() (string, error) { return security.SignSha256WithRsaUrlSafe("data") }},
	}
	for _, signer := range signers {
		t.Run(signer.name, func(t *testing.T) {
			if _, err := signer.call(); err == nil {
				t.Fatal("expected missing private key error")
			}
		})
	}
	for name, verify := range map[string]func() error{
		"sha1":   func() error { return security.VerifySignSha1WithRsa("data", "signature") },
		"sha256": func() error { return security.VerifySignSha256WithRsa("data", "signature") },
	} {
		t.Run("verify "+name, func(t *testing.T) {
			if err := verify(); err == nil {
				t.Fatal("expected missing public key error")
			}
		})
	}
}

func TestStructUtilitiesReturnEmptyForNilInput(t *testing.T) {
	var nilPointer *struct{ Name string }
	if got := Struct2Map(nil); len(got) != 0 {
		t.Fatalf("Struct2Map(nil)=%v, want empty", got)
	}
	if got := Struct2MapString(nil); len(got) != 0 {
		t.Fatalf("Struct2MapString(nil)=%v, want empty", got)
	}
	if got := GetStructFields(nil); len(got) != 0 {
		t.Fatalf("GetStructFields(nil)=%v, want empty", got)
	}
	if got := GetStructJsonTags(nil); len(got) != 0 {
		t.Fatalf("GetStructJsonTags(nil)=%v, want empty", got)
	}
	if got := Struct2Map(nilPointer); len(got) != 0 {
		t.Fatalf("Struct2Map(typed nil)=%v, want empty", got)
	}
	if got := Struct2MapString(nilPointer); len(got) != 0 {
		t.Fatalf("Struct2MapString(typed nil)=%v, want empty", got)
	}
	if got := GetStructFields(nilPointer); len(got) != 0 {
		t.Fatalf("GetStructFields(typed nil)=%v, want empty", got)
	}
	if got := GetStructJsonTags(nilPointer); len(got) != 0 {
		t.Fatalf("GetStructJsonTags(typed nil)=%v, want empty", got)
	}
}

func TestDeleteArrayInvalidIndexReturnsUnchangedData(t *testing.T) {
	source := []any{"a", "b"}
	for _, index := range []int{-1, len(source), len(source) + 1} {
		got := DeleteArray(source, index)
		if len(got) != len(source) || got[0] != source[0] || got[1] != source[1] {
			t.Errorf("DeleteArray index=%d got %v, want unchanged %v", index, got, source)
		}
	}
}

func TestDESDecryptRejectsNonBlockLengthCiphertext(t *testing.T) {
	if _, err := DesDecrypt([]byte{1}, []byte("12345678")); err == nil {
		t.Fatal("expected DES non-block ciphertext error")
	}
	if _, err := TripleDesDecrypt([]byte{1}, []byte("123456789012345678901234"), true); err == nil {
		t.Fatal("expected 3DES non-block ciphertext error")
	}
}

func TestRingBufferNormalizesExternallyCorruptedIndexes(t *testing.T) {
	rb := NewRingBuffer[int](2)
	rb.Buffer[0], rb.Buffer[1] = 10, 20
	rb.Reader, rb.Writer = 10, -1
	rb.Write(30)
	_ = rb.Read()
	rb.Reader, rb.Writer = -1, 10
	_ = rb.Latest()
	_ = rb.Oldest()
	rb.Overwrite(40)
}

func TestLinkListWalkCallbackCanMutateList(t *testing.T) {
	list := &LinkList[int]{}
	list.Push(1)
	done := make(chan struct{})
	GoSafe(func() {
		list.Walk(func(value int) bool {
			list.Push(value + 1)
			return true
		})
		close(done)
	})
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Walk callback mutation deadlocked")
	}
	if got := list.Size(); got != 2 {
		t.Fatalf("list size=%d, want 2", got)
	}
}

func TestMapUtilitiesHandleHeterogeneousAndNilInputs(t *testing.T) {
	converted := MapItoS(map[string]any{"valid": "value", "invalid": 7})
	if len(converted) != 2 || converted["valid"] != "value" || converted["invalid"] != "7" {
		t.Fatalf("MapItoS returned %v", converted)
	}
	ordered := SortMapByValue(map[string]any{"number": 3, "text": "x", "nil": nil})
	if len(ordered) != 3 {
		t.Fatalf("SortMapByValue length=%d, want 3", len(ordered))
	}
	if result, err := Struct2StringMap(nil); err == nil || result == nil {
		t.Fatalf("Struct2StringMap(nil)=(%v,%v), want empty map and error", result, err)
	}
}

func TestSafeMapNilReceiverIsConsistent(t *testing.T) {
	var safeMap *Map[int]
	if safeMap.Load("x") != nil || safeMap.LoadAndStore("x", nil) != nil || safeMap.LoadAndDelete("x") != nil {
		t.Fatal("nil SafeMap read operations should return nil")
	}
	if safeMap.Len() != 0 || len(safeMap.Map()) != 0 {
		t.Fatal("nil SafeMap should be empty")
	}
	safeMap.Store("x", nil)
	safeMap.Delete("x")
	safeMap.DeleteDirectly("x")
	safeMap.Clear()
	safeMap.Range(nil)
}

func TestSafeGoBeforeHookPanicDoesNotEscapeOrRunTask(t *testing.T) {
	called := false
	NewSafeGo(func(...any) { called = true }).SetGoBeforeHandler(func() map[string]any {
		panic("before hook panic")
	}).Run()
	if called {
		t.Fatal("task ran after its before hook panicked")
	}
}

func TestStopLimitQueueCleanerIsConcurrentAndIdempotent(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		GoSafe(func() {
			defer wg.Done()
			StopLimitQueueCleaner()
		})
	}
	wg.Wait()
	StopLimitQueueCleaner()
}
