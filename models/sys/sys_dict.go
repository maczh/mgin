package sys

// SysDictData 字典数据
type SysDict struct {
	// 主键ID
	ID int `json:"id" gorm:"primaryKey;autoIncrement;type:int;comment:主键ID"`
	// 字典类型
	Type string `json:"type" gorm:"type:varchar(50);not null;index:idx_type;comment:字典类型"`
	// 父级ID
	ParentID int `json:"parent_id" gorm:"type:int;default:0;comment:父级ID"`
	// 字典名称
	Name string `json:"name" gorm:"type:varchar(100);not null;comment:字典中文名称"`
	// 字典键名
	Key string `json:"key" gorm:"type:varchar(50);not null;index:idx_key;comment:字典键名"`
	// 字典值
	Value string `json:"value" gorm:"type:varchar(100);not null;comment:字典值"`
	// 排序
	Sort int `json:"sort" gorm:"type:int;default:0;comment:排序"`
	// 备注
	Remark string `json:"remark" gorm:"type:varchar(255);comment:备注"`
	BaseModel
}

func (e SysDict) TableName() string {
	return "sys_dict"
}
