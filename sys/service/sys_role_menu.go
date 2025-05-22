package service

import (
	"github.com/maczh/mgin/logs"
	"github.com/maczh/mgin/models/sys"
	"github.com/maczh/mgin/models/sys/request"
	"github.com/maczh/mgin/sys/dao"
)

type sysRoleMenuService struct{}

// BindRoleMenu 绑定角色和菜单,每次都是全量绑定，先删除再新增
func (s *sysRoleMenuService) BindRoleMenu(req request.BindRoleMenuReq) error {
	//删除角色的所有菜单
	err := dao.SysRoleMenuDao.Delete(sys.SysRoleMenu{RoleId: uint(req.RoleId)})
	if err != nil {
		logs.Error("删除角色{}的所有菜单时发生错误：{}", req.RoleId, err.Error())
		return err
	}
	for _, apiId := range req.MenuIds {
		err := dao.SysRoleMenuDao.Save(&sys.SysRoleMenu{RoleId: uint(req.RoleId), MenuId: uint(apiId)})
		if err != nil {
			logs.Error("绑定角色{}和菜单 {}时发生错误：{}", req.RoleId, apiId, err.Error())
		}
	}
	return nil
}

// ListRoleMenu 获取角色的菜单列表
func (s *sysRoleMenuService) ListRoleMenu(req request.ListRoleMenuReq) ([]sys.SysRoleMenu, error) {
	list, err := dao.SysRoleMenuDao.All(sys.SysRoleMenu{RoleId: uint(req.RoleId)})
	if err != nil {
		logs.Error("获取角色{}的菜单列表时发生错误：{}", req.RoleId, err.Error())
		return nil, err
	}
	role, err := dao.SysRoleDao.One(sys.SysRole{ID: uint(req.RoleId)})
	if err != nil {
		logs.Error("获取角色{}的信息时发生错误：{}", req.RoleId, err.Error())
	}
	for i, api := range list {
		list[i].Menu, err = dao.SysMenuDao.One(sys.SysMenu{ID: api.MenuId})
		if err != nil {
			logs.Error("获取菜单{}的信息时发生错误：{}", api.MenuId, err.Error())
		}
		list[i].Role = role
	}
	return list, nil
}
