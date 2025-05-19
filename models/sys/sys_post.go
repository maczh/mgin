package sys

// SysPost 岗位信息
type SysPost struct {
	Id       int64  `gorm:"type:bigint(20);primary_key;auto_increment;岗位ID;"     json:"id"  form:"id"`
	PostCode string `gorm:"type:varchar(64);comment:岗位编码;" json:"postCode" form:"postCode"`
	PostName string `gorm:"type:varchar(50);comment:岗位名称;" json:"postName" form:"postName"`
	Sort     int    `gorm:"type:int(11);comment:显示顺序;" json:"sort" form:"sort"`
	Status   string `gorm:"type:char(1);comment:状态（0正常 1停用）;" json:"status" form:"status"`
	Remark   string `gorm:"type:varchar(500);comment:备注;" json:"remark" form:"remark"`
	BaseModel
}

func (e *SysPost) TableName() string {
	return "sys_post"
}
