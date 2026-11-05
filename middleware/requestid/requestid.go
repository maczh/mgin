// Package requestid provides request ID idempotency middleware.
package requestid

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/maczh/mgin/cache"
)

const (
	headerName = "X-Request-ID"
	cacheName  = "RequestIdIdempotency"
	ttl        = time.Minute
)

var idempotencyMu sync.Mutex

// Idempotency rejects requests without an X-Request-ID and duplicate IDs
// seen in the previous 60 seconds. The middleware serializes cache insertion
// so concurrent requests with the same ID cannot both pass.
func Idempotency() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := strings.TrimSpace(c.GetHeader(headerName))
		if requestID == "" {
			c.Abort()
			c.JSON(http.StatusBadRequest, gin.H{"code": -1, "msg": "缺少 X-Request-ID"})
			return
		}

		store := cache.OnMemCache(cacheName)
		idempotencyMu.Lock()
		if store.IsExist(requestID) {
			idempotencyMu.Unlock()
			c.Abort()
			c.JSON(http.StatusConflict, gin.H{"code": -1, "msg": "重复的 X-Request-ID"})
			return
		}
		store.Add(requestID, true, ttl)
		idempotencyMu.Unlock()
		c.Next()
	}
}
