package middle

import (
	"github.com/gin-gonic/gin"
	"github.com/maczh/mgin/logs"
	"github.com/maczh/mgin/models"
	"github.com/maczh/mgin/models/sys"
	"github.com/maczh/mgin/sys/service"
	"net/http"
)

// JwtAuthorize JWT认证中间件
func JwtAuthorize() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 获取api信息
		api, err := service.SysApi.GetUri(c.Request.URL.Path)
		if err != nil || api == nil {
			logs.Error("获取api信息失败,默认为必须难")
			api = &sys.SysApi{NeedAuth: true}
		}
		logs.Debug("api={}", api)
		if !api.NeedAuth {
			c.Next()
			return
		}
		// 从请求头中获取token
		tokenStr := c.GetHeader("Authorization")
		if tokenStr == "" {
			logs.Error("未登录")
			c.AbortWithStatusJSON(http.StatusUnauthorized, models.Error(403, "未登录"))
			return
		}

		verify, claims, err := service.SysUser.VerifyJwt(tokenStr)
		// 解析token
		if err != nil {
			logs.Error("解析jwt token失败：{}", err.Error())
			c.AbortWithStatusJSON(http.StatusUnauthorized, models.Error(403, "token解析失败,"+err.Error()))
			return
		}
		if !verify {
			logs.Error("token无效")
			c.AbortWithStatusJSON(http.StatusUnauthorized, models.Error(403, "token无效"))
			return
		}
		c.Set("claims", *claims)
		// token验证通过，继续处理请求
		c.Next()
	}
}
