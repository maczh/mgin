package router

import (
	"github.com/maczh/mgin"
	"github.com/maczh/mgin/examples/quickstart/controller"
)

// RegisterRoutes 注册所有业务路由与全局中间件
func RegisterRoutes(app *mgin.App) {

	v1 := app.Router.Group("/api/v1")
	{
		v1.GET("/products", controller.ListProducts)
		v1.GET("/products/:id", controller.GetProduct)
		// TODO: 在此追加更多业务路由
	}
}
