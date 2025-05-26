package sys

// SysSocial 社会化关系模型
type SysSocial struct {
	ID           uint64 `gorm:"column:id;type:bigint unsigned;primaryKey;autoIncrement;comment:主键" json:"id"`           // 主键
	UserID       uint64 `gorm:"column:user_id;type:bigint unsigned;index;comment:用户ID" json:"userId"`                   // 用户ID
	AuthID       string `gorm:"column:auth_id;type:varchar(255);not null;index:idx_auth;comment:第三方唯一ID" json:"authId"` // 第三方唯一ID
	Source       string `gorm:"column:source;type:varchar(50);not null;index:idx_auth;comment:用户来源平台" json:"source"`    // 用户来源平台
	AccessToken  string `gorm:"column:access_token;type:varchar(1024);not null;comment:访问令牌" json:"accessToken"`        // 访问令牌
	ExpireIn     int    `gorm:"column:expire_in;type:int;default:0;comment:令牌有效期（秒，部分平台可能没有）" json:"expireIn"`          // 令牌有效期（秒，部分平台可能没有）
	RefreshToken string `gorm:"column:refresh_token;type:varchar(1024);comment:刷新令牌（部分平台可能没有）" json:"refreshToken"`     // 刷新令牌（部分平台可能没有）
	OpenID       string `gorm:"column:open_id;type:varchar(255);comment:开放ID" json:"openId"`                            // 开放ID
	UserName     string `gorm:"column:user_name;type:varchar(255);comment:第三方账号" json:"userName"`                       // 第三方账号
	NickName     string `gorm:"column:nick_name;type:varchar(255);comment:第三方昵称" json:"nickName"`                       // 第三方昵称
	Email        string `gorm:"column:email;type:varchar(255);comment:第三方邮箱" json:"email"`                              // 第三方邮箱
	Avatar       string `gorm:"column:avatar;type:varchar(1024);comment:第三方头像" json:"avatar"`                           // 第三方头像
	AccessCode   string `gorm:"column:access_code;type:varchar(512);comment:授权码（部分平台可能没有）" json:"accessCode"`           // 授权码（部分平台可能没有）
	UnionID      string `gorm:"column:union_id;type:varchar(255);comment:联合ID" json:"unionId"`                          // 联合ID
	Scope        string `gorm:"column:scope;type:varchar(255);comment:授权范围（部分平台可能没有）" json:"scope"`                     // 授权范围（部分平台可能没有）
	TokenType    string `gorm:"column:token_type;type:varchar(50);comment:令牌类型（部分平台可能没有）" json:"tokenType"`             // 令牌类型（部分平台可能没有）
	IDToken      string `gorm:"column:id_token;type:varchar(1024);comment:ID令牌（部分平台可能没有）" json:"idToken"`               // ID令牌（部分平台可能没有）
	BaseModel
}

// TableName 指定表名
func (SysSocial) TableName() string {
	return "sys_social"
}
