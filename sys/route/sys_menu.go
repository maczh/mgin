package route

import (
	"github.com/gin-gonic/gin"
	"github.com/maczh/mgin/sys/controller"
)

type sysMenuRouter struct{}

func (c *sysMenuRouter) Register(g *gin.RouterGroup) {
	group := g.Group("sys/res")
	{
		group.POST("add", handle(controller.SysResource.Add))
		group.POST("update", handle(controller.SysResource.Update))
		group.POST("del", handle(controller.SysResource.Delete))
		group.GET("get", handle(controller.SysResource.Get))
		group.GET("list", handle(controller.SysResource.List))
		group.GET("tree", handle(controller.SysResource.GetTree))
	}
}
