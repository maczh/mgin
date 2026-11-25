package casbin

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/maczh/mgin/v2/pkg/config"
)

func TestEmptySwaggerURIDoesNotBypassAuthentication(t *testing.T) {
	old := *config.Config
	t.Cleanup(func() { *config.Config = old })
	config.Config.Casbin.Enabled = true
	config.Config.Sys.Swagger.Uri = ""
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(CasbinHandler())
	router.GET("/private", func(c *gin.Context) { c.Status(http.StatusOK) })
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/private", nil))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestDocumentationPathBypassesAuthentication(t *testing.T) {
	old := *config.Config
	t.Cleanup(func() { *config.Config = old })
	config.Config.Casbin.Enabled = true
	config.Config.Sys.Swagger.Uri = ""
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(CasbinHandler())
	router.GET("/docs/index.html", func(c *gin.Context) { c.Status(http.StatusOK) })
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/docs/index.html", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("documentation path status=%d, want %d", w.Code, http.StatusOK)
	}
}

func TestClaimIDRejectsUnexpectedTypes(t *testing.T) {
	if _, ok := claimID("7"); ok {
		t.Fatal("string claim must be rejected")
	}
	if got, ok := claimID(int64(7)); !ok || got != "7" {
		t.Fatalf("got %q, %v", got, ok)
	}
}

func TestNilConfigFailsClosed(t *testing.T) {
	old := config.Config
	config.Config = nil
	t.Cleanup(func() { config.Config = old })
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(CasbinHandler())
	router.GET("/private", func(c *gin.Context) { c.Status(http.StatusOK) })
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/private", nil))
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d, want %d", w.Code, http.StatusServiceUnavailable)
	}
}
