package controller

import (
	"github.com/gin-gonic/gin"
	"github.com/maczh/mgin/models"
	"github.com/maczh/mgin/models/sys"
	"github.com/maczh/mgin/models/sys/request"
	"github.com/maczh/mgin/sys/service"
)

type sysResourceController struct {
	ctx *gin.Context
}

func (s *sysResourceController) WithContext(c *gin.Context) *sysResourceController {
	s.ctx = c
	return s
}

// Add 新增资源
// @Summary 新增资源
// @Description 新增一个系统资源信息
// @Tags 系统资源
// @Accept json
// @Produce json
// @Param Authorization header string true "用户令牌"
// @Param menu body request.CreateResourceReq true "新增资源请求体"
// @Success 200 {object} models.Result[sys.SysResource] "新增资源成功，返回新增的资源信息"
// @Failure 500 {object} models.Result[any] "参数绑定失败或新增资源失败"
// @Router /api/v1/sys/res/add [post]
func (s *sysResourceController) Add(c *gin.Context) models.Result[any] {
	var req request.CreateResourceReq
	if err := c.ShouldBindJSON(&req); err != nil {
		return models.Error(500, "参数绑定失败:"+err.Error())
	}
	menu, err := service.SysResource.WithContext(c).Add(req)
	if err != nil {
		return models.Error(500, err.Error())
	}
	return models.Success[any](menu)
}

// Get 获取资源信息
// @Summary 获取资源信息
// @Description 根据请求参数获取单个资源的详细信息
// @Tags 系统资源
// @Accept json
// @Produce json
// @Param Authorization header string true "用户令牌"
// @Param menu query request.GetResourceReq true "获取资源信息的请求参数"
// @Success 200 {object} models.Result[sys.SysResource] "获取资源信息成功，返回资源详细信息"
// @Failure 500 {object} models.Result[any] "参数绑定失败或获取资源信息失败"
// @Router /api/v1/sys/res/get [get]
func (s *sysResourceController) Get(c *gin.Context) models.Result[any] {
	var req request.GetResourceReq
	if err := c.ShouldBindQuery(&req); err != nil {
		return models.Error(500, "参数绑定失败:"+err.Error())
	}
	menu, err := service.SysResource.Get(req)
	if err != nil {
		return models.Error(500, err.Error())
	}
	return models.Success[any](menu)
}

// Update 更新资源信息
// @Summary 更新资源信息
// @Description 更新已存在的资源信息
// @Tags 系统资源
// @Accept json
// @Produce json
// @Param Authorization header string true "用户令牌"
// @Param menu body sys.SysResource true "更新后的资源信息"
// @Success 200 {object} models.Result[any] "更新资源信息成功"
// @Failure 500 {object} models.Result[any] "参数绑定失败或更新资源信息失败"
// @Router /api/v1/sys/res/update [post]
func (s *sysResourceController) Update(c *gin.Context) models.Result[any] {
	var req *sys.SysResource
	if err := c.ShouldBindJSON(&req); err != nil {
		return models.Error(500, "参数绑定失败:"+err.Error())
	}
	err := service.SysResource.WithContext(c).Update(req)
	if err != nil {
		return models.Error(500, err.Error())
	}
	return models.Success[any](nil)
}

// Delete 删除资源
// @Summary 删除资源
// @Description 根据资源 ID 删除对应的资源信息
// @Tags 系统资源
// @Accept json
// @Produce json
// @Param Authorization header string true "用户令牌"
// @Param menu query request.DeleteByIdReq true "删除资源的请求参数，包含资源 ID"
// @Success 200 {object} models.Result[any] "删除资源成功"
// @Failure 500 {object} models.Result[any] "参数绑定失败或删除资源失败"
// @Router /api/v1/sys/res/del [post]
func (s *sysResourceController) Delete(c *gin.Context) models.Result[any] {
	var req request.DeleteByIdReq
	if err := c.ShouldBindQuery(&req); err != nil {
		return models.Error(500, "参数绑定失败:"+err.Error())
	}
	err := service.SysResource.Delete(int64(uint(req.ID)))
	if err != nil {
		return models.Error(500, err.Error())
	}
	return models.Success[any](nil)
}

// List 获取资源列表
// @Summary 获取资源列表
// @Description 根据请求参数获取资源列表，支持分页
// @Tags 系统资源
// @Accept json
// @Produce json
// @Param Authorization header string true "用户令牌"
// @Param menu query request.ListResourceReq true "获取资源列表的请求参数"
// @Success 200 {object} models.ResultPage[any] "获取资源列表成功，返回资源列表和分页信息"
// @Failure 500 {object} models.Result[any] "参数绑定失败或获取资源列表失败"
// @Router /api/v1/sys/res/list [get]
func (s *sysResourceController) List(c *gin.Context) models.Result[any] {
	var req request.ListResourceReq
	if err := c.ShouldBindQuery(&req); err != nil {
		return models.Error(500, "参数绑定失败:"+err.Error())
	}
	menus, page, err := service.SysResource.List(req)
	if err != nil {
		return models.Error(500, err.Error())
	}
	return models.SuccessPage[any](menus, page)
}

// GetTree 获取资源树
// @Summary 获取资源树
// @Description 根据父资源 ID 获取资源的树形结构信息
// @Tags 系统资源
// @Accept json
// @Produce json
// @Param Authorization header string true "用户令牌"
// @Param menu query request.GetTreeResourceReq true "获取资源树的请求参数，包含父资源 ID"
// @Success 200 {object} models.Result[any] "获取资源树成功，返回资源树形结构信息"
// @Failure 500 {object} models.Result[any] "参数绑定失败或获取资源树失败"
// @Router /api/v1/sys/res/tree [get]
func (s *sysResourceController) GetTree(c *gin.Context) models.Result[any] {
	var req request.GetTreeResourceReq
	if err := c.ShouldBindQuery(&req); err != nil {
		return models.Error(500, "参数绑定失败:"+err.Error())
	}
	menus, err := service.SysResource.WithContext(c).GetTree(req)
	if err != nil {
		return models.Error(500, err.Error())
	}
	return models.Success[any](menus)
}
