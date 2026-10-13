package vo

import (
	"reflect"
	"time"

	"github.com/maczh/mgin/v2/pkg/models"
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
	return ResponsePage[any]{
		Code: r.Status,
		Data: &PageData[any]{
			List:     r.Data.([]any),
			Total:    int64(r.Page.Total),
			Page:     r.Page.Index,
			PageSize: r.Page.Size,
		},
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
	return ResponsePage[any]{
		Code: SUCCESS,
		Data: &PageData[any]{
			List: func() []any {
				// Check if data is already a []any
				if anySlice, ok := data.([]any); ok {
					return anySlice
				}

				// Use reflection to handle typed slices
				val := reflect.ValueOf(data)
				if val.Kind() != reflect.Slice {
					return nil // or handle error case
				}

				// Convert each element to any
				anyList := make([]any, val.Len())
				for i := 0; i < val.Len(); i++ {
					anyList[i] = val.Index(i).Interface()
				}
				return anyList
			}(),
			Total:    int64(page.Total),
			Page:     page.Index,
			PageSize: page.Size,
		},
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
					anyList := make([]any, len(r.Data.List))
					for i, v := range r.Data.List {
						anyList[i] = v
					}
					return anyList
				}(),
				Total:    r.Data.Total,
				Page:     r.Data.Page,
				PageSize: r.Data.PageSize,
			},
			Msg:     r.Msg,
			SrvTime: r.SrvTime,
		}
	}
}

func ResponseToT[T any](result Response[any]) Response[T] {
	return Response[T]{
		Code:    result.Code,
		Data:    result.Data.(T),
		Msg:     result.Msg,
		SrvTime: result.SrvTime,
	}
}

func ResponsePageToT[T any](result ResponsePage[any]) ResponsePage[T] {
	return ResponsePage[T]{
		Code: result.Code,
		Data: &PageData[T]{
			List: func() []T {
				anyList := make([]T, len(result.Data.List))
				for i, v := range result.Data.List {
					anyList[i] = v.(T)
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
