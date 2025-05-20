package route

import (
	"github.com/gin-gonic/gin"
	"github.com/maczh/mgin/models"
	"net/http"
)

func RootRouter(basePath string, router *gin.Engine) *gin.Engine {
	root := router.Group(basePath)
	var (
		captcha = captchaRouter{}
	)
	captcha.Register(root)
}

type handFunc func(c *gin.Context) models.Result[any]

func handle(handler handFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		result := handler(c)
		c.JSON(http.StatusOK, result)
	}
}
