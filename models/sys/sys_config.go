package sys

// SysConfig 系统配置表,相当于动态INI配置文件
// 该表用于存储系统的配置信息，包括模块、名称、键名、值和描述等字段。
// 可以根据需要添加更多字段，例如配置类型、配置范围等。
// 该表可以通过模块和键名进行查询和更新操作，方便管理系统的配置信息。
// 注意：该表中的配置项名称是唯一的，不允许重复。
type SysConfig struct {
	ID          int64  `gorm:"primaryKey;autoIncrement;comment:主键ID" json:"id"`
	Module      string `gorm:"type:varchar(100);comment:所属功能模块;index" json:"module" binding:"required"`
	Name        string `gorm:"type:varchar(50);not null;index;comment:配置名" json:"name" binding:"required"`
	Key         string `gorm:"type:varchar(100);uniqueIndex;not null;comment:配置项名称，即配置变量名，唯一" json:"key" binding:"required"`
	Value       string `gorm:"type:text;comment:该配置保存值" json:"value" binding:"required"`
	Description string `gorm:"type:varchar(255);comment:配置项描述" json:"description"`
}

func (SysConfig) TableName() string {
	return "sys_config"
}
