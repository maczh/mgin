package controller

import (
	"github.com/gin-gonic/gin"
	"github.com/maczh/mgin/models"
	"github.com/maczh/mgin/models/sys"
	"github.com/maczh/mgin/models/sys/request"
	"github.com/maczh/mgin/sys/service"
)

type sysDictController struct{}

// Add 新增字典
// @Summary 新增字典
// @Description 新增一个系统字典信息
// @Tags 系统字典
// @Accept json
// @Produce json
// @Param Authorization header string true "用户令牌"
// @Param dict body request.CreateDictReq true "新增字典请求参数"
// @Success 200 {object} models.Result[sys.SysDict] "新增成功，返回新增的字典信息"
// @Failure 500 {object} models.Result[any] "参数绑定失败或新增字典失败"
// @Router /api/v1/sys/dict/add [post]
func (s *sysDictController) Add(c *gin.Context) models.Result[any] {
	var req request.CreateDictReq
	if err := c.ShouldBindJSON(&req); err != nil {
		return models.Error(500, "参数绑定失败")
	}
	dict, err := service.SysDict.WithContext(c).Add(req)
	if err != nil {
		return models.Error(500, "新增字典失败: "+err.Error())
	}
	return models.Success[any](dict)
}

// Update 更新字典
// @Summary 更新字典
// @Description 更新系统字典信息
// @Tags 系统字典
// @Accept json
// @Produce json
// @Param Authorization header string true "用户令牌"
// @Param dict body sys.SysDict true "更新后的字典信息"
// @Success 200 {object} models.Result[any] "更新成功"
// @Failure 500 {object} models.Result[any] "参数绑定失败或更新字典失败"
// @Router /api/v1/sys/dict/update [post]
func (s *sysDictController) Update(c *gin.Context) models.Result[any] {
	var req sys.SysDict
	if err := c.ShouldBindJSON(&req); err != nil {
		return models.Error(500, "参数绑定失败")
	}
	if err := service.SysDict.WithContext(c).Update(&req); err != nil {
		return models.Error(500, "更新字典失败: "+err.Error())
	}
	return models.Success[any](nil)
}

// Delete 删除字典
// @Summary 删除字典
// @Description 根据 ID 删除系统字典信息
// @Tags 系统字典
// @Accept json
// @Produce json
// @Param Authorization header string true "用户令牌"
// @Param dict body request.DeleteByIdReq true "删除字典请求参数，包含字典 ID"
// @Success 200 {object} models.Result[any] "删除成功"
// @Failure 500 {object} models.Result[any] "参数绑定失败或删除字典失败"
// @Router /api/v1/sys/dict/del [post]
func (s *sysDictController) Delete(c *gin.Context) models.Result[any] {
	var req request.DeleteByIdReq
	if err := c.ShouldBindJSON(&req); err != nil {
		return models.Error(500, "参数绑定失败")
	}
	if err := service.SysDict.Delete(req.Id); err != nil {
		return models.Error(500, "删除字典失败: "+err.Error())
	}
	return models.Success[any](nil)
}

// List 获取字典列表
// @Summary 获取字典列表
// @Description 根据查询条件获取系统字典列表
// @Tags 系统字典
// @Accept json
// @Produce json
// @Param Authorization header string true "用户令牌"
// @Param query query request.ListDictReq true "获取字典列表请求参数"
// @Success 200 {object} models.ResultPage[any] "获取成功，返回字典列表和分页信息"
// @Failure 500 {object} models.Result[any] "参数绑定失败或获取字典列表失败"
// @Router /api/v1/sys/dict/list [get]
func (s *sysDictController) List(c *gin.Context) models.Result[any] {
	var req request.ListDictReq
	if err := c.ShouldBindQuery(&req); err != nil {
		return models.Error(500, "参数绑定失败")
	}
	list, pages, err := service.SysDict.List(req)
	if err != nil {
		return models.Error(500, "获取字典列表失败: "+err.Error())
	}
	return models.SuccessPage[any](list, pages)
}

// Get 获取单个字典
// @Summary 获取单个字典
// @Description 根据 ID 或其他组合条件获取单个系统字典信息
// @Tags 系统字典
// @Accept json
// @Produce json
// @Param Authorization header string true "用户令牌"
// @Param dict query request.GetDictReq true "获取单个字典请求参数"
// @Success 200 {object} models.Result[sys.SysDict] "获取成功，返回单个字典信息"
// @Failure 500 {object} models.Result[any] "参数绑定失败、参数不完整或获取字典失败"
// @Failure 400 {object} models.Result[any] "参数不完整"
// @Router /api/v1/sys/dict/get [get]
func (s *sysDictController) Get(c *gin.Context) models.Result[any] {
	var req request.GetDictReq
	if err := c.ShouldBindQuery(&req); err != nil {
		return models.Error(500, "参数绑定失败")
	}
	if !(req.ID > 0 || (req.Name != "" && req.Type != "" && req.Key != "")) {
		return models.Error(400, "参数不完整")
	}
	dict, err := service.SysDict.Get(req)
	if err != nil {
		return models.Error(500, "获取字典失败: "+err.Error())
	}
	return models.Success[any](dict)
}
