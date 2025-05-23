package sys

// SysRoleApi 角色API权限信息模型
type SysRoleApi struct {
	ID     uint `gorm:"primaryKey;type:uint;comment:权限ID" json:"id"`          // 权限ID
	RoleId uint `gorm:"type:uint;not null;index;comment:角色ID" json:"role_id"` // 角色ID
	ApiId  uint `gorm:"type:uint;not null;comment:API接口ID" json:"api_id"`     // API接口ID

	Role *SysRole `gorm:"-" json:"role"` // 关联的角色信息
	Api  *SysApi  `gorm:"-" json:"api"`  // 关联的API接口信息
	BaseModel
}

func (SysRoleApi) TableName() string {
	return "sys_role_api"
}
