package casbin

import (
	"encoding/json"
	"fmt"
	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
	"github.com/maczh/mgin/v2/pkg/casbin"
	"github.com/maczh/mgin/v2/pkg/config"
	"github.com/maczh/mgin/v2/pkg/middleware/routeallow"
	"github.com/maczh/mgin/v2/pkg/models"
	"net/http"
)

// CasbinHandler 拦截器
func CasbinHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		if config.Config == nil {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, models.Error(503, "权限配置不可用"))
			return
		}
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
		swaggerURI := config.Config.Sys.Swagger.Uri
		if routeallow.MatchesPrefix(path, "/docs") || routeallow.MatchesPrefix(path, "/swagger") || routeallow.MatchesPrefix(path, swaggerURI) {
			c.Next()
			return
		}
		claims, exists := c.Get("claims")
		if !exists {
			c.AbortWithStatusJSON(http.StatusUnauthorized, models.Error(401, "用户未登录"))
			return
		}
		mapClaims, ok := claims.(jwt.MapClaims)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, models.Error(401, "用户身份信息无效"))
			return
		}
		userId, userOK := claimID(mapClaims["userId"])
		roleId, roleOK := claimID(mapClaims["roleId"])
		if !userOK || !roleOK {
			c.AbortWithStatusJSON(http.StatusUnauthorized, models.Error(401, "用户身份信息无效"))
			return
		}
		// 获取请求方法
		act := c.Request.Method
		enforcer := casbin.Casbin.GetEnforcer()
		if enforcer == nil {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, models.Error(503, "权限服务不可用"))
			return
		}
		if err := enforcer.LoadPolicy(); err != nil {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, models.Error(503, "权限策略加载失败"))
			return
		}
		userPerm, userErr := enforcer.Enforce(userId, path, act)
		rolePerm, roleErr := enforcer.Enforce(roleId, path, act)
		if userErr != nil || roleErr != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, models.Error(500, "权限校验失败"))
			return
		}
		if !(userPerm || rolePerm) {
			c.AbortWithStatusJSON(http.StatusForbidden, models.Error(403, "无访问权限"))
			return
		}
		c.Next()
	}
}

func claimID(value any) (string, bool) {
	switch id := value.(type) {
	case float64:
		if id < 0 {
			return "", false
		}
		return fmt.Sprintf("%.0f", id), true
	case float32:
		if id < 0 {
			return "", false
		}
		return fmt.Sprintf("%.0f", id), true
	case int:
		if id < 0 {
			return "", false
		}
		return fmt.Sprintf("%d", id), true
	case int64:
		if id < 0 {
			return "", false
		}
		return fmt.Sprintf("%d", id), true
	case uint:
		return fmt.Sprintf("%d", id), true
	case uint64:
		return fmt.Sprintf("%d", id), true
	case json.Number:
		if _, err := id.Int64(); err == nil {
			return id.String(), true
		}
		return "", false
	default:
		return "", false
	}
}
