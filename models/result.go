package models

/*
*
通用返回结果类
*/
type Result[T any] struct {
	Status int         `json:"status" bson:"status"`
	Msg    string      `json:"msg" bson:"msg"`
	Data   T           `json:"data" bson:"data"`
	Page   *ResultPage `json:"page" bson:"page"`
}

type ResultPage struct {
	Count int `json:"count"` //总页数
	Index int `json:"index"` //页号
	Size  int `json:"size"`  //分页大小
	Total int `json:"total"` //总记录数
}

func Success[T any](data T) Result[T] {
	result := Result[T]{
		Status: 1,
		Msg:    "Success",
		Data:   data,
		Page:   nil,
	}
	return result
}

func SuccessWithMsg[T any](msg string, data T) Result[T] {
	result := Result[T]{
		Status: 1,
		Msg:    msg,
		Data:   data,
		Page:   nil,
	}
	return result
}

func SuccessWithPage[T any](data T, count, index, size, total int) Result[T] {
	page := new(ResultPage)
	page.Count = count
	page.Index = index
	page.Size = size
	page.Total = total
	result := Result[T]{
		Status: 1,
		Msg:    "Success",
		Data:   data,
		Page:   page,
	}
	return result
}

func Error(s int, m string) Result[any] {
	result := Result[any]{
		Status: s,
		Msg:    m,
		Data:   nil,
		Page:   nil,
	}
	return result
}
