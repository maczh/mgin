package main

import (
	"github.com/ekyoung/gin-nice-recovery"
	"github.com/gin-gonic/gin"
	"github.com/maczh/gintool"
	"github.com/maczh/gintool/mgresult"
	"github.com/maczh/mgin/controller"
	_ "github.com/maczh/mgin/docs"
	"github.com/maczh/mgtrace"
	"github.com/maczh/utils"
	ginSwagger "github.com/swaggo/gin-swagger"
	"github.com/swaggo/gin-swagger/swaggerFiles"
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
	engine.Use(mgtrace.TraceId())

	//设置接口日志
	engine.Use(gintool.SetRequestLogger())
	//添加跨域处理
	engine.Use(gintool.Cors())

	//添加swagger支持
	engine.GET("/docs/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	//处理全局异常
	engine.Use(nice.Recovery(recoveryHandler))

	//设置404返回的内容
	engine.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusOK, *mgresult.Error(-1, "请求的方法不存在"))
	})

	var result mgresult.Result
	//添加所需的路由映射
	//mysql
	engine.Any("/user/mysql/save", func(c *gin.Context) {
		result = controller.SaveUserMysql(utils.GinParamMap(c))
		c.JSON(http.StatusOK, result)
	})

	engine.Any("/user/mysql/update", func(c *gin.Context) {
		result = controller.UpdateUserMysql(utils.GinParamMap(c))
		c.JSON(http.StatusOK, result)
	})

	engine.Any("/user/mysql/get/mobile", func(c *gin.Context) {
		result = controller.GetUserMysqlByMobile(utils.GinParamMap(c))
		c.JSON(http.StatusOK, result)
	})

	engine.Any("/user/mysql/get/id", func(c *gin.Context) {
		result = controller.GetUserMysqlById(utils.GinParamMap(c))
		c.JSON(http.StatusOK, result)
	})

	//redis
	engine.Any("/user/redis/save", func(c *gin.Context) {
		result = controller.SaveUserRedis(utils.GinParamMap(c))
		c.JSON(http.StatusOK, result)
	})

	engine.Any("/user/redis/update", func(c *gin.Context) {
		result = controller.UpdateUserRedis(utils.GinParamMap(c))
		c.JSON(http.StatusOK, result)
	})

	engine.Any("/user/redis/get/mobile", func(c *gin.Context) {
		result = controller.GetUserRedisByMobile(utils.GinParamMap(c))
		c.JSON(http.StatusOK, result)
	})

	engine.Any("/user/redis/get/id", func(c *gin.Context) {
		result = controller.GetUserRedisById(utils.GinParamMap(c))
		c.JSON(http.StatusOK, result)
	})

	//mongo
	engine.Any("/user/mongo/save", func(c *gin.Context) {
		result = controller.SaveUserMongo(utils.GinParamMap(c))
		c.JSON(http.StatusOK, result)
	})

	engine.Any("/user/mongo/update", func(c *gin.Context) {
		result = controller.UpdateUserMongo(utils.GinParamMap(c))
		c.JSON(http.StatusOK, result)
	})

	engine.Any("/user/mysql/get", func(c *gin.Context) {
		result = controller.GetUserMongoByMobile(utils.GinParamMap(c))
		c.JSON(http.StatusOK, result)
	})

	return engine
}

func recoveryHandler(c *gin.Context, err interface{}) {
	c.JSON(http.StatusOK, *mgresult.Error(-1, "系统异常，请联系客服"))
}
