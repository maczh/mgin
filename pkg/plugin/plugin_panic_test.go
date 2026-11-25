package plugin

import (
	"context"
	"testing"
)

type panicMetadataPlugin struct{ panicName bool }

func (p *panicMetadataPlugin) Name() string {
	if p.panicName {
		panic("name panic")
	}
	return "panic-metadata"
}
func (*panicMetadataPlugin) Order() int                  { panic("order panic") }
func (*panicMetadataPlugin) Init(context.Context) error  { panic("init must not run") }
func (*panicMetadataPlugin) Close(context.Context) error { return nil }
func (*panicMetadataPlugin) Health() error               { return nil }
func (*panicMetadataPlugin) Enabled() bool               { return true }

func TestRegisterRejectsPanickingMetadata(t *testing.T) {
	Register(&panicMetadataPlugin{panicName: true})
	Register(&panicMetadataPlugin{})
	if GetByName("panic-metadata") != nil {
		t.Fatal("plugin with panicking metadata must not be registered")
	}
}

type unstableMetadataPlugin struct {
	nameCalls   int
	orderCalls  int
	closeCalls  int
	enabled     bool
	panicEnable bool
}

func (p *unstableMetadataPlugin) Name() string {
	p.nameCalls++
	if p.nameCalls > 1 {
		panic("name changed after registration")
	}
	return "unstable-metadata-test"
}
func (p *unstableMetadataPlugin) Order() int {
	p.orderCalls++
	if p.orderCalls > 1 {
		panic("order changed after registration")
	}
	return 1
}
func (*unstableMetadataPlugin) Init(context.Context) error { return nil }
func (p *unstableMetadataPlugin) Close(context.Context) error {
	p.closeCalls++
	return nil
}
func (*unstableMetadataPlugin) Health() error { return nil }
func (p *unstableMetadataPlugin) Enabled() bool {
	if p.panicEnable {
		panic("enabled panic")
	}
	return p.enabled
}

func TestLifecycleUsesCachedMetadataAndClosesWhenEnabledPanics(t *testing.T) {
	plugin := &unstableMetadataPlugin{enabled: true}
	Register(plugin)
	defer Unregister("unstable-metadata-test")

	if err := InitAll(context.Background()); err != nil {
		t.Fatalf("InitAll: %v", err)
	}
	plugin.enabled = false
	plugin.panicEnable = true
	if got := HealthAll()["unstable-metadata-test"]; got == nil {
		t.Fatal("expected Enabled panic to be reported by HealthAll")
	}
	if err := CloseAll(context.Background()); err != nil {
		t.Fatalf("CloseAll should close an initialized plugin despite Enabled panic: %v", err)
	}
	if plugin.closeCalls != 1 {
		t.Fatalf("Close calls=%d, want 1 when Enabled changes or panics", plugin.closeCalls)
	}
	if plugin.nameCalls != 1 || plugin.orderCalls != 1 {
		t.Fatalf("metadata methods called after registration: Name=%d Order=%d", plugin.nameCalls, plugin.orderCalls)
	}
}

type repeatInitPlugin struct {
	initCalls  int
	closeCalls int
	label      string
	events     *[]string
}

func (*repeatInitPlugin) Name() string { return "repeat-init-test" }
func (*repeatInitPlugin) Order() int   { return 2 }
func (p *repeatInitPlugin) Init(context.Context) error {
	p.initCalls++
	if p.events != nil {
		*p.events = append(*p.events, p.label+" init")
	}
	return nil
}
func (p *repeatInitPlugin) Close(context.Context) error {
	p.closeCalls++
	if p.events != nil {
		*p.events = append(*p.events, p.label+" close")
	}
	return nil
}
func (*repeatInitPlugin) Health() error { return nil }
func (*repeatInitPlugin) Enabled() bool { return true }

func TestInitAllIsIdempotentForInitializedPlugins(t *testing.T) {
	p := &repeatInitPlugin{}
	Register(p)
	defer Unregister("repeat-init-test")
	if err := InitAll(context.Background()); err != nil {
		t.Fatalf("first InitAll: %v", err)
	}
	if err := InitAll(context.Background()); err != nil {
		t.Fatalf("second InitAll: %v", err)
	}
	if p.initCalls != 1 {
		t.Fatalf("Init calls=%d, want 1", p.initCalls)
	}
	if err := CloseAll(context.Background()); err != nil {
		t.Fatalf("CloseAll: %v", err)
	}
	if p.closeCalls != 1 {
		t.Fatalf("Close calls=%d, want 1", p.closeCalls)
	}
}

func TestInitAllClosesPreviouslyInitializedSameNamePluginBeforeReplacement(t *testing.T) {
	events := make([]string, 0, 4)
	oldPlugin := &repeatInitPlugin{label: "old", events: &events}
	newPlugin := &repeatInitPlugin{label: "new", events: &events}
	Register(oldPlugin)
	defer Unregister("repeat-init-test")
	if err := InitAll(context.Background()); err != nil {
		t.Fatalf("initial InitAll: %v", err)
	}
	Register(newPlugin)
	if err := InitAll(context.Background()); err != nil {
		t.Fatalf("InitAll after replacement: %v", err)
	}
	if oldPlugin.closeCalls != 1 || newPlugin.initCalls != 1 {
		t.Fatalf("old close=%d, replacement init=%d, want 1 each", oldPlugin.closeCalls, newPlugin.initCalls)
	}
	if err := CloseAll(context.Background()); err != nil {
		t.Fatalf("CloseAll: %v", err)
	}
	if newPlugin.closeCalls != 1 {
		t.Fatalf("replacement close=%d, want 1", newPlugin.closeCalls)
	}
	wantEvents := []string{"old init", "old close", "new init", "new close"}
	if len(events) != len(wantEvents) {
		t.Fatalf("lifecycle events=%v, want %v", events, wantEvents)
	}
	for i := range wantEvents {
		if events[i] != wantEvents[i] {
			t.Fatalf("lifecycle events=%v, want %v", events, wantEvents)
		}
	}
}
