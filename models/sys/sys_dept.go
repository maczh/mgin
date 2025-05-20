package sys

// SysDept 系统部门表
type SysDept struct {
	Id        uint   `gorm:"type:uint;primary_key;auto_increment;部门id;" json:"id"`
	ParentId  uint   `gorm:"type:uint;comment:父部门id;" json:"parentId"`
	Ancestors string `gorm:"type:varchar(50);comment:祖级列表;index:idx_ancestors" json:"ancestors"`
	DeptName  string `gorm:"type:varchar(30);comment:部门名称;" json:"deptName"`
	Sort      int    `gorm:"type:int(10);comment:显示顺序;" json:"sort"`
	Leader    string `gorm:"type:varchar(20);comment:负责人;" json:"leader"`
	Phone     string `gorm:"type:varchar(11);comment:联系电话;" json:"phone"`
	DeptType  string `gorm:"type:varchar(50);default:0;comment:组织类型;" json:"deptType"`
	Status    uint8  `gorm:"type:tinyint;comment:部门状态（0正常 1停用）;" json:"status"`
	BaseModel
	ParentName string `gorm:"-" json:"parentName"`
}

func (e SysDept) TableName() string {
	return "sys_dept"
}
