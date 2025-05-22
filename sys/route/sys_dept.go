package route

import (
	"github.com/gin-gonic/gin"
	"github.com/maczh/mgin/sys/controller"
)

type sysDeptRouter struct{}

func (c *sysDeptRouter) Register(g *gin.RouterGroup) {
	group := g.Group("sys/dept")
	{
		group.POST("add", handle(controller.SysDept.Add))
		group.POST("update", handle(controller.SysDept.Update))
		group.POST("del", handle(controller.SysDept.Delete))
		group.GET("get", handle(controller.SysDept.Get))
		group.GET("tree", handle(controller.SysDept.GetTree))
		group.GET("list", handle(controller.SysDept.List))
	}
}
