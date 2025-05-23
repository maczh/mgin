package sys

// SysRoleMenu 角色菜单权限信息模型
type SysRoleMenu struct {
	ID     uint `gorm:"primaryKey;type:uint;comment:权限ID" json:"id"`          // 权限ID，主键
	RoleId uint `gorm:"type:uint;not null;index;comment:角色ID" json:"role_id"` // 角色ID，不能为空
	MenuId uint `gorm:"type:uint;not null;comment:菜单ID" json:"menu_id"`       // 菜单ID，不能为空

	Role *SysRole `gorm:"-" json:"role"` // 关联的角色信息
	Menu *SysMenu `gorm:"-" json:"menu"` // 关联的菜单信息
	BaseModel
}

func (SysRoleMenu) TableName() string {
	return "sys_role_menu"
}
