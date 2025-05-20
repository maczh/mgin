package service

import (
	"errors"
	"github.com/maczh/mgin/db"
	"github.com/maczh/mgin/logs"
	"github.com/maczh/mgin/models"
	"github.com/maczh/mgin/models/sys"
	"github.com/maczh/mgin/models/sys/request"
	"github.com/maczh/mgin/sys/dao"
	"time"
)

type sysRoleService struct{}

// Add 新增角色
func (s *sysRoleService) Add(req request.CreateRoleReq) (*sys.SysRole, error) {
	if dao.SysRoleDao.Exists(sys.SysRole{RoleName: req.Name}) {
		return nil, errors.New("角色名称已存在")
	}
	if dao.SysRoleDao.Exists(sys.SysRole{RoleIdent: req.Ident}) {
		return nil, errors.New("角色标识符已存在")
	}
	role := &sys.SysRole{
		RoleName:    req.Name,
		RoleIdent:   req.Ident,
		IsEnable:    true,
		Description: req.Description,
	}
	role.CreateAt = time.Now()
	err := dao.SysRoleDao.Create(role)
	if err != nil {
		return nil, err
	}
	return role, nil
}

// Get 获取角色
func (s *sysRoleService) Get(req request.GetRoleReq) (*sys.SysRole, error) {
	if req.Id > 0 {
		return dao.SysRoleDao.One(sys.SysRole{ID: req.Id})
	} else if req.Ident != "" {
		return dao.SysRoleDao.One(sys.SysRole{RoleIdent: req.Ident})
	} else if req.Name != "" {
		return dao.SysRoleDao.One(sys.SysRole{RoleName: req.Name})
	}
	return nil, errors.New("参数不能为空")
}

// Update 更新角色
func (s *sysRoleService) Update(req *sys.SysRole) error {
	if req.ID <= 0 {
		return errors.New("角色ID参数不能为空")
	}
	role, err := s.Get(request.GetRoleReq{Id: req.ID})
	if err != nil {
		return err
	}
	if role == nil {
		return errors.New("角色不存在")
	}
	role.RoleName = req.RoleName
	role.RoleIdent = req.RoleIdent
	role.IsEnable = req.IsEnable
	role.Description = req.Description
	err = dao.SysRoleDao.Updates(role)
	if err != nil {
		return err
	}
	return nil
}

// Delete 删除角色
func (s *sysRoleService) Delete(id uint) error {
	role, err := s.Get(request.GetRoleReq{Id: id})
	if err != nil {
		return err
	}
	if role == nil {
		return errors.New("角色不存在")
	}
	err = dao.SysRoleDao.Delete(*role)
	if err != nil {
		return err
	}
	return nil
}

// List 获取角色列表
func (s *sysRoleService) List(req request.ListRoleReq) ([]sys.SysRole, *models.ResultPage, error) {
	mysql, err := db.Mysql.GetConnection()
	if err != nil {
		logs.Error("获取 MySQL 连接失败: {}", err.Error())
		return nil, nil, err
	}
	return dao.SysRoleDao.Pager(mysql, req.Page, req.PageSize)
}
