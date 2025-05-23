package sys

// SysUserExt 扩展的SysUser模型
type SysUserExt struct {
	ID           int64 `gorm:"type:bigint(20);primary_key;auto_increment;ID;" json:"id" form:"id"`            // 主键ID
	UserId       int64 `gorm:"type:bigint(20);unique;not null;comment:用户ID;" json:"userId" form:"userId"`     // 用户ID
	DepartmentId int64 `gorm:"type:bigint(20);index;comment:所属部门ID;" json:"departmentId" form:"departmentId"` // 所属部门ID
	PositionId   int64 `gorm:"type:bigint(20);index;comment:所属岗位ID;" json:"positionId" form:"positionId"`     // 所属岗位ID
	RoleId       int64 `gorm:"type:bigint(20);index;comment:所属角色ID;" json:"roleId" form:"roleId"`             // 所属角色ID
}

func (e SysUserExt) TableName() string {
	return "sys_user_ext"
}
