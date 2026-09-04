package jwt

import (
	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
	"github.com/maczh/mgin/v2/pkg/config"
	"net/http"
	"strings"
)

// JwtAuthorize JWT认证中间件
func JwtAuthorize() gin.HandlerFunc {
	return func(c *gin.Context) {
		if strings.Contains(c.Request.URL.Path, "/docs/") || strings.Contains(c.Request.URL.Path, "/swagger/") || strings.Contains(c.Request.URL.Path, config.Config.Sys.Swagger.Uri) {
			c.Next()
			return
		}
		// 从请求头中获取token
		tokenStr := c.GetHeader("Authorization")
		if tokenStr == "" {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		// 解析token
		token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
			return []byte(config.Config.Jwt.Secret), nil
		})

		if err != nil || !token.Valid {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		// token验证通过，继续处理请求
		c.Next()
	}
}
