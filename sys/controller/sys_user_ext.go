package controller

import (
	"github.com/gin-gonic/gin"
	"github.com/maczh/mgin/models"
	"github.com/maczh/mgin/models/sys"
	"github.com/maczh/mgin/models/sys/request"
	"github.com/maczh/mgin/sys/service"
)

type sysUserExtController struct{}

// Create 创建用户扩展信息
func (s *sysUserExtController) Create(c *gin.Context) models.Result[any] {
	var req request.CreateSysUserExtReq
	if err := c.ShouldBindJSON(&req); err != nil {
		return models.Error(500, "参数绑定失败")
	}
	userExt, err := service.SysUserExt.Save(req)
	if err != nil {
		return models.Error(500, "创建用户扩展信息失败: "+err.Error())
	}
	return models.Success[any](userExt)
}

// Update 更新用户扩展信息
func (s *sysUserExtController) Update(c *gin.Context) models.Result[any] {
	var req sys.SysUserExt
	if err := c.ShouldBindJSON(&req); err != nil {
		return models.Error(500, "参数绑定失败")
	}
	userExt, err := service.SysUserExt.Get(req.UserId)
	if err != nil {
		return models.Error(500, "获取用户扩展信息失败: "+err.Error())
	}
	userExt.DepartmentId = req.DepartmentId
	userExt.RoleId = req.RoleId
	userExt.PositionId = req.PositionId
	err = service.SysUserExt.Update(userExt)
	if err != nil {
		return models.Error(500, "更新用户扩展信息失败: "+err.Error())
	}
	return models.Success[any](nil)
}

// Get 获取单个用户扩展信息
func (s *sysUserExtController) Get(c *gin.Context) models.Result[any] {
	var req request.GetSysUserExtReq
	if err := c.ShouldBindQuery(&req); err != nil {
		return models.Error(500, "参数绑定失败")
	}
	userExt, err := service.SysUserExt.Get(int64(req.UserId))
	if err != nil {
		return models.Error(500, "获取用户扩展信息失败: "+err.Error())
	}
	return models.Success[any](userExt)
}

// Delete 删除单个用户扩展信息
func (s *sysUserExtController) Delete(c *gin.Context) models.Result[any] {
	var req request.GetSysUserExtReq
	if err := c.ShouldBindQuery(&req); err != nil {
		return models.Error(500, "参数绑定失败")
	}
	err := service.SysUserExt.Delete(int64(req.UserId))
	if err != nil {
		return models.Error(500, "删除用户扩展信息失败: "+err.Error())
	}
	return models.Success[any](nil)
}

// List 获取用户扩展信息列表
func (s *sysUserExtController) List(c *gin.Context) models.Result[any] {
	var req request.ListSysUserExtReq
	if err := c.ShouldBindQuery(&req); err != nil {
		return models.Error(500, "参数绑定失败")
	}
	list, total, err := service.SysUserExt.List(req)
	if err != nil {
		return models.Error(500, "获取用户扩展信息列表失败: "+err.Error())
	}
	return models.Success[any](map[string]any{"list": list, "total": total})
}
