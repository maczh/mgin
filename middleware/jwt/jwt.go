package jwt

import (
	"errors"
	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
	"github.com/maczh/mgin/config"
	"net/http"
	"strings"
)

// JwtAuthorize JWT认证中间件
func JwtAuthorize() gin.HandlerFunc {
	return func(c *gin.Context) {
		// nil 防护：c.Request/c.Request.URL 可能为 nil
		if c.Request != nil && c.Request.URL != nil {
			path := c.Request.URL.Path
			swaggerUri := ""
			if config.Config != nil {
				swaggerUri = config.Config.Sys.Swagger.Uri
			}
			if strings.Contains(path, "/docs/") || strings.Contains(path, "/swagger/") || (swaggerUri != "" && strings.Contains(path, swaggerUri)) {
				c.Next()
				return
			}
		}
		// 从请求头中获取token
		tokenStr := c.GetHeader("Authorization")
		if tokenStr == "" {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		// 解析token
		token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
			if config.Config == nil {
				return nil, errors.New("jwt secret not configured")
			}
			return []byte(config.Config.Jwt.Secret), nil
		})

		if err != nil || token == nil || !token.Valid {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		// token验证通过，继续处理请求
		c.Next()
	}
}
