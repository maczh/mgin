package middle

import (
	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
	"github.com/maczh/mgin/config"
	"github.com/maczh/mgin/logs"
	"github.com/maczh/mgin/models"
	"github.com/maczh/mgin/sys/service"
	"strings"
)

func ApiAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		logs.Debug("验证权限：{},前缀: {}", c.Request.URL.Path, config.Config.Sys.BaseUri+config.Config.Sys.Swagger.Uri)
		// swagger和文档接口不验证权限
		if strings.HasPrefix(c.Request.URL.Path, "/docs/") || strings.HasPrefix(c.Request.URL.Path, "/swagger/") || strings.HasPrefix(c.Request.URL.Path, config.Config.Sys.BaseUri+config.Config.Sys.Swagger.Uri) {
			c.Next()
			return
		}
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
