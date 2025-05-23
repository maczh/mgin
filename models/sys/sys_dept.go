package sys

// SysDept 系统部门表
type SysDept struct {
	ID         uint   `gorm:"type:uint;primary_key;auto_increment;部门id;" json:"id"`                // 主键ID
	ParentId   uint   `gorm:"type:uint;index;comment:父部门id;" json:"parentId"`                      // 父部门id
	Ancestors  string `gorm:"type:varchar(100);comment:祖级列表;index:idx_ancestors" json:"ancestors"` // 祖级列表，以逗号分割
	Name       string `gorm:"type:varchar(30);unique;not null;comment:部门名称;" json:"name"`          // 部门名称
	Sort       int    `gorm:"type:int(10);comment:显示顺序;" json:"sort"`                              // 显示顺序
	Leader     string `gorm:"type:varchar(20);comment:负责人;" json:"leader"`                         // 负责人
	Mobile     string `gorm:"type:varchar(11);comment:联系电话;" json:"mobile"`                        // 联系电话
	DeptType   string `gorm:"type:varchar(50);index;default:0;comment:组织类型;" json:"deptType"`      // 组织类型(0:公司 1:部门)
	Status     uint8  `gorm:"type:tinyint;default:1;comment:部门状态（1正常 0停用）;" json:"status"`         // 部门状态(1:正常 0:停用)
	ParentName string `gorm:"-" json:"parentName"`                                                 // 父部门名称
	BaseModel
	Children []*SysDept `gorm:"-" json:"children"` // 子部门列表
}

func (e SysDept) TableName() string {
	return "sys_dept"
}
