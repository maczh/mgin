package route

import (
	"github.com/gin-gonic/gin"
	"github.com/maczh/mgin/sys/controller"
)

type sysRoleApiRouter struct{}

func (c *sysRoleApiRouter) Register(g *gin.RouterGroup) {
	group := g.Group("sys/role_api")
	{
		group.POST("bind", handle(controller.SysRoleApi.Bind))
		group.GET("list", handle(controller.SysRoleApi.List))
	}
}
