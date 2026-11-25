package jwt

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/maczh/mgin/v2/pkg/config"
)

func TestJwtAuthorizeDoesNotBypassPrivatePathForEmptyOrSubstringURI(t *testing.T) {
	gin.SetMode(gin.TestMode)
	old := *config.Config
	t.Cleanup(func() { *config.Config = old })
	config.Config.Sys.Swagger.Uri = ""
	for _, path := range []string{"/private/secret", "/private/docs/secret", "/private/swagger/secret"} {
		router := gin.New()
		router.Use(JwtAuthorize())
		router.GET(path, func(c *gin.Context) { c.Status(http.StatusOK) })
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		if recorder.Code != http.StatusUnauthorized {
			t.Errorf("path %q bypassed authentication: status=%d", path, recorder.Code)
		}
	}
}

func TestJwtAuthorizeAllowsConfiguredSwaggerPrefixOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	old := *config.Config
	t.Cleanup(func() { *config.Config = old })
	config.Config.Sys.Swagger.Uri = "/api/doc"
	for path, want := range map[string]int{
		"/api/doc":       http.StatusOK,
		"/api/doc/index": http.StatusOK,
		"/api/document":  http.StatusUnauthorized,
	} {
		router := gin.New()
		router.Use(JwtAuthorize())
		router.GET(path, func(c *gin.Context) { c.Status(http.StatusOK) })
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		if recorder.Code != want {
			t.Errorf("path %q: status=%d, want %d", path, recorder.Code, want)
		}
	}
}
