package limit

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func MaxAllowed(n int) gin.HandlerFunc {
	if n <= 0 {
		return func(c *gin.Context) { c.AbortWithStatus(http.StatusServiceUnavailable) }
	}
	sem := make(chan struct{}, n)
	release := func() { <-sem }
	return func(c *gin.Context) {
		requestCtx := c.Request.Context()
		select {
		case sem <- struct{}{}:
			defer release()
		case <-requestCtx.Done():
			c.AbortWithStatus(http.StatusRequestTimeout)
			return
		}
		c.Next()

	}
}
