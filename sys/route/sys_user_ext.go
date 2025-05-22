package route

import (
	"github.com/gin-gonic/gin"
	"github.com/maczh/mgin/sys/controller"
)

type sysUserExtRouter struct{}

func (c *sysUserExtRouter) Register(g *gin.RouterGroup) {
	group := g.Group("sys/user/ext")
	{
		group.POST("add", handle(controller.SysUserExt.Create))
		group.POST("update", handle(controller.SysUserExt.Update))
		group.POST("del", handle(controller.SysUserExt.Delete))
		group.GET("get", handle(controller.SysUserExt.Get))
		group.GET("list", handle(controller.SysUserExt.List))
	}
}
