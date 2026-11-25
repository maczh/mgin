package jwt

import (
	"net/http"
	"strings"

	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
	"github.com/maczh/mgin/v2/pkg/config"
	"github.com/maczh/mgin/v2/pkg/middleware/routeallow"
)

// JwtAuthorize JWT认证中间件
func JwtAuthorize() gin.HandlerFunc {
	return func(c *gin.Context) {
		if config.Config == nil {
			c.AbortWithStatus(http.StatusServiceUnavailable)
			return
		}
		if routeallow.MatchesPrefix(c.Request.URL.Path, "/docs") || routeallow.MatchesPrefix(c.Request.URL.Path, "/swagger") || routeallow.MatchesPrefix(c.Request.URL.Path, config.Config.Sys.Swagger.Uri) {
			c.Next()
			return
		}
		// 从请求头中获取token
		tokenStr := strings.TrimSpace(c.GetHeader("Authorization"))
		if tokenStr == "" {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		// 解析token
		if strings.HasPrefix(strings.ToLower(tokenStr), "bearer ") {
			tokenStr = strings.TrimSpace(tokenStr[7:])
		}
		if config.Config.Jwt.Secret == "" || tokenStr == "" {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
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
