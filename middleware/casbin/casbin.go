package casbin

import (
	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
	"github.com/maczh/mgin/casbin"
	"github.com/maczh/mgin/config"
	"github.com/maczh/mgin/models"
	"net/http"
)

// CasbinHandler 拦截器
func CasbinHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !config.Config.Casbin.Enabled {
			c.Next()
			return
		}
		claims, exists := c.Get("claims")
		if !exists {
			c.AbortWithStatusJSON(http.StatusUnauthorized, models.Error(401, "用户未登录"))
			return
		}
		userId := claims.(jwt.MapClaims)["userId"].(string)
		roleId := claims.(jwt.MapClaims)["roleId"].(string)
		//获取请求的PATH
		path := c.Request.URL.Path
		// 获取请求方法
		act := c.Request.Method
		casbin.Casbin.GetEnforcer().LoadPolicy()
		userPerm, _ := casbin.Casbin.GetEnforcer().Enforce(userId, path, act)
		rolePerm, _ := casbin.Casbin.GetEnforcer().Enforce(roleId, path, act)
		if !(userPerm || rolePerm) {
			c.AbortWithStatusJSON(http.StatusForbidden, models.Error(403, "无访问权限"))
			return
		}
		c.Next()
	}
}
