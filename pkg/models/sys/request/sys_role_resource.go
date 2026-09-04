package request

type BindRoleResourceReq struct {
	RoleId      int64   `json:"roleId" form:"roleId" binding:"required"`
	ResourceIds []int64 `json:"resourceIds" form:"resourceIds" binding:"required"`
}

type ListRoleResourceReq struct {
	RoleId int64 `json:"roleId" form:"roleId" binding:"required"`
}
