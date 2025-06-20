package request

type RegisterReq struct {
	LoginName string            `json:"loginName" form:"loginName" binding:"required"`
	Password  string            `json:"password" form:"password" binding:"required"`
	Email     string            `json:"email" form:"email"`
	Mobile    string            `json:"mobile" form:"mobile"`
	NickName  string            `json:"nickName" form:"nickName"`
	Sex       uint8             `json:"sex" form:"sex"`
	Avatar    string            `json:"avatar" form:"avatar"`
	Captcha   *VerifyCaptchaReq `json:"captcha" form:"captcha" binding:"required"`
}

type LoginReq struct {
	LoginName string            `json:"loginName" form:"loginName" binding:"required"` // 可以是用户名或邮箱或手机号，自动识别
	Password  string            `json:"password" form:"password" binding:"required"`
	Captcha   *VerifyCaptchaReq `json:"captcha" form:"captcha" binding:"required"`
}

type ChangePasswordReq struct {
	LoginName   string `json:"loginName" form:"loginName" binding:"required"`
	OldPassword string `json:"oldPassword" form:"oldPassword" binding:"required"`
	NewPassword string `json:"newPassword" form:"newPassword" binding:"required"`
}

type VerifyTokenReq struct {
	Token string `json:"token" form:"token" binding:"required"`
}

type ListSysUserReq struct {
	Keyword  string `json:"keyword" form:"keyword"`
	Status   uint8  `json:"status" form:"status"`
	UserType string `json:"userType" form:"userType"`
	DeptId   uint64 `json:"deptId" form:"deptId"`
	RoleId   uint64 `json:"roleId" form:"roleId"`
	PostId   uint64 `json:"postId" form:"postId"`
	Page     int    `json:"page" form:"page"`
	PageSize int    `json:"pageSize" form:"pageSize"`
}

type GetSysUserReq struct {
	ID        uint64 `json:"id" form:"id"`
	LoginName string `json:"loginName" form:"loginName"`
	Email     string `json:"email" form:"email"`
	Mobile    string `json:"mobile" form:"mobile"`
	NickName  string `json:"nickName" form:"nickName"`
}

type GetCaptchaReq struct {
	Width  int `json:"width" form:"width" binding:"required"`
	Height int `json:"height" form:"height" binding:"required"`
	Length int `json:"length" form:"length"`
	Type   int `json:"type" form:"type"` //0:数字 1:字母 2:算术
}

type VerifyCaptchaReq struct {
	ID   string `json:"id" form:"id" binding:"required"`
	Code string `json:"code" form:"code" binding:"required"`
}

type DeleteByIdReq struct {
	ID int64 `json:"id" form:"id" binding:"required"`
}

type GetByIdReq struct {
	ID *int64 `json:"id" form:"id" binding:"required"`
}

type ChangeStatusReq struct {
	ID     int64 `json:"id" form:"id" binding:"required"`
	Status *int  `json:"status" form:"status" binding:"required"`
}
