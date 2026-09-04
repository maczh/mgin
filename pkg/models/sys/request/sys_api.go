package request

type GetApiReq struct {
	ID uint `json:"id" form:"id" binding:"required"`
}

type GetUriReq struct {
	Uri string `json:"uri" form:"uri" binding:"required"`
}

type ListApiReq struct {
	Group    string `json:"group" form:"group"`
	NeedAuth int    `json:"needAuth" form:"needAuth"`
	Page     int    `json:"page" form:"page"`
	PageSize int    `json:"pageSize" form:"pageSize"`
}
