package ratelimit

import (
	"strings"
	"time"

	"github.com/maczh/mgin/v2/pkg/config"
	"github.com/maczh/mgin/v2/pkg/errcode"
)

// Algorithm 限流算法
type Algorithm string

const (
	// AlgoTokenBucket 令牌桶：允许突发流量，按固定速率补充令牌
	AlgoTokenBucket Algorithm = "token_bucket"
	// AlgoSlidingWindow 滑动窗口：精确统计窗口内请求数，无临界突刺问题
	AlgoSlidingWindow Algorithm = "sliding_window"
	// AlgoConcurrency 最大并发数：信号量控制同时在处理的请求数
	AlgoConcurrency Algorithm = "concurrency"
)

// Dimension 限流维度，决定限流计数器的分组维度
type Dimension string

const (
	// DimGlobal 全局共享一个限流器
	DimGlobal Dimension = "global"
	// DimIP 按客户端 IP 分别限流
	DimIP Dimension = "ip"
	// DimPath 按请求路径分别限流
	DimPath Dimension = "path"
	// DimIPPath 按 客户端IP + 请求路径 组合分别限流
	DimIPPath Dimension = "ip_path"
	// DimHeader 按指定请求头的值分别限流（如 X-User-Id、Authorization）
	DimHeader Dimension = "header"
)

const (
	defaultConfigPrefix = "go.ratelimit"
	defaultIdleTimeout  = 10 * time.Minute
	defaultGCInterval   = time.Minute
)

// Rule 一条限流规则。
// 全局规则由 go.ratelimit 下的一级配置项构成；
// 独立规则由 go.ratelimit.rules 数组构成，按声明顺序优先匹配，命中即用该规则，不再叠加全局规则。
type Rule struct {
	// Name 规则名，用于限流器 key 前缀与统计展示；为空时自动以路径生成
	Name string `koanf:"name"`
	// Path 匹配路径。支持三种写法：
	//   精确匹配   /api/v1/user/info
	//   前缀匹配   /api/v1/sms/*
	//   Gin 路由    /api/v1/user/:id
	// 为空表示匹配所有路径
	Path string `koanf:"path"`
	// Methods 匹配的 HTTP 方法，为空表示不限方法
	Methods []string `koanf:"methods"`
	// Algorithm 限流算法，默认 token_bucket
	Algorithm Algorithm `koanf:"algorithm"`
	// Dimension 限流维度，默认 global
	Dimension Dimension `koanf:"dimension"`
	// HeaderKey Dimension 为 header 时读取的请求头名称
	HeaderKey string `koanf:"headerKey"`
	// Rate 令牌桶：每秒生成令牌数；滑动窗口：一个窗口内允许的请求数
	Rate float64 `koanf:"rate"`
	// Burst 令牌桶容量（可突发的最大请求数），为 0 时取 Rate 向上取整
	Burst int `koanf:"burst"`
	// Window 滑动窗口时长（秒），默认 1
	Window int `koanf:"window"`
	// WindowMs 滑动窗口时长（毫秒），优先于 Window
	WindowMs int `koanf:"windowMs"`
	// MaxConcurrent concurrency 算法的最大并发请求数
	MaxConcurrent int `koanf:"maxConcurrent"`
	// Wait 触发限流时是否排队等待而非立即拒绝
	Wait bool `koanf:"wait"`
	// WaitTimeoutMs 排队等待的最长时间（毫秒），超时后拒绝
	WaitTimeoutMs int `koanf:"waitTimeoutMs"`
	// Code 被限流时返回的业务状态码，默认 errcode.TOO_MANY_REQUESTS
	Code int `koanf:"code"`
	// Message 被限流时返回的提示信息
	Message string `koanf:"message"`
	// HttpStatus 被限流时的 HTTP 状态码，默认 200（框架统一用业务码表达错误）
	HttpStatus int `koanf:"httpStatus"`

	pathPrefix string // Path 以 /* 结尾时的前缀
	prefixMode bool
}

// Config 限流中间件配置，对应 application.yml 中 go.ratelimit 节点
type Config struct {
	// Enabled 是否启用限流
	Enabled bool `koanf:"enabled"`
	// Whitelist 白名单路径（前缀匹配），命中则完全跳过限流
	Whitelist []string `koanf:"whitelist"`
	// WhiteIps 白名单 IP，命中则完全跳过限流
	WhiteIps []string `koanf:"whiteIps"`
	// IdleTimeout 空闲限流器回收时间（秒），默认 600
	IdleTimeout int `koanf:"idleTimeout"`
	// Headers 是否在响应头中输出 X-RateLimit-* 信息
	Headers bool `koanf:"headers"`
	// Rules 独立规则列表
	Rules []Rule `koanf:"rules"`

	global      Rule
	idleTimeout time.Duration
}

// LoadConfig 从 application.yml 的 go.ratelimit 节点读取限流配置。
// 未配置 go.ratelimit 节点时返回 Enabled=false 的配置，中间件将直接放行。
func LoadConfig() *Config {
	return LoadConfigFrom(defaultConfigPrefix)
}

// LoadConfigFrom 从指定配置前缀读取限流配置，便于同一进程内挂载多组互不干扰的限流策略
func LoadConfigFrom(prefix string) *Config {
	cfg := &Config{}
	if config.Config.Cnf == nil || !config.Config.Exists(prefix) {
		return cfg.normalize()
	}
	// 整体反序列化，koanf 默认使用 koanf tag
	if err := config.Config.Cnf.Unmarshal(prefix, cfg); err != nil {
		logger.Error("限流配置解析错误: " + err.Error())
		return cfg.normalize()
	}
	// 全局规则与 Config 共用同一层配置节点，需单独反序列化
	if err := config.Config.Cnf.Unmarshal(prefix, &cfg.global); err != nil {
		logger.Error("限流全局规则解析错误: " + err.Error())
	}
	return cfg.normalize()
}

// normalize 填充默认值并预处理路径匹配模式
func (c *Config) normalize() *Config {
	c.idleTimeout = defaultIdleTimeout
	if c.IdleTimeout > 0 {
		c.idleTimeout = time.Duration(c.IdleTimeout) * time.Second
	}
	c.global.Name = "global"
	c.global.normalize()
	for i := range c.Rules {
		if c.Rules[i].Name == "" {
			c.Rules[i].Name = c.Rules[i].Path
		}
		c.Rules[i].normalize()
	}
	return c
}

// normalize 单条规则的默认值填充
func (r *Rule) normalize() {
	if r.Algorithm == "" {
		r.Algorithm = AlgoTokenBucket
	}
	if r.Dimension == "" {
		r.Dimension = DimGlobal
	}
	if r.Window <= 0 {
		r.Window = 1
	}
	if r.Burst <= 0 {
		r.Burst = int(r.Rate)
		if float64(r.Burst) < r.Rate {
			r.Burst++
		}
		if r.Burst <= 0 {
			r.Burst = 1
		}
	}
	if r.Code == 0 {
		r.Code = errcode.TOO_MANY_REQUESTS
	}
	if r.Message == "" {
		r.Message = errcode.TooManyRequests
	}
	if r.HttpStatus == 0 {
		r.HttpStatus = 200
	}
	if strings.HasSuffix(r.Path, "/*") {
		r.prefixMode = true
		r.pathPrefix = strings.TrimSuffix(r.Path, "/*")
	} else if strings.HasSuffix(r.Path, "*") {
		r.prefixMode = true
		r.pathPrefix = strings.TrimSuffix(r.Path, "*")
	}
}

// window 返回滑动窗口时长
func (r *Rule) window() time.Duration {
	if r.WindowMs > 0 {
		return time.Duration(r.WindowMs) * time.Millisecond
	}
	return time.Duration(r.Window) * time.Second
}

// waitTimeout 返回排队等待时长，未开启 Wait 时返回 0
func (r *Rule) waitTimeout() time.Duration {
	if !r.Wait {
		return 0
	}
	if r.WaitTimeoutMs <= 0 {
		return time.Second
	}
	return time.Duration(r.WaitTimeoutMs) * time.Millisecond
}

// match 判断规则是否匹配当前请求。routePath 为 gin 注册的路由模板，urlPath 为实际请求路径
func (r *Rule) match(method, routePath, urlPath string) bool {
	if len(r.Methods) > 0 && !containsFold(r.Methods, method) {
		return false
	}
	if r.Path == "" || r.Path == "*" || r.Path == "/*" {
		return true
	}
	if r.prefixMode {
		return strings.HasPrefix(urlPath, r.pathPrefix) || strings.HasPrefix(routePath, r.pathPrefix)
	}
	return r.Path == urlPath || r.Path == routePath
}

// limited 判断规则是否配置了有效的限流阈值
func (r *Rule) limited() bool {
	switch r.Algorithm {
	case AlgoConcurrency:
		return r.MaxConcurrent > 0
	default:
		return r.Rate > 0
	}
}

func containsFold(list []string, v string) bool {
	for _, s := range list {
		if strings.EqualFold(strings.TrimSpace(s), v) {
			return true
		}
	}
	return false
}
