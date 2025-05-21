package route

import (
	"github.com/gin-gonic/gin"
	"github.com/maczh/mgin/sys/controller"
)

type sysMenuRouter struct{}

func (c *sysMenuRouter) Register(g *gin.RouterGroup) {
	group := g.Group("menu")
	{
		group.POST("add", handle(controller.SysMenu.Add))
		group.POST("update", handle(controller.SysMenu.Update))
		group.POST("del", handle(controller.SysMenu.Delete))
		group.GET("get", handle(controller.SysMenu.Get))
		group.GET("list", handle(controller.SysMenu.List))
		group.GET("tree", handle(controller.SysMenu.GetTree))
	}
}
