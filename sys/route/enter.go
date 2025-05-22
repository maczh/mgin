package route

import (
	"github.com/gin-gonic/gin"
	"github.com/maczh/mgin/models"
	"net/http"
)

func SysRouter(basePath string, router *gin.Engine) *gin.Engine {
	r := router.Group(basePath)
	var (
		captcha     = captchaRouter{}
		sysUser     = sysUserRouter{}
		sysApi      = sysApiRouter{}
		sysUserExt  = sysUserExtRouter{}
		sysDict     = sysDictRouter{}
		sysDept     = sysDeptRouter{}
		sysRole     = sysRoleRouter{}
		sysMenu     = sysMenuRouter{}
		sysPost     = sysPostRouter{}
		sysRoleApi  = sysRoleApiRouter{}
		sysRoleMenu = sysRoleMenuRouter{}
		SysConfig   = sysConfigRouter{}
	)
	captcha.Register(r)
	sysUser.Register(r)
	sysApi.Register(r)
	sysUserExt.Register(r)
	sysDict.Register(r)
	sysDept.Register(r)
	sysRole.Register(r)
	sysMenu.Register(r)
	sysPost.Register(r)
	sysRoleApi.Register(r)
	sysRoleMenu.Register(r)
	SysConfig.Register(r)

	return router
}

type handFunc func(c *gin.Context) models.Result[any]

func handle(handler handFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		result := handler(c)
		c.JSON(http.StatusOK, result)
	}
}
