package sys

import "time"

// BaseModel 共享属性
type BaseModel struct {
	DelFlag  bool      `gorm:"type:tinyint(1);size:1;default:0;comment:删除标记;column:del_flag;" json:"delFlag"`
	CreateAt time.Time `gorm:"type:datetime;default:null;comment:创建日期" time_format:"2006-01-02 15:04:05" json:"createAt,omit-zero"`
	UpdateAt time.Time `gorm:"type:datetime;default:null;comment:更新日期" time_format:"2006-01-02 15:04:05" json:"updateAt,omit-zero"`
	UpdateBy string    `gorm:"type:string;size:32;comment:更新者;" json:"updateBy"`
	CreateBy string    `gorm:"type:string;size:32;comment:创建者;" json:"createBy"`
	TenantId int64     `gorm:"type:string;size:32;comment:租户id;" json:"tenantId" form:"tenantId"`
}
