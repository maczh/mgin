package main

import (
	"github.com/gin-gonic/gin"
	"github.com/maczh/mgin/examples/mgin-client/controller"
	"github.com/maczh/mgin/middleware/cors"
	"github.com/maczh/mgin/middleware/postlog"
	"github.com/maczh/mgin/middleware/trace"
	"github.com/maczh/mgin/middleware/xlang"
	"github.com/maczh/mgin/models"
	"github.com/maczh/mgin/utils"
	//ginSwagger "github.com/swaggo/gin-swagger"
	//"github.com/swaggo/gin-swagger/swaggerFiles"
	"github.com/ekyoung/gin-nice-recovery"
	"net/http"
)

/**
统一路由映射入口
*/
func setupRouter() *gin.Engine {
	// Disable Console Color
	// gin.DisableConsoleColor()
	engine := gin.Default()

	//添加跟踪日志
	engine.Use(trace.TraceId())

	//设置接口日志
	engine.Use(postlog.RequestLogger())
	//添加跨域处理
	engine.Use(cors.Cors())
	//添加国际化处理
	engine.Use(xlang.RequestLanguage())

	//添加swagger支持
	//engine.GET("/docs/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	//处理全局异常
	engine.Use(nice.Recovery(recoveryHandler))

	//设置404返回的内容
	engine.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusOK, models.Error(404, "404 Not Found"))
	})

	var result models.Result
	//添加所需的路由映射
	//
	engine.POST("/client/user/add", func(c *gin.Context) {
		result = controller.User.Add(c)
		c.JSON(http.StatusOK, result)
	})

	engine.GET("/client/user/get", func(c *gin.Context) {
		result = controller.User.Query(utils.GinParamMap(c))
		c.JSON(http.StatusOK, result)
	})

	return engine
}

func recoveryHandler(c *gin.Context, err interface{}) {
	c.JSON(http.StatusOK, models.Error(1000, "System Error"))
}
