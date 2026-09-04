// Package plugin 定义 mgin v2 的统一插件契约与注册表。
//
// 所有对外框架组件（数据源、缓存、消息队列、注册中心、定时任务、对象存储等）均实现
// Plugin 接口，并在 mgin.Init 阶段由注册表按 Order 升序统一初始化，在 SafeExit 阶段按
// Order 逆序统一关闭。业务侧也可通过 mgin.Use / mgin.UsePlugin 注册自定义插件。
package plugin

import (
	"context"
	"sort"
	"sync"
)

// Plugin 是 mgin v2 的统一组件契约。
//
// 字段/方法语义：
//   - Name：唯一名称，如 "mysql" "redis" "kafka" "nacos" "job" "s3"。
//   - Order：启动顺序，数值越小越先 Init；Close 时按逆序执行。
//   - Init：框架启动时调用，完成组件连接/初始化；失败返回 error。
//   - Close：框架退出时调用，释放资源；失败返回 error（不应 panic）。
//   - Health：供 /health/ready 自报健康；无依赖可返回 nil。
//   - Enabled：是否由配置启用（通常读 go.config.used）。
type Plugin interface {
	Name() string
	Order() int
	Init(ctx context.Context) error
	Close(ctx context.Context) error
	Health() error
	Enabled() bool
}

var (
	mu      sync.RWMutex
	plugins = make(map[string]Plugin)
	order   = make([]string, 0) // 注册顺序，保证确定性
)

// Register 注册一个插件。按 Name 去重：同名插件后注册者覆盖先注册者，
// 但注册顺序（order 切片）仅在首次出现时记录。
func Register(p Plugin) {
	if p == nil {
		return
	}
	name := p.Name()
	mu.Lock()
	defer mu.Unlock()
	if _, ok := plugins[name]; !ok {
		order = append(order, name)
	}
	plugins[name] = p
}

// Unregister 按名称注销插件。
func Unregister(name string) {
	mu.Lock()
	defer mu.Unlock()
	if _, ok := plugins[name]; !ok {
		return
	}
	delete(plugins, name)
	for i, n := range order {
		if n == name {
			order = append(order[:i], order[i+1:]...)
			break
		}
	}
}

// GetByName 按名称获取已注册插件，未找到返回 nil。
func GetByName(name string) Plugin {
	mu.RLock()
	defer mu.RUnlock()
	return plugins[name]
}

// All 返回全部已注册插件（按注册顺序）。
func All() []Plugin {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]Plugin, 0, len(order))
	for _, n := range order {
		out = append(out, plugins[n])
	}
	return out
}

// sortedByOrder 返回按 Order 升序排列的插件副本。
func sortedByOrder() []Plugin {
	ps := All()
	sort.Slice(ps, func(i, j int) bool {
		return ps[i].Order() < ps[j].Order()
	})
	return ps
}

// InitAll 按 Order 升序对 Enabled()==true 的插件调用 Init。
// 任一插件 Init 失败仅记录首个错误并继续，不会中断其余插件的初始化
// （与旧 mgin.Init 的 if 链“不中断”语义一致）。
func InitAll(ctx context.Context) error {
	var firstErr error
	for _, p := range sortedByOrder() {
		if !p.Enabled() {
			continue
		}
		if err := p.Init(ctx); err != nil {
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

// CloseAll 按 Order 逆序对 Enabled()==true 的插件调用 Close。
// 设计要点（防漏关）：
//   - 单插件 Close 返回 error 仅记入首个错误并继续关闭其余插件；
//   - 单插件 Close 若 panic，由 defer+recover 兜底，保证不会中断整条关闭链。
func CloseAll(ctx context.Context) error {
	ps := sortedByOrder()
	var firstErr error
	for i := len(ps) - 1; i >= 0; i-- {
		p := ps[i]
		if !p.Enabled() {
			continue
		}
		func() {
			defer func() {
				if r := recover(); r != nil {
					// 单个插件关闭 panic 不应阻断其它插件的关闭
					_ = r
				}
			}()
			if err := p.Close(ctx); err != nil {
				if firstErr == nil {
					firstErr = err
				}
			}
		}()
	}
	return firstErr
}

// HealthAll 汇总各 Enabled()==true 插件的 Health() 结果，供 /health/ready 使用。
func HealthAll() map[string]error {
	m := make(map[string]error)
	for _, p := range sortedByOrder() {
		if !p.Enabled() {
			continue
		}
		m[p.Name()] = p.Health()
	}
	return m
}
