package cors

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestCorsOptionsDoesNotRunDownstreamHandlers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(Cors())
	called := false
	router.OPTIONS("/health", func(c *gin.Context) { called = true })

	req := httptest.NewRequest(http.MethodOptions, "/health", nil)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	if called {
		t.Fatal("OPTIONS request must not run downstream handlers")
	}
	if res.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusNoContent)
	}
}
