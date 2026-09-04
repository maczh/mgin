package request

type BindRoleApiReq struct {
	RoleId int64   `json:"roleId" form:"roleId" binding:"required"`
	ApiIds []int64 `json:"apiIds" form:"apiIds" binding:"required"`
}

type ListRoleApiReq struct {
	RoleId int64 `json:"roleId" form:"roleId" binding:"required"`
}
