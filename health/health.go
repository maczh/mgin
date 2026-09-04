// Package health 提供 Kubernetes 所需的存活（liveness）/ 就绪（readiness）/ 启动（startup）探针。
//
// 端点：
//
//	GET /live     存活探针：只判断进程能否响应，不查任何依赖，恒返回 200
//	GET /ready    就绪探针：按需实时调用已启用数据源的 Check()，任一失败返回 503
//	GET /startup  启动探针：返回应用是否已完成启动，未完成返回 503
//
// 响应体统一为 JSON：
//
//	{"status":"ok","dependencies":{"mysql":"ok","redis":"fail"}}
//
// 依赖检查口径与 mgin.go 中的 checkAll() 保持一致：依据 go.config.used 判断启用了哪些数据源，
// 只检查 checkAll() 会检查的 7 类（mysql/postgres/mongodb/redis/clickhouse/elasticsearch/kafka）。
//
// 启用方式（默认关闭，不破坏既有项目）：
//  1. 配置 go.health.enabled=true，框架在 app.baseRouter() 中最先挂载探针路由；
//  2. 代码中显式调用 App.EnableHealth()；
//  3. 自行挂载：health.Router(app.Router.Group("/health"))。
//
// 关于挂载位置的选型说明（方案 a / 方案 b）：
// 本实现选择方案 (a) —— 挂在应用主引擎 app.Router 上，但注册时机早于所有中间件，理由如下：
//  1. Gin 在路由注册时（RouterGroup.handle -> combineHandlers）就把当前已注册的中间件链
//     快照进了路由节点，之后注册的中间件不会回溯作用到已注册的路由上。因此在 baseRouter()
//     中“注册顺序先于一切 Use() ”即可保证探针不会被用户后续挂载的 casbin / jwt 等鉴权
//     中间件拦截，K8s 探活不会被 401，也不会被 ratelimit 限流中间件误伤。
//  2. 不需要新增监听端口：无需修改防火墙 / 容器端口映射 / Service 定义，运维成本最低；
//     方案 (b) 的独立管理端口需要额外暴露端口，且在多数内网环境里并无必要。
//  3. 不需要额外的 http.Server 生命周期管理：不增加优雅关闭的复杂度，也不存在管理端口
//     与业务端口状态不一致的问题。
//  4. 方案 (b) 的唯一优势是“完全隔离业务流量压力”，但探针本身是极轻量的内存操作，
//     /live 甚至不查依赖，这一优势在 mgin 场景下不值得用额外的端口与运维成本去换。
//
// 若业务确有“完全独立管理端口”的强诉求，可直接用 health.Router() 挂到自建的
// gin.New() 引擎上并自行监听，本包不限制挂载目标。
package health

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/maczh/mgin/config"
	"github.com/maczh/mgin/db"
	"github.com/maczh/mgin/logs"
)

// 响应体 status 字段的取值。
const (
	// StatusOK 表示检查通过。
	StatusOK = "ok"
	// StatusFail 表示检查未通过。
	StatusFail = "fail"
)

// response 探针统一响应体。
// Dependencies 使用 omitempty：/live 与 /startup 不返回该字段；
// 没有任何数据源被启用时也不会出现 null，而是直接省略。
type response struct {
	Status       string            `json:"status"`
	Dependencies map[string]string `json:"dependencies,omitempty"`
}

// started / startedAt 记录应用是否已完成启动，供 /startup 探针使用。
var (
	startMu   sync.RWMutex
	started   bool
	startedAt time.Time
)

// MarkStarted 标记应用已完成启动。
// 业务应在所有初始化工作（数据源、缓存、定时任务、本地预热等）完成之后调用；
// 未调用之前 /startup 一律返回 503。
// 也可以通过配置 go.health.autoStarted=true 让框架在监听启动后自动标记。
func MarkStarted() {
	startMu.Lock()
	defer startMu.Unlock()
	if !started {
		started = true
		startedAt = time.Now()
		logs.Info("健康检查: 应用启动完成标记已设置")
	}
}

// ResetStarted 清除启动完成标记，主要用于测试场景。
func ResetStarted() {
	startMu.Lock()
	defer startMu.Unlock()
	started = false
	startedAt = time.Time{}
}

// IsStarted 返回应用是否已完成启动。
func IsStarted() bool {
	startMu.RLock()
	defer startMu.RUnlock()
	return started
}

// StartedAt 返回应用完成启动的时间；未完成启动时返回零值。
func StartedAt() time.Time {
	startMu.RLock()
	defer startMu.RUnlock()
	return startedAt
}

// dependency 描述一个可被探针检查的数据源。
type dependency struct {
	// name 同时作为 go.config.used 中的关键字与响应体 dependencies 的键。
	name string
	// check 数据源的健康检查函数，签名与 mgin.go 中 checkAll() 用到的一致。
	check func() error
}

// dependencies 依据 go.config.used 实时组装待检查的数据源列表。
// 每请求都重新组装，保证配置文件热更新（如新增数据源）后探针口径同步生效。
func dependencies() []dependency {
	used := config.Config.Config.Used
	list := make([]dependency, 0, 7)
	if strings.Contains(used, "mysql") {
		list = append(list, dependency{name: "mysql", check: db.Mysql.Check})
	}
	if strings.Contains(used, "postgres") {
		list = append(list, dependency{name: "postgres", check: db.Pg.Check})
	}
	if strings.Contains(used, "mongodb") {
		list = append(list, dependency{name: "mongodb", check: db.Mongo.Check})
	}
	if strings.Contains(used, "redis") {
		list = append(list, dependency{name: "redis", check: db.Redis.Check})
	}
	if strings.Contains(used, "clickhouse") {
		list = append(list, dependency{name: "clickhouse", check: db.Clickhouse.Check})
	}
	if strings.Contains(used, "elasticsearch") {
		list = append(list, dependency{name: "elasticsearch", check: db.ElasticSearch.Check})
	}
	if strings.Contains(used, "kafka") {
		list = append(list, dependency{name: "kafka", check: db.Kafka.Check})
	}
	return list
}

// CheckDependencies 按顺序检查所有已启用的数据源。
// 返回值:
//   - bool: 全部通过为 true，任一失败为 false；没有启用任何数据源时返回 true。
//   - map[string]string: 每个数据源的健康状态，值为 StatusOK 或 StatusFail。
//
// 说明：这里直接复用各数据源的 Check()，其语义与 mgin.go 中 checkAll() 完全一致
// （部分数据源的 Check() 内部失败时会尝试重连），因此探针具备“自愈”能力。
// 注意 Check() 内部可能发起重连，耗时不可控，调用方应自行评估 K8s 探针超时时间。
func CheckDependencies() (bool, map[string]string) {
	deps := dependencies()
	result := make(map[string]string, len(deps))
	allOK := true
	for _, d := range deps {
		if err := d.check(); err != nil {
			result[d.name] = StatusFail
			allOK = false
			logs.Error("健康检查: {} 检查失败: {}", d.name, err.Error())
			continue
		}
		result[d.name] = StatusOK
	}
	return allOK, result
}

// Router 在给定的路由组上挂载三个探针端点。
// 传入 app.Router.Group("/health") 可把端点挂到 /health/live、/health/ready、/health/startup 下。
func Router(g *gin.RouterGroup) {
	g.GET("/live", liveHandler)
	g.GET("/ready", readyHandler)
	g.GET("/startup", startupHandler)
}

// liveHandler 存活探针：进程能响应即认为存活，不查任何依赖，避免依赖抖动导致容器被误重启。
func liveHandler(c *gin.Context) {
	c.JSON(http.StatusOK, response{Status: StatusOK})
}

// readyHandler 就绪探针：实时检查所有已启用数据源，任一失败返回 503 并在响应体中逐项列出状态。
func readyHandler(c *gin.Context) {
	ok, deps := CheckDependencies()
	if !ok {
		logs.Error("健康检查: 就绪探针检查失败 {}", formatDeps(deps))
		c.JSON(http.StatusServiceUnavailable, response{Status: StatusFail, Dependencies: deps})
		return
	}
	c.JSON(http.StatusOK, response{Status: StatusOK, Dependencies: deps})
}

// startupHandler 启动探针：MarkStarted() 被调用（或配置 go.health.autoStarted=true）之后才返回 200。
func startupHandler(c *gin.Context) {
	if !IsStarted() {
		logs.Debug("健康检查: 启动探针检查失败，应用尚未完成启动")
		c.JSON(http.StatusServiceUnavailable, response{Status: StatusFail})
		return
	}
	c.JSON(http.StatusOK, response{Status: StatusOK})
}

// formatDeps 把依赖状态表格式化成日志友好的字符串，避免直接把 map 塞进日志占位符。
func formatDeps(deps map[string]string) string {
	if len(deps) == 0 {
		return "{}"
	}
	sb := &strings.Builder{}
	sb.WriteString("{")
	first := true
	for _, d := range dependencies() {
		if v, ok := deps[d.name]; ok {
			if !first {
				sb.WriteString(", ")
			}
			sb.WriteString(d.name)
			sb.WriteString(":")
			sb.WriteString(v)
			first = false
		}
	}
	sb.WriteString("}")
	return sb.String()
}
