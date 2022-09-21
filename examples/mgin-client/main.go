package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/maczh/mgin"
	"github.com/maczh/mgin/config"
	"github.com/maczh/mgin/i18n"
	"github.com/maczh/mgin/logs"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const config_file = "mgin-client.yml"

//@title	mgin-client example
//@version 	0.0.1(mgin-client)
//@description	mgin-client example

//初始化命令行参数
func parseArgs() string {
	var configFile string
	flag.StringVar(&configFile, "f", os.Args[0]+".yml", "yml配置文件名")
	flag.Parse()
	path, _ := filepath.Abs(filepath.Dir(os.Args[0]))
	if !strings.Contains(configFile, "/") {
		configFile = path + "/" + configFile
	}
	return configFile
}

func main() {
	//初始化配置，自动连接数据库和Nacos服务注册
	cfgFile := parseArgs()
	mgin.Init(cfgFile)

	//GIN的模式，生产环境可以设置成release
	gin.SetMode("debug")

	//初始化国际化错误代码
	i18n.Init()

	engine := setupRouter()

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", config.Config.App.Port),
		Handler: engine,
	}
	serverSsl := &http.Server{
		Addr:    fmt.Sprintf(":%d", config.Config.App.PortSSL),
		Handler: engine,
	}

	logs.Info("|-----------------------------------|")
	logs.Info("|     mgin-client example 0.0.1     |")
	logs.Info("|-----------------------------------|")
	logs.Info("|  Go Http Server Start Successful  |")
	logs.Info("|    Port: {}     Pid: {}        |", config.Config.App.Port, os.Getpid())
	logs.Info("|-----------------------------------|")

	logs.Debug("====================================")
	logs.Debug("| {}启动成功!   侦听端口:{}     |", config.Config.App.Name, config.Config.App.Port)
	logs.Debug("====================================")

	//http端口侦听
	if config.Config.App.Port != 0 {
		go func() {
			var err error
			err = server.ListenAndServe()
			if err != nil && err != http.ErrServerClosed {
				logs.Error("HTTP server listen: {}", err.Error())
			}
		}()
	}
	//https端口侦听
	if config.Config.App.Cert != "" {
		go func() {
			var err error
			path, _ := filepath.Abs(filepath.Dir(os.Args[0]))
			err = serverSsl.ListenAndServeTLS(path+"/"+config.Config.App.Cert, path+"/"+config.Config.App.Key)
			if err != nil && err != http.ErrServerClosed {
				logs.Error("HTTPS server listen: {}", err.Error())
			}
		}()
	}

	// 等待中断信号以优雅地关闭服务器（设置 5 秒的超时时间）
	signalChan := make(chan os.Signal)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGHUP, syscall.SIGTERM, syscall.SIGQUIT)
	sig := <-signalChan
	logs.Error("Get Signal:" + sig.String())
	logs.Error("Shutdown Server ...")
	mgin.MGin.SafeExit()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		logs.Error("Server Shutdown:" + err.Error())
	}
	logs.Error("Server exiting")
}
