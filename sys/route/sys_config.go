package route

import (
	"github.com/gin-gonic/gin"
	"github.com/maczh/mgin/sys/controller"
)

type sysConfigRouter struct{}

func (c *sysConfigRouter) Register(g *gin.RouterGroup) {
	group := g.Group("sys/config")
	{
		group.POST("add", handle(controller.SysConfig.Add))
		group.POST("update", handle(controller.SysConfig.Update))
		group.POST("del", handle(controller.SysConfig.Delete))
		group.GET("get", handle(controller.SysConfig.Get))
		group.GET("get/multi", handle(controller.SysConfig.MultiGet))
		group.GET("list", handle(controller.SysConfig.List))
	}

}
