package middle

import (
	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
	"github.com/maczh/mgin/models"
	"github.com/maczh/mgin/sys/service"
)

func ApiAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 验证API权限
		claims, _ := c.Get("claims")
		if claims == nil { // 未获取到token
			c.Next()
			return
		}
		userId := claims.(jwt.MapClaims)["id"].(float64)
		hasPermission, err := service.SysRoleApi.HasApiPermission(int64(userId), c.Request.URL.Path)
		if err != nil || !hasPermission {
			c.AbortWithStatusJSON(403, models.Error(403, "无权访问此API"))
			return
		}
		c.Next()
	}
}
