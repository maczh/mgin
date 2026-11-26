package postlog

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

type countingReader struct {
	r io.Reader
	n int
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += n
	return n, err
}

// TestReadRequestBodyBoundsChunkedWithoutDataLoss verifies that a chunked body
// larger than the logging budget is not fully materialized, yet remains
// readable in full by the downstream handler.
func TestReadRequestBodyBoundsChunkedWithoutDataLoss(t *testing.T) {
	payload := bytes.Repeat([]byte("x"), maxLoggedBodyBytes+4096)
	counter := &countingReader{r: bytes.NewReader(payload)}
	req := httptest.NewRequest(http.MethodPost, "/upload", counter)
	req.ContentLength = -1

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = req

	if got := readRequestBody(c); !strings.Contains(got, "omitted") {
		t.Fatalf("logged body = %q, want omitted marker", got)
	}
	if counter.n > maxLoggedBodyBytes+1 {
		t.Fatalf("middleware read %d bytes, want at most %d", counter.n, maxLoggedBodyBytes+1)
	}
	rest, err := io.ReadAll(c.Request.Body)
	if err != nil {
		t.Fatalf("handler read: %v", err)
	}
	if !bytes.Equal(rest, payload) {
		t.Fatalf("handler got %d bytes, want %d", len(rest), len(payload))
	}
}

func TestReadRequestBodyKeepsSmallBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/echo", strings.NewReader("hello"))
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = req
	if got := readRequestBody(c); got != "hello" {
		t.Fatalf("logged body = %q", got)
	}
	rest, _ := io.ReadAll(c.Request.Body)
	if string(rest) != "hello" {
		t.Fatalf("handler body = %q", string(rest))
	}
}

// TestBodyLogWriterCountsTruncation covers the pointer-receiver regression:
// value receivers silently dropped total/truncated updates.
func TestBodyLogWriterCountsTruncation(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	w := &bodyLogWriter{ResponseWriter: c.Writer, body: bytes.NewBuffer(nil), limit: 4}
	if _, err := w.Write([]byte("0123456789")); err != nil {
		t.Fatalf("write: %v", err)
	}
	if w.body.String() != "0123" {
		t.Fatalf("buffer = %q, want %q", w.body.String(), "0123")
	}
	if w.total != 10 {
		t.Fatalf("total = %d, want 10", w.total)
	}
	if !w.truncated {
		t.Fatal("truncated = false, want true")
	}
}
