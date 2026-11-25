package utils

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestSafeMapSnapshotsAndAllowsReentrantCallbacks(t *testing.T) {
	var m Map[int]
	first, second := 1, 2
	m.Store("first", &first)
	m.Range(func(key string, _ *int) bool {
		m.Store("second", &second)
		return true
	})
	if m.Len() != 2 {
		t.Fatalf("Len=%d, want 2", m.Len())
	}
	copy := m.Map()
	delete(copy, "first")
	if m.Load("first") == nil {
		t.Fatal("mutating Map result changed original map")
	}
}

func TestRingBufferEmptyWrapAndInvalidSize(t *testing.T) {
	rb := NewRingBuffer[int](0)
	if rb.Size != 1 || rb.Read() != 0 {
		t.Fatalf("unexpected empty buffer: size=%d", rb.Size)
	}
	rb.Write(10)
	rb.Write(20)
	if rb.Latest() != 20 || rb.Oldest() != 20 || rb.Read() != 20 || rb.Read() != 0 {
		t.Fatal("single-slot ring buffer did not maintain valid empty/wrap behavior")
	}
	rb = NewRingBuffer[int](2)
	rb.Write(1)
	rb.Write(2)
	rb.Write(3)
	if rb.Oldest() != 2 || rb.Latest() != 3 || rb.Read() != 2 || rb.Read() != 3 {
		t.Fatal("ring buffer did not preserve newest values in FIFO order")
	}
}

func TestSliceDeletionAndJoinBoundaries(t *testing.T) {
	ints := []int{1, 2, 3}
	if got := IntArrayDelete(ints, -1); !reflect.DeepEqual(got, ints) {
		t.Fatalf("invalid index changed slice: %v", got)
	}
	if got := IntArrayDelete(ints, 3); !reflect.DeepEqual(got, ints) {
		t.Fatalf("out-of-range index changed slice: %v", got)
	}
	if got := IntArrayDelete(ints, 1); !reflect.DeepEqual(got, []int{1, 3}) {
		t.Fatalf("valid deletion=%v", got)
	}
	if got := UnSplitString(nil, ","); got != "" {
		t.Fatalf("empty join=%q", got)
	}
}

func TestDownloadFileUsesURLPathAndContentDispositionSafely(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Disposition", `attachment; filename="report.bin"`)
		_, _ = w.Write([]byte("payload"))
	}))
	defer server.Close()
	localDir := t.TempDir() + string(os.PathSeparator)
	got, err := DownloadFile(server.URL+"/no-extension?token=1", localDir)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(got) != "report.bin" {
		t.Fatalf("filename=%q", got)
	}
	data, err := os.ReadFile(got)
	if err != nil || string(data) != "payload" {
		t.Fatalf("downloaded data=%q, err=%v", data, err)
	}
}

func TestLimitFrequencyRejectsInvalidArguments(t *testing.T) {
	if LimitFreqSingle("", 1, 10) {
		t.Fatal("empty key should be rejected")
	}
	if LimitFreqSingle("test-invalid-count", 0, 10) {
		t.Fatal("zero limit should be rejected")
	}
	key := "test-limit-" + time.Now().Format("150405.000000000")
	if !LimitFreqSingle(key, 1, 60) || LimitFreqSingle(key, 1, 60) {
		t.Fatal("sliding-window limit did not enforce the configured count")
	}
}

func TestRSAKeyParsersRejectNonRSAKeys(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	encoded := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})
	if _, err := getPubKey(encoded); err == nil {
		t.Fatal("expected non-RSA public key error")
	}
	der, err = x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	encoded = pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	if _, err := getPriKey(encoded); err == nil {
		t.Fatal("expected non-RSA private key error")
	}
}
