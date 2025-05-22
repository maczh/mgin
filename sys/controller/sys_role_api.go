package controller

import (
	"github.com/gin-gonic/gin"
	"github.com/maczh/mgin/models"
	"github.com/maczh/mgin/models/sys/request"
	"github.com/maczh/mgin/sys/service"
)

type sysRoleApiController struct{}

// Bind 绑定角色和API
// @Summary 绑定角色和API
// @Description 将指定角色与 API 进行绑定操作,先全量解绑再批量绑定
// @Tags 角色API绑定
// @Accept json
// @Produce json
// @Param Authorization header string true "用户令牌"
// @Param bindReq body request.BindRoleApiReq true "角色与 API 绑定请求参数"
// @Success 200 {object} models.Result[any] "绑定成功"
// @Failure 500 {object} models.Result[any] "参数绑定失败或绑定操作失败"
// @Router /api/v1/sys/role_api/bind [post]
func (sysRoleApiController) Bind(ctx *gin.Context) models.Result[any] {
	var req request.BindRoleApiReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		return models.Error(500, err.Error())
	}
	err := service.SysRoleApi.BindRoleApi(req)
	if err != nil {
		return models.Error(500, err.Error())
	}
	return models.Success[any](nil)
}

// List 获取角色的API列表
// @Summary 获取角色的API列表
// @Description 根据请求参数获取指定角色关联的 API 列表
// @Tags 角色API绑定
// @Accept json
// @Produce json
// @Param Authorization header string true "用户令牌"
// @Param listReq body request.ListRoleApiReq true "获取角色 API 列表请求参数"
// @Success 200 {object} models.Result[any] "获取成功，返回角色关联的 API 列表"
// @Failure 500 {object} models.Result[any] "参数绑定失败或获取 API 列表失败"
// @Router /api/v1/sys/role_api/list [post]
func (sysRoleApiController) List(ctx *gin.Context) models.Result[any] {
	var req request.ListRoleApiReq
	if err := ctx.ShouldBindQuery(&req); err != nil {
		return models.Error(500, "参数绑定错误"+err.Error())
	}
	list, err := service.SysRoleApi.ListRoleApi(req)
	if err != nil {
		return models.Error(500, err.Error())
	}
	return models.Success[any](list)
}
