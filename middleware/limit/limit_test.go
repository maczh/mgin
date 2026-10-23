package limit

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestMaxAllowedStopsWaitingRequestWhenContextCanceled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	middleware := MaxAllowed(1)
	firstEntered := make(chan struct{})
	firstRelease := make(chan struct{})
	router := gin.New()
	router.Use(middleware)
	router.GET("/test", func(c *gin.Context) {
		close(firstEntered)
		<-firstRelease
	})

	firstDone := make(chan struct{})
	go func() {
		router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/test", nil))
		close(firstDone)
	}()
	<-firstEntered

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req := httptest.NewRequest(http.MethodGet, "/test", nil).WithContext(ctx)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != 499 {
		t.Fatalf("status = %d, want 499", res.Code)
	}
	close(firstRelease)
	<-firstDone
}
