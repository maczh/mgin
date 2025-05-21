package controller

import (
	"github.com/gin-gonic/gin"
	"github.com/maczh/mgin/models"
	"github.com/maczh/mgin/models/sys"
	"github.com/maczh/mgin/models/sys/request"
	"github.com/maczh/mgin/sys/service"
)

type sysApiController struct{}

// Create 创建API接口
// @Summary 创建API接口
// @Description 创建一个新的API接口
// @Tags 系统API
// @Accept json
// @Produce json
// @Param api body sys.SysApi true "API接口信息"
// @Success 200 {object} models.Result[sys.SysApi] "创建成功，返回创建的API接口信息"
// @Failure 500 {object} models.Result[any] "请求参数错误或创建API接口失败"
// @Router /sys/api/create [post]
func (s *sysApiController) Create(c *gin.Context) models.Result[any] {
	var api sys.SysApi
	if err := c.ShouldBindJSON(&api); err != nil {
		return models.Error(500, "请求参数错误: "+err.Error())
	}
	if err := service.SysApi.CreateApi(&api); err != nil {
		return models.Error(500, "创建API接口失败: "+err.Error())
	}
	return models.Success[any](api)
}

// Get 获取API接口
// @Summary 获取API接口
// @Description 根据ID获取API接口信息
// @Tags 系统API
// @Accept json
// @Produce json
// @Param id query request.GetApiReq true "API接口ID"
// @Success 200 {object} models.Result[sys.SysApi] "获取成功，返回API接口信息"
// @Failure 500 {object} models.Result[any] "请求参数错误或获取API接口失败"
// @Router /sys/api/get [get]
func (s *sysApiController) Get(c *gin.Context) models.Result[any] {
	var req request.GetApiReq
	if err := c.ShouldBindQuery(&req); err != nil {
		return models.Error(500, "请求参数错误: "+err.Error())
	}
	api, err := service.SysApi.GetApi(req.ID)
	if err != nil {
		return models.Error(500, "获取API接口失败: "+err.Error())
	}
	return models.Success[any](api)
}

// GetUri 获取API接口
// @Summary 根据URI获取API接口
// @Description 根据URI获取对应的API接口信息
// @Tags 系统API
// @Accept json
// @Produce json
// @Param uri query request.GetUriReq true "API接口URI"
// @Success 200 {object} models.Result[sys.SysApi] "获取成功，返回API接口信息"
// @Failure 500 {object} models.Result[any] "请求参数错误或获取API接口失败"
// @Router /sys/api/get-uri [get]
func (s *sysApiController) GetUri(c *gin.Context) models.Result[any] {
	var req request.GetUriReq
	if err := c.ShouldBindQuery(&req); err != nil {
		return models.Error(500, "请求参数错误: "+err.Error())
	}
	api, err := service.SysApi.GetUri(req.Uri)
	if err != nil {
		return models.Error(500, "获取API接口失败: "+err.Error())
	}
	return models.Success[any](api)
}

// Update 更新API接口
// @Summary 更新API接口
// @Description 更新指定API接口的信息
// @Tags 系统API
// @Accept json
// @Produce json
// @Param api body sys.SysApi true "更新后的API接口信息"
// @Success 200 {object} models.Result[any] "更新成功"
// @Failure 500 {object} models.Result[any] "参数绑定失败或更新API接口失败"
// @Router /sys/api/update [put]
func (s *sysApiController) Update(c *gin.Context) models.Result[any] {
	var api sys.SysApi
	if err := c.ShouldBindJSON(&api); err != nil {
		return models.Error(500, "参数绑定失败")
	}
	if err := service.SysApi.UpdateApi(&api); err != nil {
		return models.Error(500, "更新API接口失败: "+err.Error())
	}
	return models.Success[any](nil)
}

// List 获取API接口列表
// @Summary 获取API接口列表
// @Description 获取API接口的分页列表
// @Tags 系统API
// @Accept json
// @Produce json
// @Param page query request.ListApiReq false "页码"
// @Param pageSize query request.ListApiReq false "每页数量"
// @Param group query request.ListApiReq false "API接口分组"
// @Param needAuth query request.ListApiReq false "是否需要认证"
// @Success 200 {object} models.ResultPage[any] "获取成功，返回API接口列表和分页信息"
// @Failure 500 {object} models.Result[any] "参数绑定失败或获取API接口列表失败"
// @Router /sys/api/list [get]
func (s *sysApiController) List(c *gin.Context) models.Result[any] {
	var req request.ListApiReq
	if err := c.ShouldBindQuery(&req); err != nil {
		return models.Error(500, "参数绑定失败")
	}
	apis, pages, err := service.SysApi.ListApi(req.Page, req.PageSize, req.Group, req.NeedAuth)
	if err != nil {
		return models.Error(500, "获取API接口列表失败: "+err.Error())
	}
	return models.SuccessPage[any](apis, pages)
}

// Delete 删除API接口
// @Summary 删除API接口
// @Description 根据ID软删除指定的API接口
// @Tags 系统API
// @Accept json
// @Produce json
// @Param id body request.DeleteByIdReq true "API接口ID"
// @Success 200 {object} models.Result[any] "删除成功"
// @Failure 500 {object} models.Result[any] "参数绑定失败或删除API接口失败"
// @Router /sys/api/delete [delete]
func (s *sysApiController) Delete(c *gin.Context) models.Result[any] {
	var req request.DeleteByIdReq
	if err := c.ShouldBindJSON(&req); err != nil {
		return models.Error(500, "参数绑定失败")
	}
	if err := service.SysApi.DeleteApi(uint(req.Id)); err != nil {
		return models.Error(500, "删除API接口失败: "+err.Error())
	}
	return models.Success[any](nil)
}
