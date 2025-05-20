package sys

// SysDept 系统部门表
type SysDept struct {
	Id        uint   `gorm:"type:uint;primary_key;auto_increment;部门id;" json:"id"`
	ParentId  uint   `gorm:"type:uint;index;comment:父部门id;" json:"parentId"`
	Ancestors string `gorm:"type:varchar(100);comment:祖级列表;index:idx_ancestors" json:"ancestors"`
	Name      string `gorm:"type:varchar(30);unique;not null;comment:部门名称;" json:"name"`
	Sort      int    `gorm:"type:int(10);comment:显示顺序;" json:"sort"`
	Leader    string `gorm:"type:varchar(20);comment:负责人;" json:"leader"`
	Mobile    string `gorm:"type:varchar(11);comment:联系电话;" json:"mobile"`
	DeptType  string `gorm:"type:varchar(50);index;default:0;comment:组织类型;" json:"deptType"`
	Status    uint8  `gorm:"type:tinyint;default:1;comment:部门状态（1正常 0停用）;" json:"status"`
	BaseModel
	ParentName string     `gorm:"-" json:"parentName"`
	Children   []*SysDept `gorm:"-" json:"children"`
}

func (e SysDept) TableName() string {
	return "sys_dept"
}
