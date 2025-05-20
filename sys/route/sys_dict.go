package route

import (
	"github.com/gin-gonic/gin"
	"github.com/maczh/mgin/sys/controller"
)

type sysDictRouter struct{}

func (c *sysDictRouter) Register(g *gin.RouterGroup) {
	group := g.Group("dict")
	{
		group.POST("add", handle(controller.SysDict.Add))
		group.POST("update", handle(controller.SysDict.Update))
		group.POST("del", handle(controller.SysDict.Delete))
		group.GET("get", handle(controller.SysDict.Get))
		group.GET("list", handle(controller.SysDict.List))
	}

}
