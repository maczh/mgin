package middle

import (
	"fmt"
	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
	"github.com/maczh/mgin/casbin"
	"github.com/maczh/mgin/config"
	"github.com/maczh/mgin/logs"
	"github.com/maczh/mgin/models"
	"github.com/maczh/mgin/sys/service"
	"net/http"
	"strings"
)

func ApiAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// swagger和文档接口不验证权限
		if strings.HasPrefix(c.Request.URL.Path, "/docs/") || strings.HasPrefix(c.Request.URL.Path, "/swagger/") || strings.HasPrefix(c.Request.URL.Path, config.Config.Sys.BaseUri+config.Config.Sys.Swagger.Uri) {
			c.Next()
			return
		}
		if config.Config.Sys.Casbin {
			casbinApiAuth(c)
		} else {
			dbApiAuth(c)
		}
	}
}

func dbApiAuth(c *gin.Context) {
	// 验证API权限
	claims, _ := c.Get("claims")
	if claims == nil { // 未获取到token
		c.Next()
		return
	}
	userId := claims.(jwt.MapClaims)["userId"].(float64)
	hasPermission, err := service.SysRoleApi.HasApiPermission(int64(userId), c.Request.URL.Path)
	if err != nil || !hasPermission {
		c.AbortWithStatusJSON(403, models.Error(403, "无访问权限"))
		return
	}
	c.Next()
}

func casbinApiAuth(c *gin.Context) {
	//获取请求的PATH
	path := c.Request.URL.Path
	// 白名单路径
	for _, v := range casbin.Casbin.UnAuthPath {
		if v.Path == path && v.Method == c.Request.Method {
			c.Next()
			return
		}
	}
	logs.Debug("路径白名单不存在")
	claims, exists := c.Get("claims")
	if !exists {
		c.AbortWithStatusJSON(http.StatusUnauthorized, models.Error(401, "用户未登录"))
		return
	}
	//userId := fmt.Sprintf("%d", uint(claims.(jwt.MapClaims)["userId"].(float64)))
	roleId := fmt.Sprintf("%d", uint(claims.(jwt.MapClaims)["roleId"].(float64)))
	// 获取请求方法
	act := c.Request.Method
	e := casbin.Casbin.GetEnforcer()
	rolePerm, _ := e.Enforce(roleId, path, act)
	if !(rolePerm) {
		c.AbortWithStatusJSON(http.StatusForbidden, models.Error(403, "无访问权限"))
		return
	}
	c.Next()
}
