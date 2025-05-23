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
// @Summary 新增部门
// @Description 创建一个新的部门信息
// @Tags 系统部门
// @Accept json
// @Produce json
// @Param Authorization header string true "用户令牌"
// @Param dept body request.CreateDeptReq true "部门创建请求参数"
// @Success 200 {object} models.Result[sys.SysDept] "新增成功，返回新增的部门信息"
// @Failure 500 {object} models.Result[any] "参数绑定失败或新增部门失败"
// @Router /api/v1/sys/dept/add [post]
func (s *sysDeptController) Add(c *gin.Context) models.Result[any] {
	var req request.CreateDeptReq
	if err := c.ShouldBindJSON(&req); err != nil {
		return models.Error(500, "参数绑定失败:"+err.Error())
	}
	dept, err := service.SysDept.WithContext(c).Add(req)
	if err != nil {
		return models.Error(500, err.Error())
	}
	return models.Success[any](dept)
}

// Get 获取部门信息
// @Summary 获取部门信息
// @Description 根据请求参数获取指定部门的信息
// @Tags 系统部门
// @Accept json
// @Produce json
// @Param Authorization header string true "用户令牌"
// @Param dept query request.GetDeptReq true "部门查询请求参数"
// @Success 200 {object} models.Result[sys.SysDept] "获取成功，返回部门信息"
// @Failure 500 {object} models.Result[any] "参数绑定失败或获取部门信息失败"
// @Router /api/v1/sys/dept/get [get]
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
// @Summary 更新部门信息
// @Description 更新指定部门的相关信息
// @Tags 系统部门
// @Accept json
// @Produce json
// @Param Authorization header string true "用户令牌"
// @Param dept body sys.SysDept true "更新后的部门信息"
// @Success 200 {object} models.Result[any] "更新成功"
// @Failure 500 {object} models.Result[any] "参数绑定失败或更新部门信息失败"
// @Router /api/v1/sys/dept/update [post]
func (s *sysDeptController) Update(c *gin.Context) models.Result[any] {
	var req *sys.SysDept
	if err := c.ShouldBindJSON(&req); err != nil {
		return models.Error(500, "参数绑定失败:"+err.Error())
	}
	req.UpdateAt = time.Now()
	err := service.SysDept.WithContext(c).Update(req)
	if err != nil {
		return models.Error(500, err.Error())
	}
	return models.Success[any](nil)
}

// Delete 删除部门
// @Summary 删除部门
// @Description 根据部门 ID 删除指定部门，软删除
// @Tags 系统部门
// @Accept json
// @Produce json
// @Param Authorization header string true "用户令牌"
// @Param dept body request.DeleteByIdReq true "部门删除请求参数，包含部门 ID"
// @Success 200 {object} models.Result[any] "删除成功"
// @Failure 500 {object} models.Result[any] "参数绑定失败或删除部门失败"
// @Router /api/v1/sys/dept/del [post]
func (s *sysDeptController) Delete(c *gin.Context) models.Result[any] {
	var req request.DeleteByIdReq
	if err := c.ShouldBindJSON(&req); err != nil {
		return models.Error(500, "参数绑定失败:"+err.Error())
	}
	if err := service.SysDept.Delete(uint(req.ID)); err != nil {
		return models.Error(500, err.Error())
	}
	return models.Success[any](nil)
}

// List 列表部门
// @Summary 获取部门列表
// @Description 根据请求参数获取部门的分页列表信息
// @Tags 系统部门
// @Accept json
// @Produce json
// @Param Authorization header string true "用户令牌"
// @Param dept query request.ListDeptReq true "部门列表查询请求参数"
// @Success 200 {object} models.Result[any] "获取成功，返回部门列表和总数"
// @Failure 500 {object} models.Result[any] "参数绑定失败或获取部门列表失败"
// @Router /api/v1/sys/dept/list [get]
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
// @Summary 获取部门树
// @Description 根据部门 ID 获取部门的树形结构信息
// @Tags 系统部门
// @Accept json
// @Produce json
// @Param Authorization header string true "用户令牌"
// @Param dept query request.GetByIdReq true "部门树查询请求参数，包含部门 ID"
// @Success 200 {object} models.Result[any] "获取成功，返回部门树信息"
// @Failure 500 {object} models.Result[any] "参数绑定失败或获取部门树失败"
// @Router /api/v1/sys/dept/tree [get]
func (s *sysDeptController) GetTree(c *gin.Context) models.Result[any] {
	var req request.GetByIdReq
	if err := c.ShouldBindQuery(&req); err != nil {
		return models.Error(500, "参数绑定失败:"+err.Error())
	}
	depts, err := service.SysDept.GetTree(*req.ID)
	if err != nil {
		return models.Error(500, err.Error())
	}
	return models.Success[any](depts)
}
