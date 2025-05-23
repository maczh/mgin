package sys

import (
	"time"
)

// BaseModel 共享属性
type BaseModel struct {
	DelFlag  uint      `gorm:"type:tinyint;default:0;index;comment:删除标记;column:del_flag;" json:"delFlag"` // 删除标记
	CreateAt time.Time `gorm:"type:datetime;autoCreateTime;comment:创建日期" json:"createAt"`                 // 创建日期
	UpdateAt time.Time `gorm:"type:datetime;autoUpdateTime;comment:更新日期" json:"updateAt"`                 // 更新日期
	UpdateBy string    `gorm:"type:string;size:32;comment:更新者;" json:"updateBy"`                          // 更新者
	CreateBy string    `gorm:"type:string;size:32;comment:创建者;" json:"createBy"`                          // 创建者
	TenantId int64     `gorm:"type:string;size:32;comment:租户id;" json:"tenantId" form:"tenantId"`         // 租户id
}
