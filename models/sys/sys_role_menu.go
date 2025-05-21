package sys

// SysRoleMenu 角色菜单权限信息模型
type SysRoleMenu struct {
	// 权限ID，主键
	ID uint `gorm:"primaryKey;type:uint;comment:权限ID" json:"id"`
	// 角色ID，不能为空
	RoleId uint `gorm:"type:uint;not null;index;comment:角色ID" json:"role_id"`
	// 关联的角色信息
	Role *SysRole `gorm:"-" json:"role,omitempty"`
	// 菜单代码，不能为空
	MenuId uint `gorm:"type:uint;not null;comment:菜单ID" json:"menu_id"`
	// 关联的API接口信息
	Menu *SysMenu `gorm:"-" json:"menu,omitempty"`
	// 创建时间，自动记录创建时的时间
	BaseModel
}

func (SysRoleMenu) TableName() string {
	return "sys_role_menu"
}
