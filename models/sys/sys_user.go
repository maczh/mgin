package sys

import "time"

// SysUser 用户信息
type SysUser struct {
	Id        int64      `gorm:"type:bigint(20);primary_key;auto_increment;用户ID;"     json:"id"  form:"id"`
	DeptId    int64      `gorm:"type:bigint(20);comment:部门ID;" json:"deptId" form:"deptId"`
	LoginName string     `gorm:"type:varchar(30);comment:登录账号;" json:"loginName" form:"loginName"`
	UserName  string     `gorm:"type:varchar(30);comment:用户昵称;" json:"userName" form:"userName"`
	UserType  string     `gorm:"type:varchar(2);comment:用户类型（00系统用户）;" json:"userType" form:"userType"`
	Email     string     `gorm:"type:varchar(50);comment:用户邮箱;" json:"email" form:"email"`
	Mobile    string     `gorm:"type:varchar(11);comment:手机号码;" json:"mobile" form:"mobile"`
	Sex       uint8      `gorm:"type:tinyint;comment:用户性别（1男 2女 3未知）;" json:"sex" form:"sex"`
	Avatar    string     `gorm:"type:varchar(100);comment:头像路径;" json:"avatar" form:"avatar"`
	Password  string     `gorm:"type:varchar(50);comment:密码;" json:"password" form:"password"`
	Status    string     `gorm:"type:char(1);comment:帐号状态（0正常 1停用）;" json:"status" form:"status"`
	LoginIp   string     `gorm:"type:varchar(50);comment:最后登陆IP;" json:"loginIp" form:"loginIp"`
	LoginDate *time.Time `gorm:"type:datetime;comment:最后登陆时间;" json:"loginDate" form:"loginDate" time_format:"2006-01-02 15:04:05"`
	Remark    string     `gorm:"type:varchar(500);comment:备注;" json:"remark"   form:"remark"`
	BaseModel
	//临时属性
	RoleKeys string `gorm:"-"  json:"roleKeys"`
}

// 映射数据表
func (e *SysUser) TableName() string {
	return "sys_user"
}
