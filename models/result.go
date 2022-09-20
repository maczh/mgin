package models

/**
通用返回结果类
*/
type Result struct {
	Status int         `json:"status" bson:"status"`
	Msg    string      `json:"msg" bson:"msg"`
	Data   interface{} `json:"data" bson:"data"`
	Page   *ResultPage `json:"page" bson:"page"`
}

type ResultPage struct {
	Count int `json:"count"` //总页数
	Index int `json:"index"` //页号
	Size  int `json:"size"`  //分页大小
	Total int `json:"total"` //总记录数
}

func Success(d interface{}) Result {
	result := Result{
		Status: 1,
		Msg:    "Success",
		Data:   d,
		Page:   nil,
	}
	return result
}

func SuccessWithMsg(msg string, d interface{}) Result {
	result := Result{
		Status: 1,
		Msg:    msg,
		Data:   d,
		Page:   nil,
	}
	return result
}

func SuccessWithPage(d interface{}, count, index, size, total int) Result {
	page := new(ResultPage)
	page.Count = count
	page.Index = index
	page.Size = size
	page.Total = total
	result := Result{
		Status: 1,
		Msg:    "Success",
		Data:   d,
		Page:   page,
	}
	return result
}

func Error(s int, m string) Result {
	result := Result{
		Status: s,
		Msg:    m,
		Data:   nil,
		Page:   nil,
	}
	return result
}
