package jwt

import (
	"errors"
	"net/http"
	"strings"

	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
	"github.com/maczh/mgin/config"
	"github.com/maczh/mgin/middleware/routeallow"
)

// JwtAuthorize JWT认证中间件
func JwtAuthorize() gin.HandlerFunc {
	return func(c *gin.Context) {
		if config.Config == nil {
			c.AbortWithStatus(http.StatusServiceUnavailable)
			return
		}
		// nil 防护：c.Request/c.Request.URL 可能为 nil
		if c.Request != nil && c.Request.URL != nil {
			path := c.Request.URL.Path
			if routeallow.MatchesPrefix(path, "/docs") || routeallow.MatchesPrefix(path, "/swagger") || routeallow.MatchesPrefix(path, config.Config.Sys.Swagger.Uri) {
				c.Next()
				return
			}
		}
		// 从请求头中获取token
		tokenStr := strings.TrimSpace(c.GetHeader("Authorization"))
		if strings.HasPrefix(strings.ToLower(tokenStr), "bearer ") {
			tokenStr = strings.TrimSpace(tokenStr[len("Bearer "):])
		}
		if tokenStr == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": http.StatusUnauthorized, "msg": "unauthorized"})
			return
		}

		// 解析token
		token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, errors.New("unexpected jwt signing method")
			}
			if config.Config == nil {
				return nil, errors.New("jwt secret not configured")
			}
			if config.Config.Jwt.Secret == "" {
				return nil, errors.New("jwt secret not configured")
			}
			return []byte(config.Config.Jwt.Secret), nil
		})

		if err != nil || token == nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": http.StatusUnauthorized, "msg": "invalid token"})
			return
		}

		if claims, ok := token.Claims.(jwt.MapClaims); ok {
			c.Set("claims", claims)
		}
		// token验证通过，继续处理请求
		c.Next()
	}
}
