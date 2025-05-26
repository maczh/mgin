package controller

import (
	"github.com/gin-gonic/gin"
	"github.com/maczh/mgin/models"
	"github.com/maczh/mgin/models/sys/request"
	"github.com/maczh/mgin/sys/service"
)

type sysRoleResourceController struct{}

// Bind 绑定角色和资源
// @Summary 绑定角色和资源
// @Description 将指定角色与资源进行绑定操作
// @Tags 角色资源管理
// @Accept json
// @Produce json
// @Param Authorization header string true "用户令牌"
// @Param bindReq body request.BindRoleResourceReq true "角色资源绑定请求参数"
// @Success 200 {object} models.Result[any] "绑定成功"
// @Failure 500 {object} models.Result[any] "参数绑定失败或绑定操作失败"
// @Router /api/v1/sys/role_res/bind [post]
func (sysRoleResourceController) Bind(ctx *gin.Context) models.Result[any] {
	var req request.BindRoleResourceReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		return models.Error(500, err.Error())
	}
	err := service.SysRoleResource.WithContext(ctx).BindRoleResource(req)
	if err != nil {
		return models.Error(500, err.Error())
	}
	return models.Success[any](nil)
}

// List 获取角色的资源列表
// @Summary 获取角色的资源列表
// @Description 根据请求参数获取指定角色关联的资源列表
// @Tags 角色资源管理
// @Accept json
// @Produce json
// @Param Authorization header string true "用户令牌"
// @Param listReq query request.ListRoleResourceReq true "获取角色资源列表请求参数"
// @Success 200 {object} models.Result[any] "获取成功，返回角色关联的资源列表"
// @Failure 500 {object} models.Result[any] "参数绑定失败或获取资源列表失败"
// @Router /api/v1/sys/role_res/list [get]
func (sysRoleResourceController) List(ctx *gin.Context) models.Result[any] {
	var req request.ListRoleResourceReq
	if err := ctx.ShouldBindQuery(&req); err != nil {
		return models.Error(500, err.Error())
	}
	list, err := service.SysRoleResource.ListRoleResource(req)
	if err != nil {
		return models.Error(500, err.Error())
	}
	return models.Success[any](list)
}
