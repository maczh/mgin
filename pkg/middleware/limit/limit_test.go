package limit

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/maczh/mgin/v2/pkg/utils"
)

func TestMaxAllowedReturnsWhenWaitingRequestIsCanceled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	entered := make(chan struct{})
	release := make(chan struct{})
	var releaseOnce sync.Once
	freeFirst := func() { releaseOnce.Do(func() { close(release) }) }
	t.Cleanup(freeFirst)
	router.Use(MaxAllowed(1))
	router.GET("/", func(c *gin.Context) {
		if c.GetHeader("X-Request") == "first" {
			close(entered)
			<-release
		}
		c.Status(http.StatusOK)
	})
	firstDone := make(chan struct{})
	firstRequest := httptest.NewRequest(http.MethodGet, "/", nil)
	firstRequest.Header.Set("X-Request", "first")
	utils.GoSafe(func() {
		defer close(firstDone)
		router.ServeHTTP(httptest.NewRecorder(), firstRequest)
	})
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("first request did not acquire the permit")
	}

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("X-Request", "second")
	ctx, cancel := context.WithCancel(request.Context())
	request = request.WithContext(ctx)
	response := httptest.NewRecorder()
	secondDone := make(chan struct{})
	utils.GoSafe(func() {
		router.ServeHTTP(response, request)
		close(secondDone)
	})
	time.Sleep(20 * time.Millisecond)
	cancel()
	select {
	case <-secondDone:
		if response.Code != http.StatusRequestTimeout {
			t.Fatalf("canceled request status=%d, want %d", response.Code, http.StatusRequestTimeout)
		}
	case <-time.After(250 * time.Millisecond):
		freeFirst()
		<-firstDone
		t.Fatal("canceled request remained blocked on the semaphore")
	}
	freeFirst()
	select {
	case <-firstDone:
	case <-time.After(time.Second):
		t.Fatal("first request did not finish")
	}
}
