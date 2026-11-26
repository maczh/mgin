package vo

import (
	"reflect"
	"time"

	"github.com/maczh/mgin/models"
)

type Response[T any] struct {
	Code    int       `json:"code"`
	Data    T         `json:"data"`
	Msg     string    `json:"msg"`
	SrvTime time.Time `json:"srvTime"`
}

type ResponsePage[T any] struct {
	Code    int          `json:"code"`
	Msg     string       `json:"msg"`
	SrvTime time.Time    `json:"srvTime"`
	Data    *PageData[T] `json:"data"`
}

type PageData[T any] struct {
	List     []T   `json:"list"`
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"pageSize"`
}

const (
	ERROR   = 500
	SUCCESS = 1
	UnLogin = 999
)

func ToResponse(r models.Result[any]) Response[any] {
	return Response[any]{
		Code:    r.Status,
		Data:    r.Data,
		Msg:     r.Msg,
		SrvTime: time.Now(),
	}
}

func ToResponsePage(r models.Result[any]) ResponsePage[any] {
	page := &PageData[any]{}
	if r.Page != nil {
		page.Total, page.Page, page.PageSize = int64(r.Page.Total), r.Page.Index, r.Page.Size
	}
	page.List = anySlice(r.Data)
	return ResponsePage[any]{
		Code:    r.Status,
		Data:    page,
		Msg:     r.Msg,
		SrvTime: time.Now(),
	}
}

func Success(data any) Response[any] {
	return Response[any]{
		Code:    SUCCESS,
		Data:    data,
		Msg:     "success",
		SrvTime: time.Now(),
	}
}

func SuccessT[T any](data T) Response[T] {
	return Response[T]{
		Code:    SUCCESS,
		Data:    data,
		Msg:     "success",
		SrvTime: time.Now(),
	}
}
func SuccessPage(data any, page *models.ResultPage) ResponsePage[any] {
	pageData := &PageData[any]{}
	if page != nil {
		pageData.Total, pageData.Page, pageData.PageSize = int64(page.Total), page.Index, page.Size
	}
	pageData.List = anySlice(data)
	return ResponsePage[any]{
		Code:    SUCCESS,
		Data:    pageData,
		Msg:     "success",
		SrvTime: time.Now(),
	}
}

func Error(status int, msg string) Response[any] {
	return Response[any]{
		Code:    status,
		Data:    nil,
		Msg:     msg,
		SrvTime: time.Now(),
	}
}

func ErrorT[T any](status int, msg string) Response[T] {
	return Response[T]{
		Code:    status,
		Msg:     msg,
		SrvTime: time.Now(),
	}

}

func ErrorPage(status int, msg string) ResponsePage[any] {
	return ResponsePage[any]{
		Code:    status,
		Data:    nil,
		Msg:     msg,
		SrvTime: time.Now(),
	}
}

func ErrorPageT[T any](status int, msg string) ResponsePage[T] {
	return ResponsePage[T]{
		Code:    status,
		Data:    nil,
		Msg:     msg,
		SrvTime: time.Now(),
	}
}

func (r Response[T]) ToAny() Response[any] {
	if r.Code != SUCCESS {
		return Response[any]{
			Code:    r.Code,
			Data:    nil,
			Msg:     r.Msg,
			SrvTime: r.SrvTime,
		}
	} else {
		return Response[any]{
			Code:    r.Code,
			Data:    r.Data,
			Msg:     r.Msg,
			SrvTime: r.SrvTime,
		}
	}
}

func (r *ResponsePage[T]) ToAny() ResponsePage[any] {
	if r == nil {
		return ResponsePage[any]{Code: ERROR, Msg: "response is nil", SrvTime: time.Now()}
	}
	if r.Code != SUCCESS {
		return ResponsePage[any]{
			Code:    r.Code,
			Data:    nil,
			Msg:     r.Msg,
			SrvTime: r.SrvTime,
		}
	} else {
		return ResponsePage[any]{
			Code: r.Code,
			Data: &PageData[any]{
				List: func() []any {
					if r.Data == nil {
						return nil
					}
					anyList := make([]any, len(r.Data.List))
					for i, v := range r.Data.List {
						anyList[i] = v
					}
					return anyList
				}(),
				Total:    pageTotal(r.Data),
				Page:     pageIndex(r.Data),
				PageSize: pageSize(r.Data),
			},
			Msg:     r.Msg,
			SrvTime: r.SrvTime,
		}
	}
}

func pageTotal[T any](page *PageData[T]) int64 {
	if page == nil {
		return 0
	}
	return page.Total
}
func pageIndex[T any](page *PageData[T]) int {
	if page == nil {
		return 0
	}
	return page.Page
}
func pageSize[T any](page *PageData[T]) int {
	if page == nil {
		return 0
	}
	return page.PageSize
}

func ResponseToT[T any](result Response[any]) Response[T] {
	var data T
	if result.Data != nil {
		data, _ = result.Data.(T)
	}
	return Response[T]{
		Code:    result.Code,
		Data:    data,
		Msg:     result.Msg,
		SrvTime: result.SrvTime,
	}
}

func ResponsePageToT[T any](result ResponsePage[any]) ResponsePage[T] {
	if result.Data == nil {
		return ResponsePage[T]{Code: result.Code, Msg: result.Msg, SrvTime: result.SrvTime}
	}
	return ResponsePage[T]{
		Code: result.Code,
		Data: &PageData[T]{
			List: func() []T {
				anyList := make([]T, len(result.Data.List))
				for i, v := range result.Data.List {
					converted, ok := v.(T)
					if ok {
						anyList[i] = converted
					}
				}
				return anyList
			}(),
			Total:    result.Data.Total,
			Page:     result.Data.Page,
			PageSize: result.Data.PageSize,
		},
		Msg:     result.Msg,
		SrvTime: result.SrvTime,
	}
}

func anySlice(data any) []any {
	if values, ok := data.([]any); ok {
		return values
	}
	value := reflect.ValueOf(data)
	if !value.IsValid() || (value.Kind() != reflect.Slice && value.Kind() != reflect.Array) {
		return nil
	}
	items := make([]any, value.Len())
	for i := 0; i < value.Len(); i++ {
		items[i] = value.Index(i).Interface()
	}
	return items
}
