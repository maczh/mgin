package vo

import (
	"testing"

	"github.com/maczh/mgin/models"
)

func TestPageConversionsHandleNilAndMismatchedInputs(t *testing.T) {
	if got := ToResponsePage(models.Result[any]{}); got.Data == nil || got.Data.List != nil {
		t.Fatalf("ToResponsePage() = %#v", got)
	}
	if got := SuccessPage([]int{1, 2}, nil); got.Data == nil || len(got.Data.List) != 2 {
		t.Fatalf("SuccessPage() = %#v", got)
	}
	var response *ResponsePage[int]
	if got := response.ToAny(); got.Code != ERROR {
		t.Fatalf("nil ToAny() = %#v", got)
	}
	if got := ResponseToT[int](Response[any]{Data: "wrong type"}); got.Data != 0 {
		t.Fatalf("mismatched ResponseToT() = %#v", got)
	}
	if got := ResponsePageToT[int](ResponsePage[any]{Data: &PageData[any]{List: []any{"wrong"}}}); got.Data.List[0] != 0 {
		t.Fatalf("mismatched ResponsePageToT() = %#v", got)
	}
}
