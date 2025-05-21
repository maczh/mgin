package route

import (
	"github.com/gin-gonic/gin"
	"github.com/maczh/mgin/sys/controller"
)

type sysApiRouter struct{}

func (c *sysApiRouter) Register(g *gin.RouterGroup) {
	group := g.Group("sys_api")
	{
		group.POST("add", handle(controller.SysApi.Create))
		group.POST("update", handle(controller.SysApi.Update))
		group.POST("del", handle(controller.SysApi.Delete))
		group.GET("get", handle(controller.SysApi.Get))
		group.GET("get/uri", handle(controller.SysApi.GetUri))
		group.GET("list", handle(controller.SysApi.List))
		group.GET("group", handle(controller.SysApi.ListByGroup))
	}
}
