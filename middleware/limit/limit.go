package limit

import (
	"github.com/gin-gonic/gin"
)

func MaxAllowed(n int) gin.HandlerFunc {
	if n <= 0 {
		// n<=0 表示不限制并发，直接放行，避免零缓冲信号量导致永久阻塞
		return func(c *gin.Context) {
			c.Next()
		}
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
