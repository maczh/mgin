package mgin

import (
	"context"
	"flag"
	"fmt"

	nice "github.com/ekyoung/gin-nice-recovery"
	"github.com/gin-gonic/gin"
	"github.com/labstack/gommon/color"
	"github.com/maczh/mgin/config"
	"github.com/maczh/mgin/errcode"
	"github.com/maczh/mgin/health"
	"github.com/maczh/mgin/i18n"
	"github.com/maczh/mgin/job"
	"github.com/maczh/mgin/logs"
	"github.com/maczh/mgin/middleware/cors"
	"github.com/maczh/mgin/middleware/postlog"
	"github.com/maczh/mgin/middleware/ratelimit"
	"github.com/maczh/mgin/middleware/trace"
	"github.com/maczh/mgin/middleware/xlang"
	"github.com/scylladb/termtables"

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
	// healthEnabled 标记业务是否通过 App.EnableHealth() 显式启用了健康检查探针。
	// 为 false 时仍可能由配置 go.health.enabled 驱动挂载（见 mountHealth）。
	healthEnabled bool
	// healthMounted 标记探针路由是否已挂载，防止重复注册（Gin 会 panic）。
	healthMounted bool
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
	var getVersion bool
	// 仅在用户显式传入 -v 时才打印版本号并返回；默认值必须为 false
	flag.BoolVar(&getVersion, "v", false, "显示版本号")
	if configFile == "" {
		// 定义一个命令行参数 -f，默认值为当前可执行文件同名的 yml 文件，用于指定配置文件名
		flag.StringVar(&configFile, "f", strings.TrimSuffix(os.Args[0], ".exe")+".yml", "yml配置文件名")
		// 解析命令行参数
	}
	flag.Parse()
	// 如果启用了版本号显示，则打印版本号并退出程序
	if getVersion {
		fmt.Printf("%s, 版本号: %s\n", appName, version)
		return nil
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
	ginMode := gin.ReleaseMode
	if config.Config.App.Debug {
		ginMode = gin.DebugMode
	}
	gin.SetMode(ginMode)

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

	// 健康检查探针（health 包）必须在此处、且在所有 Use() 之前挂载。
	// Gin 在路由注册时（RouterGroup.handle -> combineHandlers）就已经把当前已注册的中间件链
	// 快照进了路由节点，之后再注册的中间件不会回溯作用到已注册的路由上。因此只要探针路由
	// 先于一切 Use() 完成注册，就不会被用户后续挂载的 casbin / jwt 等鉴权中间件拦截
	// （K8s 探活不会被 401），也不会被 ratelimit 等限流中间件误伤。
	// 选型理由详见 health 包头部注释（方案 a 与方案 b 的对比）。
	app.mountHealth()

	//添加跟踪日志
	app.Router.Use(trace.TraceId())

	//设置接口日志
	app.Router.Use(postlog.RequestLogger())
	//添加跨域处理
	allowedHeaders := config.Config.GetConfigString("go.cors.headers")
	app.Router.Use(cors.Cors(strings.Split(allowedHeaders, ",")...))
	//添加国际化支持
	if app.i18n {
		app.Router.Use(xlang.RequestLanguage())
	}

	// 单例限流中间件（middleware/ratelimit）：go.ratelimit.enabled 为 true 时挂载
	// 规则来自 application.yml 的 go.ratelimit 节点；未启用则不产生任何开销。
	if config.Config.GetConfigBool("go.ratelimit.enabled") {
		app.Router.Use(ratelimit.RateLimit())
	}

	//处理全局异常
	app.Router.Use(nice.Recovery(recoveryHandler))

	//设置404返回的内容
	app.Router.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusOK, i18n.Error(errcode.URI_NOT_FOUND, errcode.UrlNotFound))
	})

	// 定时任务管理路由组（job，类 xxl-job）按需挂载，例如：
	if config.Config.GetConfigBool("go.job.enabled") {
		job.GetManager()
		app.Router.Group("/job")
		job.RouterGroup(app.Router.Group("/job"))
	}
	// 详见 README 第 22 章。未挂载时不影响定时任务调度本身的运行。

}

// mountHealth 按配置挂载健康检查探针（/health/live /health/ready /health/startup）。
//
// 默认不启用：只有配置 go.health.enabled=true，或业务调用了 App.EnableHealth() 时才挂载。
// 未启用的项目不会多出任何路由，行为与升级前完全一致。
//
// 挂到 /health 前缀而非根路径，是为了避免与既有项目的业务路由产生冲突：
// 探针在 baseRouter() 中最先注册，会早于用户的所有业务路由。
// 若挂在根路径 (/live、/ready、/startup) 而用户已有同名业务路由，启动时将 panic。
//
// healthMounted 用于保证同一进程内只挂载一次：Gin 对重复注册同一 method+path 会直接 panic，
// 因此当配置开关与 App.EnableHealth() 同时使用时必须去重。
func (app *App) mountHealth() {
	if app.healthMounted || app.Router == nil {
		return
	}
	if !app.healthEnabled && !config.Config.GetConfigBool("go.health.enabled") {
		return
	}
	health.Router(app.Router.Group("/health"))
	app.healthMounted = true
	logs.Info("健康检查探针已挂载: /health/live /health/ready /health/startup")
}

// EnableHealth 显式启用健康检查探针，等价于配置 go.health.enabled=true。
//
// 建议在挂载 casbin / jwt 等鉴权中间件之前调用。由于 NewApp() 中 baseRouter() 已执行，
// 此时挂载的探针会带上 baseRouter() 中已注册的无害中间件（trace / postlog / cors / recovery），
// 但不会带上本方法之后才注册的中间件；若需要完全干净的调用链，请直接使用 go.health.enabled 配置。
// 重复调用或与配置开关同时使用都是安全的，探针路由只会挂载一次。
func (app *App) EnableHealth() {
	app.healthEnabled = true
	app.mountHealth()
}

// MarkHealthStarted 标记应用已完成启动，使 /startup 探针返回 200。
// 等价于直接调用 health.MarkStarted()。
func (app *App) MarkHealthStarted() {
	health.MarkStarted()
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

	// servers 收集所有实际启动的 http.Server，退出时逐个优雅关闭。
	// 注意：此前 serverSsl 从未被 Shutdown，收到退出信号后 HTTPS 的监听 goroutine 会泄漏、
	// 端口也会延迟释放，这里统一纳入管理。
	servers := make([]*http.Server, 0, 2)

	// 如果配置文件中设置的 HTTP 端口大于 0，则启动 HTTP 服务器
	if config.Config.App.Port > 0 {
		servers = append(servers, server)
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
		servers = append(servers, serverSsl)
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

	// 若配置了 go.health.autoStarted=true，则在监听启动之后自动标记启动完成，
	// 使 /startup 探针返回 200。默认不自动标记，由业务自行调用 health.MarkStarted()。
	if config.Config.GetConfigBool("go.health.autoStarted") {
		health.MarkStarted()
	}

	// 创建一个信号通道，用于接收系统信号。
	// 缓冲区设为 1：signal.Notify 内部是非阻塞投递，使用无缓冲通道时，
	// 若信号在 <-signalChan 之前到达会被直接丢弃，导致进程收不到退出信号而无法优雅关闭。
	signalChan := make(chan os.Signal, 1)
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
	// 优雅关闭超时时间：读取 go.application.shutdownTimeout（秒），
	// 未配置或 <= 0 时回退为默认 5 秒（与升级前的硬编码值一致）。
	shutdownTimeout := config.Config.GetShutdownTimeout()
	// 逐个优雅关闭所有已启动的服务器（HTTP / HTTPS），错误分别记录日志。
	// 每个服务器各自持有一个超时上下文，互不挤占；未启动的服务器不会被 Shutdown。
	for _, s := range servers {
		ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		if err := s.Shutdown(ctx); err != nil {
			// 记录服务器关闭错误信息
			logs.Error("Server[{}] Shutdown:{}", s.Addr, err.Error())
		}
		cancel()
	}
	// 记录服务器退出信息
	logs.Error("Server exiting")
	return
}

func recoveryHandler(c *gin.Context, err interface{}) {
	c.JSON(http.StatusOK, i18n.Error(errcode.SYSTEM_ERROR, errcode.SystemError))
}

func (app *App) GetVersion() string {
	return app.version
}
