package route

import (
	"github.com/gin-gonic/gin"
	"github.com/maczh/mgin/sys/controller"
)

type sysRoleMenuRouter struct{}

func (c *sysRoleMenuRouter) Register(g *gin.RouterGroup) {
	group := g.Group("sys/role_res")
	{
		group.POST("bind", handle(controller.SysRoleResource.Bind))
		group.GET("list", handle(controller.SysRoleResource.List))
	}
}
