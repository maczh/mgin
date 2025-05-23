package sys

import (
	"time"
)

// SysUser 用户信息
type SysUser struct {
	ID        int64      `gorm:"type:bigint(20);primary_key;auto_increment;用户ID;"     json:"id"  form:"id"`             // 主键ID
	LoginName string     `gorm:"type:varchar(30);not null;uniqueIndex;comment:登录账号;" json:"loginName" form:"loginName"` // 登录账号(全局唯一)
	NickName  string     `gorm:"type:varchar(30);comment:用户昵称;" json:"nickName" form:"nickName"`                        // 用户昵称或姓名
	UserType  string     `gorm:"type:varchar(10);index;comment:用户类型（根据业务自由定义）;" json:"userType" form:"userType"`        // 用户类型(根据业务自由定义)
	Email     string     `gorm:"type:varchar(50);index;comment:用户邮箱;" json:"email" form:"email"`                        // 用户邮箱(全局唯一)
	Mobile    string     `gorm:"type:varchar(11);index;comment:手机号码;" json:"mobile" form:"mobile"`                      // 手机号码(全局唯一)
	Sex       uint8      `gorm:"type:tinyint;comment:用户性别（1男 2女 3未知）;" json:"sex" form:"sex"`                           // 用户性别(1:男 2:女 3:未知)
	Avatar    string     `gorm:"type:varchar(100);comment:头像路径;" json:"avatar" form:"avatar"`                           // 头像路径
	Password  string     `gorm:"type:varchar(50);comment:密码;" json:"password" form:"password"`                          // 密码
	Status    uint8      `gorm:"type:tinyint;index;default:1;comment:帐号状态（1正常 2停用）;" json:"status" form:"status"`       // 状态(1:正常 2:停用)
	LoginIp   string     `gorm:"type:varchar(50);comment:最后登陆IP;" json:"loginIp" form:"loginIp"`                        // 最后登陆IP
	LoginDate *time.Time `gorm:"type:datetime;comment:最后登陆时间;" json:"loginDate" form:"loginDate"`                       // 最后登陆时间
	Remark    string     `gorm:"type:varchar(500);comment:备注;" json:"remark"   form:"remark"`                           // 备注
	BaseModel
}

// 映射数据表
func (e SysUser) TableName() string {
	return "sys_user"
}
