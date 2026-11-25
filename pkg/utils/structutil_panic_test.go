package utils

import "testing"

func TestAnyToMapAcceptsHeterogeneousMapsAndNil(t *testing.T) {
	if got := AnyToMap(nil); len(got) != 0 {
		t.Fatalf("nil input: %v", got)
	}
	got := AnyToMap(map[string]any{"number": 42, "nil": nil, "nested": map[string]int{"x": 1}})
	if got["number"] != "42" || got["nil"] != "<nil>" || got["nested"] == "" {
		t.Fatalf("unexpected conversion: %#v", got)
	}
	if got := AnyToMap(map[int]string{1: "ignored"}); len(got) != 0 {
		t.Fatalf("non-string keys should be ignored: %v", got)
	}
}
