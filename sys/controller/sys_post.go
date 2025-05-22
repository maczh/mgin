package controller

import (
	"github.com/gin-gonic/gin"
	"github.com/maczh/mgin/models"
	"github.com/maczh/mgin/models/sys"
	"github.com/maczh/mgin/models/sys/request"
	"github.com/maczh/mgin/sys/service"
)

type sysPostController struct{}

// Add 新增岗位
// @Summary 新增岗位
// @Description 新增一个岗位信息
// @Tags 岗位管理
// @Accept json
// @Produce json
// @Param Authorization header string true "用户令牌"
// @Param data body request.CreatePostReq true "岗位信息，包含岗位名称、编码等"
// @Success 200 {object} models.Result[sys.SysPost] "成功新增岗位，返回新增的岗位信息"
// @Failure 500 {object} models.Result[any] "参数绑定失败或新增岗位失败"
// @Router /api/v1/sys/post/add [post]
func (s *sysPostController) Add(c *gin.Context) models.Result[any] {
	var req request.CreatePostReq
	if err := c.ShouldBindJSON(&req); err != nil {
		return models.Error(500, err.Error())
	}
	post, err := service.SysPost.WithContext(c).Add(req)
	if err != nil {
		return models.Error(500, err.Error())
	}
	return models.Success[any](post)
}

// Get 获取岗位信息
// @Summary 获取岗位信息
// @Description 根据请求参数获取指定岗位的详细信息
// @Tags 岗位管理
// @Accept json
// @Produce json
// @Param Authorization header string true "用户令牌"
// @Param data query request.GetPostReq true "岗位信息，包含岗位 ID 等查询条件"
// @Success 200 {object} models.Result[sys.SysPost] "成功获取岗位信息，返回该岗位详细信息"
// @Failure 500 {object} models.Result[any] "参数绑定失败或获取岗位信息失败"
// @Router /api/v1/sys/post/get [get]
func (s *sysPostController) Get(c *gin.Context) models.Result[any] {
	var req request.GetPostReq
	if err := c.ShouldBindQuery(&req); err != nil {
		return models.Error(500, err.Error())
	}
	post, err := service.SysPost.Get(req)
	if err != nil {
		return models.Error(500, err.Error())
	}
	return models.Success[any](post)
}

// Update 更新岗位信息
// @Summary 更新岗位信息
// @Description 更新指定岗位的相关信息
// @Tags 岗位管理
// @Accept json
// @Produce json
// @Param Authorization header string true "用户令牌"
// @Param data body sys.SysPost true "更新后的岗位信息"
// @Success 200 {object} models.Result[any] "成功更新岗位信息"
// @Failure 500 {object} models.Result[any] "参数绑定失败或更新岗位信息失败"
// @Router /api/v1/sys/post/update [post]
func (s *sysPostController) Update(c *gin.Context) models.Result[any] {
	var req *sys.SysPost
	if err := c.ShouldBindJSON(&req); err != nil {
		return models.Error(500, err.Error())
	}
	err := service.SysPost.WithContext(c).Update(req)
	if err != nil {
		return models.Error(500, err.Error())
	}
	return models.Success[any](nil)
}

// Delete 删除岗位
// @Summary 删除岗位
// @Description 根据岗位 ID 软删除指定岗位
// @Tags 岗位管理
// @Accept json
// @Produce json
// @Param Authorization header string true "用户令牌"
// @Param data query request.DeleteByIdReq true "岗位信息，包含要删除的岗位 ID"
// @Success 200 {object} models.Result[any] "成功删除岗位"
// @Failure 500 {object} models.Result[any] "参数绑定失败或删除岗位失败"
// @Router /api/v1/sys/post/del [post]
func (s *sysPostController) Delete(c *gin.Context) models.Result[any] {
	var req request.DeleteByIdReq
	if err := c.ShouldBindQuery(&req); err != nil {
		return models.Error(500, err.Error())
	}
	err := service.SysPost.Delete(uint(req.Id))
	if err != nil {
		return models.Error(500, err.Error())
	}
	return models.Success[any](nil)
}

// List 岗位列表
// @Summary 岗位列表
// @Description 根据请求参数获取岗位的分页列表
// @Tags 岗位管理
// @Accept json
// @Produce json
// @Param Authorization header string true "用户令牌"
// @Param data query request.ListPostReq true "岗位列表查询条件，包含分页信息等"
// @Success 200 {object} models.ResultPage[sys.SysPost] "成功获取岗位列表，返回岗位列表及分页信息"
// @Failure 500 {object} models.Result[any] "参数绑定失败或获取岗位列表失败"
// @Router /api/v1/sys/post/list [get]
func (s *sysPostController) List(c *gin.Context) models.Result[any] {
	var req request.ListPostReq
	if err := c.ShouldBindQuery(&req); err != nil {
		return models.Error(500, err.Error())
	}
	posts, page, err := service.SysPost.List(req)
	if err != nil {
		return models.Error(500, err.Error())
	}
	return models.SuccessPage[any](posts, page)
}
