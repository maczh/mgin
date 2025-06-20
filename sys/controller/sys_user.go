package controller

import (
	"github.com/gin-gonic/gin"
	"github.com/maczh/mgin/models"
	"github.com/maczh/mgin/models/sys"
	"github.com/maczh/mgin/models/sys/request"
	"github.com/maczh/mgin/sys/service"
)

type sysUserController struct{}

// Register 用户注册
// @Summary 用户注册
// @Description 新用户进行注册操作
// @Tags 用户管理
// @Accept json
// @Produce json
// @Param registerReq body request.RegisterReq true "用户注册请求参数"
// @Success 200 {object} models.Result[any] "注册成功，返回用户信息"
// @Failure 500 {object} models.Result[any] "请求参数错误或注册失败"
// @Router /api/v1/register [post]
func (s *sysUserController) Register(c *gin.Context) models.Result[any] {
	var req request.RegisterReq
	if err := c.ShouldBindJSON(&req); err != nil {
		return models.Error(500, "请求参数错误: "+err.Error())
	}
	if req.Captcha == nil {
		return models.Error(500, "验证码不能为空")
	}
	user, err := service.SysUser.WithContext(c).Register(req)
	if err != nil {
		return models.Error(500, err.Error())
	}
	return models.Success[any](user)
}

// Login 用户登录
// @Summary 用户登录
// @Description 已注册用户进行登录操作
// @Tags 用户管理
// @Accept json
// @Produce json
// @Param loginReq body request.LoginReq true "用户登录请求参数"
// @Success 200 {object} models.Result[any] "登录成功，返回 token"
// @Failure 500 {object} models.Result[any] "请求参数错误或登录失败"
// @Router /api/v1/login [post]
func (s *sysUserController) Login(c *gin.Context) models.Result[any] {
	var req request.LoginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		return models.Error(500, "请求参数错误: "+err.Error())
	}
	if req.Captcha == nil {
		return models.Error(500, "验证码不能为空")
	}
	token, err := service.SysUser.WithContext(c).Login(req)
	if err != nil {
		return models.Error(500, err.Error())
	}
	return models.Success[any](map[string]any{"token": token})
}

// Add 新增用户
// @Summary 新增用户
// @Description 管理员新增用户信息
// @Tags 用户管理
// @Accept json
// @Produce json
// @Param Authorization header string true "用户令牌"
// @Param user body sys.SysUser true "新增用户信息"
// @Success 200 {object} models.Result[any] "新增成功，返回用户信息"
// @Failure 500 {object} models.Result[any] "请求参数错误或新增用户失败"
// @Router /api/v1/sys/users/add [post]
func (s *sysUserController) Add(c *gin.Context) models.Result[any] {
	var req sys.SysUser
	if err := c.ShouldBindJSON(&req); err != nil {
		return models.Error(500, "请求参数错误: "+err.Error())
	}
	if req.Password == "" {
		req.Password = "123456"
	}
	user, err := service.SysUser.WithContext(c).New(&req)
	if err != nil {
		return models.Error(500, err.Error())
	}
	user.Password = "******"
	return models.Success[any](user)
}

// Update 修改用户
// @Summary 修改用户
// @Description 管理员修改用户信息
// @Tags 用户管理
// @Accept json
// @Produce json
// @Param Authorization header string true "用户令牌"
// @Param user body sys.SysUser true "修改后的用户信息"
// @Success 200 {object} models.Result[any] "修改成功"
// @Failure 500 {object} models.Result[any] "请求参数错误、用户 ID 为空或修改用户失败"
// @Router /api/v1/sys/users/update [post]
func (s *sysUserController) Update(c *gin.Context) models.Result[any] {
	var req sys.SysUser
	if err := c.ShouldBindJSON(&req); err != nil {
		return models.Error(500, "请求参数错误: "+err.Error())
	}
	if req.ID == 0 {
		return models.Error(500, "用户ID不能为空")
	}
	err := service.SysUser.WithContext(c).Update(&req)
	if err != nil {
		return models.Error(500, err.Error())
	}
	return models.Success[any](nil)
}

// Delete 删除用户
// @Summary 删除用户
// @Description 管理员删除用户信息
// @Tags 用户管理
// @Accept json
// @Produce json
// @Param Authorization header string true "用户令牌"
// @Param deleteReq body request.DeleteByIdReq true "删除用户请求参数，包含用户 ID"
// @Success 200 {object} models.Result[any] "删除成功"
// @Failure 500 {object} models.Result[any] "请求参数错误或删除用户失败"
// @Router /api/v1/sys/users/del [post]
func (s *sysUserController) Delete(c *gin.Context) models.Result[any] {
	var req request.DeleteByIdReq
	if err := c.ShouldBindJSON(&req); err != nil {
		return models.Error(500, "请求参数错误: "+err.Error())
	}
	err := service.SysUser.WithContext(c).Delete(req.ID)
	if err != nil {
		return models.Error(500, err.Error())
	}
	return models.Success[any](nil)
}

// Get 获取用户信息
// @Summary 获取用户信息
// @Description 根据请求参数获取用户详细信息
// @Tags 用户管理
// @Accept json
// @Produce json
// @Param Authorization header string true "用户令牌"
// @Param getReq query request.GetSysUserReq true "获取用户信息请求参数"
// @Success 200 {object} models.Result[any] "获取成功，返回用户信息"
// @Failure 500 {object} models.Result[any] "请求参数错误或获取用户信息失败"
// @Router /api/v1/sys/users/get [get]
func (s *sysUserController) Get(c *gin.Context) models.Result[any] {
	var req request.GetSysUserReq
	if err := c.ShouldBindQuery(&req); err != nil {
		return models.Error(500, "请求参数错误: "+err.Error())
	}
	user, err := service.SysUser.GetSysUser(req)
	if err != nil {
		return models.Error(500, err.Error())
	}
	user.Password = "******"
	return models.Success[any](user)
}

// List 获取用户列表
// @Summary 获取用户列表
// @Description 根据请求参数获取用户分页列表
// @Tags 用户管理
// @Accept json
// @Produce json
// @Param Authorization header string true "用户令牌"
// @Param listReq query request.ListSysUserReq true "获取用户列表请求参数"
// @Success 200 {object} models.ResultPage[any] "获取成功，返回用户列表和分页信息"
// @Failure 500 {object} models.Result[any] "请求参数错误或获取用户列表失败"
// @Router /api/v1/sys/users/list [get]
func (s *sysUserController) List(c *gin.Context) models.Result[any] {
	var req request.ListSysUserReq
	if err := c.ShouldBindQuery(&req); err != nil {
		return models.Error(500, "请求参数错误: "+err.Error())
	}
	list, pages, err := service.SysUser.ListSysUser(req)
	if err != nil {
		return models.Error(500, err.Error())
	}
	return models.SuccessPage[any](list, pages)
}

// VerifyToken 验证token
// @Summary 验证token
// @Description 验证用户提供的 token 是否有效
// @Tags 用户管理
// @Accept json
// @Produce json
// @Param verifyReq body request.VerifyTokenReq true "验证 token 请求参数"
// @Success 200 {object} models.Result[any] "验证成功，返回验证结果和 claims"
// @Failure 500 {object} models.Result[any] "请求参数错误或验证失败"
// @Router /api/v1/token [post]
func (s *sysUserController) VerifyToken(c *gin.Context) models.Result[any] {
	var req request.VerifyTokenReq
	if err := c.ShouldBindJSON(&req); err != nil {
		return models.Error(500, "请求参数错误: "+err.Error())
	}
	success, claims, err := service.SysUser.WithContext(c).VerifyJwt(req.Token)
	if err != nil {
		return models.Error(500, err.Error())
	}
	return models.Success[any](map[string]any{"success": success, "claims": claims})
}

// ChangePassword 修改密码
// @Summary 修改密码
// @Description 用户修改自己的登录密码
// @Tags 用户管理
// @Accept json
// @Produce json
// @Param Authorization header string true "用户令牌"
// @Param changePwdReq body request.ChangePasswordReq true "修改密码请求参数"
// @Success 200 {object} models.Result[any] "修改成功"
// @Failure 500 {object} models.Result[any] "请求参数错误或修改密码失败"
// @Router /api/v1/sys/users/pwd [post]
func (s *sysUserController) ChangePassword(c *gin.Context) models.Result[any] {
	var req request.ChangePasswordReq
	if err := c.ShouldBindJSON(&req); err != nil {
		return models.Error(500, "请求参数错误: "+err.Error())
	}
	err := service.SysUser.WithContext(c).ChangePassword(req)
	if err != nil {
		return models.Error(500, err.Error())
	}
	return models.Success[any](nil)
}

// ChangeStatus 修改状态
// @Summary 修改状态
// @Description 管理员修改用户的状态
// @Tags 用户管理
// @Accept json
// @Produce json
// @Param Authorization header string true "用户令牌"
// @Param changeStatusReq body request.ChangeStatusReq true "修改用户状态请求参数"
// @Success 200 {object} models.Result[any] "修改成功"
// @Failure 500 {object} models.Result[any] "请求参数错误或修改用户状态失败"
// @Router /api/v1/sys/users/status [post]
func (s *sysUserController) ChangeStatus(c *gin.Context) models.Result[any] {
	var req request.ChangeStatusReq
	if err := c.ShouldBindJSON(&req); err != nil {
		return models.Error(500, "请求参数错误: "+err.Error())
	}
	err := service.SysUser.WithContext(c).ChangeStatus(req.ID, uint8(*req.Status))
	if err != nil {
		return models.Error(500, err.Error())
	}
	return models.Success[any](nil)
}

// Logout 退出登录
// @Summary 退出登录
// @Description 用户退出当前登录状态
// @Tags 用户管理
// @Accept json
// @Produce json
// @Param Authorization header string true "用户令牌"
// @Success 200 {object} models.Result[any] "退出成功"
// @Failure 500 {object} models.Result[any] "退出登录失败"
// @Router /api/v1/sys/users/logout [post]
func (s *sysUserController) Logout(c *gin.Context) models.Result[any] {
	err := service.SysUser.WithContext(c).Logout()
	if err != nil {
		return models.Error(500, err.Error())
	}
	return models.Success[any](nil)
}
