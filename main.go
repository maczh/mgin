package main

import (
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	config "github.com/maczh/mgconfig"
	"github.com/sadlil/gologger"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

var Logger = gologger.GetLogger()

const config_file = "mgin.yml"

//@title	MGin example
//@version 	1.0.0(mgin)
//@description	MGin微服务框架演示范例

func main() {
	//初始化配置，自动连接数据库和Nacos服务注册
	path, _ := filepath.Abs(filepath.Dir(os.Args[0]))
	config.InitConfig(path + "/" + config_file)
	var logger = gologger.GetLogger()

	//GIN的模式，生产环境可以设置成release
	gin.SetMode("debug")

	engine := setupRouter()

	server := &http.Server{
		Addr:    ":" + config.GetConfigString("go.application.port"),
		Handler: engine,
	}

	fmt.Println("|-----------------------------------|")
	fmt.Println("|             MGin  1.0.0           |")
	fmt.Println("|-----------------------------------|")
	fmt.Println("|  Go Http Server Start Successful  |")
	fmt.Println("|    Port:" + config.GetConfigString("go.application.port") + "     Pid:" + fmt.Sprintf("%d", os.Getpid()) + "        |")
	fmt.Println("|-----------------------------------|")
	fmt.Println("")

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP server listen: " + err.Error())
		}
	}()

	// 等待中断信号以优雅地关闭服务器（设置 5 秒的超时时间）
	signalChan := make(chan os.Signal)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGHUP, syscall.SIGTERM, syscall.SIGQUIT)
	sig := <-signalChan
	logger.Error("Get Signal:" + sig.String())
	logger.Error("Shutdown Server ...")
	config.SafeExit()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		logger.Error("Server Shutdown:" + err.Error())
	}
	logger.Error("Server exiting")

}
