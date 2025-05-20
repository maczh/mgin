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
func (s *sysUserController) Register(c *gin.Context) models.Result[any] {
	var req request.RegisterReq
	if err := c.ShouldBindJSON(&req); err != nil {
		return models.Error[any](500, "请求参数错误: "+err.Error())
	}
	user, err := service.SysUser.Register(req)
	if err != nil {
		return models.Error[any](500, err.Error())
	}
	return models.Success[any](user)
}

// Login 用户登录
func (s *sysUserController) Login(c *gin.Context) models.Result[any] {
	var req request.LoginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		return models.Error[any](500, "请求参数错误: "+err.Error())
	}
	token, err := service.SysUser.Login(req)
	if err != nil {
		return models.Error[any](500, err.Error())
	}
	return models.Success[any](map[string]any{"token": token})
}

// Add 新增用户
func (s *sysUserController) Add(c *gin.Context) models.Result[any] {
	var req sys.SysUser
	if err := c.ShouldBindJSON(&req); err != nil {
		return models.Error[any](500, "请求参数错误: "+err.Error())
	}
	if req.Password == "" {
		req.Password = "123456"
	}
	service.SysUser.Ctx = c
	user, err := service.SysUser.New(&req)
	if err != nil {
		return models.Error[any](500, err.Error())
	}
	user.Password = "******"
	return models.Success[any](user)
}

// Update 修改用户
func (s *sysUserController) Update(c *gin.Context) models.Result[any] {
	var req sys.SysUser
	if err := c.ShouldBindJSON(&req); err != nil {
		return models.Error[any](500, "请求参数错误: "+err.Error())
	}
	if req.Id == 0 {
		return models.Error[any](500, "用户ID不能为空")
	}
	service.SysUser.Ctx = c
	err := service.SysUser.Update(&req)
	if err != nil {
		return models.Error[any](500, err.Error())
	}
	return models.Success[any](nil)
}

// Delete 删除用户
func (s *sysUserController) Delete(c *gin.Context) models.Result[any] {
	var req request.DeleteByIdReq
	if err := c.ShouldBindJSON(&req); err != nil {
		return models.Error[any](500, "请求参数错误: "+err.Error())
	}
	service.SysUser.Ctx = c
	err := service.SysUser.Delete(req.Id)
	if err != nil {
		return models.Error[any](500, err.Error())
	}
	return models.Success[any](nil)
}

// Get 获取用户信息
func (s *sysUserController) Get(c *gin.Context) models.Result[any] {
	var req request.GetSysUserReq
	if err := c.ShouldBindQuery(&req); err != nil {
		return models.Error[any](500, "请求参数错误: "+err.Error())
	}
	service.SysUser.Ctx = c
	user, err := service.SysUser.GetSysUser(req)
	if err != nil {
		return models.Error[any](500, err.Error())
	}
	user.Password = "******"
	return models.Success[any](user)
}

// List 获取用户列表
func (s *sysUserController) List(c *gin.Context) models.Result[any] {
	var req request.ListSysUserReq
	if err := c.ShouldBindQuery(&req); err != nil {
		return models.Error[any](500, "请求参数错误: "+err.Error())
	}
	list, pages, err := service.SysUser.ListSysUser(req)
	if err != nil {
		return models.Error[any](500, err.Error())
	}
	return models.SuccessPage[any](list, pages)
}

// VerifyToken 验证token
func (s *sysUserController) VerifyToken(c *gin.Context) models.Result[any] {
	var req request.VerifyTokenReq
	if err := c.ShouldBindJSON(&req); err != nil {
		return models.Error[any](500, "请求参数错误: "+err.Error())
	}
	success, claims, err := service.SysUser.VerifyJwt(req.Token)
	if err != nil {
		return models.Error[any](500, err.Error())
	}
	return models.Success[any](map[string]any{"success": success, "claims": claims})
}

// ChangePassword 修改密码
func (s *sysUserController) ChangePassword(c *gin.Context) models.Result[any] {
	var req request.ChangePasswordReq
	if err := c.ShouldBindJSON(&req); err != nil {
		return models.Error[any](500, "请求参数错误: "+err.Error())
	}
	service.SysUser.Ctx = c
	err := service.SysUser.ChangePassword(req)
	if err != nil {
		return models.Error[any](500, err.Error())
	}
	return models.Success[any](nil)
}

// ChangeStatus 修改状态
func (s *sysUserController) ChangeStatus(c *gin.Context) models.Result[any] {
	var req request.ChangeStatusReq
	if err := c.ShouldBindJSON(&req); err != nil {
		return models.Error[any](500, "请求参数错误: "+err.Error())
	}
	service.SysUser.Ctx = c
	err := service.SysUser.ChangeStatus(req.Id, uint8(req.Status))
	if err != nil {
		return models.Error[any](500, err.Error())
	}
	return models.Success[any](nil)
}

// Logout 退出登录
func (s *sysUserController) Logout(c *gin.Context) models.Result[any] {
	service.SysUser.Ctx = c
	err := service.SysUser.Logout()
	if err != nil {
		return models.Error[any](500, err.Error())
	}
	return models.Success[any](nil)
}
