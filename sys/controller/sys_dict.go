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
func (s *sysDictController) Add(c *gin.Context) models.Result[any] {
	var req request.CreateDictReq
	if err := c.ShouldBindJSON(&req); err != nil {
		return models.Error(500, "参数绑定失败")
	}
	dict, err := service.SysDict.Add(req)
	if err != nil {
		return models.Error(500, "新增字典失败: "+err.Error())
	}
	return models.Success[any](dict)
}

// Update 更新字典
func (s *sysDictController) Update(c *gin.Context) models.Result[any] {
	var req sys.SysDict
	if err := c.ShouldBindJSON(&req); err != nil {
		return models.Error(500, "参数绑定失败")
	}
	if err := service.SysDict.Update(&req); err != nil {
		return models.Error(500, "更新字典失败: "+err.Error())
	}
	return models.Success[any](nil)
}

// Delete 删除字典
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
func (s *sysDictController) Get(c *gin.Context) models.Result[any] {
	var req request.GetDictReq
	if err := c.ShouldBindJSON(&req); err != nil {
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
