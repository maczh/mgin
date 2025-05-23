package request

type CreateRoleReq struct {
	Name        string `json:"name" form:"name" binding:"required"`
	Ident       string `json:"ident" form:"ident" binding:"required"`
	Description string `json:"description" form:"description"`
}

type GetRoleReq struct {
	ID    uint   `json:"id" form:"id"`
	Name  string `json:"name" form:"name"`
	Ident string `json:"ident" form:"ident"`
}

type ListRoleReq struct {
	Page     int `json:"page" form:"page"`
	PageSize int `json:"pageSize" form:"pageSize"`
}
