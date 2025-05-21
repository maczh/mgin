package service

import (
	"errors"
	"github.com/maczh/mgin/logs"
	"github.com/maczh/mgin/models"
	"github.com/maczh/mgin/models/sys"
	"github.com/maczh/mgin/models/sys/request"
	"github.com/maczh/mgin/sys/dao"
	"time"
)

type sysMenuService struct{}

// Add 新增菜单
func (s *sysMenuService) Add(req request.CreateMenuReq) (*sys.SysMenu, error) {
	//检查菜单名称是否存在
	if dao.SysMenuDao.Exists(sys.SysMenu{Name: req.Name}) {
		return nil, errors.New("菜单名称已存在")
	}
	//检查菜单路径是否存在
	if dao.SysMenuDao.Exists(sys.SysMenu{Path: req.Path}) {
		return nil, errors.New("菜单编码已存在")
	}
	menu := &sys.SysMenu{
		ParentID:   req.ParentID,
		Name:       req.Name,
		Path:       req.Path,
		Component:  req.Component,
		Redirect:   req.Redirect,
		Icon:       req.Icon,
		Title:      req.Title,
		Hidden:     req.Hidden,
		AlwaysShow: req.AlwaysShow,
		ActiveMenu: req.ActiveMenu,
		KeepAlive:  req.KeepAlive,
		Breadcrumb: req.Breadcrumb,
		Affix:      req.Affix,
		Sort:       req.Sort,
		Status:     1,
	}
	menu.CreateAt = time.Now()
	menu.UpdateAt = time.Now()
	err := dao.SysMenuDao.Create(menu)
	return menu, err
}

// Get 获取菜单信息
func (s *sysMenuService) Get(req request.GetMenuReq) (*sys.SysMenu, error) {
	var menu *sys.SysMenu
	var err error
	if req.Id > 0 {
		menu, err = dao.SysMenuDao.One(sys.SysMenu{ID: req.Id})
	} else if req.Path != "" {
		menu, err = dao.SysMenuDao.One(sys.SysMenu{Name: req.Path})
	}
	return menu, err
}

// Update 更新菜单信息
func (s *sysMenuService) Update(req *sys.SysMenu) error {
	if req.ID == 0 {
		return errors.New("菜单ID不能为空")
	}
	menu, err := s.Get(request.GetMenuReq{Id: req.ID})
	if err != nil {
		logs.Error("获取菜单信息失败: {}", err.Error())
		return err
	}
	//检查菜单名称是否存在
	if menu.Name != req.Name && dao.SysMenuDao.Exists(sys.SysMenu{Name: req.Name}) {
		return errors.New("菜单名称已存在")
	}
	//检查菜单路径是否存在
	if menu.Path != req.Path && dao.SysMenuDao.Exists(sys.SysMenu{Path: req.Path}) {
		return errors.New("菜单编码已存在")
	}
	req.UpdateAt = time.Now()
	err = dao.SysMenuDao.Save(req)
	return err
}

// Delete 删除菜单
func (s *sysMenuService) Delete(id int64) error {
	return dao.SysMenuDao.Delete(sys.SysMenu{ID: uint(id)})
}

// List 获取菜单列表
func (s *sysMenuService) List(req request.ListMenuReq) ([]sys.SysMenu, *models.ResultPage, error) {
	var mysql = dao.SysMenuDao.Where("del_flag = 0")
	if req.ParentID > 0 {
		mysql = mysql.Where("parent_id = ?", req.ParentID)
	}
	if req.Status > 0 {
		mysql = mysql.Where("status =?", req.Status)
	}
	return dao.SysMenuDao.Pager(mysql, req.Page, req.PageSize)
}

// GetTree 获取菜单树
func (s *sysMenuService) GetTree(parentId uint) ([]sys.SysMenu, error) {
	// 查询所有未删除的菜单
	allMenus, _, err := s.List(request.ListMenuReq{ParentID: uint(int(parentId)), Status: 1, Page: 1, PageSize: 10000})
	if err != nil {
		logs.Error("获取菜单列表失败: %v", err)
		return nil, err
	}

	// 构建菜单树
	return buildMenuTree(allMenus, parentId), nil
}

// buildMenuTree 递归构建菜单树
func buildMenuTree(menus []sys.SysMenu, parentId uint) []sys.SysMenu {
	var menuTree []sys.SysMenu
	for _, menu := range menus {
		if menu.ParentID == parentId {
			// 递归查找子菜单
			menu.Children = buildMenuTree(menus, menu.ID)
			menuTree = append(menuTree, menu)
		}
	}
	return menuTree
}
