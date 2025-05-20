package controller

import (
	"github.com/gin-gonic/gin"
	"github.com/maczh/mgin/models"
	"github.com/maczh/mgin/models/sys"
	"github.com/maczh/mgin/models/sys/request"
	"github.com/maczh/mgin/sys/service"
)

type sysRoleController struct{}

// Add 新增角色
func (s *sysRoleController) Add(c *gin.Context) models.Result[any] {
	var req request.CreateRoleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		return models.Error(500, "参数绑定失败")
	}
	role, err := service.SysRole.Add(req)
	if err != nil {
		return models.Error(500, "新增角色失败: "+err.Error())
	}
	return models.Success[any](role)
}

// Get 获取角色
func (s *sysRoleController) Get(c *gin.Context) models.Result[any] {
	var req request.GetRoleReq
	if err := c.ShouldBindQuery(&req); err != nil {
		return models.Error(500, "参数绑定失败:"+err.Error())
	}
	role, err := service.SysRole.Get(req)
	if err != nil {
		return models.Error(500, "获取角色失败: "+err.Error())
	}
	return models.Success[any](role)
}

// Update 更新角色
func (s *sysRoleController) Update(c *gin.Context) models.Result[any] {
	var req *sys.SysRole
	if err := c.ShouldBindJSON(&req); err != nil {
		return models.Error(500, "参数绑定失败")
	}
	err := service.SysRole.Update(req)
	if err != nil {
		return models.Error(500, "更新角色失败: "+err.Error())
	}
	return models.Success[any](nil)
}

// Delete 删除角色
func (s *sysRoleController) Delete(c *gin.Context) models.Result[any] {
	var req request.DeleteByIdReq
	if err := c.ShouldBindQuery(&req); err != nil {
		return models.Error(500, "参数绑定失败")
	}
	if err := service.SysRole.Delete(uint(req.Id)); err != nil {
		return models.Error(500, "删除角色失败: "+err.Error())
	}
	return models.Success[any](nil)
}

// List 获取角色列表
func (s *sysRoleController) List(c *gin.Context) models.Result[any] {
	var req request.ListRoleReq
	if err := c.ShouldBindQuery(&req); err != nil {
		return models.Error(500, "参数绑定失败")
	}
	list, pages, err := service.SysRole.List(req)
	if err != nil {
		return models.Error(500, "获取角色列表失败: "+err.Error())
	}
	return models.SuccessPage[any](list, pages)
}
