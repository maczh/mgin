package request

type CreateSysUserExtReq struct {
	UserId uint `json:"userId" form:"userId" binding:"required"`
	ListSysUserExtReq
}

type ListSysUserExtReq struct {
	DepartmentId int `json:"departmentId" form:"departmentId"`
	RoleId       int `json:"roleId" form:"roleId"`
	PositionId   int `json:"positionId" form:"positionId"`
	Page         int `json:"page" form:"page"`
	PageSize     int `json:"pageSize" form:"pageSize"`
}

type GetSysUserExtReq struct {
	UserId uint `json:"userId" form:"userId" binding:"required"`
}
