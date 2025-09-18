package mgin

import (
	"context"
	"flag"
	"fmt"
	nice "github.com/ekyoung/gin-nice-recovery"
	"github.com/gin-gonic/gin"
	"github.com/labstack/gommon/color"
	"github.com/maczh/mgin/config"
	_ "github.com/maczh/mgin/docs"
	"github.com/maczh/mgin/errcode"
	"github.com/maczh/mgin/i18n"
	"github.com/maczh/mgin/logs"
	"github.com/maczh/mgin/middleware/cors"
	"github.com/maczh/mgin/middleware/postlog"
	"github.com/maczh/mgin/middleware/trace"
	"github.com/maczh/mgin/middleware/xlang"
	"github.com/maczh/mgin/sys/dao"
	"github.com/maczh/mgin/sys/middle"
	"github.com/maczh/mgin/sys/route"
	"github.com/scylladb/termtables"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

type App struct {
	name       string
	version    string
	configFile string
	i18n       bool
	Router     *gin.Engine
	MGin       *mgin
}

// NewApp 创建一个新的 MGin App 实例。
// 参数:
//   - configFile: 配置文件的路径。如果为空，将尝试从命令行参数获取。
//   - appName: 应用程序的名称。
//   - version: 应用程序的版本号。
//   - xlang: 是否启用国际化支持。
//
// 返回值:
//   - *App: 新创建的 App 实例指针。
func NewApp(configFile, appName, version string, xlang bool) *App {
	// 检查配置文件路径是否为空，如果为空则尝试从命令行参数获取
	if configFile == "" {
		// 定义一个命令行参数 -f，默认值为当前可执行文件同名的 yml 文件，用于指定配置文件名
		flag.StringVar(&configFile, "f", strings.TrimSuffix(os.Args[0], ".exe")+".yml", "yml配置文件名")
		// 解析命令行参数
		flag.Parse()
	}
	// 获取当前可执行文件所在的绝对路径
	path, _ := filepath.Abs(filepath.Dir(os.Args[0]))
	// 检查配置文件路径是否包含路径分隔符，如果不包含则将其与当前可执行文件所在路径拼接
	if !(strings.Contains(configFile, "/") || strings.Contains(configFile, "\\")) {
		configFile = path + "/" + configFile
	}
	// 创建一个新的 App 实例
	app := &App{configFile: configFile,
		name:    appName,
		version: version,
		i18n:    xlang,
		MGin:    &mgin{},
	}
	// 初始化配置，传入配置文件路径
	Init(app.configFile)
	//设置定时任务自动检查
	ticker := time.NewTicker(time.Minute * 5)
	go func() {
		for _ = range ticker.C {
			app.MGin.checkAll()
		}
	}()

	//GIN的模式，生产环境可以设置成release
	// 设置 Gin 框架的运行模式为调试模式
	gin.SetMode("debug")

	// 国际化错误代码初始化
	// 如果启用了国际化支持，则初始化国际化模块
	if app.i18n {
		i18n.Init()
	}
	// 初始化应用的基础路由
	app.baseRouter()
	// 返回新创建的 App 实例指针
	return app
}

// baseRouter 初始化路由,添加中间件
func (app *App) baseRouter() {
	// Disable Console Color
	// gin.DisableConsoleColor()
	app.Router = gin.Default()

	//添加跟踪日志
	app.Router.Use(trace.TraceId())

	//设置接口日志
	app.Router.Use(postlog.RequestLogger())
	//添加跨域处理
	app.Router.Use(cors.Cors())
	//添加国际化支持
	if app.i18n {
		app.Router.Use(xlang.RequestLanguage())
	}

	//处理全局异常
	app.Router.Use(nice.Recovery(recoveryHandler))

	//设置404返回的内容
	app.Router.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusOK, i18n.Error(errcode.URI_NOT_FOUND, errcode.UrlNotFound))
	})

	// 启动内置系统
	if config.Config.Sys.Enabled {
		if config.Config.Sys.Initdb {
			dao.InitDB()
		}
		app.Router.Use(middle.JwtAuthorize())
		app.Router.Use(middle.ApiAuth())
		app.Router = route.SysRouter(config.Config.Sys.BaseUri, app.Router)
		//打开Swagger
		if config.Config.Sys.Swagger.Enabled {
			app.Router.GET(config.Config.Sys.BaseUri+config.Config.Sys.Swagger.Uri+"/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
		}
	}

}

// Run 方法用于启动 HTTP 和 HTTPS 服务器，并监听系统信号以实现优雅关闭。
func (app *App) Run() {
	// 获取当前可执行文件所在的绝对路径
	path, _ := filepath.Abs(filepath.Dir(os.Args[0]))

	// 创建 HTTP 服务器实例
	server := &http.Server{
		// 设置服务器监听的地址，端口从配置文件中获取
		Addr: fmt.Sprintf(":%d", config.Config.App.Port),
		// 设置服务器的处理器为应用的路由
		Handler: app.Router,
	}
	// 创建 HTTPS 服务器实例
	serverSsl := &http.Server{
		// 设置服务器监听的地址，SSL 端口从配置文件中获取
		Addr: fmt.Sprintf(":%d", config.Config.App.PortSSL),
		// 设置服务器的处理器为应用的路由
		Handler: app.Router,
	}
	// 打印应用启动信息
	table := termtables.CreateTable()
	table.Style.Width = 100
	table.Style.Alignment = termtables.AlignCenter
	table.Style.PaddingLeft = 5
	table.Style.PaddingRight = 5
	table.AddHeaders(fmt.Sprintf("%s    %s", app.name, app.version))
	table.AddRow("MGin Server Start Successful")
	table.AddRow(fmt.Sprintf("Port: %d    Pid: %d", config.Config.App.Port, os.Getpid()))
	color.Println(color.Green(table.Render()))
	// 打印应用启动信息
	//logs.Info("|-----------------------------------|")
	//logs.Info("|      " + app.name + " " + app.version + "      |")
	//logs.Info("|-----------------------------------|")
	//logs.Info("|  Go Http Server Start Successful  |")
	//logs.Info("|    Port:" + config.Config.GetConfigString("go.application.port") + "     Pid:" + fmt.Sprintf("%d", os.Getpid()) + "        |")
	//logs.Info("|-----------------------------------|")
	//logs.Info("")

	// 如果配置文件中设置的 HTTP 端口大于 0，则启动 HTTP 服务器
	if config.Config.App.Port > 0 {
		go func() {
			// 启动 HTTP 服务器监听连接
			if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				// 记录 HTTP 服务器监听错误信息
				logs.Error("HTTP server listen: " + err.Error())
			}
		}()
	}

	// 如果配置文件中设置了 SSL 证书路径，则启动 HTTPS 服务器
	if config.Config.App.Cert != "" {
		go func() {
			var err error
			// 启动 HTTPS 服务器监听连接
			err = serverSsl.ListenAndServeTLS(path+"/"+config.Config.App.Cert, path+"/"+config.Config.App.Key)
			if err != nil && err != http.ErrServerClosed {
				// 记录 HTTPS 服务器监听错误信息
				logs.Error("HTTPS server listen: {}", err.Error())
			}
		}()
	}
	// 创建一个信号通道，用于接收系统信号
	signalChan := make(chan os.Signal)
	// 监听指定的系统信号
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGHUP, syscall.SIGTERM, syscall.SIGQUIT)
	// 阻塞等待系统信号
	sig := <-signalChan
	// 记录接收到的系统信号
	logs.Error("Get Signal:" + sig.String())
	// 记录服务器开始关闭的信息
	logs.Error("Shutdown Server ...")
	// 安全退出应用
	app.MGin.SafeExit()
	// 创建一个带有 5 秒超时的上下文
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// 优雅关闭 HTTP 服务器
	if err := server.Shutdown(ctx); err != nil {
		// 记录服务器关闭错误信息
		logs.Error("Server Shutdown:" + err.Error())
	}
	// 记录服务器退出信息
	logs.Error("Server exiting")
	return
}

func recoveryHandler(c *gin.Context, err interface{}) {
	c.JSON(http.StatusOK, i18n.Error(errcode.SYSTEM_ERROR, errcode.SystemError))
}
