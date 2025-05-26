package request

type CreateResourceReq struct {
	ResourceLevel  int    `json:"menuLevel" form:"menuLevel" binding:"required"` // 资源等级(1主资源 2子资源)
	ParentId       uint   `json:"parentId" form:"parentId"`
	Path           string `json:"path" form:"path" binding:"required"`
	Name           string `json:"name" form:"name" binding:"required"`
	Component      string `json:"component" form:"component" binding:"required"`
	Redirect       string `json:"redirect" form:"redirect"`
	Icon           string `json:"icon" form:"icon"`
	Title          string `json:"title" form:"title" binding:"required"`
	Sort           int    `json:"sort" form:"sort"`
	Hidden         uint8  `json:"hidden" form:"hidden"`
	KeepAlive      uint8  `json:"keepAlive" form:"keepAlive"`
	AlwaysShow     uint8  `json:"alwaysShow" form:"alwaysShow"`
	Breadcrumb     uint8  `json:"breadcrumb" form:"breadcrumb"`
	Affix          uint8  `json:"affix" form:"affix"`
	ActiveResource string `json:"activeResource" form:"activeResource"`
	Status         int    `json:"status" form:"status"`
	ApiId          *uint  `json:"apiId" form:"apiId"` // 资源关联的apiId
}

type GetResourceReq struct {
	ID    uint   `json:"id" form:"id"`
	Title string `json:"title" form:"title"` //资源标题
}

type ListResourceReq struct {
	ParentId  uint   `json:"parentId" form:"parentId"`
	Path      string `json:"path" form:"path"`           //资源路径
	Name      string `json:"name" form:"name"`           //资源名称
	Component string `json:"component" form:"component"` //组件路径
	Title     string `json:"title" form:"title"`         //资源标题
	Status    int    `json:"status" form:"status"`
	Page      int    `json:"page" form:"page"`
	PageSize  int    `json:"pageSize" form:"pageSize"`
}

type GetTreeResourceReq struct {
	ParentId uint `json:"parentId" form:"parentId"` //父级资源id，0表示获取所有一级资源
	ByRole   bool `json:"byRole" form:"byRole"`     // 按角色获取资源
}
