package limit

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

func MaxAllowed(n int) gin.HandlerFunc {
	if n <= 0 {
		return func(c *gin.Context) { c.AbortWithStatus(http.StatusServiceUnavailable) }
	}
	sem := make(chan struct{}, n)
	acquire := func() { sem <- struct{}{} }
	release := func() { <-sem }
	return func(c *gin.Context) {
		acquire()       // before request
		defer release() // after request
		c.Next()

	}
}
