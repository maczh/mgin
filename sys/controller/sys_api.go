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
