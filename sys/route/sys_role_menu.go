package route

import (
	"github.com/gin-gonic/gin"
	"github.com/maczh/mgin/sys/controller"
)

type sysRoleMenuRouter struct{}

func (c *sysRoleMenuRouter) Register(g *gin.RouterGroup) {
	group := g.Group("sys/role_menu")
	{
		group.POST("bind", handle(controller.SysRoleMenu.Bind))
		group.GET("list", handle(controller.SysRoleMenu.List))
	}
}
