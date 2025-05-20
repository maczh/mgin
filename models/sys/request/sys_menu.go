package request

type CreateMenuReq struct {
	ParentID   uint   `json:"parentId" form:"parentId"`
	Path       string `json:"path" form:"path" binding:"required"`
	Name       string `json:"name" form:"name" binding:"required"`
	Component  string `json:"component" form:"component" binding:"required"`
	Redirect   string `json:"redirect" form:"redirect"`
	Icon       string `json:"icon" form:"icon"`
	Title      string `json:"title" form:"title" binding:"required"`
	Sort       int    `json:"sort" form:"sort"`
	Hidden     bool   `json:"hidden" form:"hidden"`
	KeepAlive  bool   `json:"keepAlive" form:"keepAlive"`
	AlwaysShow bool   `json:"alwaysShow" form:"alwaysShow"`
	Breadcrumb bool   `json:"breadcrumb" form:"breadcrumb"`
	Affix      bool   `json:"affix" form:"affix"`
	ActiveMenu string `json:"activeMenu" form:"activeMenu"`
	Status     int    `json:"status" form:"status"`
}

type GetMenuReq struct {
	Id   uint   `json:"id" form:"id"`
	Path string `json:"path" form:"path"`
}

type ListMenuReq struct {
	ParentID uint `json:"parentId" form:"parentId"`
	Status   int  `json:"status" form:"status"`
	Page     int  `json:"page" form:"page"`
	PageSize int  `json:"pageSize" form:"pageSize"`
}
