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
// @Summary 创建用户扩展信息
// @Description 创建用户的扩展信息，如部门、角色、职位等
// @Tags 用户扩展信息
// @Accept json
// @Produce json
// @Param createReq body request.CreateSysUserExtReq true "创建用户扩展信息请求参数"
// @Success 200 {object} models.Result[any] "创建成功，返回创建的用户扩展信息"
// @Failure 500 {object} models.Result[any] "参数绑定失败或创建用户扩展信息失败"
// @Router /sys/user-ext/create [post]
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
// @Summary 更新用户扩展信息
// @Description 更新用户已有的扩展信息，如部门、角色、职位等
// @Tags 用户扩展信息
// @Accept json
// @Produce json
// @Param updateReq body sys.SysUserExt true "更新用户扩展信息请求参数"
// @Success 200 {object} models.Result[any] "更新成功"
// @Failure 500 {object} models.Result[any] "参数绑定失败、获取用户扩展信息失败或更新用户扩展信息失败"
// @Router /sys/user-ext/update [put]
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
// @Summary 获取单个用户扩展信息
// @Description 根据用户 ID 获取单个用户的扩展信息
// @Tags 用户扩展信息
// @Accept json
// @Produce json
// @Param getReq query request.GetSysUserExtReq true "获取用户扩展信息请求参数，包含用户 ID"
// @Success 200 {object} models.Result[any] "获取成功，返回用户扩展信息"
// @Failure 500 {object} models.Result[any] "参数绑定失败或获取用户扩展信息失败"
// @Router /sys/user-ext/get [get]
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
// @Summary 删除单个用户扩展信息
// @Description 根据用户 ID 删除单个用户的扩展信息
// @Tags 用户扩展信息
// @Accept json
// @Produce json
// @Param deleteReq query request.GetSysUserExtReq true "删除用户扩展信息请求参数，包含用户 ID"
// @Success 200 {object} models.Result[any] "删除成功"
// @Failure 500 {object} models.Result[any] "参数绑定失败或删除用户扩展信息失败"
// @Router /sys/user-ext/delete [delete]
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
// @Summary 获取用户扩展信息列表
// @Description 根据查询条件获取用户扩展信息的列表，支持分页
// @Tags 用户扩展信息
// @Accept json
// @Produce json
// @Param listReq query request.ListSysUserExtReq true "获取用户扩展信息列表请求参数"
// @Success 200 {object} models.Result[any] "获取成功，返回用户扩展信息列表和总数"
// @Failure 500 {object} models.Result[any] "参数绑定失败或获取用户扩展信息列表失败"
// @Router /sys/user-ext/list [get]
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
