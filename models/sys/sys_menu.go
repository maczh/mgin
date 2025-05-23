package sys

// SysMenu 系统菜单表
// 用于存储前端路由和后端权限的菜单结构
type SysMenu struct {
	ID         uint   `gorm:"primaryKey;comment:菜单ID" json:"id"`                                   // 菜单ID
	MenuLevel  int    `gorm:"type:tinyint;default:1;comment:菜单等级(1主菜单 2子菜单)" json:"menu_level"`    // 菜单等级(1主菜单 2子菜单)
	ParentId   uint   `gorm:"index;comment:父菜单ID(0表示根菜单)" json:"parent_id"`                        // 父菜单ID(0表示根菜单)
	Path       string `gorm:"type:varchar(255);index;comment:路由路径(需符合Vue Router规范)" json:"path"`   // 路由路径(需符合Vue Router规范)
	Name       string `gorm:"type:varchar(50);index;comment:路由名称" json:"name"`                     // 路由名称
	Component  string `gorm:"type:varchar(255);comment:组件路径(views目录下的路径)" json:"component"`        // 组件路径(views目录下的路径)
	Redirect   string `gorm:"type:varchar(255);comment:重定向路径" json:"redirect"`                     // 重定向路径
	Icon       string `gorm:"type:varchar(50);comment:菜单图标(组件名称)" json:"icon"`                     // 菜单图标(组件名称)
	Title      string `gorm:"type:varchar(50);not null;comment:菜单标题(显示名称)" json:"title"`           // 菜单标题(显示名称)
	Sort       int    `gorm:"type:int;default:0;index;comment:排序序号(越小越靠前)" json:"sort"`            // 排序序号(越小越靠前)
	Hidden     uint8  `gorm:"type:tinyint;default:0;comment:是否隐藏(0显示 1隐藏)" json:"hidden"`          // 是否隐藏(0显示 1隐藏)
	KeepAlive  uint8  `gorm:"type:tinyint;default:1;comment:是否缓存(0否 1是)" json:"keep_alive"`        // 是否缓存(0否 1是)
	AlwaysShow uint8  `gorm:"type:tinyint;default:0;comment:是否总是显示(即使只有一个子菜单)" json:"always_show"` // 是否总是显示(即使只有一个子菜单)
	Breadcrumb uint8  `gorm:"type:tinyint;default:1;comment:是否显示面包屑" json:"breadcrumb"`            // 是否显示面包屑
	Affix      uint8  `gorm:"type:tinyint;default:0;comment:是否固定到标签页" json:"affix"`                // 是否固定到标签页
	ActiveMenu string `gorm:"type:varchar(255);comment:激活菜单的路径" json:"active_menu"`                // 激活菜单的路径
	Status     int    `gorm:"type:tinyint;default:1;index;comment:菜单状态(1正常 0停用)" json:"status"`    // 菜单状态(1正常 0停用)
	BaseModel
	// 关联关系
	Children []SysMenu `gorm:"-;comment:子菜单列表" json:"children"` // 不作为数据库字段
	Roles    []SysRole `gorm:"-" json:"roles"`                  // 多对多关联
	Parent   *SysMenu  `gorm:"-" json:"parent"`                 // 自关联
}

func (SysMenu) TableName() string {
	return "sys_menu"
}
