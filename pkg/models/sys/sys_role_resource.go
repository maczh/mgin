package sys

// SysRoleResource 角色资源权限信息模型
type SysRoleResource struct {
	ID         uint `gorm:"primaryKey;type:uint;comment:权限ID" json:"id"`          // 权限ID，主键
	RoleId     uint `gorm:"type:uint;not null;index;comment:角色ID" json:"role_id"` // 角色ID，不能为空
	ResourceId uint `gorm:"type:uint;not null;comment:资源ID" json:"resource_id"`   // 资源ID，不能为空

	Role     *SysRole     `gorm:"-" json:"role"`     // 关联的角色信息
	Resource *SysResource `gorm:"-" json:"resource"` // 关联的资源信息
	BaseModel
}

func (SysRoleResource) TableName() string {
	return "sys_role_resource"
}
