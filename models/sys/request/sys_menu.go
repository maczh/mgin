package request

type CreateMenuReq struct {
	MenuLevel  int    `json:"menuLevel" form:"menuLevel" binding:"required"` // 菜单等级(1主菜单 2子菜单)
	ParentId   uint   `json:"parentId" form:"parentId"`
	Path       string `json:"path" form:"path" binding:"required"`
	Name       string `json:"name" form:"name" binding:"required"`
	Component  string `json:"component" form:"component" binding:"required"`
	Redirect   string `json:"redirect" form:"redirect"`
	Icon       string `json:"icon" form:"icon"`
	Title      string `json:"title" form:"title" binding:"required"`
	Sort       int    `json:"sort" form:"sort"`
	Hidden     uint8  `json:"hidden" form:"hidden"`
	KeepAlive  uint8  `json:"keepAlive" form:"keepAlive"`
	AlwaysShow uint8  `json:"alwaysShow" form:"alwaysShow"`
	Breadcrumb uint8  `json:"breadcrumb" form:"breadcrumb"`
	Affix      uint8  `json:"affix" form:"affix"`
	ActiveMenu string `json:"activeMenu" form:"activeMenu"`
	Status     int    `json:"status" form:"status"`
}

type GetMenuReq struct {
	ID    uint   `json:"id" form:"id"`
	Title string `json:"title" form:"title"` //菜单标题
}

type ListMenuReq struct {
	ParentId  uint   `json:"parentId" form:"parentId"`
	Path      string `json:"path" form:"path"`           //菜单路径
	Name      string `json:"name" form:"name"`           //菜单名称
	Component string `json:"component" form:"component"` //组件路径
	Title     string `json:"title" form:"title"`         //菜单标题
	Status    int    `json:"status" form:"status"`
	Page      int    `json:"page" form:"page"`
	PageSize  int    `json:"pageSize" form:"pageSize"`
}

type GetTreeMenuReq struct {
	ParentId uint `json:"parentId" form:"parentId"` //父级菜单id，0表示获取所有一级菜单
	ByRole   bool `json:"byRole" form:"byRole"`     // 按角色获取菜单
}
