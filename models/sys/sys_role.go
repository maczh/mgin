package sys

// SysRole 角色表结构
type SysRole struct {
	ID          uint   `gorm:"primaryKey;comment:角色ID"`
	RoleName    string `gorm:"type:varchar(50);not null;uniqueIndex;comment:角色名称"`
	RoleIdent   string `gorm:"type:varchar(50);not null;uniqueIndex;comment:角色标识符(英文)"`
	IsEnable    bool   `gorm:"type:tinyint(1);default:1;index;comment:启用状态(1启用 0禁用)"`
	Description string `gorm:"type:varchar(255);comment:角色描述"`

	BaseModel
	// 关联关系
	Employees []SysUser `gorm:"-" json:"users;omitempty"`
	Menus     []SysMenu `gorm:"-" json:"menus;omitempty"`
	APIs      []SysApi  `gorm:"-" json:"apis;omitempty"`
}

func (SysRole) TableName() string {
	return "sys_role"
}
