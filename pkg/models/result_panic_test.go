package models

import "testing"

func TestToAnyRejectsMismatchedAndFailedResultData(t *testing.T) {
	got := ToAny[string](Result[any]{Status: 1, Data: 42, Msg: "ok"})
	if got.Data != "" {
		t.Fatalf("mismatched data=%q", got.Data)
	}
	got = ToAny[string](Result[any]{Status: 500, Data: "must not escape"})
	if got.Data != "" || got.Status != 500 {
		t.Fatalf("failed result conversion=%+v", got)
	}
}
