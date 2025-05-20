package sys

// SysApi 系统接口表
// 记录后端所有API接口的权限信息
type SysApi struct {
	ID          uint   `gorm:"primaryKey;comment:接口ID" json:"id"`
	Name        string `gorm:"type:varchar(100);unique;not null;comment:API接口名称" json:"name"`
	APIPath     string `gorm:"type:varchar(255);not null;uniqueIndex:api_unique;comment:接口路径(支持通配符)" json:"api_path"`
	Method      string `gorm:"type:varchar(10);not null;default:'GET';uniqueIndex:api_unique;comment:请求方法(GET|POST|PUT|DELETE...)" json:"method"`
	Description string `gorm:"type:varchar(255);comment:接口描述" json:"description"`
	APIGroup    string `gorm:"type:varchar(50);index;comment:接口分组(模块名称)" json:"api_group"`
	Enabled     bool   `gorm:"type:tinyint(1);default:1;index;comment:启用状态(1启用 0禁用)" json:"enabled"`
	NeedAuth    bool   `gorm:"type:tinyint(1);default:1;comment:是否需要认证(1是 0否)" json:"need_auth"`
	NeedLog     bool   `gorm:"type:tinyint(1);default:1;comment:是否需要记录日志" json:"need_log"`
	BaseModel

	// 关联关系
	Roles []SysRole `gorm:"-" json:"roles;omitempty"`
}

func (SysApi) TableName() string {
	return "sys_api"
}
