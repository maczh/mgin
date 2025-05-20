package route

import (
	"github.com/gin-gonic/gin"
	"github.com/maczh/mgin/sys/controller"
)

type sysUserRouter struct{}

func (c *sysUserRouter) Register(g *gin.RouterGroup) {
	//注册登录
	g.POST("login", handle(controller.SysUser.Login))
	g.POST("register", handle(controller.SysUser.Register))
	g.POST("token", handle(controller.SysUser.VerifyToken))
	group := g.Group("users")
	{
		group.POST("add", handle(controller.SysUser.Add))
		group.POST("update", handle(controller.SysUser.Update))
		group.POST("del", handle(controller.SysUser.Delete))
		group.POST("pwd", handle(controller.SysUser.ChangePassword))
		group.POST("status", handle(controller.SysUser.ChangeStatus))
		group.POST("logout", handle(controller.SysUser.Logout))
		group.GET("get", handle(controller.SysUser.Get))
		group.GET("list", handle(controller.SysUser.List))
	}
}
