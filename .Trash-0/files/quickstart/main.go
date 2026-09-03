package main

import (
	"github.com/maczh/mgin"
	"github.com/maczh/mgin/examples/quickstart/router"
)

func main() {
	// 参数: 配置文件路径, 应用名, 版本号, 是否启用国际化(xlang)
	app := mgin.NewApp("conf/application.yml", "quickstart", "1.0.0", false)
	if app == nil {
		return
	}
	router.RegisterRoutes(app)
	app.Run()
}
