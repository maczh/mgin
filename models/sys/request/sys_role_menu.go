package request

type BindRoleMenuReq struct {
	RoleId  int64   `json:"roleId" form:"roleId" binding:"required"`
	MenuIds []int64 `json:"menuIds" form:"menuIds" binding:"required"`
}

type ListRoleMenuReq struct {
	RoleId int64 `json:"roleId" form:"roleId" binding:"required"`
}
