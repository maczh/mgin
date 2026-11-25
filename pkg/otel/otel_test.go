package otel

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

type customProvider struct{ trace.TracerProvider }

// TestNoopDefault 验证未启用时 Tracer 返 noop，StartSpan 不会 panic。
func TestNoopDefault(t *testing.T) {
	if IsEnabled() {
		t.Fatalf("默认应未启用 OTel，实际 enabled")
	}
	ctx, span := StartSpan(context.Background(), "test.span")
	if span == nil {
		t.Fatal("span 不应为 nil")
	}
	defer span.End()
	if ctx == nil {
		t.Fatal("ctx 不应为 nil")
	}
}

// TestSetThenDisable 验证 SetTracerProvider 启用后，Disable 关闭。
func TestSetThenDisable(t *testing.T) {
	SetTracerProvider(noop.NewTracerProvider())
	if !IsEnabled() {
		t.Fatal("SetTracerProvider 后应启用")
	}
	Disable()
	if IsEnabled() {
		t.Fatal("Disable 后应未启用")
	}
	// 关闭后再取 Tracer 不应 panic
	_ = Tracer()
}

func TestProviderCanChangeConcreteType(t *testing.T) {
	SetTracerProvider(customProvider{TracerProvider: noop.NewTracerProvider()})
	if Tracer() == nil {
		t.Fatal("Tracer returned nil")
	}
	Disable()
	SetTracerProvider(noop.NewTracerProvider())
	if Tracer() == nil {
		t.Fatal("Tracer returned nil after provider change")
	}
	Disable()
}
