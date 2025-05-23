package controller

import (
	"github.com/gin-gonic/gin"
	"github.com/maczh/mgin/models"
	"github.com/maczh/mgin/models/sys/request"
	"github.com/maczh/mgin/sys/service"
)

type sysRoleMenuController struct{}

// Bind 绑定角色和菜单
// @Summary 绑定角色和菜单
// @Description 将指定角色与菜单进行绑定操作
// @Tags 角色菜单管理
// @Accept json
// @Produce json
// @Param Authorization header string true "用户令牌"
// @Param bindReq body request.BindRoleMenuReq true "角色菜单绑定请求参数"
// @Success 200 {object} models.Result[any] "绑定成功"
// @Failure 500 {object} models.Result[any] "参数绑定失败或绑定操作失败"
// @Router /api/v1/sys/role_menu/bind [post]
func (sysRoleMenuController) Bind(ctx *gin.Context) models.Result[any] {
	var req request.BindRoleMenuReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		return models.Error(500, err.Error())
	}
	err := service.SysRoleMenu.WithContext(ctx).BindRoleMenu(req)
	if err != nil {
		return models.Error(500, err.Error())
	}
	return models.Success[any](nil)
}

// List 获取角色的菜单列表
// @Summary 获取角色的菜单列表
// @Description 根据请求参数获取指定角色关联的菜单列表
// @Tags 角色菜单管理
// @Accept json
// @Produce json
// @Param Authorization header string true "用户令牌"
// @Param listReq query request.ListRoleMenuReq true "获取角色菜单列表请求参数"
// @Success 200 {object} models.Result[any] "获取成功，返回角色关联的菜单列表"
// @Failure 500 {object} models.Result[any] "参数绑定失败或获取菜单列表失败"
// @Router /api/v1/sys/role_menu/list [get]
func (sysRoleMenuController) List(ctx *gin.Context) models.Result[any] {
	var req request.ListRoleMenuReq
	if err := ctx.ShouldBindQuery(&req); err != nil {
		return models.Error(500, err.Error())
	}
	list, err := service.SysRoleMenu.ListRoleMenu(req)
	if err != nil {
		return models.Error(500, err.Error())
	}
	return models.Success[any](list)
}
