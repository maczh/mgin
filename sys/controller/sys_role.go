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
// @Summary 新增角色
// @Description 用于新增一个系统角色
// @Tags 系统角色
// @Accept json
// @Produce json
// @Param Authorization header string true "用户令牌"
// @Param role body request.CreateRoleReq true "角色创建请求体，包含角色名称、标识等信息"
// @Success 200 {object} models.Result[sys.SysRole] "成功新增角色，返回新增的角色信息"
// @Failure 500 {object} models.Result[any] "参数绑定失败或新增角色失败"
// @Router /api/v1/sys/role/add [post]
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
// @Summary 获取角色
// @Description 根据请求参数获取指定角色的信息
// @Tags 系统角色
// @Accept json
// @Produce json
// @Param Authorization header string true "用户令牌"
// @Param role query request.GetRoleReq true "角色查询请求参数，包含角色 ID 等信息"
// @Success 200 {object} models.Result[sys.SysRole] "成功获取角色信息，返回该角色信息"
// @Failure 500 {object} models.Result[any] "参数绑定失败或获取角色失败"
// @Router /api/v1/sys/role/get [get]
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
// @Summary 更新角色
// @Description 更新指定角色的信息
// @Tags 系统角色
// @Accept json
// @Produce json
// @Param Authorization header string true "用户令牌"
// @Param role body sys.SysRole true "角色更新请求体，包含需要更新的角色信息"
// @Success 200 {object} models.Result[any] "成功更新角色信息"
// @Failure 500 {object} models.Result[any] "参数绑定失败或更新角色失败"
// @Router /api/v1/sys/role/update [post]
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
// @Summary 删除角色
// @Description 根据角色 ID 删除指定角色
// @Tags 系统角色
// @Accept json
// @Produce json
// @Param Authorization header string true "用户令牌"
// @Param role query request.DeleteByIdReq true "角色删除请求参数，包含角色 ID"
// @Success 200 {object} models.Result[any] "成功删除角色"
// @Failure 500 {object} models.Result[any] "参数绑定失败或删除角色失败"
// @Router /api/v1/sys/role/del [post]
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
// @Summary 获取角色列表
// @Description 根据请求参数获取角色的分页列表
// @Tags 系统角色
// @Accept json
// @Produce json
// @Param Authorization header string true "用户令牌"
// @Param role query request.ListRoleReq true "角色列表查询请求参数，包含分页信息等"
// @Success 200 {object} models.ResultPage[sys.SysRole] "成功获取角色列表，返回角色列表和分页信息"
// @Failure 500 {object} models.Result[any] "参数绑定失败或获取角色列表失败"
// @Router /api/v1/sys/role/list [get]
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
