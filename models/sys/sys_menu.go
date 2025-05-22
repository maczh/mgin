package sys

// SysMenu 系统菜单表
// 用于存储前端路由和后端权限的菜单结构
type SysMenu struct {
	ID         uint   `gorm:"primaryKey;comment:菜单ID" json:"id"`
	ParentID   uint   `gorm:"index;comment:父菜单ID(0表示根菜单)" json:"parent_id"`
	Path       string `gorm:"type:varchar(255);index;comment:路由路径(需符合Vue Router规范)" json:"path"`
	Name       string `gorm:"type:varchar(50);index;comment:路由名称" json:"name"`
	Component  string `gorm:"type:varchar(255);comment:组件路径(views目录下的路径)" json:"component"`
	Redirect   string `gorm:"type:varchar(255);comment:重定向路径" json:"redirect"`
	Icon       string `gorm:"type:varchar(50);comment:菜单图标(组件名称)" json:"icon"`
	Title      string `gorm:"type:varchar(50);not null;comment:菜单标题(显示名称)" json:"title"`
	Sort       int    `gorm:"type:int;default:0;index;comment:排序序号(越小越靠前)" json:"sort"`
	Hidden     uint8  `gorm:"type:tinyint;default:0;comment:是否隐藏(0显示 1隐藏)" json:"hidden"`
	KeepAlive  uint8  `gorm:"type:tinyint;default:1;comment:是否缓存(0否 1是)" json:"keep_alive"`
	AlwaysShow uint8  `gorm:"type:tinyint;default:0;comment:是否总是显示(即使只有一个子菜单)" json:"always_show"`
	Breadcrumb uint8  `gorm:"type:tinyint;default:1;comment:是否显示面包屑" json:"breadcrumb"`
	Affix      uint8  `gorm:"type:tinyint;default:0;comment:是否固定到标签页" json:"affix"`
	ActiveMenu string `gorm:"type:varchar(255);comment:激活菜单的路径" json:"active_menu"`
	Status     int    `gorm:"type:tinyint;default:1;index;comment:菜单状态(1正常 0停用)" json:"status"`
	BaseModel
	// 关联关系
	Children []SysMenu `gorm:"-;comment:子菜单列表" json:"children;omitempty"` // 不作为数据库字段
	Roles    []SysRole `gorm:"-" json:"roles;omitempty"`                  // 多对多关联
	Parent   *SysMenu  `gorm:"-" json:"parent;omitempty"`                 // 自关联
}

func (SysMenu) TableName() string {
	return "sys_menu"
}
