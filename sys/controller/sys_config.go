package controller

import (
	"github.com/gin-gonic/gin"
	"github.com/maczh/mgin/models"
	"github.com/maczh/mgin/models/sys"
	"github.com/maczh/mgin/models/sys/request"
	"github.com/maczh/mgin/sys/service"
	"strings"
)

type sysConfigController struct{}

// Add 新增系统配置
// @Summary 新增系统配置
// @Description 新增一个系统配置信息
// @Tags 系统配置
// @Accept json
// @Produce json
// @Param Authorization header string true "用户令牌"
// @Param config body sys.SysConfig true "新增系统配置请求参数"
// @Success 200 {object} models.Result[sys.SysConfig] "新增成功，返回新增的系统配置信息"
// @Failure 500 {object} models.Result[any] "参数绑定失败或新增系统配置失败"
// @Router /api/v1/sys/config/add [post]
func (s *sysConfigController) Add(c *gin.Context) models.Result[any] {
	var req sys.SysConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		return models.Error(500, "参数绑定失败")
	}
	config, err := service.SysConfig.Create(req)
	if err != nil {
		return models.Error(500, "新增系统配置失败: "+err.Error())
	}
	return models.Success[any](config)
}

// Update 更新系统配置
// @Summary 更新系统配置
// @Description 更新系统配置信息
// @Tags 系统配置
// @Accept json
// @Produce json
// @Param Authorization header string true "用户令牌"
// @Param config body sys.SysConfig true "更新后的系统配置信息"
// @Success 200 {object} models.Result[any] "更新成功"
// @Failure 500 {object} models.Result[any] "参数绑定失败或更新系统配置失败"
// @Router /api/v1/sys/config/update [post]
func (s *sysConfigController) Update(c *gin.Context) models.Result[any] {
	var req sys.SysConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		return models.Error(500, "参数绑定失败")
	}
	err := service.SysConfig.Update(req)
	if err != nil {
		return models.Error(500, "更新系统配置失败: "+err.Error())
	}
	return models.Success[any](nil)
}

// Delete 删除系统配置
// @Summary 删除系统配置
// @Description 根据 ID 删除系统配置信息
// @Tags 系统配置
// @Accept json
// @Produce json
// @Param Authorization header string true "用户令牌"
// @Param config body request.DeleteByIdReq true "删除系统配置请求参数，包含系统配置 ID"
// @Success 200 {object} models.Result[any] "删除成功"
// @Failure 500 {object} models.Result[any] "参数绑定失败或删除系统配置失败"
// @Router /api/v1/sys/config/del [post]
func (s *sysConfigController) Delete(c *gin.Context) models.Result[any] {
	var req request.DeleteByIdReq
	if err := c.ShouldBindJSON(&req); err != nil {
		return models.Error(500, "参数绑定失败")
	}
	if err := service.SysConfig.Delete(int(req.ID)); err != nil {
		return models.Error(500, "删除系统配置失败: "+err.Error())
	}
	return models.Success[any](nil)
}

// List 获取系统配置列表
// @Summary 获取系统配置列表
// @Description 根据查询条件获取系统配置列表
// @Tags 系统配置
// @Accept json
// @Produce json
// @Param Authorization header string true "用户令牌"
// @Param query query request.ListSysConfigReq true "获取系统配置列表请求参数"
// @Success 200 {object} models.ResultPage[any] "获取成功，返回系统配置列表和分页信息"
// @Failure 500 {object} models.Result[any] "参数绑定失败或获取系统配置列表失败"
// @Router /api/v1/sys/config/list [get]
func (s *sysConfigController) List(c *gin.Context) models.Result[any] {
	var req request.ListSysConfigReq
	if err := c.ShouldBindQuery(&req); err != nil {
		return models.Error(500, "参数绑定失败")
	}
	list, pages, err := service.SysConfig.List(req)
	if err != nil {
		return models.Error(500, "获取系统配置列表失败: "+err.Error())
	}
	return models.SuccessPage[any](list, pages)
}

// Get 获取单个系统配置
// @Summary 获取单个系统配置
// @Description 根据 ID 或其他组合条件获取单个系统配置信息
// @Tags 系统配置
// @Accept json
// @Produce json
// @Param Authorization header string true "用户令牌"
// @Param config query request.GetSysConfigReq true "获取单个系统配置请求参数"
// @Success 200 {object} models.Result[sys.SysConfig] "获取成功，返回单个系统配置信息"
// @Failure 500 {object} models.Result[any] "参数绑定失败、参数不完整或获取系统配置失败"
// @Failure 400 {object} models.Result[any] "参数不完整"
// @Router /api/v1/sys/config/get [get]
func (s *sysConfigController) Get(c *gin.Context) models.Result[any] {
	var req request.GetSysConfigReq
	if err := c.ShouldBindQuery(&req); err != nil {
		return models.Error(500, "参数绑定失败")
	}
	if req.ID == 0 && req.Key == "" {
		return models.Error(400, "ID或Key必须传一个")
	}
	config, err := service.SysConfig.Get(req)
	if err != nil {
		return models.Error(500, "获取系统配置失败: "+err.Error())
	}
	return models.Success[any](config)
}

// MultiGet 批量获取多个系统配置
// @Summary 批量获取多个系统配置
// @Description 根据多个key获取多个系统配置信息
// @Tags 系统配置
// @Accept json
// @Produce json
// @Param Authorization header string true "用户令牌"
// @Param keys query string true "多个键名，以逗号分隔"
// @Success 200 {object} models.Result[sys.SysConfig] "获取成功，返回单个系统配置信息"
// @Failure 500 {object} models.Result[any] "参数绑定失败、参数不完整或获取系统配置失败"
// @Failure 400 {object} models.Result[any] "参数不完整"
// @Router /api/v1/sys/config/get/multi [get]
func (s *sysConfigController) MultiGet(c *gin.Context) models.Result[any] {
	keyStr := c.Query("keys")
	if keyStr == "" {
		return models.Error(500, "keys不能为空")
	}
	keys := strings.Split(keyStr, ",")
	configs, err := service.SysConfig.MultiGet(keys)
	if err != nil {
		return models.Error(500, "批量获取系统配置失败: "+err.Error())
	}
	return models.Success[any](configs)
}
