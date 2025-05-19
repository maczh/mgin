package sys

// SysUserExt 扩展的SysUser模型
type SysUserExt struct {
	Id           int64 `gorm:"type:bigint(20);primary_key;auto_increment;ID;"     json:"id"  form:"id"`
	UserId       int64 `gorm:"type:bigint(20);unique;not null;comment:用户ID;" json:"userId" form:"userId"`
	DepartmentId int64 `gorm:"type:bigint(20);comment:所属部门ID;" json:"departmentId" form:"departmentId"`
	PositionId   int64 `gorm:"type:bigint(20);comment:所属岗位ID;" json:"positionId" form:"positionId"`
	RoleId       int64 `gorm:"type:bigint(20);comment:所属角色ID;" json:"roleId" form:"roleId"`
}

func (e *SysUserExt) TableName() string {
	return "sys_user_ext"
}
