package route

import (
	"github.com/gin-gonic/gin"
	"github.com/maczh/mgin/sys/controller"
)

type sysRoleRouter struct{}

func (c *sysRoleRouter) Register(g *gin.RouterGroup) {
	group := g.Group("sys/role")
	{
		group.POST("add", handle(controller.SysRole.Add))
		group.POST("update", handle(controller.SysRole.Update))
		group.POST("del", handle(controller.SysRole.Delete))
		group.GET("get", handle(controller.SysRole.Get))
		group.GET("list", handle(controller.SysRole.List))
	}
}
