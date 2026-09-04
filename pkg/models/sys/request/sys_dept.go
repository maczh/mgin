package request

type CreateDeptReq struct {
	ParentId  uint   `json:"parentId" form:"parentId"`
	Ancestors string `json:"ancestors" form:"ancestors"`
	Name      string `json:"name" form:"name" binding:"required"`
	Sort      int    `json:"sort" form:"sort"`
	Leader    string `json:"leader" form:"leader"`
	Mobile    string `json:"mobile" form:"mobile"`
	Type      string `json:"type" form:"type"`
}

type GetDeptReq struct {
	ID   uint   `json:"id" form:"id"`
	Name string `json:"name" form:"name"`
}

type ListDeptReq struct {
	ParentId uint `json:"parentId" form:"parentId"`
	Page     int  `json:"page" form:"page"`
	PageSize int  `json:"pageSize" form:"pageSize"`
}
