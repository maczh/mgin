package request

type CreateDictReq struct {
	Type     string `json:"type" form:"type" binding:"required"`
	ParentId int    `json:"parentId" form:"parentId"`
	Name     string `json:"name" form:"name" binding:"required"`
	Key      string `json:"key" form:"key" binding:"required"`
	Value    string `json:"value" form:"value" binding:"required"`
	Sort     int    `json:"sort" form:"sort"`
	Remark   string `json:"remark" form:"remark"`
}

type GetDictReq struct {
	ID   uint   `json:"id" form:"id"`
	Type string `json:"type" form:"type"`
	Name string `json:"name" form:"name"`
	Key  string `json:"key" form:"key"`
}

type ListDictReq struct {
	Type     string `json:"type" form:"type"`
	ParentId int    `json:"parentId" form:"parentId"`
	Name     string `json:"name" form:"name"`
	Page     int    `json:"page" form:"page"`
	PageSize int    `json:"pageSize" form:"pageSize"`
}
