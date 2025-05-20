package controller

import (
	"github.com/gin-gonic/gin"
	"github.com/maczh/mgin/models"
	"github.com/maczh/mgin/models/sys"
	"github.com/maczh/mgin/models/sys/request"
	"github.com/maczh/mgin/sys/service"
	"time"
)

type sysDeptController struct{}

// Add 新增部门
func (s *sysDeptController) Add(c *gin.Context) models.Result[any] {
	var req request.CreateDeptReq
	if err := c.ShouldBindJSON(&req); err != nil {
		return models.Error(500, "参数绑定失败:"+err.Error())
	}
	dept, err := service.SysDept.Add(req)
	if err != nil {
		return models.Error(500, err.Error())
	}
	return models.Success[any](dept)
}

// Get 获取部门信息
func (s *sysDeptController) Get(c *gin.Context) models.Result[any] {
	var req request.GetDeptReq
	if err := c.ShouldBindQuery(&req); err != nil {
		return models.Error(500, "参数绑定失败:"+err.Error())
	}
	dept, err := service.SysDept.Get(req)
	if err != nil {
		return models.Error(500, err.Error())
	}
	return models.Success[any](dept)
}

// Update 更新部门信息
func (s *sysDeptController) Update(c *gin.Context) models.Result[any] {
	var req *sys.SysDept
	if err := c.ShouldBindJSON(&req); err != nil {
		return models.Error(500, "参数绑定失败:"+err.Error())
	}
	req.UpdateAt = time.Now()
	err := service.SysDept.Update(req)
	if err != nil {
		return models.Error(500, err.Error())
	}
	return models.Success[any](nil)
}

// Delete 删除部门
func (s *sysDeptController) Delete(c *gin.Context) models.Result[any] {
	var req request.DeleteByIdReq
	if err := c.ShouldBindQuery(&req); err != nil {
		return models.Error(500, "参数绑定失败:"+err.Error())
	}
	if err := service.SysDept.Delete(uint(req.Id)); err != nil {
		return models.Error(500, err.Error())
	}
	return models.Success[any](nil)
}

// List 列表部门
func (s *sysDeptController) List(c *gin.Context) models.Result[any] {
	var req request.ListDeptReq
	if err := c.ShouldBindQuery(&req); err != nil {
		return models.Error(500, "参数绑定失败:"+err.Error())
	}
	depts, total, err := service.SysDept.List(req)
	if err != nil {
		return models.Error(500, err.Error())
	}
	return models.Success[any](map[string]any{"list": depts, "total": total})
}

// GetTree 获取部门树
func (s *sysDeptController) GetTree(c *gin.Context) models.Result[any] {
	var req request.DeleteByIdReq
	if err := c.ShouldBindQuery(&req); err != nil {
		return models.Error(500, "参数绑定失败:"+err.Error())
	}
	depts, err := service.SysDept.GetTree(req.Id)
	if err != nil {
		return models.Error(500, err.Error())
	}
	return models.Success[any](depts)
}
