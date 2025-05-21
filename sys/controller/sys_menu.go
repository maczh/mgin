package controller

import (
	"github.com/gin-gonic/gin"
	"github.com/maczh/mgin/models"
	"github.com/maczh/mgin/models/sys"
	"github.com/maczh/mgin/models/sys/request"
	"github.com/maczh/mgin/sys/service"
)

type sysMenuController struct{}

// Add 新增菜单
// @Summary 新增菜单
// @Description 新增一个系统菜单信息
// @Tags 系统菜单
// @Accept json
// @Produce json
// @Param menu body request.CreateMenuReq true "新增菜单请求体"
// @Success 200 {object} models.Result[sys.SysMenu] "新增菜单成功，返回新增的菜单信息"
// @Failure 500 {object} models.Result[any] "参数绑定失败或新增菜单失败"
// @Router /sys/menu/add [post]
func (s *sysMenuController) Add(c *gin.Context) models.Result[any] {
	var req request.CreateMenuReq
	if err := c.ShouldBindJSON(&req); err != nil {
		return models.Error(500, "参数绑定失败:"+err.Error())
	}
	menu, err := service.SysMenu.Add(req)
	if err != nil {
		return models.Error(500, err.Error())
	}
	return models.Success[any](menu)
}

// Get 获取菜单信息
// @Summary 获取菜单信息
// @Description 根据请求参数获取单个菜单的详细信息
// @Tags 系统菜单
// @Accept json
// @Produce json
// @Param menu query request.GetMenuReq true "获取菜单信息的请求参数"
// @Success 200 {object} models.Result[sys.SysMenu] "获取菜单信息成功，返回菜单详细信息"
// @Failure 500 {object} models.Result[any] "参数绑定失败或获取菜单信息失败"
// @Router /sys/menu/get [get]
func (s *sysMenuController) Get(c *gin.Context) models.Result[any] {
	var req request.GetMenuReq
	if err := c.ShouldBindQuery(&req); err != nil {
		return models.Error(500, "参数绑定失败:"+err.Error())
	}
	menu, err := service.SysMenu.Get(req)
	if err != nil {
		return models.Error(500, err.Error())
	}
	return models.Success[any](menu)
}

// Update 更新菜单信息
// @Summary 更新菜单信息
// @Description 更新已存在的菜单信息
// @Tags 系统菜单
// @Accept json
// @Produce json
// @Param menu body sys.SysMenu true "更新后的菜单信息"
// @Success 200 {object} models.Result[any] "更新菜单信息成功"
// @Failure 500 {object} models.Result[any] "参数绑定失败或更新菜单信息失败"
// @Router /sys/menu/update [put]
func (s *sysMenuController) Update(c *gin.Context) models.Result[any] {
	var req *sys.SysMenu
	if err := c.ShouldBindJSON(&req); err != nil {
		return models.Error(500, "参数绑定失败:"+err.Error())
	}
	err := service.SysMenu.Update(req)
	if err != nil {
		return models.Error(500, err.Error())
	}
	return models.Success[any](nil)
}

// Delete 删除菜单
// @Summary 删除菜单
// @Description 根据菜单 ID 删除对应的菜单信息
// @Tags 系统菜单
// @Accept json
// @Produce json
// @Param menu query request.DeleteByIdReq true "删除菜单的请求参数，包含菜单 ID"
// @Success 200 {object} models.Result[any] "删除菜单成功"
// @Failure 500 {object} models.Result[any] "参数绑定失败或删除菜单失败"
// @Router /sys/menu/delete [delete]
func (s *sysMenuController) Delete(c *gin.Context) models.Result[any] {
	var req request.DeleteByIdReq
	if err := c.ShouldBindQuery(&req); err != nil {
		return models.Error(500, "参数绑定失败:"+err.Error())
	}
	err := service.SysMenu.Delete(int64(uint(req.Id)))
	if err != nil {
		return models.Error(500, err.Error())
	}
	return models.Success[any](nil)
}

// List 获取菜单列表
// @Summary 获取菜单列表
// @Description 根据请求参数获取菜单列表，支持分页
// @Tags 系统菜单
// @Accept json
// @Produce json
// @Param menu query request.ListMenuReq true "获取菜单列表的请求参数"
// @Success 200 {object} models.ResultPage[any] "获取菜单列表成功，返回菜单列表和分页信息"
// @Failure 500 {object} models.Result[any] "参数绑定失败或获取菜单列表失败"
// @Router /sys/menu/list [get]
func (s *sysMenuController) List(c *gin.Context) models.Result[any] {
	var req request.ListMenuReq
	if err := c.ShouldBindQuery(&req); err != nil {
		return models.Error(500, "参数绑定失败:"+err.Error())
	}
	menus, page, err := service.SysMenu.List(req)
	if err != nil {
		return models.Error(500, err.Error())
	}
	return models.SuccessPage[any](menus, page)
}

// GetTree 获取菜单树
// @Summary 获取菜单树
// @Description 根据父菜单 ID 获取菜单的树形结构信息
// @Tags 系统菜单
// @Accept json
// @Produce json
// @Param menu query request.GetTreeMenuReq true "获取菜单树的请求参数，包含父菜单 ID"
// @Success 200 {object} models.Result[any] "获取菜单树成功，返回菜单树形结构信息"
// @Failure 500 {object} models.Result[any] "参数绑定失败或获取菜单树失败"
// @Router /sys/menu/tree [get]
func (s *sysMenuController) GetTree(c *gin.Context) models.Result[any] {
	var req request.GetTreeMenuReq
	if err := c.ShouldBindQuery(&req); err != nil {
		return models.Error(500, "参数绑定失败:"+err.Error())
	}
	menus, err := service.SysMenu.GetTree(req.ParentID)
	if err != nil {
		return models.Error(500, err.Error())
	}
	return models.Success[any](menus)
}
