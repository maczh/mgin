package sys

// SysApi 系统接口表
// 记录后端所有API接口的权限信息
type SysApi struct {
	ID          uint   `gorm:"primaryKey;comment:接口ID" json:"id"`                                                                                 // 主键ID
	Name        string `gorm:"type:varchar(100);unique;not null;comment:API接口名称" json:"name"`                                                     // API接口名称
	APIPath     string `gorm:"type:varchar(255);not null;uniqueIndex:api_unique;comment:接口路径(支持通配符)" json:"api_path"`                             // API路径(支持通配符)
	Method      string `gorm:"type:varchar(10);not null;default:'GET';uniqueIndex:api_unique;comment:请求方法(GET|POST|PUT|DELETE...)" json:"method"` // 请求方法(GET|POST|PUT|DELETE...)
	Description string `gorm:"type:varchar(255);comment:接口描述" json:"description"`                                                                 // 接口描述
	APIGroup    string `gorm:"type:varchar(50);index;comment:接口分组(模块名称)" json:"api_group"`                                                        // 接口分组(模块名称)
	Enabled     uint8  `gorm:"type:tinyint;default:1;index;comment:启用状态(1启用 0禁用)" json:"enabled"`                                                 // 启用状态(1启用 0禁用)
	NeedAuth    uint8  `gorm:"type:tinyint;comment:是否需要认证(1是 0否);column:need_auth" json:"need_auth"`                                              // 是否需要JWT认证(1是 0否)
	NeedLog     uint8  `gorm:"type:tinyint;default:1;comment:是否需要记录日志" json:"need_log"`                                                           // 是否需要记录日志
	BaseModel
}

func (SysApi) TableName() string {
	return "sys_api"
}
