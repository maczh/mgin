package vo

import (
	"testing"

	"github.com/maczh/mgin/v2/pkg/models"
)

func TestResponseConversionsHandleNilAndMismatchedData(t *testing.T) {
	var nilPage *ResponsePage[int]
	if got := nilPage.ToAny(); got.Code != ERROR {
		t.Fatalf("nil page response=%+v", got)
	}
	if got := ResponseToT[string](Response[any]{Code: SUCCESS, Data: 123}); got.Data != "" {
		t.Fatalf("mismatched response data=%q", got.Data)
	}
	page := ResponsePage[any]{Code: SUCCESS, Data: &PageData[any]{List: []any{"ok", 7}, Total: 2}}
	converted := ResponsePageToT[string](page)
	if converted.Data == nil || len(converted.Data.List) != 2 || converted.Data.List[0] != "ok" || converted.Data.List[1] != "" {
		t.Fatalf("page conversion=%+v", converted)
	}
	if got := SuccessPage([]int{1, 2}, nil); got.Data == nil || len(got.Data.List) != 2 {
		t.Fatalf("nil metadata should not panic or drop data: %+v", got)
	}
	if got := ToResponsePage(models.Result[any]{Data: []int{1, 2}}); got.Data == nil || len(got.Data.List) != 2 {
		t.Fatalf("typed slice conversion=%+v", got)
	}
}
