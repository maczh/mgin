package sys

// SysDict 字典数据
type SysDict struct {
	ID       int    `json:"id" gorm:"primaryKey;autoIncrement;type:int;comment:主键ID"`          // 主键ID
	Type     string `json:"type" gorm:"type:varchar(50);not null;index:idx_type;comment:字典类型"` // 字典类型
	ParentId int    `json:"parent_id" gorm:"type:int;default:0;comment:父级ID"`                  // 父级ID
	Name     string `json:"name" gorm:"type:varchar(100);not null;index;comment:字典中文名称"`       // 字典中文名称
	Key      string `json:"key" gorm:"type:varchar(50);not null;index:idx_key;comment:字典键名"`   // 字典键名
	Value    string `json:"value" gorm:"type:varchar(100);not null;comment:字典值"`               // 字典值
	Sort     int    `json:"sort" gorm:"type:int;default:0;comment:排序"`                         // 排序
	Remark   string `json:"remark" gorm:"type:varchar(255);comment:备注"`                        // 备注
	BaseModel
}

func (e SysDict) TableName() string {
	return "sys_dict"
}
