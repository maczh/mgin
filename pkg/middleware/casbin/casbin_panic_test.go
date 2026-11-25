package casbin

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
	casbinService "github.com/maczh/mgin/v2/pkg/casbin"
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

func TestPrivatePathContainingDocumentationSegmentDoesNotBypassAuthentication(t *testing.T) {
	old := *config.Config
	t.Cleanup(func() { *config.Config = old })
	config.Config.Casbin.Enabled = true
	config.Config.Sys.Swagger.Uri = "/swagger/sys"
	for _, path := range []string{"/private/docs/secret", "/private/swagger/secret", "/api/document"} {
		config.Config.Sys.Swagger.Uri = "/api/doc"
		router := gin.New()
		router.Use(CasbinHandler())
		router.GET(path, func(c *gin.Context) { c.Status(http.StatusOK) })
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		if recorder.Code != http.StatusUnauthorized {
			t.Errorf("path %q bypassed Casbin: status=%d", path, recorder.Code)
		}
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

func TestNilEnforcerFailsClosed(t *testing.T) {
	oldConfig := *config.Config
	oldCasbin := casbinService.Casbin
	t.Cleanup(func() {
		*config.Config = oldConfig
		casbinService.Casbin = oldCasbin
	})
	config.Config.Casbin.Enabled = true
	config.Config.Sys.Swagger.Uri = ""
	casbinService.Casbin.SyncedCachedEnforcer = nil
	casbinService.Casbin.UnAuthPath = nil

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("claims", jwt.MapClaims{"userId": float64(1), "roleId": float64(1)})
		c.Next()
	})
	router.Use(CasbinHandler())
	router.GET("/private", func(c *gin.Context) { c.Status(http.StatusOK) })
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/private", nil))
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d, want %d", w.Code, http.StatusServiceUnavailable)
	}
}

func TestNilCasbinServiceFailsClosed(t *testing.T) {
	oldConfig := *config.Config
	oldCasbin := casbinService.Casbin
	t.Cleanup(func() {
		*config.Config = oldConfig
		casbinService.Casbin = oldCasbin
	})
	config.Config.Casbin.Enabled = true
	config.Config.Sys.Swagger.Uri = ""
	casbinService.Casbin = nil

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
