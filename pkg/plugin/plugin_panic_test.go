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
