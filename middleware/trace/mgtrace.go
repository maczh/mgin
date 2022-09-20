package trace

import (
	"github.com/gin-gonic/gin"
)

func TraceId() gin.HandlerFunc {
	return func(c *gin.Context) {
		PutRequestId(c)
	}
}
