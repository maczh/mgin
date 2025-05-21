package sys

// SysPost 岗位信息
type SysPost struct {
	Id       int64  `gorm:"type:bigint(20);primary_key;auto_increment;岗位ID;"     json:"id"  form:"id"`
	PostCode string `gorm:"type:varchar(64);uniqueIndex;comment:岗位编码;" json:"post_code" form:"post_code"`
	PostName string `gorm:"type:varchar(50);uniqueIndex;comment:岗位名称;" json:"post_name" form:"post_name"`
	DeptId   int64  `gorm:"type:bigint(20);index;comment:所属部门ID;" json:"dept_id" form:"dept_id"`
	Sort     int    `gorm:"type:int(11);comment:显示顺序;" json:"sort" form:"sort"`
	Status   uint8  `gorm:"type:tinyint;comment:状态（1正常 0停用）;" json:"status" form:"status"`
	Remark   string `gorm:"type:varchar(500);comment:备注;" json:"remark" form:"remark"`
	BaseModel
}

func (e SysPost) TableName() string {
	return "sys_post"
}
