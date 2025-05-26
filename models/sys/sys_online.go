package sys

// SysUserOnline 当前在线会话
type SysUserOnline struct {
	ID            int64  `gorm:"column:id;type:bigint(20);primary_key;auto_increment;ID;" json:"id" form:"id"` // 主键ID
	TokenID       string `gorm:"column:token_id;type:varchar(255);primaryKey;comment:会话编号" json:"tokenId"`     // 会话编号, md5(jwtToken)
	UserID        uint64 `gorm:"column:user_id;type:bigint unsigned;index;comment:用户ID" json:"userId"`         // 用户ID
	UserName      string `gorm:"column:user_name;type:varchar(255);index;comment:用户名称" json:"userName"`        // 用户名称
	ClientKey     string `gorm:"column:client_key;type:varchar(255);index;comment:客户端标识" json:"clientKey"`     // 客户端标识
	DeviceType    string `gorm:"column:device_type;type:varchar(50);comment:设备类型" json:"deviceType"`           // 设备类型
	IpAddr        string `gorm:"column:ip_addr;type:varchar(50);comment:登录IP地址" json:"ipAddr"`                 // 登录IP地址
	LoginLocation string `gorm:"column:login_location;type:varchar(255);comment:登录地址" json:"loginLocation"`    // 登录地址
	Browser       string `gorm:"column:browser;type:varchar(50);comment:浏览器类型" json:"browser"`                 // 浏览器类型
	Os            string `gorm:"column:os;type:varchar(50);comment:操作系统" json:"os"`                            // 操作系统
	LoginTime     int64  `gorm:"column:login_time;type:bigint;comment:登录时间" json:"loginTime"`                  // 登录时间
}

// TableName 指定表名
func (SysUserOnline) TableName() string {
	return "sys_user_online"
}
