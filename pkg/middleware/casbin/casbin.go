package casbin

import (
	"fmt"
	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
	"github.com/maczh/mgin/pkg/casbin"
	"github.com/maczh/mgin/pkg/config"
	"github.com/maczh/mgin/pkg/models"
	"net/http"
	"strings"
)

// CasbinHandler 拦截器
func CasbinHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !config.Config.Casbin.Enabled {
			c.Next()
			return
		}
		//获取请求的PATH
		path := c.Request.URL.Path
		// 白名单路径
		for _, v := range casbin.Casbin.UnAuthPath {
			if v.Path == path && v.Method == c.Request.Method {
				c.Next()
				return
			}
		}
		if strings.Contains(c.Request.URL.Path, "/docs/") || strings.Contains(c.Request.URL.Path, "/swagger/") || strings.Contains(c.Request.URL.Path, config.Config.Sys.Swagger.Uri) {
			c.Next()
			return
		}
		claims, exists := c.Get("claims")
		if !exists {
			c.AbortWithStatusJSON(http.StatusUnauthorized, models.Error(401, "用户未登录"))
			return
		}
		userId := fmt.Sprintf("%d", uint(claims.(jwt.MapClaims)["userId"].(float64)))
		roleId := fmt.Sprintf("%d", uint(claims.(jwt.MapClaims)["roleId"].(float64)))
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
