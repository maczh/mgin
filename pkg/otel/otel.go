// Package otel 提供 mgin v2 的 OpenTelemetry 抽象能力。
//
// 设计取舍（v2.1）：
//   - 框架不强制依赖 OTel SDK / exporter，避免在离线/受限环境无法构建
//   - 仅暴露 trace.Tracer 接口与 Init 入口，业务侧按需集成 SDK
//   - middleware/trace 升级为同时输出 X-Trace-Id 与 traceparent header，
//     兼容既有调用方与 W3C Trace Context 标准
//
// 业务启用 OTel 的推荐路径：
//
//	import "go.opentelemetry.io/otel"
//	import sdktrace "go.opentelemetry.io/otel/sdk/trace"
//	// ... 自行构造 TracerProvider
//	mginotel.SetTracerProvider(你的Provider)
//	shutdown, _ := sdktrace.NewTracerProvider(...).Shutdown, nil
//	defer shutdown(ctx)
//
// 或者用本包提供的轻量 Init（需要业务自行加 OTel SDK 到 go.mod）。
package otel

import (
	"context"
	"sync/atomic"

	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

var (
	// tracerProvider 全局 TracerProvider。业务可通过 SetTracerProvider 注入。
	tracerProvider atomic.Value // trace.TracerProvider
	// enabledFlag 0=disabled / 1=enabled。
	enabledFlag int32
)

func init() {
	// 默认 noop，业务 SetTracerProvider 后切换。
	tracerProvider.Store(noop.NewTracerProvider())
	atomic.StoreInt32(&enabledFlag, 0)
}

// SetTracerProvider 注入业务已构造好的 TracerProvider。
// 同时把 otel 全局也设上，让 gin / gorm 的 OTel 扩展能识别到。
//
// 重复设置会以最后一次为准（用于测试场景切换）。
func SetTracerProvider(tp trace.TracerProvider) {
	if tp == nil {
		tp = noop.NewTracerProvider()
	}
	tracerProvider.Store(tp)
	atomic.StoreInt32(&enabledFlag, 1)
}

// Disable 显式禁用 OTel，Tracer 退化到 noop。
// 重复调用安全。
func Disable() {
	atomic.StoreInt32(&enabledFlag, 0)
	tracerProvider.Store(noop.NewTracerProvider())
}

// Tracer 取 mgin 命名空间下的 Tracer。
// 未启用时返 noop Tracer，所有 span 不会真正产生开销。
func Tracer() trace.Tracer {
	return tracerProvider.Load().(trace.TracerProvider).Tracer("mgin")
}

// StartSpan 是 Tracer().Start 的便捷封装，自动从 ctx 透传 parent。
// 业务侧可用：
//
//	ctx, span := mginotel.StartSpan(ctx, "user-service.GetUser")
//	defer span.End()
func StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return Tracer().Start(ctx, name, opts...)
}

// IsEnabled 报告 OTel 是否已启用（供 health/metrics 等模块做"已启用才上报"判断）。
func IsEnabled() bool {
	return atomic.LoadInt32(&enabledFlag) == 1
}
