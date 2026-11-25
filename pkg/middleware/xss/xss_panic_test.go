package xss

import (
	"bytes"
	"testing"
)

func TestJSONArraysRejectScalarElementsAndAllowEmpty(t *testing.T) {
	mw := &XssMw{}
	if _, err := mw.jsonToStringMap(bytes.Buffer{}, []any{"scalar"}); err == nil {
		t.Fatal("expected non-object array element error")
	}
	got, err := mw.jsonToStringMap(bytes.Buffer{}, []any{})
	if err != nil || got.String() != "[]" {
		t.Fatalf("empty array output=%q, err=%v", got.String(), err)
	}
}
