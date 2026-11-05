package requestid

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestIdempotency(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Idempotency())
	r.POST("/", func(c *gin.Context) { c.Status(204) })
	serve := func(id string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", "/", nil)
		if id != "" {
			req.Header.Set(headerName, id)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}
	if got := serve("").Code; got != 400 {
		t.Fatalf("missing ID status = %d, want 400", got)
	}
	if got := serve("request-id-test").Code; got != 204 {
		t.Fatalf("first request status = %d, want 204", got)
	}
	if got := serve("request-id-test").Code; got != 409 {
		t.Fatalf("duplicate request status = %d, want 409", got)
	}
}
