package route

import (
	"github.com/gin-gonic/gin"
	"github.com/maczh/mgin/sys/controller"
)

type captchaRouter struct{}

func (c *captchaRouter) Register(g *gin.RouterGroup) {
	group := g.Group("captcha")
	{
		group.GET("/get", handle(controller.Captcha.GetCaptcha))
		group.POST("/verify", handle(controller.Captcha.VerifyCaptcha))
	}
}
