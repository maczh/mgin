package request

type GetSysConfigReq struct {
	ID  uint   `json:"id" form:"id"`   // ID
	Key string `json:"key" form:"key"` // 配置键名
}

type ListSysConfigReq struct {
	Module   string `json:"module" form:"module"`     // 模块
	Page     int    `json:"page" form:"page"`         // 页码
	PageSize int    `json:"pageSize" form:"pageSize"` // 每页数量
}
