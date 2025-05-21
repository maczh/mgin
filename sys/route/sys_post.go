package route

import (
	"github.com/gin-gonic/gin"
	"github.com/maczh/mgin/sys/controller"
)

type sysPostRouter struct{}

func (c *sysPostRouter) Register(g *gin.RouterGroup) {
	group := g.Group("post")
	{
		group.POST("add", handle(controller.SysPost.Add))
		group.POST("update", handle(controller.SysPost.Update))
		group.POST("del", handle(controller.SysPost.Delete))
		group.GET("get", handle(controller.SysPost.Get))
		group.GET("list", handle(controller.SysPost.List))
	}

}
